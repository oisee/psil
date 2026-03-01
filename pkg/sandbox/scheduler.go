package sandbox

import (
	"io"

	"github.com/psilLang/psil/pkg/micro"
)

// DayCycle is the number of ticks in one day cycle.
const DayCycle = 256

// forgeRecipes maps input item → crafted output item.
var forgeRecipes = map[byte]byte{
	ItemTool:   ItemCompass,
	ItemWeapon: ItemShield,
}

// Scheduler runs the sandbox tick loop.
type Scheduler struct {
	World  *World
	Gas    int // gas limit per NPC brain execution
	Output io.Writer

	vm           *micro.VM    // reusable VM instance
	tradeIntents map[byte]byte // NPC ID -> target NPC ID
	TradeCount   int           // total bilateral trades completed
	TeachCount   int           // total successful teach events
}

// NewScheduler creates a scheduler for the given world.
func NewScheduler(w *World, gas int, output io.Writer) *Scheduler {
	return &Scheduler{
		World:        w,
		Gas:          gas,
		Output:       output,
		vm:           micro.New(),
		tradeIntents: make(map[byte]byte),
	}
}

// Tick runs one simulation step.
func (s *Scheduler) Tick() {
	w := s.World

	for _, npc := range w.NPCs {
		if !npc.Alive() {
			continue
		}

		// 1. Sense: fill Ring0
		s.sense(npc)

		// 2. Think: run genome
		s.think(npc)

		// 3. Act: read Ring1, apply to world
		s.act(npc)

		// 4. Auto-actions: eat food (extended radius), auto-craft on forge
		s.autoActions(npc)

		// 4b. Apply and decay modifiers
		applyModifiers(npc)
		decayModifiers(npc)

		// 5. Decay
		npc.Energy--
		if npc.Energy <= 0 {
			npc.Health -= 5
			npc.Energy = 0
		}
		npc.Age++
		npc.Hunger++

		// Natural death: max age reached
		if npc.Age >= MaxAge {
			npc.Health = 0
		}

		// 5b. Stress events
		if npc.Energy < 50 {
			npc.Stress += 5 // starvation stress
		}
		if npc.Energy > 150 {
			npc.Stress-- // resting decay
		}
		if npc.Stress > 100 {
			npc.Stress = 100
		}
		if npc.Stress < 0 {
			npc.Stress = 0
		}
	}

	// Remove dead NPCs (drop items back to world)
	alive := w.NPCs[:0]
	for _, npc := range w.NPCs {
		if npc.Alive() {
			alive = append(alive, npc)
		} else {
			// Determine underlying tile to preserve (forge)
			baseTile := byte(TileEmpty)
			if w.TileAt(npc.X, npc.Y).Type() == TileForge {
				baseTile = TileForge
			}
			// Drop held item as a tile (only standard items get dropped)
			if npc.Item >= ItemTool && npc.Item <= ItemTreasure && baseTile != TileForge {
				tileType := byte(TileTool) + npc.Item - ItemTool
				w.SetTile(npc.X, npc.Y, MakeTile(tileType, 0))
			} else {
				w.SetTile(npc.X, npc.Y, MakeTile(baseTile, 0))
			}
		}
	}
	w.NPCs = alive

	// 5. Resolve bilateral trades
	s.resolveTrades()

	// 6. Respawn food and items
	w.RespawnFood()
	w.RespawnItems()

	// 6b. Decay poison tiles and trigger periodic blights
	w.DecayPoison()
	if w.Tick > 0 && w.Tick%1024 == 0 {
		w.Blight()
	}

	// 7. Score fitness (stress penalty, crafting bonus, teaching bonus)
	for _, npc := range w.NPCs {
		npc.Fitness = npc.Age + npc.FoodEaten*10 + npc.Health + npc.Gold*20 + npc.CraftCount*30 + npc.TeachCount*15 - npc.Stress/5
	}

	w.Tick++
}

