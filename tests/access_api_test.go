package tests

import (
	"context"
	"net"
	"net/http"
	"testing"
	"time"

	"github.com/coder/websocket"
	"github.com/worldweaver/worldweaver/internal/game"
	"github.com/worldweaver/worldweaver/internal/metrics"
	"github.com/worldweaver/worldweaver/internal/network"
	"github.com/worldweaver/worldweaver/internal/simulation"
	"github.com/worldweaver/worldweaver/internal/world"
)

// HTTP-level access control.
//
// access_test.go covers the registry in isolation; this file checks the wiring,
// because a correct registry behind a handler that never consults it protects
// nothing. Every case here is one a player could perform with curl.

// apiServer starts a router on a random port with no simulation running: these
// tests only exercise HTTP handlers.
func apiServer(t *testing.T) string {
	t.Helper()

	w := world.New(64, 32, 7)
	m := metrics.New()
	eng := simulation.NewEngine(w, m)
	hub := network.NewHub(w, eng, m, game.NewScoreboard(), "genesis",
		game.NewAuthManager(), game.NewWorldManager(7, 64, 32))

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}

	srv := &http.Server{Handler: network.NewRouter(hub, w, m, http.Dir("../web/dist"))}
	go srv.Serve(listener)
	t.Cleanup(func() { srv.Close() })

	return listener.Addr().String()
}

type worldRow struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	Visibility string `json:"visibility"`
	Owned      bool   `json:"owned"`
}

// createWorld posts a new world and returns its ID.
func createWorld(t *testing.T, addr, token, name, visibility string) string {
	t.Helper()
	var resp struct {
		World worldRow `json:"world"`
		Error string   `json:"error"`
	}
	status := postJSON(t, "http://"+addr+"/api/worlds", token, map[string]any{
		"name":       name,
		"seed":       1,
		"size":       "small",
		"visibility": visibility,
	}, &resp)
	if status != http.StatusCreated {
		t.Fatalf("create world %q: status %d (%s)", name, status, resp.Error)
	}
	return resp.World.ID
}

func listWorlds(t *testing.T, addr, token string) []worldRow {
	t.Helper()
	var out []worldRow
	if status := requestJSON(t, http.MethodGet, "http://"+addr+"/api/worlds", token, nil, &out); status != http.StatusOK {
		t.Fatalf("list worlds: status %d", status)
	}
	return out
}

func hasWorld(rows []worldRow, id string) bool {
	for _, r := range rows {
		if r.ID == id {
			return true
		}
	}
	return false
}

// TestWorldCreationRequiresSession: creating a world unauthenticated used to
// succeed and record the owner as the literal string "anonymous", which every
// other anonymous caller then matched.
func TestWorldCreationRequiresSession(t *testing.T) {
	addr := apiServer(t)

	status := postJSON(t, "http://"+addr+"/api/worlds", "", map[string]any{
		"name": "squatter",
	}, nil)
	if status != http.StatusUnauthorized {
		t.Errorf("unauthenticated create returned %d, want 401", status)
	}
	if rows := listWorlds(t, addr, ""); len(rows) != 1 || rows[0].ID != "genesis" {
		t.Errorf("world was created anyway: %+v", rows)
	}
}

// TestPrivateWorldNotListedToStrangers is the fix for private worlds being
// private in name only while the listing returned every world to everybody.
func TestPrivateWorldNotListedToStrangers(t *testing.T) {
	addr := apiServer(t)
	ownerTok := httpLogin(t, addr, "owner")
	strangerTok := httpLogin(t, addr, "stranger")

	priv := createWorld(t, addr, ownerTok, "hidden", "private")
	pub := createWorld(t, addr, ownerTok, "open", "public")

	ownerSees := listWorlds(t, addr, ownerTok)
	if !hasWorld(ownerSees, priv) {
		t.Error("owner cannot see their own private world")
	}

	strangerSees := listWorlds(t, addr, strangerTok)
	if hasWorld(strangerSees, priv) {
		t.Error("private world listed to a stranger")
	}
	if !hasWorld(strangerSees, pub) {
		t.Error("public world missing from a stranger's listing")
	}

	// A visitor with no session at all sees only public worlds.
	if hasWorld(listWorlds(t, addr, ""), priv) {
		t.Error("private world listed to an anonymous visitor")
	}
}

// TestListingMarksOwnership so the lobby knows which worlds to offer controls for.
func TestListingMarksOwnership(t *testing.T) {
	addr := apiServer(t)
	ownerTok := httpLogin(t, addr, "owner")
	strangerTok := httpLogin(t, addr, "stranger")
	id := createWorld(t, addr, ownerTok, "mine", "public")

	for _, row := range listWorlds(t, addr, ownerTok) {
		if row.ID == id && !row.Owned {
			t.Error("owner's own world is not marked owned")
		}
	}
	for _, row := range listWorlds(t, addr, strangerTok) {
		if row.ID == id && row.Owned {
			t.Error("someone else's world is marked owned")
		}
	}
}

