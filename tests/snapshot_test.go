package tests

import (
	"context"
	"encoding/base64"
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

// TestInitialSnapshotIsNotEmpty is a regression test for the blank-canvas bug.
//
// SendFullSnapshot derives its region from the player's camera, but the camera is
// only populated once the read pump handles the client's HELLO message. The
// server used to send the snapshot immediately on connect, before that happened,
// so it described a 0x0 region containing no cells. A freshly connected client
// therefore had nothing to draw and showed an empty world until unrelated chunk
// updates happened to trickle in.
//
// The snapshot is now sent in response to the client reporting its viewport.
func TestInitialSnapshotIsNotEmpty(t *testing.T) {
	w := world.New(256, 128, 4242)
	w.Generate()

	m := metrics.New()
	eng := simulation.NewEngine(w, m)
	hub := network.NewHub(w, eng, m, game.NewScoreboard(), "genesis",
		game.NewAuthManager(), game.NewWorldManager(4242, 256, 128))

	router := network.NewRouter(hub, w, m, http.Dir("../web/dist"))

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	addr := listener.Addr().String()

	srv := &http.Server{Handler: router}
	go srv.Serve(listener)
	defer srv.Close()

	eng.Start()
	defer eng.Stop()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	conn, _, err := websocket.Dial(ctx,
		"ws://"+addr+"/ws?world=genesis&token="+httpLogin(t, addr, "snap"), nil)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.Close(websocket.StatusNormalClosure, "done")

	// A snapshot covers the whole visible region and so exceeds the small
	// default read limit. Browsers impose no comparable cap.
	conn.SetReadLimit(8 << 20)

	// Mirror what the browser client does on connect: announce the viewport.
	hello := `{"type":"hello","viewW":256,"viewH":128}`
	if err := conn.Write(ctx, websocket.MessageText, []byte(hello)); err != nil {
		t.Fatalf("write hello: %v", err)
	}

	for i := 0; i < 20; i++ {
		_, data, err := conn.Read(ctx)
		if err != nil {
			t.Fatalf("read: %v", err)
		}

		var env struct {
			Type string `json:"type"`
			W    int    `json:"w"`
			H    int    `json:"h"`
			Data string `json:"data"`
		}
		if err := json.Unmarshal(data, &env); err != nil {
			continue // other message types are not relevant here
		}
		if env.Type != "world_snapshot" {
			continue
		}

		if env.W <= 0 || env.H <= 0 {
			t.Fatalf("snapshot region is empty: %dx%d", env.W, env.H)
		}

		raw, err := base64.StdEncoding.DecodeString(env.Data)
		if err != nil {
			t.Fatalf("snapshot payload is not valid base64: %v", err)
		}
		if len(raw) != env.W*env.H {
			t.Fatalf("snapshot payload is %d bytes, want %d for %dx%d",
				len(raw), env.W*env.H, env.W, env.H)
		}

		// A generated world must contain material, otherwise the client would
		// draw an empty scene even though a snapshot technically arrived.
		nonEmpty := 0
		for _, b := range raw {
			if b != 0 {
				nonEmpty++
			}
		}
		if nonEmpty == 0 {
			t.Fatal("snapshot contains no material — client would render nothing")
		}

		t.Logf("snapshot %dx%d, %d bytes, %d non-empty cells (%.1f%%)",
			env.W, env.H, len(raw), nonEmpty, float64(nonEmpty)/float64(len(raw))*100)
		return
	}

	t.Fatal("no world_snapshot received within 20 messages")
}
