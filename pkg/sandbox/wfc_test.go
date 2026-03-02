package sandbox

import (
	"io"
	"math/rand"
	"testing"
)

func TestWFCGeneration(t *testing.T) {
	rng := rand.New(rand.NewSource(42))
	wfc := NewWFC(16, 16, rng)
	if !wfc.Generate(16 * 16 * 10) {
		t.Fatal("WFC generation failed (contradiction)")
	}
	// All cells should be collapsed
	for i, c := range wfc.Grid {
		if c.CollapsedTo < 0 {
			t.Fatalf("cell %d not collapsed", i)
		}
		if c.CollapsedTo >= int8(NumBiomes) {
			t.Fatalf("cell %d has invalid biome %d", i, c.CollapsedTo)
		}
	}
}

func TestWFCAnchorPlacement(t *testing.T) {
	rng := rand.New(rand.NewSource(99))
	wfc := NewWFC(16, 16, rng)

	anchors := []Anchor{
		{X: 4, Y: 4, Biome: BiomeVillage, Name: "village"},
		{X: 12, Y: 12, Biome: BiomeMountain, Name: "mountain"},
		{X: 8, Y: 8, Biome: BiomeRiver, Name: "river"},
	}

	if !wfc.PlaceAnchors(anchors) {
		t.Fatal("anchor placement failed")
	}
	if !wfc.Generate(16 * 16 * 10) {
		t.Fatal("WFC generation after anchors failed")
	}

	grid := wfc.ToBiomeGrid()
	// Verify anchors survived
	if grid[4*16+4] != BiomeVillage {
		t.Errorf("village anchor lost: got biome %d", grid[4*16+4])
	}
	if grid[12*16+12] != BiomeMountain {
		t.Errorf("mountain anchor lost: got biome %d", grid[12*16+12])
	}
	if grid[8*16+8] != BiomeRiver {
		t.Errorf("river anchor lost: got biome %d", grid[8*16+8])
	}
}

func TestWFCReachability(t *testing.T) {
	// Run multiple seeds — at least one should pass reachability
	passed := false
	for seed := int64(0); seed < 20; seed++ {
		rng := rand.New(rand.NewSource(seed))
		wfc := NewWFC(16, 16, rng)
		anchors := DefaultAnchors(16, 16, rng)
		if !wfc.PlaceAnchors(anchors) {
			continue
		}
		if !wfc.Generate(16 * 16 * 10) {
			continue
		}
		if wfc.CheckReachability() {
			passed = true
			break
		}
	}
	if !passed {
		t.Fatal("no seed produced a reachable map in 20 attempts")
	}
}

func TestWFCConstraints(t *testing.T) {
	// Generate maps and verify river is never adjacent to mountain
	for seed := int64(0); seed < 10; seed++ {
		rng := rand.New(rand.NewSource(seed))
		wfc := NewWFC(16, 16, rng)
		if !wfc.Generate(16 * 16 * 10) {
			continue
		}
		grid := wfc.ToBiomeGrid()
		for y := 0; y < 16; y++ {
			for x := 0; x < 16; x++ {
				b := grid[y*16+x]
				if b == BiomeRiver {
					// Check all 4 neighbors
					for _, d := range [][2]int{{0, -1}, {1, 0}, {0, 1}, {-1, 0}} {
						nx, ny := x+d[0], y+d[1]
						if nx < 0 || nx >= 16 || ny < 0 || ny >= 16 {
							continue
						}
						nb := grid[ny*16+nx]
						if nb == BiomeMountain {
							t.Errorf("seed %d: river at (%d,%d) adjacent to mountain at (%d,%d)",
								seed, x, y, nx, ny)
						}
					}
				}
			}
		}
	}
}

func TestBiomeAwareSpawn(t *testing.T) {
	rng := rand.New(rand.NewSource(42))
	w := NewWorldWithBiomes(32, rng)

	if !w.Biomes {
		t.Fatal("expected biomes to be enabled")
	}
	if len(w.BiomeGrid) != 32*32 {
		t.Fatalf("expected biome grid size 1024, got %d", len(w.BiomeGrid))
	}

	// Count biome types to verify diversity
	counts := make(map[byte]int)
	for _, b := range w.BiomeGrid {
		counts[b]++
	}
	if len(counts) < 2 {
		t.Errorf("expected at least 2 biome types, got %d", len(counts))
	}

	// Run food spawning and verify clearing tiles get food more often
	// Seed food by setting biome grid manually for a controlled test
	clearingFood := 0
	swampFood := 0
	w.FoodRate = 1.0 // always spawn
	w.MaxFood = 10000

	for i := 0; i < 1000; i++ {
		w.RespawnFood()
	}

	for y := 0; y < w.Size; y++ {
		for x := 0; x < w.Size; x++ {
			if w.TileAt(x, y).Type() == TileFood {
				biome := w.BiomeGrid[w.idx(x, y)]
				switch biome {
				case BiomeClearing:
					clearingFood++
				case BiomeSwamp:
					swampFood++
				}
			}
		}
	}

	// Clearing should have more food than swamp (probabilistic but with 1000 spawns very likely)
	if counts[BiomeClearing] > 0 && counts[BiomeSwamp] > 0 {
		clearingDensity := float64(clearingFood) / float64(max(counts[BiomeClearing], 1))
		swampDensity := float64(swampFood) / float64(max(counts[BiomeSwamp], 1))
		if clearingDensity < swampDensity && clearingFood > 10 {
			t.Errorf("clearing food density (%.2f) should be higher than swamp (%.2f)",
				clearingDensity, swampDensity)
		}
	}
}

