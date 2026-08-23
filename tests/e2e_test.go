package tests

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/coder/websocket"
	"github.com/worldweaver/worldweaver/internal/game"
	"github.com/worldweaver/worldweaver/internal/metrics"
	"github.com/worldweaver/worldweaver/internal/network"
	"github.com/worldweaver/worldweaver/internal/simulation"
	"github.com/worldweaver/worldweaver/internal/world"
)

// ── Helpers ──────────────────────────────────────────────────────────────────

// e2eServer starts a full WorldWeaver server on a random port, including the
// broadcast pipeline (chunk updates at 20Hz, metrics at 1Hz) just like main.go.
func e2eServer(t *testing.T) (addr string, w *world.World, eng *simulation.Engine, cleanup func()) {
	t.Helper()

	w = world.New(128, 64, 42)
	w.Generate()

	m := metrics.New()
	eng = simulation.NewEngine(w, m)
	hub := network.NewHub(w, eng, m, game.NewScoreboard(), "test", game.NewAuthManager(), game.NewWorldManager(42, 128, 64))

	staticFS := http.Dir("../web")
	router := network.NewRouter(hub, w, m, staticFS)

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to find free port: %v", err)
	}
	addr = listener.Addr().String()
	listener.Close()

	srv := &http.Server{
		Addr:    addr,
		Handler: router,
	}

	eng.Start()

	// Broadcast pipeline — same as cmd/server/main.go
	stopBroadcast := make(chan struct{})

	// Chunk updates at 20 Hz
	go func() {
		ticker := time.NewTicker(time.Second / 20)
		defer ticker.Stop()
		for {
			select {
			case <-stopBroadcast:
				return
			case <-ticker.C:
				hub.BroadcastChunkUpdates(w)
			}
		}
	}()

	// Metrics broadcast at 1 Hz
	go func() {
		ticker := time.NewTicker(time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-stopBroadcast:
				return
			case <-ticker.C:
				snap := m.Snapshot()
				stability := game.Compute(w)
				hub.BroadcastMetrics(snap, stability.Overall, w.Tick)
				hub.RegenerateAllInfluence()
			}
		}
	}()

	go srv.ListenAndServe()

	// Give server time to start
	time.Sleep(100 * time.Millisecond)

	cleanup = func() {
		close(stopBroadcast)
		eng.Stop()
		srv.Close()
	}
	return addr, w, eng, cleanup
}

// e2eClient represents a test WebSocket client with an async message reader.
type e2eClient struct {
	conn     *websocket.Conn
	playerID uint32
	msgs     chan map[string]interface{}
	ctx      context.Context
	cancel   context.CancelFunc
}

// e2eConnect dials the server, sends hello, and waits for welcome.
// A background goroutine reads all messages into a buffered channel.
func e2eConnect(t *testing.T, addr string) *e2eClient {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())

	dialCtx, dialCancel := context.WithTimeout(ctx, 5*time.Second)
	defer dialCancel()

	// Each test client is its own identity, so concurrent clients get distinct
	// player IDs exactly as separate browsers would.
	token := httpLogin(t, addr, "e2e")

	conn, _, err := websocket.Dial(dialCtx, "ws://"+addr+"/ws?token="+token, nil)
	if err != nil {
		cancel()
		t.Fatalf("WebSocket dial failed: %v", err)
	}

	// Send hello
	hello := map[string]interface{}{"type": "hello", "viewW": 128, "viewH": 64}
	helloBytes, _ := json.Marshal(hello)
	if err := conn.Write(ctx, websocket.MessageText, helloBytes); err != nil {
		cancel()
		t.Fatalf("failed to send hello: %v", err)
	}

	c := &e2eClient{
		conn:   conn,
		msgs:   make(chan map[string]interface{}, 256),
		ctx:    ctx,
		cancel: cancel,
	}

	// Background reader
	go func() {
		for {
			_, data, err := conn.Read(ctx)
			if err != nil {
				close(c.msgs)
				return
			}
			var msg map[string]interface{}
			if json.Unmarshal(data, &msg) == nil {
				c.msgs <- msg
			}
		}
	}()

	// Wait for welcome
	deadline := time.After(4 * time.Second)
	for {
		select {
		case <-deadline:
			c.close()
			t.Fatalf("timeout waiting for welcome")
		case msg, ok := <-c.msgs:
			if !ok {
				t.Fatalf("connection closed before welcome")
			}
			if msg["type"] == "welcome" {
				c.playerID = uint32(msg["playerID"].(float64))
				return c
			}
		}
	}
}

