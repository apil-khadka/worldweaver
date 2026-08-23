// Package network owns the WebSocket transport, client management, and the
// message protocol between server and browser clients.
//
// # Protocol overview
//
// All messages are JSON for the MVP.  Binary encoding will be introduced once
// the protocol is stable (see ADR-007).
//
// ## Client → Server messages
//
//	HELLO        { "type": "hello", "viewW": 800, "viewH": 600 }
//	POWER_INPUT  { "type": "power", "power": 0, "x": 500, "y": 220, "radius": 24, "intensity": 0.8 }
//	VIEWPORT     { "type": "viewport", "x": 0, "y": 0, "w": 800, "h": 600 }
//	PING         { "type": "ping" }
//
// ## Server → Client messages
//
//	WELCOME        { "type": "welcome", "playerID": 1, "worldW": 1024, "worldH": 512, "tick": 0 }
//	WORLD_SNAPSHOT { "type": "world_snapshot", "tick": N, "w": W, "h": H, "data": "<base64>" }
//	CHUNK_UPDATE   { "type": "chunk_update",   "tick": N, "chunks": [...] }
//	PLAYER_STATE   { "type": "player_state",   "playerID": N, "influence": 72.5 }
//	WORLD_METRICS  { "type": "world_metrics",  "tps": 60, "activeCells": 183000, ... }
//	ERROR          { "type": "error",           "message": "..." }
//	PONG           { "type": "pong" }
package network

// Inbound message types (client → server)
const (
	MsgHello        = "hello"
	MsgPowerInput   = "power"
	MsgViewport     = "viewport"
	MsgPing         = "ping"
	MsgCursor       = "cursor"
	MsgChat         = "chat"
	MsgPingLocation = "ping_location"
	MsgEmote        = "emote"
)

// Outbound message types (server → client)
const (
	MsgWelcome       = "welcome"
	MsgWorldSnapshot = "world_snapshot"
	MsgChunkUpdate   = "chunk_update"
	MsgPlayerState   = "player_state"
	MsgWorldMetrics  = "world_metrics"
	MsgError         = "error"
	MsgPong          = "pong"
	MsgCursorUpdate  = "cursor_update"
	MsgPlayerJoin    = "player_join"
	MsgPlayerLeave   = "player_leave"
	MsgChatBroadcast = "chat"
	MsgPingBroadcast = "ping_location"
	MsgEmoteBroadcast = "emote"
	MsgCombo         = "combo"
	MsgGoalUpdate    = "goal_update"
)

// ---- Inbound message structs ----

// InboundEnvelope is used for initial type dispatch.
type InboundEnvelope struct {
	Type string `json:"type"`
}

// HelloMsg is the first message a client sends after connecting.
type HelloMsg struct {
	Type  string `json:"type"`
	ViewW uint16 `json:"viewW"`
	ViewH uint16 `json:"viewH"`
}

// PowerInputMsg carries a player's world-shaping request.
// The server validates and rate-limits before acting on this.
//
// Tool selects between applying an elemental force and editing the world
// directly. It is optional: an absent value means "force", which keeps the
// message compatible with clients that predate the god-mode tools.
type PowerInputMsg struct {
	Type      string  `json:"type"`
	Tool      string  `json:"tool,omitempty"`
	Power     uint8   `json:"power"`
	Material  uint8   `json:"material,omitempty"`
	X         int     `json:"x"`
	Y         int     `json:"y"`
	Radius    int     `json:"radius"`
	Intensity float32 `json:"intensity"`
}

// ViewportMsg tells the server where the player is looking.
// The server uses this to decide which chunks to stream.
type ViewportMsg struct {
	Type string `json:"type"`
	X    int32  `json:"x"`
	Y    int32  `json:"y"`
	W    uint16 `json:"w"`
	H    uint16 `json:"h"`
}

// ---- Outbound message structs ----

// WelcomeMsg is the first message the server sends to a new client.
type WelcomeMsg struct {
	Type    string `json:"type"`
	PlayerID uint32 `json:"playerID"`
	WorldW  int    `json:"worldW"`
	WorldH  int    `json:"worldH"`
	Tick    uint64 `json:"tick"`
}

// WorldSnapshotMsg carries a full material snapshot of the visible region.
// Data is a base64-encoded flat array of uint8 material IDs.
type WorldSnapshotMsg struct {
	Type string `json:"type"`
	Tick uint64 `json:"tick"`
	X    int    `json:"x"`
	Y    int    `json:"y"`
	W    int    `json:"w"`
	H    int    `json:"h"`
	Data []byte `json:"data"` // raw bytes; JSON marshaller will base64-encode
}

