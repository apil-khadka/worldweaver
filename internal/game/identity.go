package game

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"sync"
	"time"
)

// Keypair identity.
//
// There are no accounts and no passwords. A client generates an Ed25519 keypair
// on first run, keeps the private key locally, and proves who it is by signing a
// server-issued challenge. The public key is the durable identity: it survives
// restarts, it is what world ownership is recorded against, and a display name is
// cosmetic metadata that can change at any time without affecting it.
//
// This replaces an earlier scheme where POST /api/login accepted any nickname and
// returned a token. That gave no assurance of identity at all: ownership was a
// nickname string comparison, so anyone could claim another player's worlds by
// logging in under their name.

const (
	// ChallengeTTL is how long an unanswered challenge stays valid. Short, because
	// a client requests one immediately before signing it.
	ChallengeTTL = 2 * time.Minute

	// SessionTTL is how long a session survives without being used. Sessions are
	// held in memory and were previously never expired or pruned, so the store
	// grew without bound and any leaked token was valid until the process exited.
	SessionTTL = 24 * time.Hour

	// MaxNicknameLen bounds a display name.
	MaxNicknameLen = 20

	// challengeBytes is the size of the random challenge nonce.
	challengeBytes = 32
)

var (
	ErrBadPublicKey  = errors.New("malformed public key")
	ErrNoChallenge   = errors.New("no pending challenge for this key")
	ErrChallengeUsed = errors.New("challenge already used or expired")
	ErrBadSignature  = errors.New("signature does not match public key")
)

// Identity is a verified player, keyed by public key.
type Identity struct {
	// PublicKey is the raw Ed25519 public key.
	PublicKey ed25519.PublicKey

	// KeyID is the base64url form of the public key, used as a map key and as
	// the value stored against world ownership.
	KeyID string

	// PlayerID is derived from the public key so it is stable across restarts.
	PlayerID uint32

	// Nickname is a changeable display name.
	Nickname string
}

// Fingerprint returns a short human-readable form of the key, for display where
// a full key would be unwieldy. Not used for authorization.
func (id Identity) Fingerprint() string {
	sum := sha256.Sum256(id.PublicKey)
	return hex.EncodeToString(sum[:4])
}

// AuthSession is an authenticated session bound to an identity.
type AuthSession struct {
	Token    string
	PlayerID uint32
	Nickname string

	// KeyID identifies the owning key. Authorization decisions use this rather
	// than the nickname, which is not unique and can be changed freely.
	KeyID string

	lastSeen time.Time
}

type challenge struct {
	nonce   []byte
	expires time.Time
}

// AuthManager issues challenges, verifies signatures and tracks sessions.
type AuthManager struct {
	mu         sync.RWMutex
	sessions   map[string]*AuthSession // token -> session
	challenges map[string]challenge    // keyID -> outstanding challenge
	nicknames  map[string]string       // keyID -> last chosen display name
}

// NewAuthManager creates an empty AuthManager.
func NewAuthManager() *AuthManager {
	return &AuthManager{
		sessions:   make(map[string]*AuthSession),
		challenges: make(map[string]challenge),
		nicknames:  make(map[string]string),
	}
}

// DecodePublicKey parses a base64url-encoded Ed25519 public key.
func DecodePublicKey(encoded string) (ed25519.PublicKey, string, error) {
	raw, err := base64.RawURLEncoding.DecodeString(encoded)
	if err != nil {
		return nil, "", ErrBadPublicKey
	}
	if len(raw) != ed25519.PublicKeySize {
		return nil, "", fmt.Errorf("%w: expected %d bytes, got %d",
			ErrBadPublicKey, ed25519.PublicKeySize, len(raw))
	}
	return ed25519.PublicKey(raw), base64.RawURLEncoding.EncodeToString(raw), nil
}

// PlayerIDForKey derives a stable player ID from a public key.
//
// Deriving rather than allocating from a counter means the same key is the same
// player after a restart. Previously IDs came from an in-memory counter that
// restarted at zero, so IDs were reused by unrelated players and any stored
// reference to one became wrong.
//
// This is a 32-bit truncation of a hash, so distinct keys can in principle
// collide. With the player counts this server supports the probability is
// negligible, and a collision costs a shared scoreboard row rather than shared
// authority: ownership is checked against the full key, never the ID.
func PlayerIDForKey(pub ed25519.PublicKey) uint32 {
	sum := sha256.Sum256(pub)
	id := binary.BigEndian.Uint32(sum[:4])
	if id == 0 {
		// Zero marks "no identity" in the wire protocol.
		id = 1
	}
	return id
}

// NewChallenge issues a fresh challenge for a public key.
//
// Any previous outstanding challenge for the same key is discarded, so a client
// that abandons a login attempt cannot leave a usable nonce behind.
func (am *AuthManager) NewChallenge(keyID string) (string, time.Time, error) {
	nonce := make([]byte, challengeBytes)
	if _, err := rand.Read(nonce); err != nil {
		return "", time.Time{}, fmt.Errorf("generate challenge: %w", err)
	}
	expires := time.Now().Add(ChallengeTTL)

	am.mu.Lock()
	am.challenges[keyID] = challenge{nonce: nonce, expires: expires}
	am.mu.Unlock()

	return base64.RawURLEncoding.EncodeToString(nonce), expires, nil
}

