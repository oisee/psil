package sandbox

import (
	"fmt"
	"math/rand"
	"testing"
)

// genomeStats holds structural metrics for a batch of genomes.
type genomeStats struct {
	count          int
	avgLen         float64
	avgTokens      float64
	avgSense       float64 // avg sensor reads per genome
	avgAct         float64 // avg action writes (move+action+target)
	avgBranch      float64
	avgYield       float64
	senseActPairs  float64 // genomes with at least one sense→act pattern
	startsWithSens float64 // fraction starting with sensor read
	endsWithYield  float64 // fraction ending with yield/halt
	validBranches  float64 // fraction of branches with valid forward offsets
	tokenDiversity float64 // avg unique token types per genome
}

func analyzeGenomes(genomes [][]byte) genomeStats {
	s := genomeStats{count: len(genomes)}
	totalLen, totalTok := 0, 0
	totalSense, totalAct, totalBranch, totalYield := 0, 0, 0, 0
	senseActCount := 0
	sensStart, yieldEnd := 0, 0
	totalValidBr, totalBr := 0, 0
	totalDiversity := 0

	for _, g := range genomes {
		totalLen += len(g)
		tokens := TokenizeGenome(g)
		totalTok += len(tokens)

		seen := make(map[TokenType]bool)
		hasSenseAct := false
		for i, tok := range tokens {
			seen[tok] = true
			switch tok {
			case TokSense:
				totalSense++
			case TokMove, TokAction, TokTarget:
				totalAct++
			case TokBranch:
				totalBranch++
			case TokYield:
				totalYield++
			}
			// Check sense→(something)→act pattern
			if tok == TokSense && i+1 < len(tokens) {
				for j := i + 1; j < len(tokens) && j <= i+4; j++ {
					if tokens[j] == TokMove || tokens[j] == TokAction || tokens[j] == TokTarget {
						hasSenseAct = true
						break
					}
				}
			}
		}
		if hasSenseAct {
			senseActCount++
		}
		totalDiversity += len(seen)

		if len(tokens) > 0 && tokens[0] == TokSense {
			sensStart++
		}
		if len(tokens) > 0 && tokens[len(tokens)-1] == TokYield {
			yieldEnd++
		}

		// Check branch validity
		pc := 0
		for pc < len(g) {
			op := g[pc]
			size := opcodeSize(op, g, pc)
			if op == 0x88 || op == 0x87 || op == 0x85 { // jnz, jz, jmp
				totalBr++
				if pc+1 < len(g) {
					offset := int(g[pc+1])
					target := pc + 2 + offset
					if target <= len(g) && offset > 0 {
						totalValidBr++
					}
				}
			}
			pc += size
		}
	}

	n := float64(len(genomes))
	s.avgLen = float64(totalLen) / n
	s.avgTokens = float64(totalTok) / n
	s.avgSense = float64(totalSense) / n
	s.avgAct = float64(totalAct) / n
	s.avgBranch = float64(totalBranch) / n
	s.avgYield = float64(totalYield) / n
	s.senseActPairs = float64(senseActCount) / n
	s.startsWithSens = float64(sensStart) / n
	s.endsWithYield = float64(yieldEnd) / n
	if totalBr > 0 {
		s.validBranches = float64(totalValidBr) / float64(totalBr)
	}
	s.tokenDiversity = float64(totalDiversity) / n
	return s
}

func printStats(label string, s genomeStats) {
	fmt.Printf("\n=== %s (%d genomes) ===\n", label, s.count)
	fmt.Printf("  avgLen:          %.1f bytes\n", s.avgLen)
	fmt.Printf("  avgTokens:       %.1f\n", s.avgTokens)
	fmt.Printf("  avgSense:        %.2f per genome\n", s.avgSense)
	fmt.Printf("  avgAct:          %.2f per genome (move+action+target)\n", s.avgAct)
	fmt.Printf("  avgBranch:       %.2f per genome\n", s.avgBranch)
	fmt.Printf("  avgYield:        %.2f per genome\n", s.avgYield)
	fmt.Printf("  senseActPairs:   %.0f%% have sense→act within 4 tokens\n", s.senseActPairs*100)
	fmt.Printf("  startsWithSense: %.0f%%\n", s.startsWithSens*100)
	fmt.Printf("  endsWithYield:   %.0f%%\n", s.endsWithYield*100)
	fmt.Printf("  validBranches:   %.0f%%\n", s.validBranches*100)
	fmt.Printf("  tokenDiversity:  %.1f unique types per genome\n", s.tokenDiversity)
}