// ChunkUpdateEntry carries a single dirty chunk's material data.
type ChunkUpdateEntry struct {
	CX   int    `json:"cx"`   // chunk grid X
	CY   int    `json:"cy"`   // chunk grid Y
	Tick uint64 `json:"tick"`
	Data []byte `json:"data"` // flat material array for this chunk
}

// ChunkUpdateMsg carries all dirty chunks since last broadcast.
type ChunkUpdateMsg struct {
	Type   string             `json:"type"`
	Tick   uint64             `json:"tick"`
	Chunks []ChunkUpdateEntry `json:"chunks"`
}

// PlayerStateMsg carries the player's current game-layer state.
type PlayerStateMsg struct {
	Type           string  `json:"type"`
	PlayerID       uint32  `json:"playerID"`
	Influence      float32 `json:"influence"`
	MaxInfluence   float32 `json:"maxInfluence"`
	Level          int     `json:"level"`
	Score          int     `json:"score"`
	NextLevelScore int     `json:"nextLevelScore"`
}

// WorldMetricsMsg carries live performance/game telemetry.
type WorldMetricsMsg struct {
	Type         string  `json:"type"`
	Tick         uint64  `json:"tick"`
	TPS          float64 `json:"tps"`
	TickP95Ms    float64 `json:"tickP95Ms"`
	ActiveCells  int64   `json:"activeCells"`
	ActiveChunks int64   `json:"activeChunks"`
	PlayerCount  int     `json:"playerCount"`
	OutboundBPS  int64   `json:"outboundBPS"`
	Stability    float32 `json:"stability"`
}

// ErrorMsg is sent when the server rejects or cannot process a client request.
type ErrorMsg struct {
	Type    string `json:"type"`
	Message string `json:"message"`
}

// ---- Cursor & Presence messages ----

// CursorMsg is sent by a client to report its cursor position in world coordinates.
type CursorMsg struct {
	Type  string `json:"type"`
	X     int    `json:"x"`
	Y     int    `json:"y"`
	Power uint8  `json:"power"` // currently selected power (for cursor color)
}

// CursorUpdateMsg is broadcast to all OTHER clients with another player's cursor position.
type CursorUpdateMsg struct {
	Type     string `json:"type"`
	PlayerID uint32 `json:"playerID"`
	X        int    `json:"x"`
	Y        int    `json:"y"`
	Power    uint8  `json:"power"`
}

// PlayerJoinMsg is broadcast when a new player connects.
type PlayerJoinMsg struct {
	Type     string `json:"type"`
	PlayerID uint32 `json:"playerID"`
}

// PlayerLeaveMsg is broadcast when a player disconnects.
type PlayerLeaveMsg struct {
	Type     string `json:"type"`
	PlayerID uint32 `json:"playerID"`
}

// ---- Social / Chat messages ----

// ChatInboundMsg is sent by a client to broadcast a text message.
type ChatInboundMsg struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

// ChatBroadcastMsg is broadcast to all clients in the same world.
type ChatBroadcastMsg struct {
	Type     string `json:"type"`
	PlayerID uint32 `json:"playerID"`
	Nickname string `json:"nickname"`
	Text     string `json:"text"`
	X        int    `json:"x"`
	Y        int    `json:"y"`
}

// PingLocationInboundMsg is sent when a player pings a world location.
type PingLocationInboundMsg struct {
	Type string `json:"type"`
	X    int    `json:"x"`
	Y    int    `json:"y"`
}

// PingLocationBroadcastMsg is broadcast to all clients showing a location ping.
type PingLocationBroadcastMsg struct {
	Type     string `json:"type"`
	PlayerID uint32 `json:"playerID"`
	X        int    `json:"x"`
	Y        int    `json:"y"`
}

// EmoteInboundMsg is sent when a player emotes.
type EmoteInboundMsg struct {
	Type  string `json:"type"`
	Emote string `json:"emote"`
}

// EmoteBroadcastMsg is broadcast to all clients showing an emote.
type EmoteBroadcastMsg struct {
	Type     string `json:"type"`
	PlayerID uint32 `json:"playerID"`
	Emote    string `json:"emote"`
	X        int    `json:"x"`
	Y        int    `json:"y"`
}

// ComboBroadcastMsg is broadcast when 2+ players apply different powers near each other.
type ComboBroadcastMsg struct {
	Type      string   `json:"type"`
	PlayerIDs []uint32 `json:"playerIDs"`
	Powers    []uint8  `json:"powers"`
	X         int      `json:"x"`
	Y         int      `json:"y"`
}

// ---- Goal messages ----

// GoalUpdateMsg is broadcast to all clients when the cooperative goal state changes.
type GoalUpdateMsg struct {
	Type      string `json:"type"`
	GoalText  string `json:"goalText"`
	Progress  int    `json:"progress"`
	Target    int    `json:"target"`
	Completed bool   `json:"completed"`
}
