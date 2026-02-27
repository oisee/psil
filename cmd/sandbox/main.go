package main

import (
	"flag"
	"fmt"
	"io"
	"math/rand"
	"os"

	"github.com/psilLang/psil/pkg/sandbox"
)

func main() {
	npcs := flag.Int("npcs", 20, "number of NPCs")
	worldSize := flag.Int("world", 32, "world size (NxN)")
	ticks := flag.Int("ticks", 10000, "number of ticks to simulate")
	gas := flag.Int("gas", 200, "gas limit per NPC brain")
	evolveEvery := flag.Int("evolve-every", 100, "ticks between evolution rounds")
	seed := flag.Int64("seed", 42, "random seed")
	verbose := flag.Bool("verbose", false, "verbose output")
	flag.Parse()

	rng := rand.New(rand.NewSource(*seed))

	w := sandbox.NewWorld(*worldSize, rng)
	ga := sandbox.NewGA(rng)

	// Discard NPC brain output (no print spam)
	sched := sandbox.NewScheduler(w, *gas, io.Discard)

	// Spawn initial population with random genomes
	for i := 0; i < *npcs; i++ {
		genome := ga.RandomGenome(24 + rng.Intn(16))
		npc := sandbox.NewNPC(genome)
		npc.X = rng.Intn(*worldSize)
		npc.Y = rng.Intn(*worldSize)
		w.Spawn(npc)
	}

	// Seed some food
	for i := 0; i < *worldSize; i++ {
		x := rng.Intn(*worldSize)
		y := rng.Intn(*worldSize)
		if w.TileAt(x, y).Type() == sandbox.TileEmpty && w.TileAt(x, y).Occupant() == 0 {
			w.SetTile(x, y, sandbox.MakeTile(sandbox.TileFood, 0))
		}
	}

	reportInterval := *evolveEvery
	if reportInterval < 1 {
		reportInterval = 100
	}

	for tick := 0; tick < *ticks; tick++ {
		sched.Tick()

		// Evolution
		if tick > 0 && tick%*evolveEvery == 0 {
			w.NPCs = ga.Evolve(w.NPCs)

			// Respawn dead NPCs if population too low
			for len(w.NPCs) < *npcs/2 {
				genome := ga.RandomGenome(24 + rng.Intn(16))
				npc := sandbox.NewNPC(genome)
				npc.X = rng.Intn(*worldSize)
				npc.Y = rng.Intn(*worldSize)
				w.Spawn(npc)
			}
		}

		// Periodic report
		if *verbose && tick%reportInterval == 0 {
			alive := 0
			totalFit := 0
			bestFit := 0
			var bestNPC *sandbox.NPC
			for _, npc := range w.NPCs {
				if npc.Alive() {
					alive++
					totalFit += npc.Fitness
					if npc.Fitness > bestFit {
						bestFit = npc.Fitness
						bestNPC = npc
					}
				}
			}
			avgFit := 0
			if alive > 0 {
				avgFit = totalFit / alive
			}
			fmt.Fprintf(os.Stderr, "tick=%d alive=%d food=%d avg_fit=%d best_fit=%d\n",
				tick, alive, w.FoodCount(), avgFit, bestFit)
			if *verbose && bestNPC != nil {
				fmt.Fprintf(os.Stderr, "  best_genome=")
				for _, b := range bestNPC.Genome {
					fmt.Fprintf(os.Stderr, "%02x", b)
				}
				fmt.Fprintf(os.Stderr, "\n")
			}
		}

		// Bail if everyone died
		if len(w.NPCs) == 0 {
			fmt.Fprintf(os.Stderr, "Population extinct at tick %d\n", tick)
			break
		}
	}

	// Final report
	fmt.Fprintf(os.Stderr, "\n=== Final Stats (tick %d) ===\n", w.Tick)
	fmt.Fprintf(os.Stderr, "alive=%d food_on_map=%d total_food_spawned=%d\n",
		len(w.NPCs), w.FoodCount(), w.FoodSpawned)

	bestFit := 0
	var bestNPC *sandbox.NPC
	for _, npc := range w.NPCs {
		if npc.Fitness > bestFit {
			bestFit = npc.Fitness
			bestNPC = npc
		}
	}

	if bestNPC != nil {
		fmt.Fprintf(os.Stderr, "best_fitness=%d best_age=%d best_food=%d\n",
			bestNPC.Fitness, bestNPC.Age, bestNPC.FoodEaten)
		fmt.Printf("Best genome: ")
		for _, b := range bestNPC.Genome {
			fmt.Printf("%02x", b)
		}
		fmt.Println()
	}
}
