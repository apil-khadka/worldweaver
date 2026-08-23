package tests

import (
	"testing"
	"time"

	"github.com/worldweaver/worldweaver/internal/game"
)

// World access control.
//
// These tests exist because the previous scheme had none worth the name: a world
// recorded its creator's display name, and the delete check compared that string
// against whatever name the requester happened to log in under.

// owner returns a registry with one world owned by ownerKey.
func worldOwnedBy(vis game.Visibility) (*game.AccessRegistry, string, string) {
	r := game.NewAccessRegistry()
	ownerKey, strangerKey := "owner-key", "stranger-key"
	r.Register("w1", ownerKey, vis)
	return r, ownerKey, strangerKey
}

// TestOnlyOwnerIsOwner is the check that ownership rests on the key.
func TestOnlyOwnerIsOwner(t *testing.T) {
	r, ownerKey, strangerKey := worldOwnedBy(game.VisibilityPublic)

	if !r.IsOwner("w1", ownerKey) {
		t.Error("the creating key is not recognised as owner")
	}
	if r.IsOwner("w1", strangerKey) {
		t.Error("a different key was accepted as owner")
	}
	if r.IsOwner("w1", "") {
		t.Error("an empty key was accepted as owner; unauthenticated callers would own everything")
	}
}

// TestUnownedWorldHasNoOwner: the built-in world belongs to nobody, and nobody
// may claim owner powers over it.
func TestUnownedWorldHasNoOwner(t *testing.T) {
	r := game.NewAccessRegistry()
	r.Register("genesis", "", game.VisibilityPublic)

	if r.IsOwner("genesis", "") {
		t.Error("an empty key matched the empty owner of the built-in world")
	}
	if !r.CanJoin("genesis", "anyone") {
		t.Error("the built-in world must stay open to everyone")
	}
}

// TestPrivateWorldHiddenFromList is the point of a private world: a stranger
// must not even learn that it exists.
func TestPrivateWorldHiddenFromList(t *testing.T) {
	r, ownerKey, strangerKey := worldOwnedBy(game.VisibilityPrivate)

	if !r.Visible("w1", ownerKey) {
		t.Error("owner cannot see their own private world")
	}
	if r.Visible("w1", strangerKey) {
		t.Error("private world is listed to a stranger")
	}
	if r.CanJoin("w1", strangerKey) {
		t.Error("stranger can join a private world")
	}
}

// TestUnlistedIsHiddenButJoinable draws the line between the two hidden modes:
// unlisted keeps a world off the lobby while a shared link still works.
func TestUnlistedIsHiddenButJoinable(t *testing.T) {
	r, _, strangerKey := worldOwnedBy(game.VisibilityUnlisted)

	if r.Visible("w1", strangerKey) {
		t.Error("unlisted world appears in the listing")
	}
	if !r.CanJoin("w1", strangerKey) {
		t.Error("unlisted world refuses someone who knows its ID; a shared link would be useless")
	}
}

// TestUnknownWorldTreatedAsPublic keeps worlds created before the registry
// existed reachable rather than silently sealed.
func TestUnknownWorldTreatedAsPublic(t *testing.T) {
	r := game.NewAccessRegistry()
	if !r.CanJoin("never-registered", "someone") {
		t.Error("a world with no access record should behave as public")
	}
}

// TestInviteGrantsMembership is the collaboration path: a code turns a stranger
// into a member of a private world.
func TestInviteGrantsMembership(t *testing.T) {
	r, ownerKey, strangerKey := worldOwnedBy(game.VisibilityPrivate)

	inv, err := r.CreateInvite("w1", ownerKey, 0, 0)
	if err != nil {
		t.Fatalf("create invite: %v", err)
	}

	worldID, err := r.RedeemInvite(inv.Code, strangerKey)
	if err != nil {
		t.Fatalf("redeem: %v", err)
	}
	if worldID != "w1" {
		t.Errorf("redeem returned world %q, want w1", worldID)
	}
	if !r.CanJoin("w1", strangerKey) {
		t.Error("invited player still cannot join")
	}
	if !r.Visible("w1", strangerKey) {
		t.Error("invited player cannot see the world they were invited to")
	}
}

