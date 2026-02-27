package sandbox

import (
	"io"

	"github.com/psilLang/psil/pkg/micro"
)

// DayCycle is the number of ticks in one day cycle.
const DayCycle = 256

// Scheduler runs the sandbox tick loop.
type Scheduler struct {
	World  *World
	Gas    int // gas limit per NPC brain execution
	Output io.Writer

	vm *micro.VM // reusable VM instance
}

// NewScheduler creates a scheduler for the given world.
func NewScheduler(w *World, gas int, output io.Writer) *Scheduler {
	return &Scheduler{
		World:  w,
		Gas:    gas,
		Output: output,
		vm:     micro.New(),
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

		// 4. Decay
		npc.Energy--
		if npc.Energy <= 0 {
			npc.Health -= 5
			npc.Energy = 0
		}
		npc.Age++
		npc.Hunger++
	}

	// Remove dead NPCs
	alive := w.NPCs[:0]
	for _, npc := range w.NPCs {
		if npc.Alive() {
			alive = append(alive, npc)
		} else {
			w.SetTile(npc.X, npc.Y, MakeTile(TileEmpty, 0))
		}
	}
	w.NPCs = alive

	// 5. Respawn food
	w.RespawnFood()

	// 6. Score fitness
	for _, npc := range w.NPCs {
		npc.Fitness = npc.Age + npc.FoodEaten*10 + npc.Health
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
	vm.MemWrite(Ring0Danger, 0) // placeholder
	vm.MemWrite(Ring0Near, int16(w.NearestNPC(npc.X, npc.Y, npc.ID)))
	vm.MemWrite(Ring0X, int16(npc.X))
	vm.MemWrite(Ring0Y, int16(npc.Y))
	vm.MemWrite(Ring0Day, int16(w.Tick%DayCycle))
}

// think runs the NPC's genome on the VM.
func (s *Scheduler) think(npc *NPC) {
	vm := s.vm
	vm.Reset()
	vm.MaxGas = s.Gas
	vm.Gas = s.Gas
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
			// Clear old position
			w.SetTile(npc.X, npc.Y, MakeTile(TileEmpty, 0))
			npc.X = nx
			npc.Y = ny
			w.SetTile(npc.X, npc.Y, MakeTile(w.TileAt(npc.X, npc.Y).Type(), npc.ID))
		}
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
					other.Health -= 10
					npc.Energy -= 5
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
		return true
	}
	return false
}
