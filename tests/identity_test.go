package tests

import (
	"crypto/ed25519"
	"encoding/base64"
	"testing"
	"time"

	"github.com/worldweaver/worldweaver/internal/game"
)

// newKey returns a fresh Ed25519 keypair and the base64url public key.
func newKey(t *testing.T) (ed25519.PrivateKey, string) {
	t.Helper()
	pub, priv, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	return priv, base64.RawURLEncoding.EncodeToString(pub)
}

// signChallenge signs a base64url challenge, as a client would.
func signChallenge(t *testing.T, priv ed25519.PrivateKey, challenge string) string {
	t.Helper()
	nonce, err := base64.RawURLEncoding.DecodeString(challenge)
	if err != nil {
		t.Fatalf("decode challenge: %v", err)
	}
	return base64.RawURLEncoding.EncodeToString(ed25519.Sign(priv, nonce))
}

// login performs the full challenge-response handshake.
func login(t *testing.T, am *game.AuthManager, priv ed25519.PrivateKey, pubKey, nick string) *game.AuthSession {
	t.Helper()
	ch, _, err := am.NewChallenge(mustKeyID(t, pubKey))
	if err != nil {
		t.Fatalf("challenge: %v", err)
	}
	sess, err := am.VerifyAndLogin(pubKey, signChallenge(t, priv, ch), nick)
	if err != nil {
		t.Fatalf("login: %v", err)
	}
	return sess
}

func mustKeyID(t *testing.T, pubKey string) string {
	t.Helper()
	_, keyID, err := game.DecodePublicKey(pubKey)
	if err != nil {
		t.Fatalf("decode public key: %v", err)
	}
	return keyID
}

// TestValidSignatureLogsIn is the happy path: possession of the private key is
// what grants a session.
func TestValidSignatureLogsIn(t *testing.T) {
	am := game.NewAuthManager()
	priv, pubKey := newKey(t)

	sess := login(t, am, priv, pubKey, "Pablo")

	if sess.Token == "" {
		t.Error("no token issued")
	}
	if sess.Nickname != "Pablo" {
		t.Errorf("nickname is %q, want %q", sess.Nickname, "Pablo")
	}
	if sess.PlayerID == 0 {
		t.Error("player ID is zero, which the protocol uses to mean no identity")
	}
	if got := am.Validate(sess.Token); got == nil {
		t.Error("freshly issued token does not validate")
	}
}

// TestWrongSignatureRejected covers the case that made the previous scheme
// worthless: presenting an identity you cannot prove you hold.
func TestWrongSignatureRejected(t *testing.T) {
	am := game.NewAuthManager()
	_, victimKey := newKey(t)
	attackerPriv, _ := newKey(t)

	ch, _, err := am.NewChallenge(mustKeyID(t, victimKey))
	if err != nil {
		t.Fatalf("challenge: %v", err)
	}

	// The attacker signs the victim's challenge with their own key.
	sig := signChallenge(t, attackerPriv, ch)

	if _, err := am.VerifyAndLogin(victimKey, sig, "impostor"); err == nil {
		t.Fatal("login succeeded while impersonating another key")
	}
}

// TestChallengeIsSingleUse means a captured handshake cannot be replayed.
func TestChallengeIsSingleUse(t *testing.T) {
	am := game.NewAuthManager()
	priv, pubKey := newKey(t)

	ch, _, err := am.NewChallenge(mustKeyID(t, pubKey))
	if err != nil {
		t.Fatalf("challenge: %v", err)
	}
	sig := signChallenge(t, priv, ch)

	if _, err := am.VerifyAndLogin(pubKey, sig, "first"); err != nil {
		t.Fatalf("first login failed: %v", err)
	}
	if _, err := am.VerifyAndLogin(pubKey, sig, "replay"); err == nil {
		t.Fatal("the same challenge and signature were accepted twice")
	}
}

// TestFailedAttemptConsumesChallenge stops an attacker retrying signatures
// against one nonce.
func TestFailedAttemptConsumesChallenge(t *testing.T) {
	am := game.NewAuthManager()
	priv, pubKey := newKey(t)

	ch, _, err := am.NewChallenge(mustKeyID(t, pubKey))
	if err != nil {
		t.Fatalf("challenge: %v", err)
	}

	if _, err := am.VerifyAndLogin(pubKey, "not-a-signature", "x"); err == nil {
		t.Fatal("a malformed signature was accepted")
	}
	// Even the correct signature must now fail: the nonce is spent.
	if _, err := am.VerifyAndLogin(pubKey, signChallenge(t, priv, ch), "x"); err == nil {
		t.Fatal("challenge survived a failed attempt")
	}
}

// TestPlayerIDIsStableForKey is what makes ownership and scores survive a
// restart. IDs previously came from a counter that reset to zero, so they were
// reused by unrelated players.
func TestPlayerIDIsStableForKey(t *testing.T) {
	priv, pubKey := newKey(t)
	_, otherKey := newKey(t)

	first := login(t, game.NewAuthManager(), priv, pubKey, "a")
	// A completely fresh manager stands in for a restarted server.
	second := login(t, game.NewAuthManager(), priv, pubKey, "b")

	if first.PlayerID != second.PlayerID {
		t.Errorf("same key produced different IDs across restarts: %d then %d",
			first.PlayerID, second.PlayerID)
	}

	otherPub, _, err := game.DecodePublicKey(otherKey)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	samePub, _, err := game.DecodePublicKey(pubKey)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if game.PlayerIDForKey(otherPub) == game.PlayerIDForKey(samePub) {
		t.Error("two different keys produced the same player ID")
	}
}