// sense fills Ring0 slots from world state.
func (s *Scheduler) sense(npc *NPC) {
	vm := s.vm
	w := s.World

	vm.MemWrite(Ring0Self, int16(npc.ID))
	vm.MemWrite(Ring0Health, int16(npc.Health))
	vm.MemWrite(Ring0Energy, int16(npc.Energy))
	vm.MemWrite(Ring0Hunger, int16(npc.Hunger))
	vm.MemWrite(Ring0Fear, int16(w.NearestNPC(npc.X, npc.Y, npc.ID))) // treat all NPCs as potential enemies
	vm.MemWrite(Ring0Food, int16(w.NearestFood(npc.X, npc.Y)))
	vm.MemWrite(Ring0Danger, int16(w.NearestPoison(npc.X, npc.Y)))
	vm.MemWrite(Ring0Near, int16(w.NearestNPC(npc.X, npc.Y, npc.ID)))
	vm.MemWrite(Ring0X, int16(npc.X))
	vm.MemWrite(Ring0Y, int16(npc.Y))
	vm.MemWrite(Ring0Day, int16(w.Tick%DayCycle))
	vm.MemWrite(Ring0NearID, int16(w.NearestNPCID(npc.X, npc.Y, npc.ID)))
	vm.MemWrite(Ring0FoodDir, int16(w.NearestFoodDir(npc.X, npc.Y)))

	// Extended Ring0 slots
	vm.MemWrite(Ring0MyGold, int16(npc.Gold))
	vm.MemWrite(Ring0MyItem, int16(npc.Item))
	dist, _ := w.NearestItem(npc.X, npc.Y)
	vm.MemWrite(Ring0NearItem, int16(dist))
	vm.MemWrite(Ring0NearTrust, 0) // stub for Phase 3
	vm.MemWrite(Ring0NearDir, int16(w.NearestNPCDir(npc.X, npc.Y, npc.ID)))
	vm.MemWrite(Ring0ItemDir, int16(w.NearestItemDir(npc.X, npc.Y)))
	vm.MemWrite(Ring0Rng, int16(npc.Rand()))
	vm.MemWrite(Ring0Stress, int16(npc.Stress))

	// Check if standing on forge
	onForge := int16(0)
	if w.TileAt(npc.X, npc.Y).Type() == TileForge {
		onForge = 1
	}
	vm.MemWrite(Ring0OnForge, onForge)

	// Phase 3 sensors
	vm.MemWrite(Ring0MyAge, int16(MaxAge-npc.Age))
	vm.MemWrite(Ring0Taught, int16(npc.Taught))

	// Effective gas: base + modifier bonus with diminishing returns
	gasBonus := 0
	add := npc.ModSum(ModGas)
	for add > 0 {
		if add >= 50 {
			gasBonus += 50
			add -= 50
			add /= 2 // diminishing returns
		} else {
			gasBonus += add
			add = 0
		}
	}
	effectiveGas := s.Gas + gasBonus
	if effectiveGas > 500 {
		effectiveGas = 500
	}
	vm.MemWrite(Ring0MyGas, int16(effectiveGas))
}

// think runs the NPC's genome on the VM.
func (s *Scheduler) think(npc *NPC) {
	vm := s.vm
	vm.Reset()

	// Compute effective gas with modifier bonus and diminishing returns
	gasBonus := 0
	add := npc.ModSum(ModGas)
	for add > 0 {
		if add >= 50 {
			gasBonus += 50
			add -= 50
			add /= 2
		} else {
			gasBonus += add
			add = 0
		}
	}
	effectiveGas := s.Gas + gasBonus
	if effectiveGas > 500 {
		effectiveGas = 500
	}
	vm.MaxGas = effectiveGas
	vm.Gas = effectiveGas
	vm.Output = s.Output

	// Clear Ring1 slots
	vm.MemWrite(64+Ring1Move, 0)
	vm.MemWrite(64+Ring1Action, 0)
	vm.MemWrite(64+Ring1Target, 0)
	vm.MemWrite(64+Ring1Emotion, 0)

	// Load genome and run
	vm.Load(npc.Genome)
	vm.Run() // ignores error (gas exhaustion is normal)
}

