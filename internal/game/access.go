package game

import (
	"crypto/rand"
	"errors"
	"math/big"
	"strings"
	"sync"
	"time"
)

// World access control.
//
// A world is public, unlisted or private. Ownership is recorded against the
// owner's public key rather than their display name: names are neither unique nor
// stable, so comparing them — as the previous implementation did — meant anyone
// could claim another player's world by choosing their name.

// Visibility controls who can see and join a world.
type Visibility string

const (
	// VisibilityPublic worlds appear in the public list and anyone signed in may join.
	VisibilityPublic Visibility = "public"

	// VisibilityUnlisted worlds are hidden from the list but joinable with the code.
	VisibilityUnlisted Visibility = "unlisted"

	// VisibilityPrivate worlds are hidden and joinable by members only.
	VisibilityPrivate Visibility = "private"
)

// ParseVisibility resolves a client-supplied value, defaulting to public.
func ParseVisibility(s string) Visibility {
	switch Visibility(strings.ToLower(strings.TrimSpace(s))) {
	case VisibilityUnlisted:
		return VisibilityUnlisted
	case VisibilityPrivate:
		return VisibilityPrivate
	default:
		return VisibilityPublic
	}
}

var (
	ErrNotOwner        = errors.New("only the owner can do that")
	ErrNotInvited      = errors.New("this world is private")
	ErrInviteInvalid   = errors.New("invite code is not valid")
	ErrInviteExpired   = errors.New("invite code has expired")
	ErrInviteExhausted = errors.New("invite code has already been used the maximum number of times")
)

// Invite is a redeemable code granting membership of a world.
type Invite struct {
	Code    string `json:"code"`
	WorldID string `json:"worldId"`

	// MaxUses of zero means unlimited.
	MaxUses int `json:"maxUses"`
	Uses    int `json:"uses"`

	// ExpiresAt zero means no expiry.
	ExpiresAt time.Time `json:"expiresAt,omitempty"`

	CreatedAt time.Time `json:"createdAt"`
	CreatedBy string    `json:"-"` // owner key ID; not exposed to clients
}

// Valid reports whether the invite may still be redeemed.
func (i *Invite) Valid(now time.Time) error {
	if !i.ExpiresAt.IsZero() && now.After(i.ExpiresAt) {
		return ErrInviteExpired
	}
	if i.MaxUses > 0 && i.Uses >= i.MaxUses {
		return ErrInviteExhausted
	}
	return nil
}

// Access holds the ownership and membership of one world.
type Access struct {
	// OwnerKey is the owner's public key ID. Empty for the built-in world, which
	// nobody owns.
	OwnerKey string

	Visibility Visibility

	// members is the set of key IDs allowed into a private world. The owner is
	// always permitted and is not required to appear here.
	members map[string]time.Time
}

// AccessRegistry tracks access control for every world and all live invites.
type AccessRegistry struct {
	mu      sync.RWMutex
	worlds  map[string]*Access // worldID -> access
	invites map[string]*Invite // code -> invite
}

// NewAccessRegistry creates an empty registry.
func NewAccessRegistry() *AccessRegistry {
	return &AccessRegistry{
		worlds:  make(map[string]*Access),
		invites: make(map[string]*Invite),
	}
}

// Register records access settings for a newly created world.
func (r *AccessRegistry) Register(worldID, ownerKey string, vis Visibility) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.worlds[worldID] = &Access{
		OwnerKey:   ownerKey,
		Visibility: vis,
		members:    make(map[string]time.Time),
	}
}

// Forget drops a world's access record and any invites pointing at it.
func (r *AccessRegistry) Forget(worldID string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.worlds, worldID)
	for code, inv := range r.invites {
		if inv.WorldID == worldID {
			delete(r.invites, code)
		}
	}
}

// Get returns a copy of a world's access settings.
func (r *AccessRegistry) Get(worldID string) (ownerKey string, vis Visibility, ok bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	a, found := r.worlds[worldID]
	if !found {
		return "", VisibilityPublic, false
	}
	return a.OwnerKey, a.Visibility, true
}

// IsOwner reports whether a key owns a world.
func (r *AccessRegistry) IsOwner(worldID, keyID string) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	a, ok := r.worlds[worldID]
	// A world with no recorded owner (the built-in one) has no owner to match.
	return ok && a.OwnerKey != "" && a.OwnerKey == keyID
}

// CanJoin reports whether a key may connect to a world.
//
// A world with no access record is treated as public, which keeps the built-in
// world reachable without special-casing it at every call site.
func (r *AccessRegistry) CanJoin(worldID, keyID string) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()

	a, ok := r.worlds[worldID]
	if !ok {
		return true
	}
	switch a.Visibility {
	case VisibilityPublic, VisibilityUnlisted:
		// Unlisted worlds are hidden, not sealed: knowing the ID is enough, which
		// is what makes a shared link work.
		return true
	case VisibilityPrivate:
		if a.OwnerKey == keyID {
			return true
		}
		_, member := a.members[keyID]
		return member
	}
	return false
}

// Visible reports whether a world should appear in a listing for this key.
func (r *AccessRegistry) Visible(worldID, keyID string) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()

	a, ok := r.worlds[worldID]
	if !ok {
		return true // built-in world
	}
	if a.Visibility == VisibilityPublic {
		return true
	}
	// Owners and members always see their own worlds, however they are hidden.
	if a.OwnerKey == keyID {
		return true
	}
	_, member := a.members[keyID]
	return member
}

// SetVisibility changes a world's visibility. Owner only.
func (r *AccessRegistry) SetVisibility(worldID, keyID string, vis Visibility) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	a, ok := r.worlds[worldID]
	if !ok {
		return errors.New("unknown world")
	}
	if a.OwnerKey != keyID {
		return ErrNotOwner
	}
	a.Visibility = vis
	return nil
}

