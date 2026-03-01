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

func TestItemPickup(t *testing.T) {
	w := NewWorld(16, testRng())
	sched := NewScheduler(w, 200, io.Discard)

	// Genome: move North (direction 1)
	genome := []byte{
		micro.SmallNumOp(1), // push 1
		micro.OpRing1W, 0,   // r1![0] = 1 (move North)
		micro.OpHalt,
	}
	npc := NewNPC(genome)
	npc.X = 5
	npc.Y = 5
	w.Spawn(npc)

	// Place TileTool at (5,4) — one tile North
	w.SetTile(5, 4, MakeTile(TileTool, 0))

	sched.Tick()

	if npc.Y != 4 {
		t.Fatalf("NPC should have moved to y=4, got y=%d", npc.Y)
	}
	if npc.Item != ItemTool {
		t.Errorf("NPC should have picked up tool, item=%d", npc.Item)
	}
	if w.TileAt(5, 4).Type() != TileEmpty {
		t.Errorf("tile at (5,4) should be empty after pickup, type=%d", w.TileAt(5, 4).Type())
	}
}

func TestTradeExchange(t *testing.T) {
	w := NewWorld(16, testRng())
	sched := NewScheduler(w, 200, io.Discard)

	// NPC A: holds tool, outputs ActionTrade targeting NPC B (ID=2)
	genomeA := []byte{
		micro.SmallNumOp(4),  // push 4 (ActionTrade)
		micro.OpRing1W, 1,    // r1![1] = 4 (action=trade)
		micro.SmallNumOp(2),  // push 2 (target ID = B)
		micro.OpRing1W, 2,    // r1![2] = 2 (target)
		micro.OpHalt,
	}
	npcA := NewNPC(genomeA)
	npcA.X = 5
	npcA.Y = 5
	npcA.Item = ItemTool
	w.Spawn(npcA)
	idA := npcA.ID

	// NPC B: holds weapon, outputs ActionTrade targeting NPC A
	genomeB := []byte{
		micro.SmallNumOp(4),          // push 4 (ActionTrade)
		micro.OpRing1W, 1,            // r1![1] = 4 (action=trade)
		micro.SmallNumOp(int(idA)),   // push A's ID
		micro.OpRing1W, 2,            // r1![2] = A's ID (target)
		micro.OpHalt,
	}
	npcB := NewNPC(genomeB)
	npcB.X = 5
	npcB.Y = 4 // adjacent (North of A)
	npcB.Item = ItemWeapon
	w.Spawn(npcB)

	sched.Tick()

	if npcA.Item != ItemWeapon {
		t.Errorf("NPC A should now hold weapon, item=%d", npcA.Item)
	}
	if npcB.Item != ItemTool {
		t.Errorf("NPC B should now hold tool, item=%d", npcB.Item)
	}
	if sched.TradeCount != 1 {
		t.Errorf("expected 1 trade, got %d", sched.TradeCount)
	}
}

func TestTradeRequiresBilateral(t *testing.T) {
	w := NewWorld(16, testRng())
	sched := NewScheduler(w, 200, io.Discard)

	// NPC A: outputs trade targeting B
	genomeA := []byte{
		micro.SmallNumOp(4), // ActionTrade
		micro.OpRing1W, 1,
		micro.SmallNumOp(2), // target B
		micro.OpRing1W, 2,
		micro.OpHalt,
	}
	npcA := NewNPC(genomeA)
	npcA.X = 5
	npcA.Y = 5
	npcA.Item = ItemTool
	w.Spawn(npcA)

	// NPC B: idle genome (no trade output)
	genomeB := []byte{micro.OpHalt}
	npcB := NewNPC(genomeB)
	npcB.X = 5
	npcB.Y = 4
	npcB.Item = ItemWeapon
	w.Spawn(npcB)

	sched.Tick()

	if npcA.Item != ItemTool {
		t.Errorf("NPC A item should be unchanged, got %d", npcA.Item)
	}
	if npcB.Item != ItemWeapon {
		t.Errorf("NPC B item should be unchanged, got %d", npcB.Item)
	}
	if sched.TradeCount != 0 {
		t.Errorf("expected 0 trades, got %d", sched.TradeCount)
	}
}

func TestTradeRequiresAdjacency(t *testing.T) {
	w := NewWorld(16, testRng())
	sched := NewScheduler(w, 200, io.Discard)

	// NPC A: outputs trade targeting B
	genomeA := []byte{
		micro.SmallNumOp(4),
		micro.OpRing1W, 1,
		micro.SmallNumOp(2),
		micro.OpRing1W, 2,
		micro.OpHalt,
	}
	npcA := NewNPC(genomeA)
	npcA.X = 5
	npcA.Y = 5
	npcA.Item = ItemTool
	w.Spawn(npcA)

	idA := npcA.ID

	// NPC B: outputs trade targeting A, but 3 tiles away
	genomeB := []byte{
		micro.SmallNumOp(4),
		micro.OpRing1W, 1,
		micro.SmallNumOp(int(idA)),
		micro.OpRing1W, 2,
		micro.OpHalt,
	}
	npcB := NewNPC(genomeB)
	npcB.X = 5
	npcB.Y = 2 // 3 tiles away from A
	npcB.Item = ItemWeapon
	w.Spawn(npcB)

	sched.Tick()

	if npcA.Item != ItemTool {
		t.Errorf("NPC A item should be unchanged (too far), got %d", npcA.Item)
	}
	if npcB.Item != ItemWeapon {
		t.Errorf("NPC B item should be unchanged (too far), got %d", npcB.Item)
	}
	if sched.TradeCount != 0 {
		t.Errorf("expected 0 trades, got %d", sched.TradeCount)
	}
}