// TestNonOwnerCannotDeleteWorld: the old check compared display names, so
// logging in as "owner" was enough to delete owner's worlds.
func TestNonOwnerCannotDeleteWorld(t *testing.T) {
	addr := apiServer(t)
	ownerTok := httpLogin(t, addr, "owner")
	id := createWorld(t, addr, ownerTok, "mine", "public")

	// Same display name, different key.
	impostorTok := httpLogin(t, addr, "owner")

	url := "http://" + addr + "/api/worlds/" + id
	if status := requestJSON(t, http.MethodDelete, url, impostorTok, nil, nil); status != http.StatusForbidden {
		t.Errorf("impostor delete returned %d, want 403", status)
	}
	if status := requestJSON(t, http.MethodDelete, url, "", nil, nil); status != http.StatusUnauthorized {
		t.Errorf("anonymous delete returned %d, want 401", status)
	}
	if !hasWorld(listWorlds(t, addr, ownerTok), id) {
		t.Fatal("world disappeared despite both deletions being refused")
	}
	if status := requestJSON(t, http.MethodDelete, url, ownerTok, nil, nil); status != http.StatusOK {
		t.Errorf("owner delete returned %d, want 200", status)
	}
	if hasWorld(listWorlds(t, addr, ownerTok), id) {
		t.Error("world survived deletion by its owner")
	}
}

// TestGenesisCannotBeDeleted.
func TestGenesisCannotBeDeleted(t *testing.T) {
	addr := apiServer(t)
	tok := httpLogin(t, addr, "someone")

	status := requestJSON(t, http.MethodDelete, "http://"+addr+"/api/worlds/genesis", tok, nil, nil)
	if status != http.StatusForbidden {
		t.Errorf("deleting genesis returned %d, want 403", status)
	}
}

// TestInviteFlowOverHTTP walks the whole collaboration path: the owner mints a
// code, a stranger redeems it and the world becomes visible to them.
func TestInviteFlowOverHTTP(t *testing.T) {
	addr := apiServer(t)
	ownerTok := httpLogin(t, addr, "owner")
	strangerTok := httpLogin(t, addr, "friend")
	id := createWorld(t, addr, ownerTok, "clubhouse", "private")

	var inv struct {
		Code  string `json:"code"`
		Error string `json:"error"`
	}
	status := postJSON(t, "http://"+addr+"/api/invites", ownerTok, map[string]any{
		"worldId":   id,
		"maxUses":   1,
		"expiresIn": "1h",
	}, &inv)
	if status != http.StatusCreated || inv.Code == "" {
		t.Fatalf("create invite: status %d (%s)", status, inv.Error)
	}

	// A stranger cannot mint codes for a world they do not own.
	if status := postJSON(t, "http://"+addr+"/api/invites", strangerTok, map[string]any{
		"worldId": id,
	}, nil); status != http.StatusForbidden {
		t.Errorf("stranger invite creation returned %d, want 403", status)
	}

	var redeem struct {
		WorldID string `json:"worldId"`
		Error   string `json:"error"`
	}
	status = postJSON(t, "http://"+addr+"/api/invites/redeem", strangerTok,
		map[string]any{"code": inv.Code}, &redeem)
	if status != http.StatusOK {
		t.Fatalf("redeem: status %d (%s)", status, redeem.Error)
	}
	if redeem.WorldID != id {
		t.Errorf("redeem returned world %q, want %q", redeem.WorldID, id)
	}
	if !hasWorld(listWorlds(t, addr, strangerTok), id) {
		t.Error("invited player still cannot see the world")
	}

	// The code was single-use.
	if status := postJSON(t, "http://"+addr+"/api/invites/redeem", httpLogin(t, addr, "gatecrasher"),
		map[string]any{"code": inv.Code}, nil); status != http.StatusBadRequest {
		t.Errorf("second redemption of a one-use code returned %d, want 400", status)
	}
}

// TestVisibilityChangeOverHTTP.
func TestVisibilityChangeOverHTTP(t *testing.T) {
	addr := apiServer(t)
	ownerTok := httpLogin(t, addr, "owner")
	strangerTok := httpLogin(t, addr, "stranger")
	id := createWorld(t, addr, ownerTok, "shifty", "private")

	url := "http://" + addr + "/api/worlds/" + id + "/visibility"
	if status := requestJSON(t, http.MethodPut, url, strangerTok,
		map[string]string{"visibility": "public"}, nil); status != http.StatusForbidden {
		t.Errorf("stranger visibility change returned %d, want 403", status)
	}
	if status := requestJSON(t, http.MethodPut, url, ownerTok,
		map[string]string{"visibility": "public"}, nil); status != http.StatusOK {
		t.Errorf("owner visibility change returned %d, want 200", status)
	}
	if !hasWorld(listWorlds(t, addr, strangerTok), id) {
		t.Error("world still hidden after being made public")
	}
}