// TestInviteCodeIsCaseAndPunctuationTolerant: codes get retyped by hand and
// pasted with stray dashes.
func TestInviteCodeIsCaseAndPunctuationTolerant(t *testing.T) {
	r, ownerKey, strangerKey := worldOwnedBy(game.VisibilityPrivate)
	inv, _ := r.CreateInvite("w1", ownerKey, 0, 0)

	messy := "  " + inv.Code[:4] + "-" + inv.Code[4:] + " "
	if _, err := r.RedeemInvite(lower(messy), strangerKey); err != nil {
		t.Errorf("a lowercased, dashed code was refused: %v", err)
	}
}

// TestOnlyOwnerCreatesInvites stops a member from inviting the world open.
func TestOnlyOwnerCreatesInvites(t *testing.T) {
	r, ownerKey, strangerKey := worldOwnedBy(game.VisibilityPrivate)

	inv, _ := r.CreateInvite("w1", ownerKey, 0, 0)
	if _, err := r.RedeemInvite(inv.Code, strangerKey); err != nil {
		t.Fatalf("redeem: %v", err)
	}

	// The stranger is now a member, but membership is not authority.
	if _, err := r.CreateInvite("w1", strangerKey, 0, 0); err == nil {
		t.Error("a member was allowed to issue invites")
	}
}

// TestExhaustedInviteRefused: a single-use code is single-use.
func TestExhaustedInviteRefused(t *testing.T) {
	r, ownerKey, _ := worldOwnedBy(game.VisibilityPrivate)
	inv, _ := r.CreateInvite("w1", ownerKey, 1, 0)

	if _, err := r.RedeemInvite(inv.Code, "first"); err != nil {
		t.Fatalf("first redeem: %v", err)
	}
	if _, err := r.RedeemInvite(inv.Code, "second"); err == nil {
		t.Error("a one-use code was redeemed twice")
	}
	if r.CanJoin("w1", "second") {
		t.Error("the second redeemer was admitted anyway")
	}
}

// TestExpiredInviteRefused.
func TestExpiredInviteRefused(t *testing.T) {
	r, ownerKey, strangerKey := worldOwnedBy(game.VisibilityPrivate)

	// A negative TTL is not achievable through the API, so expire by hand: ask
	// for a very short life and wait it out.
	inv, err := r.CreateInvite("w1", ownerKey, 0, 20*time.Millisecond)
	if err != nil {
		t.Fatalf("create invite: %v", err)
	}
	time.Sleep(40 * time.Millisecond)

	if _, err := r.RedeemInvite(inv.Code, strangerKey); err == nil {
		t.Error("an expired code was accepted")
	}
}

// TestRevokedInviteRefused.
func TestRevokedInviteRefused(t *testing.T) {
	r, ownerKey, strangerKey := worldOwnedBy(game.VisibilityPrivate)
	inv, _ := r.CreateInvite("w1", ownerKey, 0, 0)

	if err := r.RevokeInvite(inv.Code, strangerKey); err == nil {
		t.Error("a stranger revoked someone else's invite")
	}
	if err := r.RevokeInvite(inv.Code, ownerKey); err != nil {
		t.Fatalf("owner revoke: %v", err)
	}
	if _, err := r.RedeemInvite(inv.Code, strangerKey); err == nil {
		t.Error("a revoked code still works")
	}
}

// TestInvitesDieWithTheirWorld: a code must not outlive the world it opens.
func TestInvitesDieWithTheirWorld(t *testing.T) {
	r, ownerKey, strangerKey := worldOwnedBy(game.VisibilityPrivate)
	inv, _ := r.CreateInvite("w1", ownerKey, 0, 0)

	r.Forget("w1")

	if _, err := r.RedeemInvite(inv.Code, strangerKey); err == nil {
		t.Error("a code for a deleted world was redeemed")
	}
}

// TestListInvitesIsOwnerOnly: the codes to a world are as good as the world.
func TestListInvitesIsOwnerOnly(t *testing.T) {
	r, ownerKey, strangerKey := worldOwnedBy(game.VisibilityPrivate)
	if _, err := r.CreateInvite("w1", ownerKey, 0, 0); err != nil {
		t.Fatalf("create invite: %v", err)
	}

	if _, err := r.ListInvites("w1", strangerKey); err == nil {
		t.Error("a stranger listed another owner's invite codes")
	}

	live, err := r.ListInvites("w1", ownerKey)
	if err != nil {
		t.Fatalf("owner list: %v", err)
	}
	if len(live) != 1 {
		t.Errorf("owner sees %d invites, want 1", len(live))
	}
}