func TestEvalWFCvsRandom(t *testing.T) {
	const N = 1000
	rng := rand.New(rand.NewSource(42))

	archetypes := [][]byte{
		testTraderGenome, testForagerGenome, testCrafterGenome, testTeacherGenome,
	}

	// Generate random genomes
	gaRand := NewGA(rand.New(rand.NewSource(42)))
	randomGenomes := make([][]byte, N)
	for i := range randomGenomes {
		randomGenomes[i] = gaRand.RandomGenome(24 + rng.Intn(16))
	}

	// Generate WFC genomes
	gaWFC := NewGA(rand.New(rand.NewSource(42)))
	gaWFC.WFCEnabled = true
	gaWFC.Archetypes = archetypes
	gaWFC.UpdateConstraints(archetypes)
	wfcGenomes := make([][]byte, N)
	for i := range wfcGenomes {
		wfcGenomes[i] = gaWFC.WFCGenome(24 + rng.Intn(16))
	}

	// Also generate with evolved constraints (mine from WFC genomes bootstrapped from archetypes)
	gaWFC2 := NewGA(rand.New(rand.NewSource(99)))
	gaWFC2.WFCEnabled = true
	gaWFC2.Archetypes = archetypes
	// Bootstrap: mine from first batch of WFC genomes + archetypes
	bootstrap := append(archetypes, wfcGenomes[:100]...)
	gaWFC2.UpdateConstraints(bootstrap)
	wfcGenomes2 := make([][]byte, N)
	for i := range wfcGenomes2 {
		wfcGenomes2[i] = gaWFC2.WFCGenome(24 + rng.Intn(16))
	}

	rs := analyzeGenomes(randomGenomes)
	ws := analyzeGenomes(wfcGenomes)
	ws2 := analyzeGenomes(wfcGenomes2)
	as := analyzeGenomes(archetypes)

	printStats("Archetypes (reference)", as)
	printStats("Random genomes", rs)
	printStats("WFC genomes (archetype constraints)", ws)
	printStats("WFC genomes (bootstrapped constraints)", ws2)

	// Token distribution comparison
	fmt.Printf("\n=== Token type distribution (per genome avg) ===\n")
	fmt.Printf("%-10s %8s %8s %8s %8s\n", "Token", "Archtyp", "Random", "WFC", "WFC-boot")
	for tok := TokenType(0); tok < NumTokenTypes; tok++ {
		names := []string{"Sense", "Push", "Cmp", "Branch", "Move", "Action", "Target", "Stack", "Math", "Yield"}
		ar, rr, wr, wr2 := countToken(archetypes, tok), countToken(randomGenomes, tok), countToken(wfcGenomes, tok), countToken(wfcGenomes2, tok)
		fmt.Printf("%-10s %8.2f %8.2f %8.2f %8.2f\n", names[tok], ar, rr, wr, wr2)
	}

	// Structural assertions: WFC should outperform random on key metrics
	if ws.startsWithSens < rs.startsWithSens {
		t.Errorf("WFC startsWithSense (%.0f%%) should be >= random (%.0f%%)", ws.startsWithSens*100, rs.startsWithSens*100)
	}
	if ws.senseActPairs < rs.senseActPairs {
		t.Errorf("WFC senseActPairs (%.0f%%) should be >= random (%.0f%%)", ws.senseActPairs*100, rs.senseActPairs*100)
	}
}

func countToken(genomes [][]byte, tok TokenType) float64 {
	total := 0
	for _, g := range genomes {
		for _, t := range TokenizeGenome(g) {
			if t == tok {
				total++
			}
		}
	}
	return float64(total) / float64(len(genomes))
}

