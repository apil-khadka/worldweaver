package tests

import (
	"net/http"
	"testing"

	"github.com/worldweaver/worldweaver/internal/game"
	"github.com/worldweaver/worldweaver/internal/metrics"
	"github.com/worldweaver/worldweaver/internal/network"
	"github.com/worldweaver/worldweaver/internal/simulation"
	"github.com/worldweaver/worldweaver/internal/world"
)

// The collaboration endpoint.
//
// The ledger itself is covered in contribution_test.go. What matters here is that
// the hub hands out the right view of it: a shared milestone rather than a
// ranking, and nothing at all about a world the caller cannot see.

func collabHub() *network.Hub {
	w := world.New(64, 32, 3)
	m := metrics.New()
	return network.NewHub(w, simulation.NewEngine(w, m), m, game.NewScoreboard(), "genesis",
		game.NewAuthManager(), game.NewWorldManager(3, 64, 32))
}

// TestCollaborationSnapshotSumsTheWholeWorld: the milestone is the population's,
// so it has to add up every contributor rather than report the leader.
func TestCollaborationSnapshotSumsTheWholeWorld(t *testing.T) {
	hub := collabHub()

	// Far apart, so no assists muddy the arithmetic.
	hub.Contributions.RecordPlacement("genesis", 1, 0, 0, 0, 100)
	hub.Contributions.RecordPlacement("genesis", 2, 0, 2000, 0, 300)

	snap := hub.CollaborationFor("genesis")
	if len(snap.Contributors) != 2 {
		t.Fatalf("contributors %d, want 2", len(snap.Contributors))
	}

	var sum float64
	for _, c := range snap.Contributors {
		sum += c.Score
	}
	if snap.Milestone.Progress != sum {
		t.Errorf("milestone progress %v, want the sum of contributions %v", snap.Milestone.Progress, sum)
	}
	if snap.Milestone.Target != network.SharedMilestoneTarget {
		t.Errorf("milestone target %v, want %v", snap.Milestone.Target, network.SharedMilestoneTarget)
	}
	if snap.Milestone.Progress > snap.Milestone.Target && !snap.Milestone.Complete {
		t.Error("progress past the target but the milestone is not complete")
	}
	// The bigger builder leads the roster; that ordering is what a client renders.
	if snap.Contributors[0].PlayerID != 2 {
		t.Errorf("roster leader %d, want 2", snap.Contributors[0].PlayerID)
	}
}

// TestCollaborationHiddenWorldIsNotReadable: a private world's roster would leak
// both that the world exists and who is inside it.
func TestCollaborationHiddenWorldIsNotReadable(t *testing.T) {
	addr := apiServer(t)
	ownerTok := httpLogin(t, addr, "owner")
	strangerTok := httpLogin(t, addr, "stranger")
	id := createWorld(t, addr, ownerTok, "clubhouse", "private")

	url := "http://" + addr + "/api/contributions?world=" + id

	if status := requestJSON(t, http.MethodGet, url, strangerTok, nil, nil); status != http.StatusNotFound {
		t.Errorf("stranger read a private world's contributions: status %d, want 404", status)
	}
	if status := requestJSON(t, http.MethodGet, url, "", nil, nil); status != http.StatusNotFound {
		t.Errorf("anonymous caller read a private world's contributions: status %d, want 404", status)
	}
	if status := requestJSON(t, http.MethodGet, url, ownerTok, nil, nil); status != http.StatusOK {
		t.Errorf("owner cannot read their own world's contributions: status %d", status)
	}
}

// TestCollaborationPublicWorldIsReadableAnonymously so the landing page can show
// who is building a world without asking a visitor to sign in.
func TestCollaborationPublicWorldIsReadableAnonymously(t *testing.T) {
	addr := apiServer(t)
	var snap struct {
		WorldID   string `json:"worldId"`
		Milestone struct {
			Target float64 `json:"target"`
		} `json:"milestone"`
	}

	status := requestJSON(t, http.MethodGet, "http://"+addr+"/api/contributions", "", nil, &snap)
	if status != http.StatusOK {
		t.Fatalf("status %d, want 200", status)
	}
	if snap.WorldID != "genesis" {
		t.Errorf("worldId %q, want the default world when none is named", snap.WorldID)
	}
	if snap.Milestone.Target != network.SharedMilestoneTarget {
		t.Errorf("milestone target %v, want %v", snap.Milestone.Target, network.SharedMilestoneTarget)
	}
}

// TestForgetWorldStateClearsOwnershipAndRoster: a recreated world must not
// inherit the deleted one's contributors, and neither structure may outlive it.
func TestForgetWorldStateClearsOwnershipAndRoster(t *testing.T) {
	hub := collabHub()
	hub.Access.Register("doomed", "owner-key", game.VisibilityPrivate)
	hub.Contributions.RecordPlacement("doomed", 1, 0, 10, 10, 50)

	hub.ForgetWorldState("doomed")

	if hub.Access.IsOwner("doomed", "owner-key") {
		t.Error("ownership survived the world it described")
	}
	if hub.Contributions.ContributorCount("doomed") != 0 {
		t.Error("contribution roster survived the world it described")
	}
}
