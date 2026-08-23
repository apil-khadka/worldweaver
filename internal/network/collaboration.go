package network

import (
	"log"

	"github.com/worldweaver/worldweaver/internal/game"
)

// Collaboration plumbing.
//
// The hub already knew how to rank players against each other. This file is the
// other half: noticing when two of them are working the same ground and saying so
// out loud. Nothing here changes what the simulation does — it observes actions
// that have already been accepted and turns them into credit and a message.

// worldIDOf returns the world a client is playing in.
//
// Client.WorldID was recorded at connection time and then never read: every
// per-world figure was filed under the hub's single hard-coded world name, so two
// players in different worlds shared one scoreboard and one contribution roster.
func (h *Hub) worldIDOf(c *Client) string {
	if c.WorldID != "" {
		return c.WorldID
	}
	return h.WorldName
}

// recordContribution books an accepted action in the ledger and announces any
// assist it earned.
func (h *Hub) recordContribution(c *Client, req *game.PowerRequest, cells int) {
	worldID := h.worldIDOf(c)

	// A force is categorised by the power; a direct edit by the material it lays
	// down. Using the power id for both would file every painted cell under
	// whichever force happened to be selected at the time.
	category := game.CategoryForPower(req.Power)
	if req.Tool != game.ToolForce {
		category = game.CategoryForMaterial(req.Material)
	}

	result := h.Contributions.Record(worldID, game.ContributionAction{
		PlayerID: c.Player.ID,
		Category: category,
		X:        req.X,
		Y:        req.Y,
		Cells:    cells,
	})

	if len(result.Assisted) == 0 {
		return
	}
	h.broadcastAssist(worldID, c, result, req.X, req.Y)
}

// broadcastAssist tells the world who just worked together.
//
// Sent to everyone in the world rather than only the pair: seeing other people
// collaborate is what makes a newcomer try it, and the message is small.
func (h *Hub) broadcastAssist(worldID string, actor *Client, result game.ContributionResult, x, y int) {
	names := h.nicknamesFor(worldID, result.Assisted)

	msg := mustMarshal(AssistMsg{
		Type:       MsgAssist,
		PlayerID:   actor.Player.ID,
		Nickname:   actor.Player.Nickname,
		PartnerIDs: result.Assisted,
		Partners:   names,
		Multiplier: result.Multiplier,
		Awarded:    result.Awarded,
		TotalScore: result.TotalScore,
		Category:   result.Category.String(),
		X:          x,
		Y:          y,
	})

	h.mu.RLock()
	defer h.mu.RUnlock()
	for client := range h.clients {
		if h.worldIDOf(client) == worldID {
			client.sendRaw(msg)
		}
	}
}

// nicknamesFor resolves player IDs to display names for the clients currently
// connected to a world. A player who has since disconnected resolves to an empty
// string, which the client renders as an anonymous partner rather than dropping
// the message.
func (h *Hub) nicknamesFor(worldID string, ids []uint32) []string {
	byID := make(map[uint32]string, len(ids))

	h.mu.RLock()
	for client := range h.clients {
		if h.worldIDOf(client) != worldID {
			continue
		}
		byID[client.Player.ID] = client.Player.Nickname
	}
	h.mu.RUnlock()

	out := make([]string, len(ids))
	for i, id := range ids {
		out[i] = byID[id]
	}
	return out
}

// CollaborationSnapshot is the shape returned by GET /api/contributions.
type CollaborationSnapshot struct {
	game.WorldContribution

	// Milestone is the world's shared target: progress is the sum of everybody's
	// contribution, so it is something the population reaches together rather
	// than a race between them.
	Milestone game.MilestoneProgress `json:"milestone"`

	// MostCollaborativeName saves the client a second lookup for the one figure
	// it is most likely to display.
	MostCollaborativeName string `json:"mostCollaborativeName,omitempty"`
}

// SharedMilestoneTarget is the contribution score a world's players are working
// towards together.
//
// Sized so a full world reaches it in a session rather than in a minute: at 8
// players a milestone is meant to be an afternoon's shared project.
const SharedMilestoneTarget = 25000

// CollaborationFor assembles the collaboration view of one world.
func (h *Hub) CollaborationFor(worldID string) CollaborationSnapshot {
	snap := CollaborationSnapshot{
		WorldContribution: h.Contributions.World(worldID),
		Milestone:         h.Contributions.Milestone(worldID, SharedMilestoneTarget),
	}

	if snap.MostCollaborative != 0 {
		names := h.nicknamesFor(worldID, []uint32{snap.MostCollaborative})
		snap.MostCollaborativeName = names[0]
	}
	return snap
}

// ForgetWorldState drops everything the hub remembers about a deleted world.
//
// Ownership, invites and contributions all outlived the worlds they described
// before this existed, which both leaked memory and let a recreated world inherit
// the previous one's roster.
func (h *Hub) ForgetWorldState(worldID string) {
	h.Access.Forget(worldID)
	h.Contributions.ForgetWorld(worldID)
	log.Printf("hub: dropped state for deleted world %q", worldID)
}