func TestEvalWFCSimFitness(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping sim fitness eval in short mode")
	}

	const (
		npcs      = 100
		ticks     = 5000
		worldSize = 40
	)

	type trial struct {
		label string
		wfc   bool
	}
	trials := []trial{
		{"random-only", false},
		{"wfc-genome", true},
	}

	archetypes := [][]byte{
		testTraderGenome, testForagerGenome, testCrafterGenome, testTeacherGenome,
	}

	fmt.Printf("\n=== Sim fitness comparison (%d NPCs, %d ticks) ===\n", npcs, ticks)
	fmt.Printf("%-14s %8s %8s %8s %8s %8s\n", "Mode", "Alive", "AvgFit", "BestFit", "Trades", "Teaches")

	for _, tr := range trials {
		rng := rand.New(rand.NewSource(42))
		w := NewWorld(worldSize, rng)
		w.MaxFood = npcs * 3
		w.FoodRate = 0.5
		w.MaxItems = npcs / 2

		ga := NewGA(rng)
		if tr.wfc {
			ga.WFCEnabled = true
			ga.Archetypes = archetypes
			ga.UpdateConstraints(archetypes)
		}

		sched := NewScheduler(w, 200, devNull{})

		// Seed initial population: 25% trader, 25% forager, 10% crafter, 5% teacher, rest random/WFC
		numTraders := npcs / 4
		numForagers := npcs / 4
		numCrafters := npcs / 10
		numTeachers := npcs / 20
		for i := 0; i < npcs; i++ {
			var genome []byte
			switch {
			case i < numTraders:
				genome = copyGenome(testTraderGenome)
			case i < numTraders+numForagers:
				genome = copyGenome(testForagerGenome)
			case i < numTraders+numForagers+numCrafters:
				genome = copyGenome(testCrafterGenome)
			case i < numTraders+numForagers+numCrafters+numTeachers:
				genome = copyGenome(testTeacherGenome)
			default:
				if tr.wfc {
					genome = ga.WFCGenome(24 + rng.Intn(16))
				} else {
					genome = ga.RandomGenome(24 + rng.Intn(16))
				}
			}
			npc := NewNPC(genome)
			npc.X = rng.Intn(worldSize)
			npc.Y = rng.Intn(worldSize)
			if i < numTraders {
				npc.Item = byte(ItemTool + rng.Intn(3))
			}
			w.Spawn(npc)
		}

		// Seed food
		for i := 0; i < npcs; i++ {
			x, y := rng.Intn(worldSize), rng.Intn(worldSize)
			if w.TileAt(x, y).Type() == TileEmpty && w.OccAt(x, y) == 0 {
				w.SetTile(x, y, MakeTile(TileFood))
			}
		}

		// Run simulation
		for tick := 0; tick < ticks; tick++ {
			sched.Tick()
			if tick > 0 && tick%100 == 0 {
				w.NPCs = ga.Evolve(w.NPCs)
				// Refill
				refillIdx := 0
				for len(w.NPCs) < npcs/2 {
					var genome []byte
					if tr.wfc && refillIdx%5 < 3 {
						genome = ga.WFCGenome(24 + rng.Intn(16))
					} else {
						switch refillIdx % 4 {
						case 0:
							genome = copyGenome(testTraderGenome)
						case 1:
							genome = copyGenome(testForagerGenome)
						case 2:
							genome = copyGenome(testCrafterGenome)
						default:
							genome = copyGenome(testTeacherGenome)
						}
					}
					npc := NewNPC(genome)
					npc.X = rng.Intn(worldSize)
					npc.Y = rng.Intn(worldSize)
					w.Spawn(npc)
					refillIdx++
				}
			}
		}

		// Collect results
		totalFit, bestFit := 0, 0
		for _, npc := range w.NPCs {
			totalFit += npc.Fitness
			if npc.Fitness > bestFit {
				bestFit = npc.Fitness
			}
		}
		avgFit := 0
		if len(w.NPCs) > 0 {
			avgFit = totalFit / len(w.NPCs)
		}

		fmt.Printf("%-14s %8d %8d %8d %8d %8d\n",
			tr.label, len(w.NPCs), avgFit, bestFit, sched.TradeCount, sched.TeachCount)
	}
}