// TestVisibilityChangeIsOwnerOnly stops a member from opening a private world up.
func TestVisibilityChangeIsOwnerOnly(t *testing.T) {
	r, ownerKey, strangerKey := worldOwnedBy(game.VisibilityPrivate)

	if err := r.SetVisibility("w1", strangerKey, game.VisibilityPublic); err == nil {
		t.Error("a stranger made someone else's world public")
	}
	if err := r.SetVisibility("w1", ownerKey, game.VisibilityPublic); err != nil {
		t.Fatalf("owner set visibility: %v", err)
	}
	if !r.Visible("w1", strangerKey) {
		t.Error("world is still hidden after being made public")
	}
}

// TestMembershipRevocation: an owner can put someone out again.
func TestMembershipRevocation(t *testing.T) {
	r, ownerKey, strangerKey := worldOwnedBy(game.VisibilityPrivate)
	r.AddMember("w1", strangerKey)

	if err := r.RemoveMember("w1", strangerKey, strangerKey); err == nil {
		t.Error("a member removed a member")
	}
	if err := r.RemoveMember("w1", ownerKey, ownerKey); err == nil {
		t.Error("the owner removed themselves, orphaning the world")
	}
	if err := r.RemoveMember("w1", ownerKey, strangerKey); err != nil {
		t.Fatalf("owner remove: %v", err)
	}
	if r.CanJoin("w1", strangerKey) {
		t.Error("removed member can still join")
	}
}

// TestPruneInvitesDropsDeadCodes keeps the in-memory store from growing forever.
func TestPruneInvitesDropsDeadCodes(t *testing.T) {
	r, ownerKey, _ := worldOwnedBy(game.VisibilityPrivate)

	dead, _ := r.CreateInvite("w1", ownerKey, 0, 10*time.Millisecond)
	alive, _ := r.CreateInvite("w1", ownerKey, 0, time.Hour)
	time.Sleep(30 * time.Millisecond)

	if n := r.PruneInvites(); n != 1 {
		t.Errorf("pruned %d codes, want 1", n)
	}
	if _, err := r.RedeemInvite(dead.Code, "x"); err == nil {
		t.Error("pruned code still redeemable")
	}
	if _, err := r.RedeemInvite(alive.Code, "y"); err != nil {
		t.Errorf("pruning removed a live code: %v", err)
	}
}

// TestInviteCodesAreDistinct: colliding codes would hand one world's access to
// another world's guests.
func TestInviteCodesAreDistinct(t *testing.T) {
	r, ownerKey, _ := worldOwnedBy(game.VisibilityPrivate)

	seen := make(map[string]bool, 200)
	for range 200 {
		inv, err := r.CreateInvite("w1", ownerKey, 0, time.Hour)
		if err != nil {
			t.Fatalf("create invite: %v", err)
		}
		if seen[inv.Code] {
			t.Fatalf("duplicate invite code %q", inv.Code)
		}
		seen[inv.Code] = true

		// Codes must not carry characters that are ambiguous when read aloud.
		for _, c := range inv.Code {
			switch c {
			case 'I', 'L', 'O', '0', '1':
				t.Fatalf("code %q contains the easily-confused character %q", inv.Code, c)
			}
		}
	}
}

// TestParseVisibilityDefaultsToPublic: a malformed value must not accidentally
// hide a world its creator meant to share.
func TestParseVisibilityDefaultsToPublic(t *testing.T) {
	cases := map[string]game.Visibility{
		"public":    game.VisibilityPublic,
		"PRIVATE":   game.VisibilityPrivate,
		" unlisted": game.VisibilityUnlisted,
		"":          game.VisibilityPublic,
		"nonsense":  game.VisibilityPublic,
	}
	for in, want := range cases {
		if got := game.ParseVisibility(in); got != want {
			t.Errorf("ParseVisibility(%q) = %q, want %q", in, got, want)
		}
	}
}

// TestDeleteWorldByOwnerRefusesGenesis: the founding world is not deletable.
func TestDeleteWorldByOwnerRefusesGenesis(t *testing.T) {
	wm := game.NewWorldManager(1, 128, 64)
	if err := wm.DeleteWorldByOwner("genesis"); err == nil {
		t.Error("the built-in world was deleted")
	}
	if err := wm.DeleteWorldByOwner("no-such-world"); err == nil {
		t.Error("deleting a missing world reported success")
	}
}

// lower is a local strings.ToLower to keep the import list of this file to the
// two packages the assertions actually need.
func lower(s string) string {
	out := []rune(s)
	for i, r := range out {
		if r >= 'A' && r <= 'Z' {
			out[i] = r + 32
		}
	}
	return string(out)
}