// AddMember grants a key membership of a world.
func (r *AccessRegistry) AddMember(worldID, keyID string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if a, ok := r.worlds[worldID]; ok {
		a.members[keyID] = time.Now()
	}
}

// RemoveMember revokes membership. Owner only; an owner cannot remove themselves.
func (r *AccessRegistry) RemoveMember(worldID, ownerKey, memberKey string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	a, ok := r.worlds[worldID]
	if !ok {
		return errors.New("unknown world")
	}
	if a.OwnerKey != ownerKey {
		return ErrNotOwner
	}
	if memberKey == ownerKey {
		return errors.New("the owner cannot be removed")
	}
	delete(a.members, memberKey)
	return nil
}

// Members lists the member key IDs of a world. Owner only.
func (r *AccessRegistry) Members(worldID, ownerKey string) ([]string, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	a, ok := r.worlds[worldID]
	if !ok {
		return nil, errors.New("unknown world")
	}
	if a.OwnerKey != ownerKey {
		return nil, ErrNotOwner
	}
	out := make([]string, 0, len(a.members))
	for k := range a.members {
		out = append(out, k)
	}
	return out, nil
}

// ── Invites ─────────────────────────────────────────────────────────────────

// CreateInvite issues a code for a world. Owner only.
func (r *AccessRegistry) CreateInvite(worldID, ownerKey string, maxUses int, ttl time.Duration) (*Invite, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	a, ok := r.worlds[worldID]
	if !ok {
		return nil, errors.New("unknown world")
	}
	if a.OwnerKey != ownerKey {
		return nil, ErrNotOwner
	}
	if maxUses < 0 {
		maxUses = 0
	}

	inv := &Invite{
		Code:      newInviteCode(),
		WorldID:   worldID,
		MaxUses:   maxUses,
		CreatedAt: time.Now(),
		CreatedBy: ownerKey,
	}
	if ttl > 0 {
		inv.ExpiresAt = time.Now().Add(ttl)
	}
	r.invites[inv.Code] = inv

	out := *inv
	return &out, nil
}

// RedeemInvite validates a code and adds the redeemer as a member.
//
// Returns the world the code belongs to so the caller can send the player
// straight into it.
func (r *AccessRegistry) RedeemInvite(code, keyID string) (string, error) {
	code = NormaliseInviteCode(code)

	r.mu.Lock()
	defer r.mu.Unlock()

	inv, ok := r.invites[code]
	if !ok {
		return "", ErrInviteInvalid
	}
	if err := inv.Valid(time.Now()); err != nil {
		return "", err
	}
	a, ok := r.worlds[inv.WorldID]
	if !ok {
		// The world went away but the code outlived it.
		delete(r.invites, code)
		return "", ErrInviteInvalid
	}

	// Redeeming as the owner is a no-op rather than an error, so following your
	// own link does not look broken.
	if a.OwnerKey != keyID {
		a.members[keyID] = time.Now()
	}
	inv.Uses++

	return inv.WorldID, nil
}

// ListInvites returns the live invites for a world. Owner only.
func (r *AccessRegistry) ListInvites(worldID, ownerKey string) ([]Invite, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	a, ok := r.worlds[worldID]
	if !ok {
		return nil, errors.New("unknown world")
	}
	if a.OwnerKey != ownerKey {
		return nil, ErrNotOwner
	}

	now := time.Now()
	out := make([]Invite, 0)
	for _, inv := range r.invites {
		if inv.WorldID != worldID {
			continue
		}
		if inv.Valid(now) != nil {
			continue
		}
		out = append(out, *inv)
	}
	return out, nil
}

// RevokeInvite deletes a code. Owner only.
func (r *AccessRegistry) RevokeInvite(code, ownerKey string) error {
	code = NormaliseInviteCode(code)

	r.mu.Lock()
	defer r.mu.Unlock()

	inv, ok := r.invites[code]
	if !ok {
		return ErrInviteInvalid
	}
	a, ok := r.worlds[inv.WorldID]
	if !ok || a.OwnerKey != ownerKey {
		return ErrNotOwner
	}
	delete(r.invites, code)
	return nil
}

// PruneInvites drops expired and exhausted codes, returning how many went.
func (r *AccessRegistry) PruneInvites() int {
	now := time.Now()

	r.mu.Lock()
	defer r.mu.Unlock()

	removed := 0
	for code, inv := range r.invites {
		if inv.Valid(now) != nil {
			delete(r.invites, code)
			removed++
		}
	}
	return removed
}

// inviteAlphabet omits characters that are easily confused when a code is read
// aloud or retyped: I, L, O, 0, 1.
const inviteAlphabet = "ABCDEFGHJKMNPQRSTUVWXYZ23456789"

const inviteCodeLen = 8

// newInviteCode returns a short, human-transcribable random code.
func newInviteCode() string {
	out := make([]byte, inviteCodeLen)
	max := big.NewInt(int64(len(inviteAlphabet)))
	for i := range out {
		n, err := rand.Int(rand.Reader, max)
		if err != nil {
			panic("crypto/rand unavailable while generating an invite code")
		}
		out[i] = inviteAlphabet[n.Int64()]
	}
	return string(out)
}

// NormaliseInviteCode makes codes forgiving to type: case-insensitive, and
// tolerant of the spaces and dashes people add when copying them.
func NormaliseInviteCode(code string) string {
	var b strings.Builder
	for _, r := range strings.ToUpper(code) {
		if r == ' ' || r == '-' || r == '_' {
			continue
		}
		b.WriteRune(r)
	}
	return b.String()
}