// TestWebSocketRequiresSession: the socket is where world state actually flows,
// so an unauthenticated upgrade has to be refused rather than downgraded to an
// anonymous player as it was before.
func TestWebSocketRequiresSession(t *testing.T) {
	addr := apiServer(t)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	for _, q := range []string{"", "?token=not-a-real-token"} {
		conn, resp, err := websocket.Dial(ctx, "ws://"+addr+"/ws"+q, nil)
		if err == nil {
			conn.CloseNow()
			t.Fatalf("dial %q succeeded without a valid session", q)
		}
		if resp == nil || resp.StatusCode != http.StatusUnauthorized {
			t.Errorf("dial %q: want 401, got %v", q, resp)
		}
	}
}

// TestWebSocketRefusesPrivateWorld: membership is enforced at the socket, not
// only in the listing. Hiding a world from the lobby is worthless if anyone who
// learns its ID can connect to it.
func TestWebSocketRefusesPrivateWorld(t *testing.T) {
	addr := apiServer(t)
	ownerTok := httpLogin(t, addr, "owner")
	strangerTok := httpLogin(t, addr, "stranger")
	id := createWorld(t, addr, ownerTok, "sealed", "private")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	conn, resp, err := websocket.Dial(ctx, "ws://"+addr+"/ws?world="+id+"&token="+strangerTok, nil)
	if err == nil {
		conn.CloseNow()
		t.Fatal("stranger connected to a private world")
	}
	if resp == nil || resp.StatusCode != http.StatusForbidden {
		t.Errorf("want 403 for a private world, got %v", resp)
	}

	// The owner is admitted.
	ownerConn, _, err := websocket.Dial(ctx, "ws://"+addr+"/ws?world="+id+"&token="+ownerTok, nil)
	if err != nil {
		t.Fatalf("owner refused entry to their own world: %v", err)
	}
	ownerConn.Close(websocket.StatusNormalClosure, "done")
}

// TestWebSocketAllowsUnlistedWorldWithID: unlisted is hidden, not sealed, which
// is what makes a shared link work.
func TestWebSocketAllowsUnlistedWorldWithID(t *testing.T) {
	addr := apiServer(t)
	ownerTok := httpLogin(t, addr, "owner")
	strangerTok := httpLogin(t, addr, "stranger")
	id := createWorld(t, addr, ownerTok, "linkonly", "unlisted")

	if hasWorld(listWorlds(t, addr, strangerTok), id) {
		t.Error("unlisted world appeared in the listing")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	conn, _, err := websocket.Dial(ctx, "ws://"+addr+"/ws?world="+id+"&token="+strangerTok, nil)
	if err != nil {
		t.Fatalf("unlisted world refused a player who had the link: %v", err)
	}
	conn.Close(websocket.StatusNormalClosure, "done")
}

// TestSessionEndpointRejectsQueryToken: the token used to be accepted from the
// query string, which wrote credentials into access logs and browser history.
func TestSessionEndpointRejectsQueryToken(t *testing.T) {
	addr := apiServer(t)
	tok := httpLogin(t, addr, "someone")

	if status := requestJSON(t, http.MethodGet,
		"http://"+addr+"/api/session?token="+tok, "", nil, nil); status != http.StatusUnauthorized {
		t.Errorf("query-string token accepted: status %d, want 401", status)
	}
	if status := requestJSON(t, http.MethodGet,
		"http://"+addr+"/api/session", tok, nil, nil); status != http.StatusOK {
		t.Errorf("header token rejected: status %d, want 200", status)
	}
}

// TestLogoutInvalidatesSessionOverHTTP.
func TestLogoutInvalidatesSessionOverHTTP(t *testing.T) {
	addr := apiServer(t)
	tok := httpLogin(t, addr, "leaver")

	if status := postJSON(t, "http://"+addr+"/api/logout", tok, nil, nil); status != http.StatusOK {
		t.Fatalf("logout returned %d", status)
	}
	if status := requestJSON(t, http.MethodGet, "http://"+addr+"/api/session", tok, nil, nil); status != http.StatusUnauthorized {
		t.Errorf("token still valid after logout: status %d", status)
	}
}

// TestRenameKeepsOwnership: a display name is cosmetic, so changing it must not
// affect what you own.
func TestRenameKeepsOwnership(t *testing.T) {
	addr := apiServer(t)
	tok := httpLogin(t, addr, "before")
	id := createWorld(t, addr, tok, "mine", "public")

	if status := postJSON(t, "http://"+addr+"/api/rename", tok,
		map[string]string{"nickname": "after"}, nil); status != http.StatusOK {
		t.Fatalf("rename failed")
	}

	url := "http://" + addr + "/api/worlds/" + id
	if status := requestJSON(t, http.MethodDelete, url, tok, nil, nil); status != http.StatusOK {
		t.Errorf("owner lost control of their world after renaming: status %d", status)
	}
}