// TestRenameKeepsIdentity: a display name is cosmetic.
func TestRenameKeepsIdentity(t *testing.T) {
	am := game.NewAuthManager()
	priv, pubKey := newKey(t)

	sess := login(t, am, priv, pubKey, "Original")
	renamed, err := am.Rename(sess.Token, "Something Else")
	if err != nil {
		t.Fatalf("rename: %v", err)
	}

	if renamed.Nickname != "Something Else" {
		t.Errorf("nickname is %q after rename", renamed.Nickname)
	}
	if renamed.PlayerID != sess.PlayerID {
		t.Error("player ID changed on rename")
	}
	if renamed.KeyID != sess.KeyID {
		t.Error("key ID changed on rename")
	}
}

// TestMalformedPublicKeyRejected guards the decode path.
func TestMalformedPublicKeyRejected(t *testing.T) {
	am := game.NewAuthManager()
	for _, bad := range []string{"", "!!!!", "c2hvcnQ"} {
		if _, err := am.VerifyAndLogin(bad, "sig", "x"); err == nil {
			t.Errorf("accepted malformed public key %q", bad)
		}
	}
}

// TestLoginWithoutChallengeFails: a signature alone is not enough.
func TestLoginWithoutChallengeFails(t *testing.T) {
	am := game.NewAuthManager()
	priv, pubKey := newKey(t)

	sig := base64.RawURLEncoding.EncodeToString(ed25519.Sign(priv, []byte("anything")))
	if _, err := am.VerifyAndLogin(pubKey, sig, "x"); err == nil {
		t.Fatal("logged in with no outstanding challenge")
	}
}

// TestLogoutInvalidatesToken.
func TestLogoutInvalidatesToken(t *testing.T) {
	am := game.NewAuthManager()
	priv, pubKey := newKey(t)

	sess := login(t, am, priv, pubKey, "x")
	am.Logout(sess.Token)

	if am.Validate(sess.Token) != nil {
		t.Error("token still valid after logout")
	}
}

// TestPruneRemovesNothingWhileActive: pruning must not log out live players.
func TestPruneRemovesNothingWhileActive(t *testing.T) {
	am := game.NewAuthManager()
	priv, pubKey := newKey(t)
	sess := login(t, am, priv, pubKey, "x")

	if removed := am.PruneExpired(); removed != 0 {
		t.Errorf("pruned %d fresh sessions", removed)
	}
	if am.Validate(sess.Token) == nil {
		t.Error("pruning invalidated an active session")
	}
	if am.SessionCount() != 1 {
		t.Errorf("session count is %d, want 1", am.SessionCount())
	}
}

// TestNicknameSanitising bounds length and strips control characters, which
// would otherwise reach logs and other players' screens.
func TestNicknameSanitising(t *testing.T) {
	cases := []struct{ in, want string }{
		{"  Pablo  ", "Pablo"},
		{"a\x00b", "ab"},
		{"line\nbreak", "linebreak"},
		{"", ""},
	}
	for _, tc := range cases {
		if got := game.SanitiseNickname(tc.in); got != tc.want {
			t.Errorf("SanitiseNickname(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}

	long := game.SanitiseNickname("aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa")
	if len([]rune(long)) > game.MaxNicknameLen {
		t.Errorf("nickname length %d exceeds the %d cap", len([]rune(long)), game.MaxNicknameLen)
	}
}

// TestChallengeExpiryIsSet confirms challenges carry a deadline.
func TestChallengeExpiryIsSet(t *testing.T) {
	am := game.NewAuthManager()
	_, pubKey := newKey(t)

	_, expires, err := am.NewChallenge(mustKeyID(t, pubKey))
	if err != nil {
		t.Fatalf("challenge: %v", err)
	}

	if until := time.Until(expires); until <= 0 || until > game.ChallengeTTL+time.Second {
		t.Errorf("challenge expiry is %v away, want between 0 and %v", until, game.ChallengeTTL)
	}
}

// TestNewChallengeSupersedesPrevious: abandoning a login leaves no usable nonce.
func TestNewChallengeSupersedesPrevious(t *testing.T) {
	am := game.NewAuthManager()
	priv, pubKey := newKey(t)
	keyID := mustKeyID(t, pubKey)

	first, _, err := am.NewChallenge(keyID)
	if err != nil {
		t.Fatalf("challenge: %v", err)
	}
	if _, _, err := am.NewChallenge(keyID); err != nil {
		t.Fatalf("second challenge: %v", err)
	}

	if _, err := am.VerifyAndLogin(pubKey, signChallenge(t, priv, first), "x"); err == nil {
		t.Fatal("a superseded challenge was still accepted")
	}
}