// act reads Ring1 outputs and applies movement/action.
func (s *Scheduler) act(npc *NPC) {
	vm := s.vm
	w := s.World

	// Read Ring1 outputs
	moveDir := int(vm.MemRead(64 + Ring1Move))
	action := int(vm.MemRead(64 + Ring1Action))

	// Stress output override: if stress > 30, (stress-30)% chance of random action
	if npc.Stress > 30 {
		roll := int(npc.Rand()) // 0-31
		threshold := (npc.Stress - 30) * 31 / 100
		if roll < threshold {
			moveDir = int(npc.Rand()%4) + 1 // random direction 1-4
			action = int(npc.Rand() % 3)     // random action 0-2 (idle/eat/attack)
		}
	}

	// Apply movement
	nx, ny := npc.X, npc.Y
	switch moveDir {
	case DirNorth:
		ny--
	case DirEast:
		nx++
	case DirSouth:
		ny++
	case DirWest:
		nx--
	}

	if w.InBounds(nx, ny) {
		dest := w.TileAt(nx, ny)
		if dest.Type() != TileWall && dest.Occupant() == 0 {
			// Clear old position (preserve persistent tile types like forge)
			oldType := w.TileAt(npc.X, npc.Y).Type()
			if oldType == TileForge {
				w.SetTile(npc.X, npc.Y, MakeTile(TileForge, 0))
			} else {
				w.SetTile(npc.X, npc.Y, MakeTile(TileEmpty, 0))
			}
			npc.X = nx
			npc.Y = ny
			w.SetTile(npc.X, npc.Y, MakeTile(w.TileAt(npc.X, npc.Y).Type(), npc.ID))
		}
	}

	// Handle poison tile
	destType := w.TileAt(npc.X, npc.Y).Type()
	if destType == TilePoison {
		npc.Health -= 15
		npc.Stress += 10
		if npc.Stress > 100 {
			npc.Stress = 100
		}
		w.SetTile(npc.X, npc.Y, MakeTile(TileEmpty, npc.ID)) // consumed on contact
		delete(w.PoisonTTL, w.idx(npc.X, npc.Y))
	}

	// Pick up item if NPC walked onto an item tile
	destType = w.TileAt(npc.X, npc.Y).Type()
	if destType == TileCrystal {
		// Crystal: consumed on pickup, grants permanent gas modifier
		npc.AddMod(Modifier{Kind: ModGas, Mag: 50, Duration: -1, Source: ItemCrystal})
		w.SetTile(npc.X, npc.Y, MakeTile(TileEmpty, npc.ID))
	} else if destType >= TileTool && destType <= TileTreasure && npc.Item == ItemNone {
		npc.Item = destType - TileTool + ItemTool // map tile type to item type
		grantItemModifier(npc, npc.Item)
		w.SetTile(npc.X, npc.Y, MakeTile(TileEmpty, npc.ID))
	}

	// Apply action
	switch action {
	case ActionEat:
		// Eat food at current position or adjacent
		if s.tryEat(npc, npc.X, npc.Y) {
			return
		}
		// Try adjacent tiles
		for _, d := range [][2]int{{0, -1}, {1, 0}, {0, 1}, {-1, 0}} {
			if s.tryEat(npc, npc.X+d[0], npc.Y+d[1]) {
				return
			}
		}
	case ActionAttack:
		targetID := byte(vm.MemRead(64 + Ring1Target))
		for _, other := range w.NPCs {
			if other.ID == targetID && other.Alive() {
				d := abs(other.X-npc.X) + abs(other.Y-npc.Y)
				if d <= 1 {
					dmg := 10 - other.ModSum(ModDefense)
					if dmg < 1 {
						dmg = 1
					}
					other.Health -= dmg
					npc.Energy -= 5
					// Stress: target gets combat stress
					other.Stress += 15
					if other.Stress > 100 {
						other.Stress = 100
					}
				}
			}
		}
	case ActionShare:
		targetID := byte(vm.MemRead(64 + Ring1Target))
		for _, other := range w.NPCs {
			if other.ID == targetID && other.Alive() {
				d := abs(other.X-npc.X) + abs(other.Y-npc.Y)
				if d <= 1 && npc.Energy > 20 {
					npc.Energy -= 10
					other.Energy += 10
				}
			}
		}
	case ActionTrade:
		targetID := byte(vm.MemRead(64 + Ring1Target))
		if npc.Item != ItemNone {
			s.tradeIntents[npc.ID] = targetID
		}
	case ActionCraft:
		// Craft anywhere: free on forge, costs 20 energy off forge
		if npc.Item != ItemNone {
			if output, ok := forgeRecipes[npc.Item]; ok {
				onForge := w.TileAt(npc.X, npc.Y).Type() == TileForge
				if onForge || npc.Energy >= 20 {
					if !onForge {
						npc.Energy -= 20
					}
					removeItemModifier(npc, npc.Item)
					npc.Item = output
					grantItemModifier(npc, npc.Item)
					npc.Fitness += 50
					npc.CraftCount++
				}
			}
		}
	case ActionTeach:
		targetID := byte(vm.MemRead(64 + Ring1Target))
		for _, other := range w.NPCs {
			if other.ID == targetID && other.Alive() {
				d := abs(other.X-npc.X) + abs(other.Y-npc.Y)
				if d <= 1 && npc.Energy >= 10 {
					s.memeticTransfer(npc, other)
					npc.Energy -= 10
				}
			}
		}
	}
}