// VerifyAndLogin checks a signature over the outstanding challenge and, on
// success, creates a session.
//
// The challenge is consumed whether or not verification succeeds, so a captured
// pair cannot be replayed and a failed attempt cannot be retried against the
// same nonce.
func (am *AuthManager) VerifyAndLogin(encodedKey, encodedSig, nickname string) (*AuthSession, error) {
	pub, keyID, err := DecodePublicKey(encodedKey)
	if err != nil {
		return nil, err
	}

	sig, err := base64.RawURLEncoding.DecodeString(encodedSig)
	if err != nil {
		return nil, ErrBadSignature
	}

	am.mu.Lock()
	ch, ok := am.challenges[keyID]
	delete(am.challenges, keyID)
	am.mu.Unlock()

	if !ok {
		return nil, ErrNoChallenge
	}
	if time.Now().After(ch.expires) {
		return nil, ErrChallengeUsed
	}

	if !ed25519.Verify(pub, ch.nonce, sig) {
		return nil, ErrBadSignature
	}

	return am.createSession(pub, keyID, nickname), nil
}

// createSession mints a token for a verified identity.
func (am *AuthManager) createSession(pub ed25519.PublicKey, keyID, nickname string) *AuthSession {
	nickname = SanitiseNickname(nickname)

	am.mu.Lock()
	defer am.mu.Unlock()

	// Remember the chosen name so a later login without one keeps it.
	if nickname != "" {
		am.nicknames[keyID] = nickname
	} else if remembered, ok := am.nicknames[keyID]; ok {
		nickname = remembered
	} else {
		nickname = "Anonymous"
	}

	sess := &AuthSession{
		Token:    generateToken(),
		PlayerID: PlayerIDForKey(pub),
		Nickname: nickname,
		KeyID:    keyID,
		lastSeen: time.Now(),
	}
	am.sessions[sess.Token] = sess
	return sess
}

// Validate returns the session for a token, or nil if absent or expired.
//
// A successful lookup refreshes the session's activity timestamp, so an active
// player is never logged out mid-session.
func (am *AuthManager) Validate(token string) *AuthSession {
	if token == "" {
		return nil
	}

	am.mu.Lock()
	defer am.mu.Unlock()

	sess, ok := am.sessions[token]
	if !ok {
		return nil
	}
	if time.Since(sess.lastSeen) > SessionTTL {
		delete(am.sessions, token)
		return nil
	}
	sess.lastSeen = time.Now()

	// Return a copy so callers cannot mutate shared session state.
	out := *sess
	return &out
}

// Rename changes the display name for the identity holding this token.
func (am *AuthManager) Rename(token, nickname string) (*AuthSession, error) {
	nickname = SanitiseNickname(nickname)
	if nickname == "" {
		return nil, errors.New("display name cannot be empty")
	}

	am.mu.Lock()
	defer am.mu.Unlock()

	sess, ok := am.sessions[token]
	if !ok {
		return nil, errors.New("not signed in")
	}
	sess.Nickname = nickname
	sess.lastSeen = time.Now()
	am.nicknames[sess.KeyID] = nickname

	out := *sess
	return &out, nil
}

// Logout removes a session.
func (am *AuthManager) Logout(token string) {
	am.mu.Lock()
	delete(am.sessions, token)
	am.mu.Unlock()
}

// PruneExpired drops stale sessions and challenges, and reports how many
// sessions were removed. Intended to be called periodically.
func (am *AuthManager) PruneExpired() int {
	now := time.Now()

	am.mu.Lock()
	defer am.mu.Unlock()

	removed := 0
	for token, sess := range am.sessions {
		if now.Sub(sess.lastSeen) > SessionTTL {
			delete(am.sessions, token)
			removed++
		}
	}
	for keyID, ch := range am.challenges {
		if now.After(ch.expires) {
			delete(am.challenges, keyID)
		}
	}
	return removed
}

// SessionCount reports how many sessions are held, for metrics and tests.
func (am *AuthManager) SessionCount() int {
	am.mu.RLock()
	defer am.mu.RUnlock()
	return len(am.sessions)
}

// SanitiseNickname trims a display name and bounds its length.
func SanitiseNickname(n string) string {
	out := make([]rune, 0, MaxNicknameLen)
	for _, r := range n {
		// Drop control characters so a name cannot disrupt rendering or logs.
		if r < 0x20 || r == 0x7f {
			continue
		}
		out = append(out, r)
		if len(out) >= MaxNicknameLen {
			break
		}
	}

	// Trim surrounding spaces without pulling in strings for one call.
	start, end := 0, len(out)
	for start < end && out[start] == ' ' {
		start++
	}
	for end > start && out[end-1] == ' ' {
		end--
	}
	return string(out[start:end])
}

// TokensEqual compares two tokens in constant time.
func TokensEqual(a, b string) bool {
	return subtle.ConstantTimeCompare([]byte(a), []byte(b)) == 1
}

// generateToken returns a random 32-byte session token.
func generateToken() string {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		// crypto/rand failing is not recoverable: a guessable token would be
		// worse than refusing to mint one, so this panics rather than falling
		// back to something predictable as the previous implementation did.
		panic(fmt.Sprintf("crypto/rand unavailable: %v", err))
	}
	return base64.RawURLEncoding.EncodeToString(b)
}
