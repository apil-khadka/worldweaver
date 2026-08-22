package game

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"sync"
)

// AuthSession represents an authenticated player session.
type AuthSession struct {
	Token    string
	PlayerID uint32
	Nickname string
}

// AuthManager handles simple token-based authentication.
// In-memory only — sessions are lost on restart.
type AuthManager struct {
	mu       sync.RWMutex
	sessions map[string]*AuthSession // token -> session
}

// NewAuthManager creates a new in-memory auth manager.
func NewAuthManager() *AuthManager {
	return &AuthManager{
		sessions: make(map[string]*AuthSession),
	}
}

// Login creates a new session for the given nickname.
// Returns the session with a generated token.
func (am *AuthManager) Login(nickname string) *AuthSession {
	token := generateToken()
	id := nextPlayerID()

	sess := &AuthSession{
		Token:    token,
		PlayerID: id,
		Nickname: nickname,
	}

	am.mu.Lock()
	am.sessions[token] = sess
	am.mu.Unlock()

	return sess
}

// Validate checks if a token is valid and returns the associated session.
// Returns nil if the token is invalid.
func (am *AuthManager) Validate(token string) *AuthSession {
	am.mu.RLock()
	defer am.mu.RUnlock()
	return am.sessions[token]
}

// Logout removes a session by token.
func (am *AuthManager) Logout(token string) {
	am.mu.Lock()
	delete(am.sessions, token)
	am.mu.Unlock()
}

// nextPlayerID generates a unique player ID using the existing atomic counter.
func nextPlayerID() uint32 {
	return playerIDCounter.Add(1)
}

// generateToken returns a random 16-byte hex token.
func generateToken() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		// Fallback: this should never happen
		return fmt.Sprintf("fallback-%d", nextPlayerID())
	}
	return hex.EncodeToString(b)
}