// resolveTrades matches bilateral trade intents and swaps items.
func (s *Scheduler) resolveTrades() {
	for idA, targetA := range s.tradeIntents {
		targetB, ok := s.tradeIntents[targetA]
		if !ok || targetB != idA {
			continue // not bilateral
		}
		npcA := s.findNPC(idA)
		npcB := s.findNPC(targetA)
		if npcA == nil || npcB == nil {
			continue
		}
		if abs(npcA.X-npcB.X)+abs(npcA.Y-npcB.Y) > 1 {
			continue // must be adjacent
		}
		// Remove old item modifiers, swap items, grant new modifiers
		removeItemModifier(npcA, npcA.Item)
		removeItemModifier(npcB, npcB.Item)
		npcA.Item, npcB.Item = npcB.Item, npcA.Item
		grantItemModifier(npcA, npcA.Item)
		grantItemModifier(npcB, npcB.Item)
		// Scarcity-based gold transfer: value difference flows as gold, plus base reward
		valA := s.World.MarketValue(npcA.Item) // A now holds what B had
		valB := s.World.MarketValue(npcB.Item) // B now holds what A had
		baseGold := 3
		diff := (valA - valB) / 2
		npcA.Gold += baseGold - diff
		npcB.Gold += baseGold + diff
		if npcA.Gold < 0 {
			npcA.Gold = 0
		}
		if npcB.Gold < 0 {
			npcB.Gold = 0
		}
		// Trading relieves stress
		npcA.Stress -= 5
		if npcA.Stress < 0 {
			npcA.Stress = 0
		}
		npcB.Stress -= 5
		if npcB.Stress < 0 {
			npcB.Stress = 0
		}
		s.TradeCount++
		delete(s.tradeIntents, idA)
		delete(s.tradeIntents, targetA)
	}
	// Clear remaining intents
	for k := range s.tradeIntents {
		delete(s.tradeIntents, k)
	}
}

// findNPC returns the NPC with the given ID, or nil.
func (s *Scheduler) findNPC(id byte) *NPC {
	for _, npc := range s.World.NPCs {
		if npc.ID == id {
			return npc
		}
	}
	return nil
}

// memeticTransfer copies a genome fragment from teacher to student.
func (s *Scheduler) memeticTransfer(teacher, student *NPC) {
	// Pick instruction-aligned fragment from teacher (4 bytes)
	points := OpcodeAlignedPoints(teacher.Genome)
	if len(points) < 2 {
		return
	}
	srcIdx := s.World.Rng.Intn(len(points) - 1)
	srcStart := points[srcIdx]
	srcEnd := srcStart + 4
	if srcEnd > len(teacher.Genome) {
		srcEnd = len(teacher.Genome)
	}
	fragment := teacher.Genome[srcStart:srcEnd]

	// Success probability: teacher.Fitness / (teacher.Fitness + student.Fitness + 1)
	prob := float64(teacher.Fitness+1) / float64(teacher.Fitness+student.Fitness+2)
	if s.World.Rng.Float64() > prob {
		return // student resisted
	}

	// Pick random instruction-aligned insertion point in student genome
	studPoints := OpcodeAlignedPoints(student.Genome)
	if len(studPoints) < 2 {
		return
	}
	dstIdx := s.World.Rng.Intn(len(studPoints) - 1)
	dstStart := studPoints[dstIdx]

	// Overwrite (not insert — keeps genome size stable)
	g := make([]byte, len(student.Genome))
	copy(g, student.Genome)
	for i, b := range fragment {
		pos := dstStart + i
		if pos < len(g) {
			g[pos] = b
		}
	}
	student.Genome = g
	student.Taught++

	// Teaching rewards fitness and relieves stress
	teacher.Fitness += 10
	teacher.TeachCount++
	teacher.Stress -= 3
	if teacher.Stress < 0 {
		teacher.Stress = 0
	}
	s.TeachCount++
}