type simTrialResult struct {
	alive, avgFit, bestFit, trades, teaches int
	genomeAvg                               int
}

func runTrialSim(npcs, ticks, worldSize int, wfc bool, maxGenome int, seed int64) simTrialResult {
	archetypes := [][]byte{
		testTraderGenome, testForagerGenome, testCrafterGenome, testTeacherGenome,
	}

	rng := rand.New(rand.NewSource(seed))
	w := NewWorld(worldSize, rng)
	w.MaxFood = npcs * 3
	w.FoodRate = 0.5
	w.MaxItems = npcs / 2

	ga := NewGA(rng)
	ga.MaxGenomeSize = maxGenome
	if wfc {
		ga.WFCEnabled = true
		ga.Archetypes = archetypes
		ga.UpdateConstraints(archetypes)
	}

	sched := NewScheduler(w, 200, devNull{})

	numTraders := npcs / 4
	numForagers := npcs / 4
	numCrafters := npcs / 10
	numTeachers := npcs / 20
	for i := 0; i < npcs; i++ {
		var genome []byte
		switch {
		case i < numTraders:
			genome = copyGenome(testTraderGenome)
		case i < numTraders+numForagers:
			genome = copyGenome(testForagerGenome)
		case i < numTraders+numForagers+numCrafters:
			genome = copyGenome(testCrafterGenome)
		case i < numTraders+numForagers+numCrafters+numTeachers:
			genome = copyGenome(testTeacherGenome)
		default:
			if wfc {
				genome = ga.WFCGenome(24 + rng.Intn(16))
			} else {
				genome = ga.RandomGenome(24 + rng.Intn(16))
			}
		}
		npc := NewNPC(genome)
		npc.X = rng.Intn(worldSize)
		npc.Y = rng.Intn(worldSize)
		if i < numTraders {
			npc.Item = byte(ItemTool + rng.Intn(3))
		}
		w.Spawn(npc)
	}

	for i := 0; i < npcs; i++ {
		x, y := rng.Intn(worldSize), rng.Intn(worldSize)
		if w.TileAt(x, y).Type() == TileEmpty && w.OccAt(x, y) == 0 {
			w.SetTile(x, y, MakeTile(TileFood))
		}
	}

	for tick := 0; tick < ticks; tick++ {
		sched.Tick()
		if tick > 0 && tick%100 == 0 {
			w.NPCs = ga.Evolve(w.NPCs)
			refillIdx := 0
			for len(w.NPCs) < npcs/2 {
				var genome []byte
				if wfc && refillIdx%5 < 3 {
					genome = ga.WFCGenome(24 + rng.Intn(16))
				} else {
					switch refillIdx % 4 {
					case 0:
						genome = copyGenome(testTraderGenome)
					case 1:
						genome = copyGenome(testForagerGenome)
					case 2:
						genome = copyGenome(testCrafterGenome)
					default:
						genome = copyGenome(testTeacherGenome)
					}
				}
				npc := NewNPC(genome)
				npc.X = rng.Intn(worldSize)
				npc.Y = rng.Intn(worldSize)
				w.Spawn(npc)
				refillIdx++
			}
		}
	}

	r := simTrialResult{
		alive:  len(w.NPCs),
		trades: sched.TradeCount,
		teaches: sched.TeachCount,
	}
	totalFit, totalGenome := 0, 0
	for _, npc := range w.NPCs {
		totalFit += npc.Fitness
		if npc.Fitness > r.bestFit {
			r.bestFit = npc.Fitness
		}
		totalGenome += len(npc.Genome)
	}
	if r.alive > 0 {
		r.avgFit = totalFit / r.alive
		r.genomeAvg = totalGenome / r.alive
	}
	return r
}

