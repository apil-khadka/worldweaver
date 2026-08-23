package tests

import (
	"bytes"
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"testing"
)

// HTTP-level login helper.
//
// The WebSocket endpoint now requires a verified session, so any test that dials
// /ws has to authenticate first. Rather than reaching into the server's
// AuthManager, these helpers drive the same two-step handshake a browser does,
// which means the tests also cover the challenge and login routes.

// httpLogin generates a fresh keypair, completes the challenge-response
// handshake against a running server and returns the session token.
func httpLogin(t *testing.T, addr, nick string) string {
	t.Helper()

	pub, priv, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	pubKey := base64.RawURLEncoding.EncodeToString(pub)

	var chResp struct {
		Challenge string `json:"challenge"`
		Error     string `json:"error"`
	}
	postJSON(t, "http://"+addr+"/api/challenge", "", map[string]string{
		"publicKey": pubKey,
	}, &chResp)
	if chResp.Challenge == "" {
		t.Fatalf("challenge request failed: %s", chResp.Error)
	}

	nonce, err := base64.RawURLEncoding.DecodeString(chResp.Challenge)
	if err != nil {
		t.Fatalf("decode challenge: %v", err)
	}

	var loginResp struct {
		Token string `json:"token"`
		Error string `json:"error"`
	}
	postJSON(t, "http://"+addr+"/api/login", "", map[string]string{
		"publicKey": pubKey,
		"signature": base64.RawURLEncoding.EncodeToString(ed25519.Sign(priv, nonce)),
		"nickname":  nick,
	}, &loginResp)
	if loginResp.Token == "" {
		t.Fatalf("login failed: %s", loginResp.Error)
	}
	return loginResp.Token
}

// postJSON sends a JSON body and decodes the JSON reply into out.
// An empty token omits the Authorization header.
func postJSON(t *testing.T, url, token string, body any, out any) int {
	t.Helper()
	return requestJSON(t, http.MethodPost, url, token, body, out)
}

// requestJSON performs a JSON request with an optional bearer token and returns
// the status code, decoding the body into out when out is non-nil.
func requestJSON(t *testing.T, method, url, token string, body any, out any) int {
	t.Helper()

	var buf bytes.Buffer
	if body != nil {
		if err := json.NewEncoder(&buf).Encode(body); err != nil {
			t.Fatalf("encode body: %v", err)
		}
	}

	req, err := http.NewRequest(method, url, &buf)
	if err != nil {
		t.Fatalf("build request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("%s %s: %v", method, url, err)
	}
	defer resp.Body.Close()

	if out != nil {
		// A non-JSON error page is not a test failure in itself; the status code
		// is what most callers assert on.
		_ = json.NewDecoder(resp.Body).Decode(out)
	}
	return resp.StatusCode
}
