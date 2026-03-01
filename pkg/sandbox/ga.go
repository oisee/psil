package sandbox

import (
	"math/rand"
	"sort"

	"github.com/psilLang/psil/pkg/micro"
)

const (
	MinGenome = 16
	MaxGenome = 128
)

// GA is the genetic algorithm engine for evolving NPC genomes.
type GA struct {
	Rng          *rand.Rand
	MutationRate float64 // probability of mutation per offspring (0-1)
}

// NewGA creates a GA engine.
func NewGA(rng *rand.Rand) *GA {
	return &GA{
		Rng:          rng,
		MutationRate: 0.8,
	}
}

// Evolve replaces the bottom 25% and any aged-out NPCs with offspring from the top 50%.
func (ga *GA) Evolve(npcs []*NPC) []*NPC {
	if len(npcs) < 4 {
		return npcs
	}

	// Sort by fitness descending
	sorted := make([]*NPC, len(npcs))
	copy(sorted, npcs)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].Fitness > sorted[j].Fitness
	})

	// Top 50% are breeding pool
	poolSize := len(sorted) / 2
	pool := sorted[:poolSize]

	// Collect victims: bottom 25% + any NPC at MaxAge
	replaceCount := len(sorted) / 4
	if replaceCount < 1 {
		replaceCount = 1
	}

	// Mark bottom 25% as victims
	victims := make(map[*NPC]bool)
	for i := 0; i < replaceCount; i++ {
		victims[sorted[len(sorted)-1-i]] = true
	}
	// Also mark aged-out NPCs (even if they're in the top 50%)
	for _, npc := range sorted {
		if npc.Age >= MaxAge {
			victims[npc] = true
		}
	}

	// Generate offspring for all victims
	for victim := range victims {
		parentA := ga.tournamentSelect(pool)
		parentB := ga.tournamentSelect(pool)

		childGenome := ga.crossover(parentA.Genome, parentB.Genome)

		if ga.Rng.Float64() < ga.MutationRate {
			childGenome = ga.mutate(childGenome)
		}

		victim.Genome = childGenome
		victim.Health = 100
		victim.Energy = 100
		victim.Age = 0
		victim.Fitness = 0
		victim.Hunger = 0
		victim.FoodEaten = 0
		victim.Gold = (parentA.Gold + parentB.Gold) / 4 // economic memory persists (diminished)
		victim.Item = ItemNone
		victim.Mods = [4]Modifier{}
		victim.Stress = 0
		victim.CraftCount = 0
		victim.Taught = 0
		victim.TeachCount = 0
	}

	return npcs
}

// tournamentSelect picks the best of 3 random candidates.
func (ga *GA) tournamentSelect(pool []*NPC) *NPC {
	best := pool[ga.Rng.Intn(len(pool))]
	for i := 0; i < 2; i++ {
		c := pool[ga.Rng.Intn(len(pool))]
		if c.Fitness > best.Fitness {
			best = c
		}
	}
	return best
}

// crossover performs instruction-aligned single-point crossover.
func (ga *GA) crossover(a, b []byte) []byte {
	pointsA := OpcodeAlignedPoints(a)
	pointsB := OpcodeAlignedPoints(b)

	if len(pointsA) < 2 || len(pointsB) < 2 {
		// Can't crossover, return copy of better parent (a)
		r := make([]byte, len(a))
		copy(r, a)
		return r
	}

	splitA := pointsA[ga.Rng.Intn(len(pointsA))]
	splitB := pointsB[ga.Rng.Intn(len(pointsB))]

	child := make([]byte, 0, splitA+len(b)-splitB)
	child = append(child, a[:splitA]...)
	child = append(child, b[splitB:]...)

	// Enforce size limits
	if len(child) < MinGenome {
		// Pad with NOPs
		for len(child) < MinGenome {
			child = append(child, micro.OpNop)
		}
	}
	if len(child) > MaxGenome {
		child = child[:MaxGenome]
	}

	return child
}

// opcodeAlignedPoints returns valid instruction boundaries in bytecode.
func OpcodeAlignedPoints(code []byte) []int {
	points := []int{0}
	pc := 0
	for pc < len(code) {
		op := code[pc]
		size := opcodeSize(op, code, pc)
		pc += size
		if pc <= len(code) {
			points = append(points, pc)
		}
	}
	return points
}

// opcodeSize returns the byte size of an instruction at the given position.
func opcodeSize(op byte, code []byte, pc int) int {
	switch {
	case op <= 0x7F:
		return 1
	case micro.Is2ByteOp(op):
		return 2
	case micro.Is3ByteOp(op):
		return 3
	case micro.IsVarLenOp(op):
		if pc+1 < len(code) {
			return 2 + int(code[pc+1])
		}
		return 1
	default:
		return 1 // special ops
	}
}