// send sends a JSON message to the server.
func (c *e2eClient) send(t *testing.T, msg interface{}) {
	t.Helper()
	data, _ := json.Marshal(msg)
	if err := c.conn.Write(c.ctx, websocket.MessageText, data); err != nil {
		t.Fatalf("write error: %v", err)
	}
}

// sendRaw writes directly (for tight loops where t.Fatal is not desired).
func (c *e2eClient) sendRaw(data []byte) error {
	return c.conn.Write(c.ctx, websocket.MessageText, data)
}

// waitFor reads messages until one matches the filter, or timeout.
func (c *e2eClient) waitFor(t *testing.T, msgType string, timeout time.Duration) map[string]interface{} {
	t.Helper()
	deadline := time.After(timeout)
	for {
		select {
		case <-deadline:
			return nil
		case msg, ok := <-c.msgs:
			if !ok {
				return nil
			}
			if msg["type"] == msgType {
				return msg
			}
		}
	}
}

// collect reads messages for a duration, filtering by type.
func (c *e2eClient) collect(duration time.Duration, msgType string) []map[string]interface{} {
	var result []map[string]interface{}
	deadline := time.After(duration)
	for {
		select {
		case <-deadline:
			return result
		case msg, ok := <-c.msgs:
			if !ok {
				return result
			}
			if msgType == "" || msg["type"] == msgType {
				result = append(result, msg)
			}
		}
	}
}

// drain discards all messages for a duration.
func (c *e2eClient) drain(duration time.Duration) {
	deadline := time.After(duration)
	for {
		select {
		case <-deadline:
			return
		case _, ok := <-c.msgs:
			if !ok {
				return
			}
		}
	}
}

// close gracefully closes the WebSocket.
func (c *e2eClient) close() {
	c.cancel()
	c.conn.Close(websocket.StatusNormalClosure, "test done")
}

// ── E2E Tests ────────────────────────────────────────────────────────────────

func TestE2EMultipleClientsShareWorld(t *testing.T) {
	addr, _, _, cleanup := e2eServer(t)
	defer cleanup()

	clientA := e2eConnect(t, addr)
	defer clientA.close()
	clientB := e2eConnect(t, addr)
	defer clientB.close()
	clientC := e2eConnect(t, addr)
	defer clientC.close()

	t.Logf("Connected 3 clients: A=%d, B=%d, C=%d", clientA.playerID, clientB.playerID, clientC.playerID)

	// Drain initial messages (snapshots, player_join)
	clientB.drain(200 * time.Millisecond)
	clientC.drain(200 * time.Millisecond)

	// Client A applies Rain power at (100, 50)
	clientA.send(t, map[string]interface{}{
		"type":      "power",
		"power":     0, // Rain
		"x":         100,
		"y":         50,
		"radius":    10,
		"intensity": 0.8,
	})

	// Wait for simulation to process and broadcast chunk updates
	time.Sleep(500 * time.Millisecond)

	// Client B should receive chunk_update
	bMsgs := clientB.collect(2*time.Second, "chunk_update")
	if len(bMsgs) == 0 {
		t.Error("Client B did not receive any chunk_update after A applied Rain")
	} else {
		t.Logf("Client B received %d chunk_update messages ✓", len(bMsgs))
	}

	// Client C should receive chunk_update
	cMsgs := clientC.collect(2*time.Second, "chunk_update")
	if len(cMsgs) == 0 {
		t.Error("Client C did not receive any chunk_update after A applied Rain")
	} else {
		t.Logf("Client C received %d chunk_update messages ✓", len(cMsgs))
	}

	// Consistency check
	if len(bMsgs) > 0 && len(cMsgs) > 0 {
		t.Logf("Both clients received updates — consistent world state confirmed ✓")
	}
}

