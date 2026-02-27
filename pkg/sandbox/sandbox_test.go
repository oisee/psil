package sandbox

import (
	"io"
	"math/rand"
	"testing"

	"github.com/psilLang/psil/pkg/micro"
)

func testRng() *rand.Rand {
	return rand.New(rand.NewSource(42))
}

func TestWorldCreation(t *testing.T) {
	w := NewWorld(32, testRng())
	if w.Size != 32 {
		t.Fatalf("expected size 32, got %d", w.Size)
	}
	if len(w.Grid) != 1024 {
		t.Fatalf("expected 1024 tiles, got %d", len(w.Grid))
	}
}

func TestTilePackUnpack(t *testing.T) {
	tile := MakeTile(TileFood, 5)
	if tile.Type() != TileFood {
		t.Errorf("type: got %d want %d", tile.Type(), TileFood)
	}
	if tile.Occupant() != 5 {
		t.Errorf("occupant: got %d want 5", tile.Occupant())
	}
}

func TestNPCSpawn(t *testing.T) {
	w := NewWorld(16, testRng())
	npc := NewNPC([]byte{micro.OpHalt})
	w.Spawn(npc)
	if len(w.NPCs) != 1 {
		t.Fatalf("expected 1 NPC, got %d", len(w.NPCs))
	}
	if npc.ID == 0 {
		t.Fatal("NPC should have non-zero ID")
	}
	tile := w.TileAt(npc.X, npc.Y)
	if tile.Occupant() != npc.ID {
		t.Errorf("tile occupant %d != NPC ID %d", tile.Occupant(), npc.ID)
	}
}

func TestSingleTick(t *testing.T) {
	w := NewWorld(16, testRng())
	sched := NewScheduler(w, 200, io.Discard)

	// Spawn NPC with simple genome: push 1, write to Ring1 move slot, halt
	// This writes move=1 (North) to Ring1
	genome := []byte{
		micro.SmallNumOp(1), // push 1
		micro.OpRing1W, 0,   // r1![0] = 1 (move North)
		micro.OpHalt,
	}
	npc := NewNPC(genome)
	npc.X = 8
	npc.Y = 8
	w.Spawn(npc)

	startY := npc.Y
	sched.Tick()

	if npc.Y != startY-1 {
		t.Errorf("NPC should have moved North: Y was %d, now %d (expected %d)", startY, npc.Y, startY-1)
	}
}

func TestFoodEating(t *testing.T) {
	w := NewWorld(16, testRng())
	sched := NewScheduler(w, 200, io.Discard)

	// Genome: write eat action (1) to Ring1 action slot
	genome := []byte{
		micro.SmallNumOp(1), // push 1
		micro.OpRing1W, 1,   // r1![1] = 1 (action=eat)
		micro.OpHalt,
	}
	npc := NewNPC(genome)
	npc.X = 5
	npc.Y = 5
	w.Spawn(npc)

	// Place food adjacent
	w.SetTile(5, 4, MakeTile(TileFood, 0))

	startEnergy := npc.Energy
	sched.Tick()

	if npc.FoodEaten != 1 {
		t.Errorf("NPC should have eaten food, food_eaten=%d", npc.FoodEaten)
	}
	// Energy: +30 (eat) -1 (decay) = +29 net
	expectedEnergy := startEnergy + 29
	if npc.Energy != expectedEnergy {
		t.Errorf("energy: got %d want %d", npc.Energy, expectedEnergy)
	}
}

func TestGACrossoverValid(t *testing.T) {
	ga := NewGA(testRng())

	a := []byte{micro.SmallNumOp(5), micro.OpDup, micro.OpAdd, micro.OpHalt}
	b := []byte{micro.SmallNumOp(10), micro.OpSub, micro.OpPrint, micro.OpHalt}

	child := ga.crossover(a, b)
	if len(child) < MinGenome {
		// Padded to min
		if len(child) < MinGenome {
			t.Errorf("child genome too small: %d < %d", len(child), MinGenome)
		}
	}
	if len(child) > MaxGenome {
		t.Errorf("child genome too large: %d > %d", len(child), MaxGenome)
	}
}

func TestGAMutationPreservesSize(t *testing.T) {
	ga := NewGA(testRng())

	genome := ga.RandomGenome(32)
	for i := 0; i < 100; i++ {
		mutated := ga.mutate(genome)
		if len(mutated) < MinGenome-1 || len(mutated) > MaxGenome+1 {
			t.Errorf("mutation produced genome of size %d (original %d)", len(mutated), len(genome))
		}
	}
}

func Test100TickSimulation(t *testing.T) {
	rng := testRng()
	w := NewWorld(16, rng)
	ga := NewGA(rng)
	sched := NewScheduler(w, 200, io.Discard)

	// Spawn 10 NPCs
	for i := 0; i < 10; i++ {
		genome := ga.RandomGenome(24)
		npc := NewNPC(genome)
		npc.X = rng.Intn(16)
		npc.Y = rng.Intn(16)
		w.Spawn(npc)
	}

	// Seed food
	for i := 0; i < 16; i++ {
		x := rng.Intn(16)
		y := rng.Intn(16)
		if w.TileAt(x, y).Type() == TileEmpty && w.TileAt(x, y).Occupant() == 0 {
			w.SetTile(x, y, MakeTile(TileFood, 0))
		}
	}

	// Run 100 ticks
	for tick := 0; tick < 100; tick++ {
		sched.Tick()

		if tick > 0 && tick%50 == 0 {
			w.NPCs = ga.Evolve(w.NPCs)
		}
	}

	// Check simulation didn't crash - population may have shrunk but shouldn't be zero
	// (with food respawning and short duration)
	if w.Tick != 100 {
		t.Errorf("expected 100 ticks, got %d", w.Tick)
	}
	t.Logf("After 100 ticks: alive=%d food=%d", len(w.NPCs), w.FoodCount())
}

func TestRandomGenomeSize(t *testing.T) {
	ga := NewGA(testRng())

	g := ga.RandomGenome(24)
	if len(g) != 24 {
		t.Errorf("expected size 24, got %d", len(g))
	}
	// Last byte should be halt
	if g[len(g)-1] != micro.OpHalt {
		t.Errorf("genome should end with halt, got %02x", g[len(g)-1])
	}
}