// autoActions makes NPC passively eat food (extended radius with ModForage)
// and auto-craft on forge tiles.
func (s *Scheduler) autoActions(npc *NPC) {
	w := s.World

	// Auto-eat with foraging radius: 1 + ModForage bonus
	if npc.Energy < 200 {
		radius := 1 + npc.ModSum(ModForage)
		if radius > 5 {
			radius = 5 // cap to prevent excessive scanning
		}
		ate := false
		for dy := -radius; dy <= radius && !ate; dy++ {
			for dx := -radius; dx <= radius && !ate; dx++ {
				if abs(dx)+abs(dy) > radius {
					continue // Manhattan distance check
				}
				if s.tryEat(npc, npc.X+dx, npc.Y+dy) {
					ate = true
				}
			}
		}
	}

	// Auto-craft on forge: if on forge tile with a craftable item, craft for free
	if w.TileAt(npc.X, npc.Y).Type() == TileForge && npc.Item != ItemNone {
		if output, ok := forgeRecipes[npc.Item]; ok {
			removeItemModifier(npc, npc.Item)
			npc.Item = output
			grantItemModifier(npc, npc.Item)
			npc.Fitness += 50
			npc.CraftCount++
		}
	}
}

func (s *Scheduler) tryEat(npc *NPC, x, y int) bool {
	w := s.World
	if !w.InBounds(x, y) {
		return false
	}
	t := w.TileAt(x, y)
	if t.Type() == TileFood {
		w.SetTile(x, y, MakeTile(TileEmpty, t.Occupant()))
		npc.Energy += 30
		if npc.Energy > 200 {
			npc.Energy = 200
		}
		npc.Health += 5
		if npc.Health > 100 {
			npc.Health = 100
		}
		npc.FoodEaten++
		npc.Hunger = 0
		// Eating relieves stress
		npc.Stress -= 2
		if npc.Stress < 0 {
			npc.Stress = 0
		}
		return true
	}
	return false
}

// applyModifiers applies per-tick effects from active modifiers.
func applyModifiers(npc *NPC) {
	for _, m := range npc.Mods {
		if m.Duration == 0 {
			continue
		}
		switch m.Kind {
		case ModEnergy:
			npc.Energy += int(m.Mag)
			if npc.Energy > 200 {
				npc.Energy = 200
			}
			if npc.Energy < 0 {
				npc.Energy = 0
			}
		case ModHealth:
			npc.Health += int(m.Mag)
			if npc.Health > 100 {
				npc.Health = 100
			}
		case ModStress:
			npc.Stress += int(m.Mag)
			if npc.Stress > 100 {
				npc.Stress = 100
			}
			if npc.Stress < 0 {
				npc.Stress = 0
			}
		}
	}
}

// decayModifiers ticks down durations and clears expired modifiers.
func decayModifiers(npc *NPC) {
	for i := range npc.Mods {
		if npc.Mods[i].Duration > 0 {
			npc.Mods[i].Duration--
			// Duration just hit 0 → expired
		}
		// Duration == -1 means permanent, no decay
	}
}

// grantItemModifier adds the item's modifier to the NPC.
func grantItemModifier(npc *NPC, item byte) {
	if m, ok := ItemModifiers[item]; ok {
		npc.AddMod(m)
	}
}

// removeItemModifier removes modifiers granted by the given item.
func removeItemModifier(npc *NPC, item byte) {
	if _, ok := ItemModifiers[item]; ok {
		npc.RemoveMod(item)
	}
}

// computeGasBonus calculates the gas bonus with diminishing returns.
func computeGasBonus(modSum int) int {
	bonus := 0
	add := modSum
	for add > 0 {
		if add >= 50 {
			bonus += 50
			add -= 50
			add /= 2
		} else {
			bonus += add
			add = 0
		}
	}
	return bonus
}