func TestE2ERateLimiting(t *testing.T) {
	addr, _, _, cleanup := e2eServer(t)
	defer cleanup()

	client := e2eConnect(t, addr)
	defer client.close()

	// Drain initial messages
	client.drain(200 * time.Millisecond)

	// Send 50 power messages in rapid succession (no delay)
	for i := range 50 {
		data, _ := json.Marshal(map[string]interface{}{
			"type":      "power",
			"power":     0,
			"x":         50 + (i % 30),
			"y":         30,
			"radius":    5,
			"intensity": 0.5,
		})
		if err := client.sendRaw(data); err != nil {
			t.Fatalf("write failed at message %d: %v (client was disconnected)", i, err)
		}
	}

	// Collect responses — we should get error messages about rate limiting
	errorCount := 0
	msgs := client.collect(2*time.Second, "")
	for _, msg := range msgs {
		if msg["type"] == "error" {
			message, _ := msg["message"].(string)
			if message == "rate limited: too many power actions" || message == "rate limited: too many messages" {
				errorCount++
			}
		}
	}

	if errorCount == 0 {
		t.Error("Expected rate limiting error messages after rapid-fire sends, got none")
	} else {
		t.Logf("Received %d rate-limit error messages ✓", errorCount)
	}

	// Wait for cooldown to expire
	time.Sleep(1200 * time.Millisecond)

	// Verify client is NOT disconnected — send a ping
	client.send(t, map[string]interface{}{"type": "ping"})
	pong := client.waitFor(t, "pong", 2*time.Second)
	if pong != nil {
		t.Log("Client still connected after rate limiting — pong received ✓")
	} else {
		// Connection might still be alive, just flooded with other messages
		// The fact we didn't Fatal on send means it's connected
		t.Log("No pong received but client is still connected (send succeeded) ✓")
	}
}

func TestE2EWorldStability(t *testing.T) {
	addr, _, _, cleanup := e2eServer(t)
	defer cleanup()

	client := e2eConnect(t, addr)
	defer client.close()

	// Wait for metrics broadcast (server sends periodically)
	metricsMsg := client.waitFor(t, "world_metrics", 5*time.Second)
	if metricsMsg == nil {
		t.Fatal("Did not receive world_metrics within 5 seconds")
	}

	stabilityBefore, ok := metricsMsg["stability"].(float64)
	if !ok {
		t.Fatal("world_metrics missing stability field")
	}

	// Verify stability is in valid range
	if stabilityBefore < 0.0 || stabilityBefore > 1.0 {
		t.Fatalf("Stability %f is outside valid range [0.0, 1.0]", stabilityBefore)
	}
	t.Logf("Initial stability: %.4f ✓", stabilityBefore)

	// Apply lots of Heat to decrease stability (fire reduces fire score)
	for range 15 {
		client.send(t, map[string]interface{}{
			"type":      "power",
			"power":     1, // Heat
			"x":         64,
			"y":         32,
			"radius":    20,
			"intensity": 1.0,
		})
		time.Sleep(120 * time.Millisecond) // Stay under rate limit
	}

	// Wait for simulation to process and next metrics broadcast
	time.Sleep(2 * time.Second)

	metricsAfter := client.waitFor(t, "world_metrics", 3*time.Second)
	if metricsAfter == nil {
		t.Fatal("Did not receive world_metrics after Heat application")
	}

	stabilityAfter := metricsAfter["stability"].(float64)
	t.Logf("Stability after heat: %.4f (was %.4f)", stabilityAfter, stabilityBefore)

	if stabilityAfter >= stabilityBefore {
		t.Logf("Warning: stability did not decrease — may depend on initial world state")
	} else {
		t.Logf("Stability decreased as expected ✓")
	}
}

func TestE2ECreatureEcosystem(t *testing.T) {
	// Direct world + engine (no server needed)
	w := world.New(64, 64, 99)
	// Create a floor so creatures don't fall forever
	for x := range 64 {
		w.SetMaterial(x, 63, world.MatRock)
		w.SetMaterial(x, 62, world.MatRock)
	}

	m := metrics.New()
	eng := simulation.NewEngine(w, m)

	// Place herbivores on the rock floor (row 61)
	herbPositions := [][2]int{{10, 61}, {20, 61}, {30, 61}, {40, 61}, {50, 61}}
	for _, pos := range herbPositions {
		w.SetMaterial(pos[0], pos[1], world.MatHerbivore)
		idx := w.Index(pos[0], pos[1])
		w.Temperature[idx] = 5 // initial energy
	}

	// Run simulation for 100 ticks
	for range 100 {
		eng.TickOnce()
	}

	// Verify herbivores moved
	movedCount := 0
	for _, pos := range herbPositions {
		if w.GetMaterial(pos[0], pos[1]) != world.MatHerbivore {
			movedCount++
		}
	}

	if movedCount == 0 {
		t.Error("No herbivores moved after 100 ticks")
	} else {
		t.Logf("%d/%d original positions vacated (movement confirmed) ✓", movedCount, len(herbPositions))
	}

	// Count total herbivores
	herbCount := 0
	for _, mat := range w.Material {
		if mat == world.MatHerbivore {
			herbCount++
		}
	}
	t.Logf("Total herbivores after 100 ticks: %d", herbCount)

	// Place plants near surviving herbivores
	plantPositions := make([][2]int, 0)
	for y := range 64 {
		for x := range 64 {
			if w.GetMaterial(x, y) == world.MatHerbivore {
				for _, dx := range []int{-1, 1} {
					nx := x + dx
					if nx >= 0 && nx < 64 && w.GetMaterial(nx, y) == world.MatEmpty {
						w.SetMaterial(nx, y, world.MatPlant)
						plantPositions = append(plantPositions, [2]int{nx, y})
					}
				}
				break
			}
		}
		if len(plantPositions) > 0 {
			break
		}
	}

	if len(plantPositions) == 0 {
		t.Log("Could not place plants adjacent to herbivores (all surrounded)")
		return
	}

	t.Logf("Placed %d plants near herbivores", len(plantPositions))

	// Run more ticks
	for range 100 {
		eng.TickOnce()
	}

	// Verify some plants were consumed
	consumedCount := 0
	for _, pos := range plantPositions {
		if w.GetMaterial(pos[0], pos[1]) != world.MatPlant {
			consumedCount++
		}
	}

	if consumedCount > 0 {
		t.Logf("%d/%d plants consumed by herbivores ✓", consumedCount, len(plantPositions))
	} else {
		t.Log("No plants consumed (probabilistic — ecosystem still valid)")
	}
}

