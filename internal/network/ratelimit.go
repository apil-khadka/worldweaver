package network

import (
	"sync"
	"sync/atomic"
	"time"
)

// ---------- Token bucket (per-client, in-memory) ----------

// tokenBucket implements a classic token-bucket rate limiter.
// It is goroutine-safe via atomic operations for the hot path.
type tokenBucket struct {
	tokens     atomic.Int64
	maxTokens  int64
	refillRate int64 // tokens added per refill interval
	refillNs   int64 // refill interval in nanoseconds
	lastRefill atomic.Int64
}

func newTokenBucket(maxTokens, refillRate int64, refillInterval time.Duration) *tokenBucket {
	tb := &tokenBucket{
		maxTokens:  maxTokens,
		refillRate: refillRate,
		refillNs:   refillInterval.Nanoseconds(),
	}
	tb.tokens.Store(maxTokens)
	tb.lastRefill.Store(time.Now().UnixNano())
	return tb
}

// allow attempts to consume one token. Returns true if allowed.
func (tb *tokenBucket) allow() bool {
	tb.refill()
	for {
		cur := tb.tokens.Load()
		if cur <= 0 {
			return false
		}
		if tb.tokens.CompareAndSwap(cur, cur-1) {
			return true
		}
	}
}

func (tb *tokenBucket) refill() {
	now := time.Now().UnixNano()
	last := tb.lastRefill.Load()
	elapsed := now - last
	if elapsed < tb.refillNs {
		return
	}
	intervals := elapsed / tb.refillNs
	add := intervals * tb.refillRate
	newTokens := tb.tokens.Load() + add
	if newTokens > tb.maxTokens {
		newTokens = tb.maxTokens
	}
	// CAS the lastRefill; if another goroutine won, that's fine — no double refill.
	if tb.lastRefill.CompareAndSwap(last, last+intervals*tb.refillNs) {
		tb.tokens.Store(newTokens)
	}
}

// ---------- Per-client rate limiter (message + power) ----------

// ClientRateLimiter enforces per-client message and power-application rates.
type ClientRateLimiter struct {
	msgBucket   *tokenBucket
	powerBucket *tokenBucket

	// cooldown tracks whether the client is in a 1-second penalty period.
	cooldownUntil atomic.Int64 // unix nanoseconds
}

// NewClientRateLimiter creates a limiter with:
//   - 30 messages/second general limit
//   - 10 power applications/second
func NewClientRateLimiter() *ClientRateLimiter {
	return &ClientRateLimiter{
		// 30 tokens, refill 30 every second
		msgBucket: newTokenBucket(30, 30, time.Second),
		// 10 tokens, refill 10 every second
		powerBucket: newTokenBucket(10, 10, time.Second),
	}
}

// AllowMessage checks the general message rate limit.
// Returns true if the message should be processed.
// If the limit is exceeded, the client enters a 1-second cooldown.
func (rl *ClientRateLimiter) AllowMessage() bool {
	// If in cooldown, reject immediately.
	if cd := rl.cooldownUntil.Load(); cd > 0 {
		if time.Now().UnixNano() < cd {
			return false
		}
		// Cooldown expired, clear it.
		rl.cooldownUntil.Store(0)
	}

	if !rl.msgBucket.allow() {
		// Enter cooldown for 1 second.
		rl.cooldownUntil.Store(time.Now().Add(time.Second).UnixNano())
		return false
	}
	return true
}

// AllowPower checks the power-application rate limit.
// Returns true if the power action should be processed.
func (rl *ClientRateLimiter) AllowPower() bool {
	return rl.powerBucket.allow()
}

// ---------- Connection rate limiter (per-IP sliding window) ----------

// ConnRateLimiter tracks connection attempts per IP address.
// It uses a sliding-window counter: max 20 connections per minute per IP.
type ConnRateLimiter struct {
	mu      sync.Mutex
	windows map[string]*connWindow
}

type connWindow struct {
	timestamps []time.Time
}

// NewConnRateLimiter creates a connection rate limiter.
func NewConnRateLimiter() *ConnRateLimiter {
	crl := &ConnRateLimiter{
		windows: make(map[string]*connWindow),
	}
	// Background cleanup every 2 minutes to prevent unbounded memory growth.
	go crl.cleanup()
	return crl
}

const (
	connRateLimit  = 20
	connRateWindow = time.Minute
)

// AllowConnection returns true if the IP has not exceeded 20 connections/minute.
func (crl *ConnRateLimiter) AllowConnection(ip string) bool {
	crl.mu.Lock()
	defer crl.mu.Unlock()

	now := time.Now()
	cutoff := now.Add(-connRateWindow)

	w, ok := crl.windows[ip]
	if !ok {
		w = &connWindow{}
		crl.windows[ip] = w
	}

	// Trim expired timestamps.
	valid := w.timestamps[:0]
	for _, t := range w.timestamps {
		if t.After(cutoff) {
			valid = append(valid, t)
		}
	}
	w.timestamps = valid

	if len(w.timestamps) >= connRateLimit {
		return false
	}

	w.timestamps = append(w.timestamps, now)
	return true
}

// cleanup periodically removes stale entries.
func (crl *ConnRateLimiter) cleanup() {
	ticker := time.NewTicker(2 * time.Minute)
	for range ticker.C {
		crl.mu.Lock()
		now := time.Now()
		cutoff := now.Add(-connRateWindow)
		for ip, w := range crl.windows {
			valid := w.timestamps[:0]
			for _, t := range w.timestamps {
				if t.After(cutoff) {
					valid = append(valid, t)
				}
			}
			if len(valid) == 0 {
				delete(crl.windows, ip)
			} else {
				w.timestamps = valid
			}
		}
		crl.mu.Unlock()
	}
}