func TestEvalBrainSizeSweep(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping brain size sweep in short mode")
	}

	const (
		npcs      = 100
		ticks     = 10000
		worldSize = 40
		seed      = 42
	)

	sizes := []int{32, 64, 128, 256, 512}

	fmt.Printf("\n=== Brain Size Sweep: WFC genome (%d NPCs, %d ticks, seed %d) ===\n", npcs, ticks, seed)
	fmt.Printf("%-10s %6s %7s %8s %7s %7s %8s\n",
		"MaxGenome", "Alive", "AvgFit", "BestFit", "Trades", "Teach", "GenAvg")

	for _, sz := range sizes {
		r := runTrialSim(npcs, ticks, worldSize, true, sz, seed)
		fmt.Printf("%-10d %6d %7d %8d %7d %7d %8d\n",
			sz, r.alive, r.avgFit, r.bestFit, r.trades, r.teaches, r.genomeAvg)
	}

	fmt.Printf("\n=== Brain Size Sweep: Random genome (%d NPCs, %d ticks, seed %d) ===\n", npcs, ticks, seed)
	fmt.Printf("%-10s %6s %7s %8s %7s %7s %8s\n",
		"MaxGenome", "Alive", "AvgFit", "BestFit", "Trades", "Teach", "GenAvg")

	for _, sz := range sizes {
		r := runTrialSim(npcs, ticks, worldSize, false, sz, seed)
		fmt.Printf("%-10d %6d %7d %8d %7d %7d %8d\n",
			sz, r.alive, r.avgFit, r.bestFit, r.trades, r.teaches, r.genomeAvg)
	}
}

func TestEvalBrainSizeMultiSeed(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping multi-seed brain size eval in short mode")
	}

	const (
		npcs      = 100
		ticks     = 10000
		worldSize = 40
		numSeeds  = 5
	)

	sizes := []int{32, 64, 128, 256, 512}
	seeds := []int64{42, 123, 456, 789, 1337}

	fmt.Printf("\n=== Brain Size × WFC: Multi-Seed Average (%d NPCs, %d ticks, %d seeds) ===\n",
		npcs, ticks, numSeeds)
	fmt.Printf("%-10s %6s %7s %8s %7s %7s %8s\n",
		"MaxGenome", "Alive", "AvgFit", "BestFit", "Trades", "Teach", "GenAvg")

	for _, sz := range sizes {
		var total simTrialResult
		for _, seed := range seeds {
			r := runTrialSim(npcs, ticks, worldSize, true, sz, seed)
			total.alive += r.alive
			total.avgFit += r.avgFit
			total.bestFit += r.bestFit
			total.trades += r.trades
			total.teaches += r.teaches
			total.genomeAvg += r.genomeAvg
		}
		n := numSeeds
		fmt.Printf("%-10d %6d %7d %8d %7d %7d %8d\n",
			sz, total.alive/n, total.avgFit/n, total.bestFit/n,
			total.trades/n, total.teaches/n, total.genomeAvg/n)
	}

	fmt.Printf("\n=== Brain Size × Random: Multi-Seed Average (%d NPCs, %d ticks, %d seeds) ===\n",
		npcs, ticks, numSeeds)
	fmt.Printf("%-10s %6s %7s %8s %7s %7s %8s\n",
		"MaxGenome", "Alive", "AvgFit", "BestFit", "Trades", "Teach", "GenAvg")

	for _, sz := range sizes {
		var total simTrialResult
		for _, seed := range seeds {
			r := runTrialSim(npcs, ticks, worldSize, false, sz, seed)
			total.alive += r.alive
			total.avgFit += r.avgFit
			total.bestFit += r.bestFit
			total.trades += r.trades
			total.teaches += r.teaches
			total.genomeAvg += r.genomeAvg
		}
		n := numSeeds
		fmt.Printf("%-10d %6d %7d %8d %7d %7d %8d\n",
			sz, total.alive/n, total.avgFit/n, total.bestFit/n,
			total.trades/n, total.teaches/n, total.genomeAvg/n)
	}
}

func copyGenome(g []byte) []byte {
	c := make([]byte, len(g))
	copy(c, g)
	return c
}

type devNull struct{}

func (devNull) Write(p []byte) (int, error) { return len(p), nil }
