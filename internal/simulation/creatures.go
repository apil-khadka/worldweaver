package simulation

// Shared movement offsets used by creature and exotic-material behaviour.
//
// Per-species creature logic lives in species.go, driven by the Traits table.
// Behaviour used to be one near-duplicate function per animal, with energy
// stored in the Temperature array; both are described there.

// cardinalDirs are the four orthogonal offsets.
var cardinalDirs = [4][2]int{{0, -1}, {0, 1}, {-1, 0}, {1, 0}}

// eightDirs are all eight surrounding offsets, used for adjacency checks.
var eightDirs = [8][2]int{
	{-1, -1}, {0, -1}, {1, -1},
	{-1, 0}, {1, 0},
	{-1, 1}, {0, 1}, {1, 1},
}
