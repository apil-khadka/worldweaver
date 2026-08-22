package network

import (
	"context"
	"encoding/json"
	"log"
	"time"

	"github.com/coder/websocket"
	"github.com/worldweaver/worldweaver/internal/game"
)

const (
	// writeTimeout is the maximum time to write a single message to a client.
	// If exceeded the client is disconnected to prevent slow clients from
	// blocking the broadcast pipeline.
	writeTimeout = 5 * time.Second

	// writeQueueDepth is the size of the per-client send queue.
	// Old updates are dropped when the queue fills (newest wins).
	writeQueueDepth = 64
)

// Client represents a single connected browser.
type Client struct {
	Player  *game.Player
	WorldID string
	hub     *Hub
	conn    *websocket.Conn
	send    chan []byte
	limiter *ClientRateLimiter
}

// newClient constructs a Client and immediately starts its write pump.
func newClient(hub *Hub, conn *websocket.Conn) *Client {
	p := game.NewPlayer()
	c := &Client{
		Player:  p,
		WorldID: "genesis",
		hub:     hub,
		conn:    conn,
		send:    make(chan []byte, writeQueueDepth),
		limiter: NewClientRateLimiter(),
	}
	return c
}

// newClientWithIdentity constructs a Client with a pre-assigned player ID and nickname.
func newClientWithIdentity(hub *Hub, conn *websocket.Conn, playerID uint32, nickname string, worldID string) *Client {
	p := game.NewPlayerWithID(playerID, nickname)
	c := &Client{
		Player:  p,
		WorldID: worldID,
		hub:     hub,
		conn:    conn,
		send:    make(chan []byte, writeQueueDepth),
		limiter: NewClientRateLimiter(),
	}
	return c
}

// writePump drains the send queue and writes messages to the WebSocket.
// It runs in its own goroutine.  Simulation never blocks waiting for it.
func (c *Client) writePump(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case msg, ok := <-c.send:
			if !ok {
				// Hub closed the channel.
				return
			}
			wctx, cancel := context.WithTimeout(ctx, writeTimeout)
			err := c.conn.Write(wctx, websocket.MessageText, msg)
			cancel()
			if err != nil {
				log.Printf("client %d write error: %v", c.Player.ID, err)
				return
			}
		}
	}
}

// readPump reads inbound messages and dispatches them.
// It runs in its own goroutine.  When it returns the client is unregistered.
func (c *Client) readPump(ctx context.Context) {
	defer c.hub.unregister(c)

	for {
		_, data, err := c.conn.Read(ctx)
		if err != nil {
			return
		}

		// Per-client message rate limit (30 msg/s, 1s cooldown on breach).
		if !c.limiter.AllowMessage() {
			c.sendError("rate limited: too many messages")
			continue
		}

		var env InboundEnvelope
		if err := json.Unmarshal(data, &env); err != nil {
			c.sendError("malformed JSON")
			continue
		}

		switch env.Type {
		case MsgHello:
			var msg HelloMsg
			if err := json.Unmarshal(data, &msg); err == nil {
				c.Player.SetCamera(0, 0, msg.ViewW, msg.ViewH)
			}

		case MsgPowerInput:
			// Per-client power rate limit (10 actions/s).
			if !c.limiter.AllowPower() {
				c.sendError("rate limited: too many power actions")
				continue
			}
			var msg PowerInputMsg
			if err := json.Unmarshal(data, &msg); err != nil {
				c.sendError("malformed power message")
				continue
			}
			c.hub.handlePowerInput(c, &msg)

		case MsgViewport:
			var msg ViewportMsg
			if err := json.Unmarshal(data, &msg); err == nil {
				c.Player.SetCamera(msg.X, msg.Y, msg.W, msg.H)
			}

		case MsgPing:
			c.sendRaw(mustMarshal(map[string]string{"type": MsgPong}))

		case MsgCursor:
			var msg CursorMsg
			if err := json.Unmarshal(data, &msg); err == nil {
				c.hub.handleCursorMove(c, &msg)
			}

		default:
			c.sendError("unknown message type")
		}
	}
}

// sendJSON marshals v and queues the message for delivery.
// If the send queue is full, the oldest item is dropped so the queue never
// blocks the simulation.
func (c *Client) sendJSON(v any) {
	b, err := json.Marshal(v)
	if err != nil {
		log.Printf("client %d marshal error: %v", c.Player.ID, err)
		return
	}
	c.sendRaw(b)
}

func (c *Client) sendRaw(b []byte) {
	select {
	case c.send <- b:
	default:
		// Queue full — drop oldest, push newest.
		select {
		case <-c.send:
		default:
		}
		c.send <- b
	}
}

func (c *Client) sendError(msg string) {
	c.sendJSON(ErrorMsg{Type: MsgError, Message: msg})
}

func mustMarshal(v any) []byte {
	b, _ := json.Marshal(v)
	return b
}