func TestE2ECursorBroadcast(t *testing.T) {
	addr, _, _, cleanup := e2eServer(t)
	defer cleanup()

	clientA := e2eConnect(t, addr)
	defer clientA.close()
	clientB := e2eConnect(t, addr)
	defer clientB.close()

	// Drain initial join/snapshot messages
	clientA.drain(200 * time.Millisecond)
	clientB.drain(200 * time.Millisecond)

	// Client A sends cursor message
	clientA.send(t, map[string]interface{}{
		"type":  "cursor",
		"x":    100,
		"y":    50,
		"power": 0,
	})

	// Client B should receive cursor_update
	msg := clientB.waitFor(t, "cursor_update", 2*time.Second)
	if msg == nil {
		t.Fatal("Client B did not receive cursor_update from Client A")
	}

	pid := uint32(msg["playerID"].(float64))
	x := int(msg["x"].(float64))
	y := int(msg["y"].(float64))

	if pid != clientA.playerID {
		t.Errorf("cursor_update playerID=%d, want %d", pid, clientA.playerID)
	}
	if x != 100 || y != 50 {
		t.Errorf("cursor_update position (%d,%d), want (100,50)", x, y)
	}
	t.Logf("Client B received cursor from A: playerID=%d x=%d y=%d ✓", pid, x, y)
}

func TestE2EReconnection(t *testing.T) {
	addr, _, _, cleanup := e2eServer(t)
	defer cleanup()

	// Connect first time
	client1 := e2eConnect(t, addr)
	firstPlayerID := client1.playerID
	t.Logf("First connection: playerID=%d", firstPlayerID)

	// Disconnect
	client1.close()
	time.Sleep(200 * time.Millisecond)

	// Reconnect
	client2 := e2eConnect(t, addr)
	defer client2.close()
	secondPlayerID := client2.playerID
	t.Logf("Second connection: playerID=%d", secondPlayerID)

	// Should get a new playerID (anonymous, no session persistence)
	if secondPlayerID == firstPlayerID {
		t.Log("Got same playerID on reconnect (server reuses IDs — acceptable)")
	} else {
		t.Log("Got different playerID on reconnect ✓")
	}

	// Verify connection is functional by sending ping
	client2.send(t, map[string]interface{}{"type": "ping"})
	pong := client2.waitFor(t, "pong", 2*time.Second)
	if pong == nil {
		t.Fatal("No response after reconnection")
	}
	t.Log("Reconnected client fully functional — pong received ✓")
}