// mutate applies one random mutation operator.
func (ga *GA) mutate(genome []byte) []byte {
	if len(genome) == 0 {
		return genome
	}

	op := ga.Rng.Intn(6)
	switch op {
	case 0: // Point mutation: replace one byte
		g := make([]byte, len(genome))
		copy(g, genome)
		pos := ga.Rng.Intn(len(g))
		g[pos] = ga.randomOpcode()
		return g

	case 1: // Insert: add 1 random opcode
		if len(genome) >= MaxGenome {
			return genome
		}
		pos := ga.Rng.Intn(len(genome) + 1)
		g := make([]byte, 0, len(genome)+1)
		g = append(g, genome[:pos]...)
		g = append(g, ga.randomOpcode())
		g = append(g, genome[pos:]...)
		return g

	case 2: // Delete: remove 1 byte
		if len(genome) <= MinGenome {
			return genome
		}
		pos := ga.Rng.Intn(len(genome))
		g := make([]byte, 0, len(genome)-1)
		g = append(g, genome[:pos]...)
		g = append(g, genome[pos+1:]...)
		return g

	case 3: // Constant tweak: find a small number or 2-byte op operand and +/- 1
		g := make([]byte, len(genome))
		copy(g, genome)
		// Find tweakable positions: small numbers (0x20-0x3F) and operands of 2-byte ops
		candidates := []int{}
		for i := 0; i < len(g); i++ {
			if micro.IsSmallNum(g[i]) {
				candidates = append(candidates, i)
			} else if micro.Is2ByteOp(g[i]) && i+1 < len(g) {
				candidates = append(candidates, i+1) // operand byte
				i++ // skip operand
			}
		}
		if len(candidates) > 0 {
			idx := candidates[ga.Rng.Intn(len(candidates))]
			if micro.IsSmallNum(g[idx]) {
				// Small number: tweak within range 0x20-0x3F
				if ga.Rng.Intn(2) == 0 && g[idx] < 0x3F {
					g[idx]++
				} else if g[idx] > 0x20 {
					g[idx]--
				}
			} else {
				// 2-byte op operand: tweak +/- 1 within 0-255
				if ga.Rng.Intn(2) == 0 && g[idx] < 0xFF {
					g[idx]++
				} else if g[idx] > 0 {
					g[idx]--
				}
			}
		}
		return g

	case 4: // Block swap: swap two instruction-aligned segments
		points := OpcodeAlignedPoints(genome)
		if len(points) < 4 {
			return genome
		}
		g := make([]byte, len(genome))
		copy(g, genome)
		i := ga.Rng.Intn(len(points) - 1)
		j := ga.Rng.Intn(len(points) - 1)
		if i == j {
			return g
		}
		if i > j {
			i, j = j, i
		}
		segA := g[points[i]:points[i+1]]
		segB := g[points[j]:points[min(j+1, len(points)-1)]]
		tmpA := make([]byte, len(segA))
		copy(tmpA, segA)
		tmpB := make([]byte, len(segB))
		copy(tmpB, segB)
		// Simple: just swap the first bytes
		if len(tmpA) > 0 && len(tmpB) > 0 {
			tmpA[0], tmpB[0] = tmpB[0], tmpA[0]
			copy(g[points[i]:], tmpA)
			copy(g[points[j]:], tmpB)
		}
		return g

	case 5: // Block duplicate: copy a short segment elsewhere
		if len(genome) >= MaxGenome-4 {
			return genome
		}
		points := OpcodeAlignedPoints(genome)
		if len(points) < 3 {
			return genome
		}
		src := ga.Rng.Intn(len(points) - 1)
		end := src + 1
		if end >= len(points) {
			end = len(points) - 1
		}
		seg := genome[points[src]:points[end]]
		if len(genome)+len(seg) > MaxGenome {
			return genome
		}
		dst := ga.Rng.Intn(len(genome) + 1)
		g := make([]byte, 0, len(genome)+len(seg))
		g = append(g, genome[:dst]...)
		g = append(g, seg...)
		g = append(g, genome[dst:]...)
		if len(g) > MaxGenome {
			g = g[:MaxGenome]
		}
		return g
	}
	return genome
}

// randomOpcode returns a random valid 1-byte opcode weighted toward useful ones.
func (ga *GA) randomOpcode() byte {
	// Weighted distribution:
	// 30% commands (0x00-0x1F) - stack ops, math, control flow
	// 30% small numbers (0x20-0x3F) - constants
	// 15% ring ops (r0@, r1!) - sensor reads and action writes
	// 10% inline symbols (0x40-0x5F) - sensor references
	// 10% inline quotations (0x60-0x67) - first 8 quots
	// 5% special (yield, halt)
	r := ga.Rng.Float64()
	switch {
	case r < 0.30:
		return byte(ga.Rng.Intn(0x20))
	case r < 0.60:
		return byte(0x20 + ga.Rng.Intn(0x20))
	case r < 0.75:
		// Ring ops: 50% r0@ (read sensor), 50% r1! (write action)
		if ga.Rng.Intn(2) == 0 {
			return micro.OpRing0R
		}
		return micro.OpRing1W
	case r < 0.85:
		return byte(0x40 + ga.Rng.Intn(0x1A)) // only defined symbols
	case r < 0.95:
		return byte(0x60 + ga.Rng.Intn(8))
	default:
		if ga.Rng.Intn(2) == 0 {
			return micro.OpYield
		}
		return micro.OpHalt
	}
}

// RandomGenome creates a random genome of the given size.
func (ga *GA) RandomGenome(size int) []byte {
	if size < MinGenome {
		size = MinGenome
	}
	if size > MaxGenome {
		size = MaxGenome
	}
	g := make([]byte, size)
	for i := range g {
		g[i] = ga.randomOpcode()
	}
	// Ensure it ends with halt
	g[len(g)-1] = micro.OpHalt
	return g
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