func TestRiverBlocks(t *testing.T) {
	rng := rand.New(rand.NewSource(42))
	w := NewWorldWithBiomes(32, rng)
	sched := NewScheduler(w, 200, io.Discard)

	// Find a river tile and an adjacent passable tile
	var riverX, riverY int
	var adjX, adjY int
	found := false

	for y := 1; y < w.Size-1 && !found; y++ {
		for x := 1; x < w.Size-1 && !found; x++ {
			if w.BiomeGrid[w.idx(x, y)] == BiomeRiver {
				riverX, riverY = x, y
				// Find adjacent passable tile
				for _, d := range [][2]int{{0, -1}, {1, 0}, {0, 1}, {-1, 0}} {
					nx, ny := x+d[0], y+d[1]
					if w.InBounds(nx, ny) && BiomeTable[w.BiomeGrid[w.idx(nx, ny)]].Passable {
						adjX, adjY = nx, ny
						found = true
						break
					}
				}
			}
		}
	}

	if !found {
		t.Skip("no river tile with adjacent passable tile found")
	}

	// Place NPC on the adjacent tile
	npc := NewNPC([]byte{0xFF}) // halt
	npc.X = adjX
	npc.Y = adjY
	w.Spawn(npc)

	origX, origY := npc.X, npc.Y

	// Determine direction toward river
	var moveDir int
	dx := riverX - adjX
	dy := riverY - adjY
	if dx == 1 {
		moveDir = DirEast
	} else if dx == -1 {
		moveDir = DirWest
	} else if dy == -1 {
		moveDir = DirNorth
	} else if dy == 1 {
		moveDir = DirSouth
	}

	// Run a tick - NPC genome halts immediately, so override Ring1 manually
	sched.sense(npc)
	sched.vm.MemWrite(64+Ring1Move, int16(moveDir))
	sched.act(npc)

	// NPC should not have moved into river
	if npc.X == riverX && npc.Y == riverY {
		t.Errorf("NPC moved into river tile (%d,%d)", riverX, riverY)
	}
	// NPC should still be at original position
	if npc.X != origX || npc.Y != origY {
		t.Errorf("NPC moved to unexpected position (%d,%d), expected (%d,%d)",
			npc.X, npc.Y, origX, origY)
	}
}

func TestBiomeSensor(t *testing.T) {
	rng := rand.New(rand.NewSource(42))
	w := NewWorldWithBiomes(32, rng)
	sched := NewScheduler(w, 200, io.Discard)

	// Find a tile of each biome type and verify sensor reads correctly
	tested := make(map[byte]bool)
	for y := 0; y < w.Size && len(tested) < int(NumBiomes); y++ {
		for x := 0; x < w.Size && len(tested) < int(NumBiomes); x++ {
			biome := w.BiomeGrid[w.idx(x, y)]
			if tested[biome] || !BiomeTable[biome].Passable {
				continue
			}
			if w.OccAt(x, y) != 0 || w.TileAt(x, y).Type() == TileWall {
				continue
			}

			npc := NewNPC([]byte{0xFF}) // halt
			npc.X = x
			npc.Y = y
			w.Spawn(npc)

			sched.sense(npc)
			sensorVal := sched.vm.MemRead(Ring0Biome)

			if byte(sensorVal) != biome {
				t.Errorf("Ring0Biome at (%d,%d): got %d, expected %d (biome=%d)",
					x, y, sensorVal, biome, biome)
			}

			w.Remove(npc.ID)
			tested[biome] = true
		}
	}

	if len(tested) < 2 {
		t.Errorf("only tested %d biome types, expected at least 2", len(tested))
	}
}

func TestGenerateBiomeGrid(t *testing.T) {
	rng := rand.New(rand.NewSource(42))
	grid, ok := GenerateBiomeGrid(16, 16, rng, 10)
	if !ok {
		t.Fatal("GenerateBiomeGrid failed after 10 retries")
	}
	if len(grid) != 256 {
		t.Fatalf("expected 256 cells, got %d", len(grid))
	}

	// Verify all values are valid biome types
	for i, b := range grid {
		if b >= NumBiomes {
			t.Fatalf("invalid biome %d at index %d", b, i)
		}
	}
}

func TestExpandBiomeGrid(t *testing.T) {
	small := []byte{BiomeClearing, BiomeMountain, BiomeForest, BiomeVillage}
	expanded := ExpandBiomeGrid(small, 2, 2, 3)
	// 2x2 → 6x6
	if len(expanded) != 36 {
		t.Fatalf("expected 36 cells, got %d", len(expanded))
	}
	// Top-left 3x3 should all be Clearing
	for y := 0; y < 3; y++ {
		for x := 0; x < 3; x++ {
			if expanded[y*6+x] != BiomeClearing {
				t.Errorf("(%d,%d): expected Clearing, got %d", x, y, expanded[y*6+x])
			}
		}
	}
	// Top-right 3x3 should all be Mountain
	for y := 0; y < 3; y++ {
		for x := 3; x < 6; x++ {
			if expanded[y*6+x] != BiomeMountain {
				t.Errorf("(%d,%d): expected Mountain, got %d", x, y, expanded[y*6+x])
			}
		}
	}
}
