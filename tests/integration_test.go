package tests

import (
	"context"
	"encoding/json"
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

// TestFullStackIntegration verifies the complete server pipeline:
// world generation → simulation → WebSocket → client receives welcome + snapshot.
func TestFullStackIntegration(t *testing.T) {
	// ── Setup ────────────────────────────────────────────────────────────────
	w := world.New(128, 64, 42)
	w.Generate()

	m := metrics.New()
	eng := simulation.NewEngine(w, m)
	hub := network.NewHub(w, eng, m, game.NewScoreboard(), "test", game.NewAuthManager(), game.NewWorldManager(42, 128, 64))

	staticFS := http.Dir("../web")
	router := network.NewRouter(hub, w, m, staticFS)

	// Find a free port
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to find free port: %v", err)
	}
	addr := listener.Addr().String()
	listener.Close()

	srv := &http.Server{
		Addr:    addr,
		Handler: router,
	}

	eng.Start()
	defer eng.Stop()

	go srv.ListenAndServe()
	defer srv.Close()

	// Give the server a moment to start
	time.Sleep(100 * time.Millisecond)

	// ── Test HTTP serves index.html ──────────────────────────────────────────
	t.Run("HTTP_serves_index", func(t *testing.T) {
		resp, err := http.Get("http://" + addr + "/")
		if err != nil {
			t.Fatalf("HTTP GET / failed: %v", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != 200 {
			t.Fatalf("expected 200, got %d", resp.StatusCode)
		}
		if ct := resp.Header.Get("Content-Type"); ct == "" {
			t.Fatal("no Content-Type header")
		}
	})

	// ── Test WebSocket connection + welcome message ──────────────────────────
	t.Run("WebSocket_welcome", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		conn, _, err := websocket.Dial(ctx, "ws://"+addr+"/ws", nil)
		if err != nil {
			t.Fatalf("WebSocket dial failed: %v", err)
		}
		defer conn.CloseNow()

		// Send hello
		hello := map[string]interface{}{
			"type":  "hello",
			"viewW": 128,
			"viewH": 64,
		}
		helloBytes, _ := json.Marshal(hello)
		err = conn.Write(ctx, websocket.MessageText, helloBytes)
		if err != nil {
			t.Fatalf("failed to send hello: %v", err)
		}

		// Read messages until we get a welcome
		gotWelcome := false
		gotSnapshot := false
		deadline := time.After(4 * time.Second)

		for !gotWelcome || !gotSnapshot {
			select {
			case <-deadline:
				t.Fatalf("timeout waiting for messages (welcome=%v, snapshot=%v)", gotWelcome, gotSnapshot)
			default:
			}

			_, data, err := conn.Read(ctx)
			if err != nil {
				t.Fatalf("read error: %v", err)
			}

			var msg map[string]interface{}
			if err := json.Unmarshal(data, &msg); err != nil {
				t.Fatalf("invalid JSON from server: %v", err)
			}

			switch msg["type"] {
			case "welcome":
				gotWelcome = true
				if msg["worldW"] == nil || msg["worldH"] == nil {
					t.Fatal("welcome missing worldW/worldH")
				}
				t.Logf("Welcome: playerID=%v world=%vx%v", msg["playerID"], msg["worldW"], msg["worldH"])
			case "world_snapshot":
				gotSnapshot = true
				if msg["data"] == nil {
					t.Fatal("snapshot missing data")
				}
				t.Logf("Snapshot: tick=%v region=%vx%v", msg["tick"], msg["w"], msg["h"])
			}
		}

		conn.Close(websocket.StatusNormalClosure, "test done")
	})

	// ── Test power application ───────────────────────────────────────────────
	t.Run("WebSocket_power", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		conn, _, err := websocket.Dial(ctx, "ws://"+addr+"/ws", nil)
		if err != nil {
			t.Fatalf("WebSocket dial failed: %v", err)
		}
		defer conn.CloseNow()

		// Send hello
		hello := map[string]interface{}{"type": "hello", "viewW": 128, "viewH": 64}
		helloBytes, _ := json.Marshal(hello)
		conn.Write(ctx, websocket.MessageText, helloBytes)

		// Wait for welcome
		time.Sleep(200 * time.Millisecond)

		// Send a power (Rain = 0)
		power := map[string]interface{}{
			"type":      "power",
			"power":     0,
			"x":         64,
			"y":         32,
			"radius":    10,
			"intensity": 0.8,
		}
		powerBytes, _ := json.Marshal(power)
		err = conn.Write(ctx, websocket.MessageText, powerBytes)
		if err != nil {
			t.Fatalf("failed to send power: %v", err)
		}

		// If we get here without an error/disconnect, the server accepted the power
		t.Log("Power command accepted (no disconnect/error)")

		// Read a couple messages to verify server is still sending updates
		_, _, err = conn.Read(ctx)
		if err != nil {
			t.Fatalf("server stopped sending after power command: %v", err)
		}
		t.Log("Server still streaming after power application ✓")

		conn.Close(websocket.StatusNormalClosure, "test done")
	})
}