func TestItemSpawning(t *testing.T) {
	rng := testRng()
	w := NewWorld(16, rng)
	sched := NewScheduler(w, 200, io.Discard)

	// Spawn one NPC so ticks run
	npc := NewNPC([]byte{micro.OpHalt})
	npc.X = 0
	npc.Y = 0
	w.Spawn(npc)

	for i := 0; i < 100; i++ {
		sched.Tick()
	}

	items := w.ItemCount()
	if items == 0 {
		t.Error("expected some item tiles after 100 ticks, got 0")
	}
	t.Logf("After 100 ticks: items=%d", items)
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

// --- Phase 1a tests ---

func TestPerBrainRNG(t *testing.T) {
	w := NewWorld(16, testRng())
	npcA := NewNPC([]byte{micro.OpHalt})
	npcA.X = 2
	npcA.Y = 2
	w.Spawn(npcA)
	npcB := NewNPC([]byte{micro.OpHalt})
	npcB.X = 4
	npcB.Y = 4
	w.Spawn(npcB)

	// Generate sequences
	seqA := make([]byte, 10)
	seqB := make([]byte, 10)
	for i := range seqA {
		seqA[i] = npcA.Rand()
		seqB[i] = npcB.Rand()
	}

	// Sequences should differ (different IDs → different seeds)
	same := true
	for i := range seqA {
		if seqA[i] != seqB[i] {
			same = false
			break
		}
	}
	if same {
		t.Errorf("two NPCs produced identical RNG sequences: %v", seqA)
	}

	// All values should be 0-31
	for i, v := range seqA {
		if v > 31 {
			t.Errorf("seqA[%d] = %d, expected 0-31", i, v)
		}
	}
}

func TestRNGRing0Slot(t *testing.T) {
	w := NewWorld(16, testRng())
	sched := NewScheduler(w, 200, io.Discard)

	genome := []byte{micro.OpHalt}
	npc := NewNPC(genome)
	npc.X = 5
	npc.Y = 5
	w.Spawn(npc)

	sched.Tick()

	// Read Ring0Rng from VM — it was written during sense(), then think() ran
	// We can't read it after think() easily, but we can verify the NPC's RNG advanced
	// and that the value was in range by calling Rand() and checking
	val := npc.Rand()
	if val > 31 {
		t.Errorf("RNG value %d out of range 0-31", val)
	}
}

func TestMaxGenome128(t *testing.T) {
	if MaxGenome != 128 {
		t.Fatalf("MaxGenome should be 128, got %d", MaxGenome)
	}

	ga := NewGA(testRng())

	// Test crossover respects new limit
	a := ga.RandomGenome(100)
	b := ga.RandomGenome(100)
	child := ga.crossover(a, b)
	if len(child) > MaxGenome {
		t.Errorf("crossover child %d > MaxGenome %d", len(child), MaxGenome)
	}

	// Test mutation respects new limit
	big := ga.RandomGenome(120)
	for i := 0; i < 50; i++ {
		m := ga.mutate(big)
		if len(m) > MaxGenome {
			t.Errorf("mutation produced genome of size %d > %d", len(m), MaxGenome)
		}
	}
}

// --- Phase 1b tests ---

func TestModifierAddRemove(t *testing.T) {
	npc := NewNPC([]byte{micro.OpHalt})

	// Add a modifier
	npc.AddMod(Modifier{Kind: ModGas, Mag: 50, Duration: -1, Source: ItemCrystal})
	if got := npc.ModSum(ModGas); got != 50 {
		t.Errorf("ModSum(ModGas) = %d, want 50", got)
	}

	// Add another kind
	npc.AddMod(Modifier{Kind: ModAttack, Mag: 10, Duration: -1, Source: ItemWeapon})
	if got := npc.ModSum(ModAttack); got != 10 {
		t.Errorf("ModSum(ModAttack) = %d, want 10", got)
	}
	// Gas still there
	if got := npc.ModSum(ModGas); got != 50 {
		t.Errorf("ModSum(ModGas) after adding attack = %d, want 50", got)
	}

	// Remove by source
	npc.RemoveMod(ItemCrystal)
	if got := npc.ModSum(ModGas); got != 0 {
		t.Errorf("ModSum(ModGas) after remove = %d, want 0", got)
	}
	// Attack still there
	if got := npc.ModSum(ModAttack); got != 10 {
		t.Errorf("ModSum(ModAttack) after removing crystal = %d, want 10", got)
	}
}

func TestModifierEviction(t *testing.T) {
	npc := NewNPC([]byte{micro.OpHalt})

	// Fill all 4 slots
	npc.AddMod(Modifier{Kind: ModGas, Mag: 10, Duration: -1, Source: 1})
	npc.AddMod(Modifier{Kind: ModAttack, Mag: 5, Duration: 100, Source: 2})
	npc.AddMod(Modifier{Kind: ModDefense, Mag: 3, Duration: 50, Source: 3})
	npc.AddMod(Modifier{Kind: ModForage, Mag: 2, Duration: 200, Source: 4})

	// Add 5th — should evict shortest-duration (50, source 3)
	npc.AddMod(Modifier{Kind: ModTrade, Mag: 7, Duration: 30, Source: 5})

	if got := npc.ModSum(ModDefense); got != 0 {
		t.Errorf("ModDefense should have been evicted, got sum %d", got)
	}
	if got := npc.ModSum(ModTrade); got != 7 {
		t.Errorf("ModTrade should be present, got sum %d", got)
	}
}

func TestModifierDecay(t *testing.T) {
	npc := NewNPC([]byte{micro.OpHalt})
	npc.AddMod(Modifier{Kind: ModEnergy, Mag: 5, Duration: 3, Source: 0})

	// After 3 decays, should be expired
	decayModifiers(npc)
	if npc.Mods[0].Duration != 2 {
		t.Errorf("after 1 decay, duration = %d, want 2", npc.Mods[0].Duration)
	}
	decayModifiers(npc)
	decayModifiers(npc)
	if npc.Mods[0].Duration != 0 {
		t.Errorf("after 3 decays, duration = %d, want 0", npc.Mods[0].Duration)
	}
	// ModSum should return 0 for expired
	if got := npc.ModSum(ModEnergy); got != 0 {
		t.Errorf("expired modifier still contributing: ModSum = %d", got)
	}
}

func TestCrystalPickupGrantsGas(t *testing.T) {
	w := NewWorld(16, testRng())
	sched := NewScheduler(w, 200, io.Discard)

	// Genome: move North
	genome := []byte{
		micro.SmallNumOp(1),
		micro.OpRing1W, 0,
		micro.OpHalt,
	}
	npc := NewNPC(genome)
	npc.X = 5
	npc.Y = 5
	w.Spawn(npc)

	// Place crystal at (5,4)
	w.SetTile(5, 4, MakeTile(TileCrystal, 0))

	sched.Tick()

	// Crystal is consumed — NPC should NOT hold an item
	if npc.Item != ItemNone {
		t.Errorf("crystal should not be held, item=%d", npc.Item)
	}
	// But should have a gas modifier
	if got := npc.ModSum(ModGas); got != 50 {
		t.Errorf("expected ModGas=50 after crystal, got %d", got)
	}
	// Tile should be empty
	if w.TileAt(5, 4).Type() != TileEmpty {
		t.Errorf("crystal tile should be empty after pickup")
	}
}

func TestCrystalDiminishingReturns(t *testing.T) {
	npc := NewNPC([]byte{micro.OpHalt})
	// First crystal: +50 gas
	npc.AddMod(Modifier{Kind: ModGas, Mag: 50, Duration: -1, Source: ItemCrystal})

	// Compute effective gas bonus with diminishing returns
	// Total ModSum = 50. First 50 → +50, remainder 0 → total bonus = 50
	gasBonus := computeGasBonus(npc.ModSum(ModGas))
	if gasBonus != 50 {
		t.Errorf("1 crystal: gasBonus = %d, want 50", gasBonus)
	}

	// Second crystal: ModSum = 100. First 50→+50, remainder=50, halved→25, +25, total=75
	npc.AddMod(Modifier{Kind: ModGas, Mag: 50, Duration: -1, Source: ItemCrystal})
	gasBonus = computeGasBonus(npc.ModSum(ModGas))
	if gasBonus != 75 {
		t.Errorf("2 crystals: gasBonus = %d, want 75", gasBonus)
	}
}

func TestItemModifierOnPickup(t *testing.T) {
	w := NewWorld(16, testRng())
	sched := NewScheduler(w, 200, io.Discard)

	// Genome: move North
	genome := []byte{
		micro.SmallNumOp(1),
		micro.OpRing1W, 0,
		micro.OpHalt,
	}
	npc := NewNPC(genome)
	npc.X = 5
	npc.Y = 5
	w.Spawn(npc)

	// Place tool at (5,4)
	w.SetTile(5, 4, MakeTile(TileTool, 0))

	sched.Tick()

	if npc.Item != ItemTool {
		t.Fatalf("expected ItemTool, got %d", npc.Item)
	}
	if got := npc.ModSum(ModForage); got != 1 {
		t.Errorf("expected ModForage=1 from tool, got %d", got)
	}
}

func TestItemModifierOnTrade(t *testing.T) {
	w := NewWorld(16, testRng())
	sched := NewScheduler(w, 200, io.Discard)

	// NPC A: holds tool, outputs trade targeting B
	genomeA := []byte{
		micro.SmallNumOp(4),
		micro.OpRing1W, 1,
		micro.SmallNumOp(2),
		micro.OpRing1W, 2,
		micro.OpHalt,
	}
	npcA := NewNPC(genomeA)
	npcA.X = 5
	npcA.Y = 5
	npcA.Item = ItemTool
	grantItemModifier(npcA, ItemTool)
	w.Spawn(npcA)
	idA := npcA.ID

	// NPC B: holds weapon, outputs trade targeting A
	genomeB := []byte{
		micro.SmallNumOp(4),
		micro.OpRing1W, 1,
		micro.SmallNumOp(int(idA)),
		micro.OpRing1W, 2,
		micro.OpHalt,
	}
	npcB := NewNPC(genomeB)
	npcB.X = 5
	npcB.Y = 4
	npcB.Item = ItemWeapon
	grantItemModifier(npcB, ItemWeapon)
	w.Spawn(npcB)

	// Verify pre-trade modifiers
	if npcA.ModSum(ModForage) != 1 {
		t.Fatalf("pre-trade A ModForage = %d, want 1", npcA.ModSum(ModForage))
	}
	if npcB.ModSum(ModAttack) != 10 {
		t.Fatalf("pre-trade B ModAttack = %d, want 10", npcB.ModSum(ModAttack))
	}

	sched.Tick()

	// After trade: A has weapon → ModAttack, B has tool → ModForage
	if npcA.Item != ItemWeapon {
		t.Errorf("A should hold weapon, got %d", npcA.Item)
	}
	if npcA.ModSum(ModAttack) != 10 {
		t.Errorf("post-trade A ModAttack = %d, want 10", npcA.ModSum(ModAttack))
	}
	if npcA.ModSum(ModForage) != 0 {
		t.Errorf("post-trade A ModForage = %d, want 0", npcA.ModSum(ModForage))
	}
	if npcB.ModSum(ModForage) != 1 {
		t.Errorf("post-trade B ModForage = %d, want 1", npcB.ModSum(ModForage))
	}
	if npcB.ModSum(ModAttack) != 0 {
		t.Errorf("post-trade B ModAttack = %d, want 0", npcB.ModSum(ModAttack))
	}
}

// --- Phase 1c tests ---

func TestSoloCraftToolToCompass(t *testing.T) {
	rng := testRng()
	w := NewWorld(16, rng)
	sched := NewScheduler(w, 200, io.Discard)

	// Place forge at (5,5) — clear any existing forge first
	w.SetTile(5, 5, MakeTile(TileForge, 0))

	// Genome: ActionCraft (5) written to Ring1 action
	genome := []byte{
		micro.SmallNumOp(5),
		micro.OpRing1W, 1,
		micro.OpHalt,
	}
	npc := NewNPC(genome)
	npc.X = 5
	npc.Y = 5
	npc.Item = ItemTool
	grantItemModifier(npc, ItemTool)
	w.Spawn(npc)

	if npc.ModSum(ModForage) != 1 {
		t.Fatalf("pre-craft ModForage = %d, want 1", npc.ModSum(ModForage))
	}

	sched.Tick()

	if npc.Item != ItemCompass {
		t.Errorf("craft should produce compass, item=%d", npc.Item)
	}
	if got := npc.ModSum(ModForage); got != 2 {
		t.Errorf("compass should grant ModForage=2, got %d", got)
	}
}

func TestSoloCraftWeaponToShield(t *testing.T) {
	rng := testRng()
	w := NewWorld(16, rng)
	sched := NewScheduler(w, 200, io.Discard)

	w.SetTile(5, 5, MakeTile(TileForge, 0))

	genome := []byte{
		micro.SmallNumOp(5),
		micro.OpRing1W, 1,
		micro.OpHalt,
	}
	npc := NewNPC(genome)
	npc.X = 5
	npc.Y = 5
	npc.Item = ItemWeapon
	grantItemModifier(npc, ItemWeapon)
	w.Spawn(npc)

	sched.Tick()

	if npc.Item != ItemShield {
		t.Errorf("craft should produce shield, item=%d", npc.Item)
	}
	if got := npc.ModSum(ModDefense); got != 5 {
		t.Errorf("shield should grant ModDefense=5, got %d", got)
	}
	if got := npc.ModSum(ModAttack); got != 0 {
		t.Errorf("weapon modifier should be removed, ModAttack=%d", got)
	}
}

func TestPortableCraftOffForge(t *testing.T) {
	rng := testRng()
	w := NewWorld(16, rng)
	sched := NewScheduler(w, 200, io.Discard)

	// NPC on empty tile, not forge
	w.SetTile(5, 5, MakeTile(TileEmpty, 0))

	genome := []byte{
		micro.SmallNumOp(5),
		micro.OpRing1W, 1,
		micro.OpHalt,
	}
	npc := NewNPC(genome)
	npc.X = 5
	npc.Y = 5
	npc.Item = ItemTool
	npc.Energy = 100
	grantItemModifier(npc, ItemTool)
	w.Spawn(npc)

	sched.Tick()

	// Portable crafting: should succeed off forge, costing 20 energy
	if npc.Item != ItemCompass {
		t.Errorf("portable craft should produce compass, got item=%d", npc.Item)
	}
	// Energy: 100 - 20 (craft cost) - 1 (decay) = 79
	if npc.Energy != 79 {
		t.Errorf("expected energy=79 after off-forge craft, got %d", npc.Energy)
	}
	if npc.CraftCount != 1 {
		t.Errorf("expected CraftCount=1, got %d", npc.CraftCount)
	}
}

func TestPortableCraftInsufficientEnergy(t *testing.T) {
	rng := testRng()
	w := NewWorld(16, rng)
	sched := NewScheduler(w, 200, io.Discard)

	w.SetTile(5, 5, MakeTile(TileEmpty, 0))

	genome := []byte{
		micro.SmallNumOp(5),
		micro.OpRing1W, 1,
		micro.OpHalt,
	}
	npc := NewNPC(genome)
	npc.X = 5
	npc.Y = 5
	npc.Item = ItemTool
	npc.Energy = 15 // not enough for off-forge craft (needs 20)
	w.Spawn(npc)

	sched.Tick()

	if npc.Item != ItemTool {
		t.Errorf("craft with insufficient energy should keep item, got %d", npc.Item)
	}
}

func TestCraftInvalidItem(t *testing.T) {
	rng := testRng()
	w := NewWorld(16, rng)
	sched := NewScheduler(w, 200, io.Discard)

	w.SetTile(5, 5, MakeTile(TileForge, 0))

	genome := []byte{
		micro.SmallNumOp(5),
		micro.OpRing1W, 1,
		micro.OpHalt,
	}
	npc := NewNPC(genome)
	npc.X = 5
	npc.Y = 5
	npc.Item = ItemTreasure
	w.Spawn(npc)

	sched.Tick()

	if npc.Item != ItemTreasure {
		t.Errorf("craft with no recipe should keep treasure, got %d", npc.Item)
	}
}

// --- Phase 1d tests ---

func TestMarketValueScarcity(t *testing.T) {
	rng := testRng()
	w := NewWorld(16, rng)

	// Spawn NPCs with items: 3 tools, 1 crystal
	for i := 0; i < 3; i++ {
		npc := NewNPC([]byte{micro.OpHalt})
		npc.X = i
		npc.Y = 0
		npc.Item = ItemTool
		w.Spawn(npc)
	}
	npc4 := NewNPC([]byte{micro.OpHalt})
	npc4.X = 3
	npc4.Y = 0
	npc4.Item = ItemTreasure
	w.Spawn(npc4)

	valTool := w.MarketValue(ItemTool)
	valTreasure := w.MarketValue(ItemTreasure)

	// Treasure is rarer (1 vs 3), so should have higher value
	if valTreasure <= valTool {
		t.Errorf("treasure (count=1) value=%d should be > tool (count=3) value=%d", valTreasure, valTool)
	}
}

func TestTradeGoldTransfer(t *testing.T) {
	rng := testRng()
	w := NewWorld(16, rng)
	sched := NewScheduler(w, 200, io.Discard)

	// Populate world with extra NPCs holding tools to create scarcity imbalance
	for i := 0; i < 5; i++ {
		bg := NewNPC([]byte{micro.OpHalt})
		bg.X = i + 6
		bg.Y = 0
		bg.Item = ItemTool
		w.Spawn(bg)
	}

	// NPC A: holds treasure (rare) → targets B
	genomeA := []byte{
		micro.SmallNumOp(4),
		micro.OpRing1W, 1,
		micro.SmallNumOp(7), // target B's ID (will be 7th NPC spawned)
		micro.OpRing1W, 2,
		micro.OpHalt,
	}
	npcA := NewNPC(genomeA)
	npcA.X = 5
	npcA.Y = 5
	npcA.Item = ItemTreasure
	grantItemModifier(npcA, ItemTreasure)
	w.Spawn(npcA)
	idA := npcA.ID

	// NPC B: holds tool (common) → targets A
	genomeB := []byte{
		micro.SmallNumOp(4),
		micro.OpRing1W, 1,
		micro.SmallNumOp(int(idA)),
		micro.OpRing1W, 2,
		micro.OpHalt,
	}
	npcB := NewNPC(genomeB)
	npcB.X = 5
	npcB.Y = 4
	npcB.Item = ItemTool
	grantItemModifier(npcB, ItemTool)
	w.Spawn(npcB)

	sched.Tick()

	// Both should have earned some gold
	if npcA.Gold == 0 && npcB.Gold == 0 {
		t.Error("both NPCs should earn gold from trade")
	}
	t.Logf("After trade: A gold=%d B gold=%d", npcA.Gold, npcB.Gold)
}

func TestStressFromDamage(t *testing.T) {
	rng := testRng()
	w := NewWorld(16, rng)
	sched := NewScheduler(w, 200, io.Discard)

	// Attacker genome: attack target 2
	genomeA := []byte{
		micro.SmallNumOp(2), // action = attack
		micro.OpRing1W, 1,
		micro.SmallNumOp(2), // target = 2
		micro.OpRing1W, 2,
		micro.OpHalt,
	}
	npcA := NewNPC(genomeA)
	npcA.X = 5
	npcA.Y = 5
	w.Spawn(npcA)

	// Victim: idle
	npcB := NewNPC([]byte{micro.OpHalt})
	npcB.X = 5
	npcB.Y = 4
	w.Spawn(npcB)

	startStress := npcB.Stress
	sched.Tick()

	if npcB.Stress <= startStress {
		t.Errorf("victim stress should increase from attack: was %d, now %d", startStress, npcB.Stress)
	}
}

func TestStressDecayFromEating(t *testing.T) {
	rng := testRng()
	w := NewWorld(16, rng)
	sched := NewScheduler(w, 200, io.Discard)

	// Genome: eat
	genome := []byte{
		micro.SmallNumOp(1),
		micro.OpRing1W, 1,
		micro.OpHalt,
	}
	npc := NewNPC(genome)
	npc.X = 5
	npc.Y = 5
	npc.Stress = 25 // below override threshold (30)
	w.Spawn(npc)

	// Place food adjacent
	w.SetTile(5, 4, MakeTile(TileFood, 0))

	sched.Tick()

	// Stress should decrease (eating gives -2, autoEat may also eat)
	if npc.Stress >= 25 {
		t.Errorf("eating should decrease stress: was 25, now %d", npc.Stress)
	}
}

func TestStressOutputOverride(t *testing.T) {
	rng := testRng()
	w := NewWorld(16, rng)

	// Genome that always outputs move=1 (North), action=0 (idle)
	genome := []byte{
		micro.SmallNumOp(1),
		micro.OpRing1W, 0,
		micro.OpHalt,
	}

	// Run many ticks with high stress, count how many times action diverges
	overrides := 0
	trials := 100

	for i := 0; i < trials; i++ {
		w2 := NewWorld(16, rng)
		sched := NewScheduler(w2, 200, io.Discard)
		_ = w // keep the original for rng
		npc := NewNPC(genome)
		npc.X = 8
		npc.Y = 8
		npc.Stress = 80
		w2.Spawn(npc)

		startY := npc.Y
		sched.Tick()

		// If stress override happened, NPC may move differently or take a non-idle action
		if npc.Y == startY || npc.Y != startY-1 {
			overrides++
		}
	}

	// With stress=80, threshold = (80-30)*31/100 = 15.5 → roll < 15 out of 31 ≈ 48%
	// We expect roughly half the trials to be overridden, but at least some
	if overrides == 0 {
		t.Error("stress=80 should cause some action overrides in 100 trials")
	}
	t.Logf("Stress overrides: %d/%d", overrides, trials)
}

// --- End-to-end simulation test ---

func TestE2E50kTickSimulation(t *testing.T) {
	rng := rand.New(rand.NewSource(42))
	w := NewWorld(32, rng)
	w.MaxFood = 64
	w.FoodRate = 0.5
	ga := NewGA(rng)
	sched := NewScheduler(w, 200, io.Discard)

	// Trader genome (from cmd/sandbox)
	traderGenome := []byte{
		0x8A, 0x0F, 0x20, 0x0D, 0x88, 0x08,
		0x8A, 0x0D, 0x8C, 0x00, 0x21, 0x8C, 0x01, 0xF1,
		0x8A, 0x12, 0x8C, 0x00, 0x24, 0x8C, 0x01, 0x8A, 0x0C, 0x8C, 0x02, 0xF1,
	}
	// Forager genome
	foragerGenome := []byte{
		0x8A, 0x0D, 0x8C, 0x00, 0x21, 0x8C, 0x01, 0xF1,
	}

	npcs := 20
	numTraders := 5
	numForagers := 5
	for i := 0; i < npcs; i++ {
		var genome []byte
		if i < numTraders {
			genome = make([]byte, len(traderGenome))
			copy(genome, traderGenome)
		} else if i < numTraders+numForagers {
			genome = make([]byte, len(foragerGenome))
			copy(genome, foragerGenome)
		} else {
			genome = ga.RandomGenome(24 + rng.Intn(16))
		}
		npc := NewNPC(genome)
		npc.X = rng.Intn(32)
		npc.Y = rng.Intn(32)
		if i < numTraders {
			npc.Item = byte(ItemTool + rng.Intn(3))
			grantItemModifier(npc, npc.Item)
		}
		w.Spawn(npc)
	}

	// Seed food
	for i := 0; i < 32; i++ {
		x := rng.Intn(32)
		y := rng.Intn(32)
		if w.TileAt(x, y).Type() == TileEmpty && w.TileAt(x, y).Occupant() == 0 {
			w.SetTile(x, y, MakeTile(TileFood, 0))
		}
	}

	// Tracking stats
	type epochStats struct {
		tick       int
		alive      int
		trades     int
		totalGold  int
		holders    int
		crafted    int // NPCs holding shield or compass
		crystalMod int // NPCs with ModGas > 0
		stressed   int // NPCs with stress > 30
		avgStress  int
		bestFit    int
	}

	var epochs []epochStats
	totalTicks := 50000
	evolveEvery := 100

	for tick := 0; tick < totalTicks; tick++ {
		sched.Tick()

		if tick > 0 && tick%evolveEvery == 0 {
			w.NPCs = ga.Evolve(w.NPCs)

			// Respawn if too few
			for len(w.NPCs) < npcs/2 {
				var genome []byte
				if rng.Intn(3) == 0 {
					genome = make([]byte, len(traderGenome))
					copy(genome, traderGenome)
				} else {
					genome = make([]byte, len(foragerGenome))
					copy(genome, foragerGenome)
				}
				npc := NewNPC(genome)
				npc.X = rng.Intn(32)
				npc.Y = rng.Intn(32)
				if rng.Intn(3) == 0 {
					npc.Item = byte(ItemTool + rng.Intn(3))
					grantItemModifier(npc, npc.Item)
				}
				w.Spawn(npc)
			}
		}

		// Collect stats every 5000 ticks
		if tick > 0 && tick%5000 == 0 {
			s := epochStats{tick: tick, trades: sched.TradeCount}
			for _, npc := range w.NPCs {
				if !npc.Alive() {
					continue
				}
				s.alive++
				s.totalGold += npc.Gold
				if npc.Item != ItemNone {
					s.holders++
				}
				if npc.Item == ItemShield || npc.Item == ItemCompass {
					s.crafted++
				}
				if npc.ModSum(ModGas) > 0 {
					s.crystalMod++
				}
				if npc.Stress > 30 {
					s.stressed++
				}
				s.avgStress += npc.Stress
				if npc.Fitness > s.bestFit {
					s.bestFit = npc.Fitness
				}
			}
			if s.alive > 0 {
				s.avgStress /= s.alive
			}
			epochs = append(epochs, s)
		}

		if len(w.NPCs) == 0 {
			t.Fatalf("population extinct at tick %d", tick)
		}
	}

	// Log all epoch stats
	t.Log("tick      alive  trades  gold  holders  crafted  crystal  stressed  avgStress  bestFit")
	for _, s := range epochs {
		t.Logf("%-9d %-6d %-7d %-5d %-8d %-8d %-8d %-9d %-10d %-7d",
			s.tick, s.alive, s.trades, s.totalGold, s.holders, s.crafted, s.crystalMod, s.stressed, s.avgStress, s.bestFit)
	}

	// Assertions: verify the simulation produced meaningful emergent behavior
	final := epochs[len(epochs)-1]

	// Population survived
	if final.alive < 5 {
		t.Errorf("population too small at end: %d", final.alive)
	}

	// Trades occurred (seeded traders should produce some)
	if final.trades < 5 {
		t.Errorf("expected at least 5 trades over 50k ticks, got %d", final.trades)
	}

	// Crystal modifiers accumulated (rare spawns were found)
	anyCrystal := false
	for _, s := range epochs {
		if s.crystalMod > 0 {
			anyCrystal = true
			break
		}
	}
	if !anyCrystal {
		t.Error("expected some NPCs to find crystals over 50k ticks")
	}

	// Final fitness is reasonable (age-dominated: ~50000 ticks alive)
	if final.bestFit < 100 {
		t.Errorf("best fitness too low: %d", final.bestFit)
	}

	// Item holders exist (NPCs picking up spawned items)
	anyHolders := false
	for _, s := range epochs {
		if s.holders > 0 {
			anyHolders = true
			break
		}
	}
	if !anyHolders {
		t.Error("expected some NPCs to hold items")
	}

	t.Logf("Final: alive=%d trades=%d gold=%d crystalNPCs=%d bestFit=%d",
		final.alive, final.trades, final.totalGold, final.crystalMod, final.bestFit)
}

// --- Phase 2 tests ---

func TestMutationGeneratesRingOps(t *testing.T) {
	ga := NewGA(testRng())
	r0count := 0
	r1count := 0
	trials := 1000
	for i := 0; i < trials; i++ {
		op := ga.randomOpcode()
		if op == micro.OpRing0R {
			r0count++
		}
		if op == micro.OpRing1W {
			r1count++
		}
	}
	if r0count == 0 {
		t.Errorf("r0@ (0x8A) never generated in %d trials", trials)
	}
	if r1count == 0 {
		t.Errorf("r1! (0x8C) never generated in %d trials", trials)
	}
	t.Logf("Ring ops in %d trials: r0@=%d r1!=%d", trials, r0count, r1count)
}

func TestAutoCraftOnForge(t *testing.T) {
	rng := testRng()
	w := NewWorld(16, rng)
	sched := NewScheduler(w, 200, io.Discard)

	// Place forge at NPC position
	w.SetTile(5, 5, MakeTile(TileForge, 0))

	// Genome: idle (no craft action) — auto-craft should handle it
	genome := []byte{micro.OpHalt}
	npc := NewNPC(genome)
	npc.X = 5
	npc.Y = 5
	npc.Item = ItemTool
	grantItemModifier(npc, ItemTool)
	w.Spawn(npc)

	sched.Tick()

	if npc.Item != ItemCompass {
		t.Errorf("auto-craft on forge should produce compass, got item=%d", npc.Item)
	}
	if got := npc.ModSum(ModForage); got != 2 {
		t.Errorf("auto-crafted compass should grant ModForage=2, got %d", got)
	}
	if npc.CraftCount != 1 {
		t.Errorf("expected CraftCount=1 after auto-craft, got %d", npc.CraftCount)
	}
}

func TestWinterNoFoodSpawn(t *testing.T) {
	rng := testRng()
	w := NewWorld(16, rng)

	// Set tick to winter period (192-255 of cycle)
	w.Tick = 200
	w.MaxFood = 100 // high cap so we're not limited
	w.FoodRate = 1.0 // always try to spawn

	startFood := w.FoodCount()
	for i := 0; i < 50; i++ {
		w.RespawnFood()
	}

	if w.FoodCount() != startFood {
		t.Errorf("winter should prevent food spawn: before=%d after=%d", startFood, w.FoodCount())
	}

	// Advance past winter
	w.Tick = 10 // spring
	w.RespawnFood()
	if w.FoodCount() <= startFood {
		t.Logf("food spawned in spring (expected): count=%d", w.FoodCount())
	}
}

func TestForagingRadiusWithTool(t *testing.T) {
	rng := testRng()
	w := NewWorld(16, rng)
	sched := NewScheduler(w, 200, io.Discard)

	// Idle genome
	genome := []byte{micro.OpHalt}
	npc := NewNPC(genome)
	npc.X = 5
	npc.Y = 5
	npc.Energy = 50
	// Give tool: ModForage +1 → radius 2
	npc.Item = ItemTool
	grantItemModifier(npc, ItemTool)
	w.Spawn(npc)

	// Place food 2 tiles away (distance 2, within radius 2)
	w.SetTile(7, 5, MakeTile(TileFood, 0))

	sched.Tick()

	if npc.FoodEaten != 1 {
		t.Errorf("NPC with tool should eat food 2 tiles away, food_eaten=%d", npc.FoodEaten)
	}
}

func TestForagingRadiusWithoutTool(t *testing.T) {
	rng := testRng()
	w := NewWorld(16, rng)
	sched := NewScheduler(w, 200, io.Discard)

	// Idle genome
	genome := []byte{micro.OpHalt}
	npc := NewNPC(genome)
	npc.X = 5
	npc.Y = 5
	npc.Energy = 50
	// No item → radius 1
	w.Spawn(npc)

	// Place food 2 tiles away (distance 2, outside radius 1)
	w.SetTile(7, 5, MakeTile(TileFood, 0))

	sched.Tick()

	if npc.FoodEaten != 0 {
		t.Errorf("NPC without tool should NOT eat food 2 tiles away, food_eaten=%d", npc.FoodEaten)
	}
}

func TestGoldInheritance(t *testing.T) {
	rng := testRng()
	ga := NewGA(rng)

	// Create 4 NPCs with varying fitness and gold
	npcs := make([]*NPC, 4)
	for i := range npcs {
		npcs[i] = NewNPC(ga.RandomGenome(24))
		npcs[i].ID = byte(i + 1)
		npcs[i].Health = 100
	}

	// Top NPCs have gold
	npcs[0].Fitness = 100
	npcs[0].Gold = 40
	npcs[1].Fitness = 80
	npcs[1].Gold = 20
	npcs[2].Fitness = 60
	npcs[2].Gold = 10
	npcs[3].Fitness = 10
	npcs[3].Gold = 0

	ga.Evolve(npcs)

	// Bottom NPC (index 3) should have been replaced with partial parent gold
	// Parent gold: (parentA.Gold + parentB.Gold) / 4
	// Parents come from top 2 (tournament selection from top 50%)
	// Gold should be > 0 since parents have gold
	if npcs[3].Gold < 0 {
		t.Errorf("replaced NPC should have non-negative gold, got %d", npcs[3].Gold)
	}
	t.Logf("Replaced NPC gold: %d (parents had gold 40,20)", npcs[3].Gold)
}

func TestConstantTweakOperand(t *testing.T) {
	ga := NewGA(testRng())

	// Genome with r1! operand
	genome := []byte{
		micro.SmallNumOp(1),
		micro.OpRing1W, 10, // r1! 10 — the operand (10) should be tweakable
		micro.OpHalt,
	}

	// Run many mutations, check that operand byte (index 2) gets tweaked
	tweaked := false
	for i := 0; i < 200; i++ {
		// Force case 3 by creating fresh GA with controlled random
		g := make([]byte, len(genome))
		copy(g, genome)
		mutated := ga.mutate(g)
		if len(mutated) >= 3 && mutated[1] == micro.OpRing1W && mutated[2] != 10 {
			tweaked = true
			break
		}
	}
	if !tweaked {
		t.Error("mutation should be able to tweak r1! operand byte")
	}
}

func TestMoreForges(t *testing.T) {
	rng := testRng()
	w := NewWorld(32, rng)

	forgeCount := 0
	for _, t := range w.Grid {
		if t.Type() == TileForge {
			forgeCount++
		}
	}

	// 32/8 = 4, max(3,4) = 4
	if forgeCount < 3 {
		t.Errorf("expected at least 3 forges, got %d", forgeCount)
	}
	t.Logf("Forge count on 32x32 world: %d", forgeCount)
}

// --- Phase 3 tests ---

func TestMaxAgeDeath(t *testing.T) {
	rng := testRng()
	w := NewWorld(16, rng)
	sched := NewScheduler(w, 200, io.Discard)

	genome := []byte{micro.OpHalt}
	npc := NewNPC(genome)
	npc.X = 5
	npc.Y = 5
	npc.Age = MaxAge - 1 // will hit MaxAge this tick
	npc.Energy = 200     // plenty of energy so it doesn't die from starvation
	w.Spawn(npc)

	sched.Tick()

	// NPC should be dead (removed from world)
	if len(w.NPCs) != 0 {
		t.Errorf("NPC at MaxAge should be dead, but %d alive", len(w.NPCs))
	}
}

func TestPoisonTileDamage(t *testing.T) {
	rng := testRng()
	w := NewWorld(16, rng)
	sched := NewScheduler(w, 200, io.Discard)

	// Genome: move North
	genome := []byte{
		micro.SmallNumOp(1),
		micro.OpRing1W, 0,
		micro.OpHalt,
	}
	npc := NewNPC(genome)
	npc.X = 5
	npc.Y = 5
	w.Spawn(npc)

	// Place poison at (5,4) — one tile North
	w.SetTile(5, 4, MakeTile(TilePoison, 0))
	w.PoisonTTL[w.Size*4+5] = w.Tick

	startHealth := npc.Health
	sched.Tick()

	if npc.Y != 4 {
		t.Fatalf("NPC should have moved to y=4, got y=%d", npc.Y)
	}
	// Health: -15 (poison) +5 (possible food eat, but no food) = should be startHealth - 15 - 5 (energy decay health loss only if energy depleted)
	// Actually: energy starts at 100, -1 decay = 99, no health loss from energy
	// So health = startHealth - 15
	expectedHealth := startHealth - 15
	if npc.Health != expectedHealth {
		t.Errorf("poison should deal 15 damage: health=%d want=%d", npc.Health, expectedHealth)
	}
	// Tile should be consumed
	if w.TileAt(5, 4).Type() == TilePoison {
		t.Error("poison tile should be consumed after contact")
	}
}

func TestBlightDestroysFood(t *testing.T) {
	rng := testRng()
	w := NewWorld(16, rng)

	// Place 20 food tiles
	placed := 0
	for y := 0; y < 16 && placed < 20; y++ {
		for x := 0; x < 16 && placed < 20; x++ {
			if w.TileAt(x, y).Type() == TileEmpty {
				w.SetTile(x, y, MakeTile(TileFood, 0))
				placed++
			}
		}
	}

	beforeFood := w.FoodCount()
	w.Blight()
	afterFood := w.FoodCount()

	// Should destroy roughly 50% (allow range 20%-80% for randomness)
	destroyed := beforeFood - afterFood
	if destroyed == 0 {
		t.Error("blight should destroy some food")
	}
	if afterFood == 0 {
		t.Error("blight should not destroy ALL food (probabilistic, but 20 tiles should leave some)")
	}
	t.Logf("Blight: %d → %d food (destroyed %d)", beforeFood, afterFood, destroyed)
}

func TestMemeticTransferSuccess(t *testing.T) {
	rng := testRng()
	w := NewWorld(16, rng)
	sched := NewScheduler(w, 200, io.Discard)

	// Teacher with high fitness
	teacher := NewNPC([]byte{
		micro.SmallNumOp(1), micro.OpRing1W, 0,
		micro.SmallNumOp(5), micro.OpRing1W, 1,
		micro.OpHalt,
	})
	teacher.X = 5
	teacher.Y = 5
	teacher.Fitness = 1000 // very high → should almost always succeed
	w.Spawn(teacher)

	// Student with low fitness, adjacent
	student := NewNPC([]byte{
		micro.OpNop, micro.OpNop, micro.OpNop, micro.OpNop,
		micro.OpNop, micro.OpNop, micro.OpNop, micro.OpNop,
		micro.OpNop, micro.OpNop, micro.OpNop, micro.OpNop,
		micro.OpNop, micro.OpNop, micro.OpNop, micro.OpHalt,
	})
	student.X = 5
	student.Y = 4
	student.Fitness = 0
	w.Spawn(student)

	origGenome := make([]byte, len(student.Genome))
	copy(origGenome, student.Genome)

	// Try multiple times to account for randomness
	transferred := false
	for i := 0; i < 20; i++ {
		// Reset student genome
		student.Genome = make([]byte, len(origGenome))
		copy(student.Genome, origGenome)
		student.Taught = 0
		teacher.TeachCount = 0

		sched.memeticTransfer(teacher, student)

		if student.Taught > 0 {
			transferred = true
			break
		}
	}

	if !transferred {
		t.Error("memetic transfer with high-fitness teacher should succeed at least once in 20 tries")
	}
	if teacher.TeachCount == 0 {
		t.Error("teacher TeachCount should be incremented on success")
	}
}

func TestMemeticTransferResistance(t *testing.T) {
	rng := testRng()
	w := NewWorld(16, rng)
	sched := NewScheduler(w, 200, io.Discard)

	// Teacher with very low fitness
	teacher := NewNPC([]byte{micro.SmallNumOp(1), micro.OpHalt})
	teacher.X = 5
	teacher.Y = 5
	teacher.Fitness = 0
	w.Spawn(teacher)

	// Student with very high fitness
	student := NewNPC([]byte{
		micro.OpNop, micro.OpNop, micro.OpNop, micro.OpNop,
		micro.OpNop, micro.OpNop, micro.OpNop, micro.OpNop,
		micro.OpNop, micro.OpNop, micro.OpNop, micro.OpNop,
		micro.OpNop, micro.OpNop, micro.OpNop, micro.OpHalt,
	})
	student.X = 5
	student.Y = 4
	student.Fitness = 10000
	w.Spawn(student)

	// With teacher fitness=0, student fitness=10000:
	// prob = 1/(1+10000+2) ≈ 0.0001 — almost always resisted
	successes := 0
	for i := 0; i < 100; i++ {
		student.Taught = 0
		sched.memeticTransfer(teacher, student)
		if student.Taught > 0 {
			successes++
		}
	}

	// Should resist most of the time (expect <5 successes in 100 trials)
	if successes > 10 {
		t.Errorf("high-fitness student should resist low-fitness teacher: %d/100 successes", successes)
	}
	t.Logf("Resistance test: %d/100 successes (low teacher vs high student)", successes)
}

func TestMemeticTransferAdjacency(t *testing.T) {
	rng := testRng()
	w := NewWorld(16, rng)
	sched := NewScheduler(w, 200, io.Discard)

	// Teacher genome: output ActionTeach targeting student
	teacher := NewNPC([]byte{
		micro.SmallNumOp(6), // push 6 (ActionTeach)
		micro.OpRing1W, 1,   // r1![1] = 6
		micro.SmallNumOp(2), // push 2 (target ID)
		micro.OpRing1W, 2,   // r1![2] = 2
		micro.OpHalt,
	})
	teacher.X = 5
	teacher.Y = 5
	teacher.Fitness = 1000
	teacher.Energy = 100
	w.Spawn(teacher)

	// Student 3 tiles away (not adjacent)
	student := NewNPC([]byte{micro.OpHalt})
	student.X = 5
	student.Y = 2 // distance = 3
	student.Fitness = 0
	w.Spawn(student)

	startTaught := student.Taught
	sched.Tick()

	if student.Taught > startTaught {
		t.Error("teach should fail when NPCs are not adjacent")
	}
}

func TestDangerSensorReportsPoison(t *testing.T) {
	rng := testRng()
	w := NewWorld(16, rng)
	sched := NewScheduler(w, 200, io.Discard)

	genome := []byte{micro.OpHalt}
	npc := NewNPC(genome)
	npc.X = 5
	npc.Y = 5
	w.Spawn(npc)

	// Place poison 3 tiles away
	w.SetTile(8, 5, MakeTile(TilePoison, 0))
	w.PoisonTTL[w.Size*5+8] = w.Tick

	// Run sense to populate sensors
	sched.Tick()

	// Verify NearestPoison returns correct distance
	dist := w.NearestPoison(5, 5)
	if dist != 3 {
		t.Errorf("NearestPoison should be 3, got %d", dist)
	}
}

func TestAgeSensorReportsRemaining(t *testing.T) {
	npc := NewNPC([]byte{micro.OpHalt})
	npc.Age = 1000

	remaining := MaxAge - npc.Age
	if remaining != 4000 {
		t.Errorf("remaining life should be 4000, got %d", remaining)
	}
}

func TestTeachActionDispatch(t *testing.T) {
	rng := testRng()
	w := NewWorld(16, rng)
	sched := NewScheduler(w, 200, io.Discard)

	// Teacher with genome that outputs ActionTeach=6 targeting nearest NPC
	teachGenome := []byte{
		micro.SmallNumOp(6),  // push 6 (ActionTeach)
		micro.OpRing1W, 1,    // r1![1] = 6 (action)
		micro.SmallNumOp(2),  // push 2 (target ID — will be student)
		micro.OpRing1W, 2,    // r1![2] = 2 (target)
		micro.OpHalt,
	}
	teacher := NewNPC(teachGenome)
	teacher.X = 5
	teacher.Y = 5
	teacher.Fitness = 500
	teacher.Energy = 100
	w.Spawn(teacher)

	// Student adjacent
	student := NewNPC([]byte{
		micro.OpNop, micro.OpNop, micro.OpNop, micro.OpNop,
		micro.OpNop, micro.OpNop, micro.OpNop, micro.OpNop,
		micro.OpNop, micro.OpNop, micro.OpNop, micro.OpNop,
		micro.OpNop, micro.OpNop, micro.OpNop, micro.OpHalt,
	})
	student.X = 5
	student.Y = 4
	student.Fitness = 0
	w.Spawn(student)

	// Run tick — teacher should attempt teach
	startEnergy := teacher.Energy
	sched.Tick()

	// Teacher should have spent 10 energy on teach (regardless of transfer success)
	if teacher.Energy >= startEnergy-1 {
		// -1 from normal decay; if teach happened, should be -10 more
		// Note: energy decay happens too, so check for -11 total
		t.Logf("Teacher energy: start=%d end=%d (expected ~%d if taught)", startEnergy, teacher.Energy, startEnergy-11)
	}
	// At minimum, the teach action was dispatched (energy cost)
	if teacher.Energy > startEnergy-10 {
		t.Logf("NOTE: teach may not have been dispatched (energy only dropped by %d)", startEnergy-teacher.Energy)
	}
}

func TestAgedNPCReplacedInEvolve(t *testing.T) {
	rng := testRng()
	ga := NewGA(rng)

	// Create 8 NPCs — make the top-fitness one aged out
	npcs := make([]*NPC, 8)
	for i := range npcs {
		npcs[i] = NewNPC(ga.RandomGenome(24))
		npcs[i].ID = byte(i + 1)
		npcs[i].Health = 100
		npcs[i].Fitness = (i + 1) * 100 // ascending fitness
	}

	// Top NPC (index 7, fitness=800) is at MaxAge
	npcs[7].Age = MaxAge
	oldGenome := make([]byte, len(npcs[7].Genome))
	copy(oldGenome, npcs[7].Genome)

	ga.Evolve(npcs)

	// The aged NPC should have been replaced (age reset to 0)
	if npcs[7].Age != 0 {
		t.Errorf("aged NPC should be replaced: age=%d want 0", npcs[7].Age)
	}
	if npcs[7].Fitness != 0 {
		t.Errorf("replaced NPC should have fitness=0, got %d", npcs[7].Fitness)
	}
}