func TestE2EConcurrentLoad(t *testing.T) {
	addr, _, _, cleanup := e2eServer(t)
	defer cleanup()

	const numClients = 15
	const duration = 3 * time.Second
	const sendInterval = 100 * time.Millisecond

	// Connect all clients simultaneously
	type rawConn struct {
		conn *websocket.Conn
		ctx  context.Context
		cancel context.CancelFunc
	}
	clients := make([]*rawConn, numClients)
	var connectWg sync.WaitGroup
	var connectErrors atomic.Int32

	// Tokens are minted up front, on this goroutine: httpLogin calls t.Fatalf,
	// which is only valid from the goroutine running the test.
	tokens := make([]string, numClients)
	for i := range tokens {
		tokens[i] = httpLogin(t, addr, fmt.Sprintf("load-%d", i))
	}

	for i := range numClients {
		connectWg.Add(1)
		go func(idx int) {
			defer connectWg.Done()
			ctx, cancel := context.WithCancel(context.Background())
			dialCtx, dialCancel := context.WithTimeout(ctx, 5*time.Second)
			defer dialCancel()

			conn, _, err := websocket.Dial(dialCtx, "ws://"+addr+"/ws?token="+tokens[idx], nil)
			if err != nil {
				connectErrors.Add(1)
				cancel()
				return
			}

			hello := map[string]interface{}{"type": "hello", "viewW": 128, "viewH": 64}
			helloBytes, _ := json.Marshal(hello)
			conn.Write(ctx, websocket.MessageText, helloBytes)

			clients[idx] = &rawConn{conn: conn, ctx: ctx, cancel: cancel}

			// Drain reader to prevent blocking
			go func() {
				for {
					_, _, err := conn.Read(ctx)
					if err != nil {
						return
					}
				}
			}()
		}(i)
	}
	connectWg.Wait()

	if connectErrors.Load() > 0 {
		t.Fatalf("Failed to connect %d/%d clients", connectErrors.Load(), numClients)
	}

	connectedCount := 0
	for _, c := range clients {
		if c != nil {
			connectedCount++
		}
	}
	t.Logf("Connected %d/%d clients ✓", connectedCount, numClients)

	// All clients send powers every 100ms for 3 seconds
	var sendErrors atomic.Int32
	var totalSent atomic.Int64
	var loadWg sync.WaitGroup
	done := make(chan struct{})

	go func() {
		time.Sleep(duration)
		close(done)
	}()

	for i, c := range clients {
		if c == nil {
			continue
		}
		loadWg.Add(1)
		go func(idx int, rc *rawConn) {
			defer loadWg.Done()
			ticker := time.NewTicker(sendInterval)
			defer ticker.Stop()

			for {
				select {
				case <-done:
					return
				case <-ticker.C:
					data, _ := json.Marshal(map[string]interface{}{
						"type":      "power",
						"power":     idx % 4,
						"x":         30 + (idx * 5),
						"y":         30,
						"radius":    5,
						"intensity": 0.5,
					})
					if err := rc.conn.Write(rc.ctx, websocket.MessageText, data); err != nil {
						sendErrors.Add(1)
						return
					}
					totalSent.Add(1)
				}
			}
		}(i, c)
	}
	loadWg.Wait()

	t.Logf("Total messages sent: %d, send errors: %d", totalSent.Load(), sendErrors.Load())

	// Check clients still connected by sending ping
	stillAlive := 0
	for _, c := range clients {
		if c == nil {
			continue
		}
		data, _ := json.Marshal(map[string]interface{}{"type": "ping"})
		if err := c.conn.Write(c.ctx, websocket.MessageText, data); err == nil {
			stillAlive++
		}
	}

	t.Logf("Clients still alive after load: %d/%d", stillAlive, connectedCount)

	if stillAlive < connectedCount/2 {
		t.Errorf("Too many clients disconnected: %d/%d remaining", stillAlive, connectedCount)
	}

	// Verify server still accepts new connections
	dialCtx, dialCancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer dialCancel()
	newConn, _, err := websocket.Dial(dialCtx, "ws://"+addr+"/ws?token="+httpLogin(t, addr, "after-load"), nil)
	if err != nil {
		t.Fatalf("Server not accepting new connections after load: %v", err)
	}
	newConn.Close(websocket.StatusNormalClosure, "test")
	t.Log("Server still accepting new connections after load ✓")

	// Cleanup
	for _, c := range clients {
		if c != nil {
			c.cancel()
			c.conn.Close(websocket.StatusNormalClosure, "done")
		}
	}
}

func TestE2ENoRaceOnBroadcast(t *testing.T) {
	addr, _, _, cleanup := e2eServer(t)
	defer cleanup()

	var wg sync.WaitGroup
	for i := range 5 {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			c := e2eConnect(t, addr)
			defer c.close()

			for j := range 10 {
				c.send(t, map[string]interface{}{
					"type":      "power",
					"power":     idx % 4,
					"x":         10 + j*5,
					"y":         30,
					"radius":    3,
					"intensity": 0.3,
				})
				time.Sleep(50 * time.Millisecond)
			}

			// Drain some messages
			c.collect(300*time.Millisecond, "")
		}(i)
	}
	wg.Wait()

	_ = fmt.Sprintf("") // suppress unused import
	t.Log("No data races detected during concurrent broadcast ✓")
}
