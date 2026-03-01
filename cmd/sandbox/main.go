package main

import (
	"encoding/csv"
	"flag"
	"fmt"
	"io"
	"math"
	"math/rand"
	"os"
	"strconv"
	"strings"

	"github.com/psilLang/psil/pkg/sandbox"
)

type timePoint struct {
	tick        int
	alive       int
	trades      int // cumulative
	teaches     int // cumulative
	gold        int // total across alive NPCs
	avgStress   int // 0-100
	food        int // on map
	items       int // on map
	avgFit      int
	bestFit     int
	holders     int // NPCs with items
	crafted     int // shield+compass holders
	crystalNPCs int
}

// Trader genome: goal-based navigation
// If holding item → move toward nearest NPC, trade with them
// Else → move toward food, eat
// Bytecode layout:
//   0-5:   r0@ 15, push 0, >, jnz +8    (check item)
//   6-13:  forage: r0@ 13(food_dir), r1! 0, push 1, r1! 1, yield
//   14-24: trade:  r0@ 18(near_dir), r1! 0, push 4, r1! 1, r0@ 12(near_id), r1! 2, yield
var traderGenome = []byte{
	0x8A, 0x0F, 0x20, 0x0D, 0x88, 0x08, // r0@ 15, push 0, >, jnz +8
	// forage: move toward food, eat (bytes 6-13)
	0x8A, 0x0D, // r0@ 13 (food direction)
	0x8C, 0x00, // r1! 0 (move)
	0x21,       // push 1 (eat)
	0x8C, 0x01, // r1! 1 (action)
	0xF1,       // yield
	// trade: move toward nearest NPC, trade (bytes 14-24)
	0x8A, 0x12, // r0@ 18 (nearest NPC direction)
	0x8C, 0x00, // r1! 0 (move toward them)
	0x24,       // push 4 (ActionTrade)
	0x8C, 0x01, // r1! 1 (action)
	0x8A, 0x0C, // r0@ 12 (nearest NPC ID)
	0x8C, 0x02, // r1! 2 (target)
	0xF1,       // yield
}

// Forager genome: goal-based — move toward food, eat
var foragerGenome = []byte{
	0x8A, 0x0D, // r0@ 13 (food direction)
	0x8C, 0x00, // r1! 0 (move toward food)
	0x21,       // push 1 (eat)
	0x8C, 0x01, // r1! 1 (action=eat)
	0xF1,       // yield
}

// Crafter genome: if on forge AND holding item → craft, else forage
// Bytecode layout:
//   0-5:   r0@ 23(on_forge), push 0, >, jnz +skip_to_craft
//   6-13:  forage: r0@ 13(food_dir), r1! 0, push 1, r1! 1, yield
//   14-19: craft:  r0@ 15(my_item), push 0, >, jnz +do_craft (if holding item)
//   20-24: do_craft: push 5(ActionCraft), r1! 1, yield
// Teacher genome: if holding item AND nearest NPC adjacent → teach, else forage
// Bytecode layout (no unreachable halts — yield ends tick):
//   0-5:   r0@ 15(my_item), push 0, >, jnz +8
//   6-13:  forage: r0@ 13(food_dir), r1! 0, push 1, r1! 1, yield
//   14-19: r0@ 7(near_dist), push 2, <, jnz +8 → teach
//   20-27: move toward NPC, forage: r0@ 18(near_dir), r1! 0, push 1, r1! 1, yield
//   28-39: teach: push 6, r1! 1, r0@ 12(near_id), r1! 2, r0@ 13(food_dir), r1! 0, yield
var teacherGenome = []byte{
	// Check if holding item (bytes 0-5)
	0x8A, 0x0F, // r0@ 15 (Ring0MyItem)
	0x20,       // push 0
	0x0D,       // >
	0x88, 0x08, // jnz +8 → teach check (PC=6, 6+8=14)
	// forage: move toward food, eat (bytes 6-13)
	0x8A, 0x0D, // r0@ 13 (food direction)
	0x8C, 0x00, // r1! 0 (move)
	0x21,       // push 1 (eat)
	0x8C, 0x01, // r1! 1 (action)
	0xF1,       // yield (ends tick)
	// teach check: if nearest NPC dist < 2 → teach (bytes 14-19)
	0x8A, 0x07, // r0@ 7 (Ring0Near)
	0x22,       // push 2
	0x0C,       // < (near_dist < 2 → adjacent)
	0x88, 0x08, // jnz +8 → teach (PC=20, 20+8=28)
	// NPC not adjacent: move toward them (bytes 20-27)
	0x8A, 0x12, // r0@ 18 (nearest NPC direction)
	0x8C, 0x00, // r1! 0 (move)
	0x21,       // push 1 (eat)
	0x8C, 0x01, // r1! 1 (action)
	0xF1,       // yield (ends tick)
	// teach: push ActionTeach, target nearest NPC (bytes 28-39)
	0x26,       // push 6 (ActionTeach)
	0x8C, 0x01, // r1! 1 (action)
	0x8A, 0x0C, // r0@ 12 (nearest NPC ID)
	0x8C, 0x02, // r1! 2 (target)
	0x8A, 0x0D, // r0@ 13 (food direction — move toward food while teaching)
	0x8C, 0x00, // r1! 0 (move)
	0xF1,       // yield
}

var crafterGenome = []byte{
	// Check if on forge
	0x8A, 0x17, // r0@ 23 (Ring0OnForge)
	0x20,       // push 0
	0x0D,       // >
	0x88, 0x08, // jnz +8 → skip to craft check (byte 14)
	// forage: move toward food, eat (bytes 6-13)
	0x8A, 0x0D, // r0@ 13 (food direction)
	0x8C, 0x00, // r1! 0 (move)
	0x21,       // push 1 (eat)
	0x8C, 0x01, // r1! 1 (action)
	0xF1,       // yield
	0xFF,       // halt (unreachable)
	// craft check: if holding item → craft (bytes 14-19)
	0x8A, 0x0F, // r0@ 15 (Ring0MyItem)
	0x20,       // push 0
	0x0D,       // >
	0x88, 0x04, // jnz +4 → do craft (byte 24)
	// no item: forage instead (bytes 20-23)
	0x8A, 0x0D, // r0@ 13 (food direction)
	0x8C, 0x00, // r1! 0 (move)
	// do craft (bytes 24-28)
	0x25,       // push 5 (ActionCraft)
	0x8C, 0x01, // r1! 1 (action)
	0xF1,       // yield
}

func main() {
	npcs := flag.Int("npcs", 20, "number of NPCs")
	worldSize := flag.Int("world", 0, "world size (NxN), 0=auto")
	ticks := flag.Int("ticks", 10000, "number of ticks to simulate")
	gas := flag.Int("gas", 200, "gas limit per NPC brain")
	evolveEvery := flag.Int("evolve-every", 100, "ticks between evolution rounds")
	seed := flag.Int64("seed", 42, "random seed")
	verbose := flag.Bool("verbose", false, "verbose output")
	traderFrac := flag.Float64("traders", 0.25, "fraction of initial population seeded with trader genome")
	snapEvery := flag.Int("snap-every", 0, "print spatial snapshot every N ticks (0=off)")
	timelineEvery := flag.Int("timeline", 0, "sample stats every N ticks for sparkline chart (0=auto ~80 cols)")
	csvOut := flag.Bool("csv", false, "output timeline as CSV to stdout")
	flag.Parse()

	rng := rand.New(rand.NewSource(*seed))

	// Auto-scale world size
	ws := *worldSize
	if ws == 0 {
		ws = sandbox.AutoWorldSize(*npcs)
	}

	w := sandbox.NewWorld(ws, rng)
	w.MaxFood = *npcs * 3
	w.FoodRate = 0.5
	maxItems := *npcs / 2
	if maxItems < 4 {
		maxItems = 4
	}
	w.MaxItems = maxItems
	ga := sandbox.NewGA(rng)

	// Discard NPC brain output (no print spam)
	sched := sandbox.NewScheduler(w, *gas, io.Discard)

	numTraders := int(float64(*npcs) * *traderFrac)
	numForagers := *npcs / 4   // 25% foragers for stable food gathering
	numCrafters := *npcs / 10  // 10% crafters to bootstrap crafting behavior
	numTeachers := *npcs / 20  // 5% teachers for memetic transmission
	if numTeachers < 1 {
		numTeachers = 1
	}

	// Spawn initial population
	for i := 0; i < *npcs; i++ {
		var genome []byte
		if i < numTraders {
			genome = make([]byte, len(traderGenome))
			copy(genome, traderGenome)
		} else if i < numTraders+numForagers {
			genome = make([]byte, len(foragerGenome))
			copy(genome, foragerGenome)
		} else if i < numTraders+numForagers+numCrafters {
			genome = make([]byte, len(crafterGenome))
			copy(genome, crafterGenome)
		} else if i < numTraders+numForagers+numCrafters+numTeachers {
			genome = make([]byte, len(teacherGenome))
			copy(genome, teacherGenome)
		} else {
			genome = ga.RandomGenome(24 + rng.Intn(16))
		}
		npc := sandbox.NewNPC(genome)
		npc.X = rng.Intn(ws)
		npc.Y = rng.Intn(ws)
		// Give seeded traders a starting item
		if i < numTraders {
			npc.Item = byte(sandbox.ItemTool + rng.Intn(3))
		}
		// Give seeded crafters a starting tool
		if i >= numTraders+numForagers && i < numTraders+numForagers+numCrafters {
			npc.Item = sandbox.ItemTool
		}
		// Give seeded teachers a starting item
		if i >= numTraders+numForagers+numCrafters && i < numTraders+numForagers+numCrafters+numTeachers {
			npc.Item = byte(sandbox.ItemTool + rng.Intn(3))
		}
		w.Spawn(npc)
	}

	// Seed some food
	seedFood := ws
	if seedFood < *npcs {
		seedFood = *npcs
	}
	for i := 0; i < seedFood; i++ {
		x := rng.Intn(ws)
		y := rng.Intn(ws)
		if w.TileAt(x, y).Type() == sandbox.TileEmpty && w.OccAt(x, y) == 0 {
			w.SetTile(x, y, sandbox.MakeTile(sandbox.TileFood))
		}
	}

	reportInterval := *evolveEvery
	if reportInterval < 1 {
		reportInterval = 100
	}

	// Timeline sampling interval
	tlEvery := *timelineEvery
	if tlEvery <= 0 {
		tlEvery = *ticks / 80
		if tlEvery < 1 {
			tlEvery = 1
		}
	}
	var timeline []timePoint

	for tick := 0; tick < *ticks; tick++ {
		sched.Tick()

		// Timeline sampling
		if tick%tlEvery == 0 {
			timeline = append(timeline, sampleStats(w, sched, tick))
		}

		// Evolution
		if tick > 0 && tick%*evolveEvery == 0 {
			w.NPCs = ga.Evolve(w.NPCs)

			// Respawn dead NPCs if population too low — mix of traders, foragers, crafters
			refillIdx := 0
			for len(w.NPCs) < *npcs/2 {
				var genome []byte
				switch refillIdx % 6 {
				case 0:
					genome = make([]byte, len(traderGenome))
					copy(genome, traderGenome)
				case 1:
					genome = make([]byte, len(crafterGenome))
					copy(genome, crafterGenome)
				case 2:
					genome = make([]byte, len(teacherGenome))
					copy(genome, teacherGenome)
				default:
					genome = make([]byte, len(foragerGenome))
					copy(genome, foragerGenome)
				}
				npc := sandbox.NewNPC(genome)
				npc.X = rng.Intn(ws)
				npc.Y = rng.Intn(ws)
				if refillIdx%5 == 0 {
					npc.Item = byte(sandbox.ItemTool + rng.Intn(3))
				}
				if refillIdx%5 == 1 {
					npc.Item = sandbox.ItemTool
				}
				w.Spawn(npc)
				refillIdx++
			}
		}

		// Periodic report
		if *verbose && tick%reportInterval == 0 {
			printStatus(w, sched, tick)
		}

		// Spatial snapshot
		if *snapEvery > 0 && tick > 0 && tick%*snapEvery == 0 {
			printSnapshot(w, sched, tick)
		}

		// Bail if everyone died
		if len(w.NPCs) == 0 {
			fmt.Fprintf(os.Stderr, "Population extinct at tick %d\n", tick)
			break
		}
	}

	// Final report
	fmt.Fprintf(os.Stderr, "\n=== Final Stats (tick %d) ===\n", w.Tick)
	fmt.Fprintf(os.Stderr, "alive=%d food_on_map=%d items_on_map=%d total_food_spawned=%d trades=%d teaches=%d\n",
		len(w.NPCs), w.FoodCount(), w.ItemCount(), w.FoodSpawned, sched.TradeCount, sched.TeachCount)

	bestFit := 0
	var bestNPC *sandbox.NPC
	totalGold := 0
	totalStress := 0
	crystalNPCs := 0
	craftedItems := 0
	totalCrafts := 0
	totalTaught := 0
	totalTeachCount := 0
	for _, npc := range w.NPCs {
		if npc.Fitness > bestFit {
			bestFit = npc.Fitness
			bestNPC = npc
		}
		totalGold += npc.Gold
		totalStress += npc.Stress
		totalCrafts += npc.CraftCount
		totalTaught += npc.Taught
		totalTeachCount += npc.TeachCount
		if npc.ModSum(sandbox.ModGas) > 0 {
			crystalNPCs++
		}
		if npc.Item == sandbox.ItemShield || npc.Item == sandbox.ItemCompass {
			craftedItems++
		}
	}

	fmt.Fprintf(os.Stderr, "total_gold=%d crystal_npcs=%d crafted_items=%d total_crafts=%d avg_stress=%d taught=%d teach_count=%d\n",
		totalGold, crystalNPCs, craftedItems, totalCrafts, totalStress/max(len(w.NPCs), 1), totalTaught, totalTeachCount)

	// Item distribution
	itemCounts := make(map[byte]int)
	for _, npc := range w.NPCs {
		if npc.Item != sandbox.ItemNone {
			itemCounts[npc.Item]++
		}
	}
	itemNames := map[byte]string{
		sandbox.ItemTool: "tool", sandbox.ItemWeapon: "weapon", sandbox.ItemTreasure: "treasure",
		sandbox.ItemCrystal: "crystal", sandbox.ItemShield: "shield", sandbox.ItemCompass: "compass",
	}
	fmt.Fprintf(os.Stderr, "item_distribution:")
	for item, count := range itemCounts {
		fmt.Fprintf(os.Stderr, " %s=%d", itemNames[item], count)
	}
	fmt.Fprintln(os.Stderr)

	// Guru leaderboard — top teachers by TeachCount
	type guru struct {
		id         uint16
		teachCount int
		age        int
		fitness    int
	}
	var gurus []guru
	for _, npc := range w.NPCs {
		if npc.TeachCount > 0 {
			gurus = append(gurus, guru{npc.ID, npc.TeachCount, npc.Age, npc.Fitness})
		}
	}
	if len(gurus) > 0 {
		// Sort descending by teach count (simple selection for top 5)
		for i := 0; i < len(gurus) && i < 5; i++ {
			best := i
			for j := i + 1; j < len(gurus); j++ {
				if gurus[j].teachCount > gurus[best].teachCount {
					best = j
				}
			}
			gurus[i], gurus[best] = gurus[best], gurus[i]
		}
		n := len(gurus)
		if n > 5 {
			n = 5
		}
		fmt.Fprintf(os.Stderr, "gurus (%d teachers): ", len(gurus))
		for i := 0; i < n; i++ {
			g := gurus[i]
			fmt.Fprintf(os.Stderr, "NPC#%d(%dx,age=%d,fit=%d) ", g.id, g.teachCount, g.age, g.fitness)
		}
		fmt.Fprintln(os.Stderr)
	}

	if bestNPC != nil {
		fmt.Fprintf(os.Stderr, "best: fitness=%d age=%d food=%d gold=%d item=%d stress=%d gas_bonus=%d\n",
			bestNPC.Fitness, bestNPC.Age, bestNPC.FoodEaten, bestNPC.Gold, bestNPC.Item,
			bestNPC.Stress, bestNPC.ModSum(sandbox.ModGas))
		fmt.Fprintf(os.Stderr, "Best genome: ")
		for _, b := range bestNPC.Genome {
			fmt.Fprintf(os.Stderr, "%02x", b)
		}
		fmt.Fprintln(os.Stderr)
	}

	// Timeline output
	if *csvOut {
		printCSV(timeline, os.Stdout)
	}
	if len(timeline) > 1 {
		printTimeline(timeline, tlEvery)
	}

	// Final snapshot
	printSnapshot(w, sched, w.Tick)
}

func printStatus(w *sandbox.World, sched *sandbox.Scheduler, tick int) {
	alive := 0
	totalFit := 0
	bestFit := 0
	totalGold := 0
	holders := 0
	for _, npc := range w.NPCs {
		if npc.Alive() {
			alive++
			totalFit += npc.Fitness
			totalGold += npc.Gold
			if npc.Item != sandbox.ItemNone {
				holders++
			}
			if npc.Fitness > bestFit {
				bestFit = npc.Fitness
			}
		}
	}
	avgFit := 0
	if alive > 0 {
		avgFit = totalFit / alive
	}
	fmt.Fprintf(os.Stderr, "tick=%d alive=%d food=%d items=%d trades=%d teaches=%d gold=%d holders=%d avg_fit=%d best_fit=%d\n",
		tick, alive, w.FoodCount(), w.ItemCount(), sched.TradeCount, sched.TeachCount, totalGold, holders, avgFit, bestFit)
}

func printSnapshot(w *sandbox.World, sched *sandbox.Scheduler, tick int) {
	fmt.Fprintf(os.Stderr, "\n--- Snapshot at tick %d ---\n", tick)

	// NPC table
	alive := make([]*sandbox.NPC, 0, len(w.NPCs))
	for _, npc := range w.NPCs {
		if npc.Alive() {
			alive = append(alive, npc)
		}
	}

	fmt.Fprintf(os.Stderr, "%-6s %-5s %-5s %-6s %-6s %-5s %-5s %-6s %-7s\n",
		"ID", "X,Y", "HP", "Energy", "Item", "Gold", "Age", "Stress", "Fitness")
	for _, npc := range alive {
		itemNames := []string{"none", "food", "tool", "weapon", "treasure", "crystal", "shield", "compass"}
		itemName := "?"
		if int(npc.Item) < len(itemNames) {
			itemName = itemNames[npc.Item]
		}
		fmt.Fprintf(os.Stderr, "%-6d %2d,%-2d %-5d %-6d %-6s %-5d %-5d %-6d %-7d\n",
			npc.ID, npc.X, npc.Y, npc.Health, npc.Energy, itemName, npc.Gold, npc.Age, npc.Stress, npc.Fitness)
	}

	// Cluster analysis — skip at high population to avoid O(n^2)
	if len(alive) <= 500 {
		clusters := findClusters(alive, 3)
		fmt.Fprintf(os.Stderr, "\nClusters (distance ≤ 3): %d groups\n", len(clusters))
		for i, c := range clusters {
			cx, cy := centroid(c)
			totalGold := 0
			items := 0
			for _, npc := range c {
				totalGold += npc.Gold
				if npc.Item != sandbox.ItemNone {
					items++
				}
			}
			fmt.Fprintf(os.Stderr, "  cluster %d: %d NPCs at ~(%d,%d) gold=%d items=%d\n",
				i+1, len(c), cx, cy, totalGold, items)
		}
	} else {
		fmt.Fprintf(os.Stderr, "\nClusters: skipped (population=%d > 500)\n", len(alive))
	}

	// Mini-map (world grid with NPCs marked)
	if w.Size <= 48 {
		fmt.Fprintf(os.Stderr, "\nMap (%dx%d):\n", w.Size, w.Size)
		for y := 0; y < w.Size; y++ {
			for x := 0; x < w.Size; x++ {
				occ := w.OccAt(x, y)
				typ := w.TileAt(x, y).Type()
				if occ != 0 {
					// Find the NPC to check item
					npc := w.NPCByID(occ)
					if npc != nil && npc.Item != sandbox.ItemNone {
						fmt.Fprint(os.Stderr, "T") // trader (has item)
					} else {
						fmt.Fprint(os.Stderr, "@") // NPC
					}
				} else {
					switch typ {
					case sandbox.TileFood:
						fmt.Fprint(os.Stderr, "f")
					case sandbox.TileTool:
						fmt.Fprint(os.Stderr, "t")
					case sandbox.TileWeapon:
						fmt.Fprint(os.Stderr, "w")
					case sandbox.TileTreasure:
						fmt.Fprint(os.Stderr, "$")
					case sandbox.TileCrystal:
						fmt.Fprint(os.Stderr, "*")
					case sandbox.TileForge:
						fmt.Fprint(os.Stderr, "F")
					case sandbox.TilePoison:
						fmt.Fprint(os.Stderr, "!")
					default:
						fmt.Fprint(os.Stderr, "·")
					}
				}
			}
			fmt.Fprintln(os.Stderr)
		}
		fmt.Fprintf(os.Stderr, "Legend: @=NPC T=NPC+item f=food t=tool w=weapon $=treasure *=crystal F=forge !=poison ·=empty\n")
	}
}

// findClusters groups NPCs by Manhattan proximity using union-find.
func findClusters(npcs []*sandbox.NPC, maxDist int) [][]*sandbox.NPC {
	if len(npcs) == 0 {
		return nil
	}

	parent := make([]int, len(npcs))
	for i := range parent {
		parent[i] = i
	}
	var find func(int) int
	find = func(i int) int {
		if parent[i] != i {
			parent[i] = find(parent[i])
		}
		return parent[i]
	}
	union := func(a, b int) {
		ra, rb := find(a), find(b)
		if ra != rb {
			parent[ra] = rb
		}
	}

	for i := 0; i < len(npcs); i++ {
		for j := i + 1; j < len(npcs); j++ {
			d := int(math.Abs(float64(npcs[i].X-npcs[j].X))) + int(math.Abs(float64(npcs[i].Y-npcs[j].Y)))
			if d <= maxDist {
				union(i, j)
			}
		}
	}

	groups := map[int][]*sandbox.NPC{}
	for i, n := range npcs {
		r := find(i)
		groups[r] = append(groups[r], n)
	}

	result := make([][]*sandbox.NPC, 0, len(groups))
	for _, g := range groups {
		result = append(result, g)
	}
	return result
}

func centroid(npcs []*sandbox.NPC) (int, int) {
	sx, sy := 0, 0
	for _, n := range npcs {
		sx += n.X
		sy += n.Y
	}
	return sx / len(npcs), sy / len(npcs)
}

func sampleStats(w *sandbox.World, sched *sandbox.Scheduler, tick int) timePoint {
	tp := timePoint{
		tick:   tick,
		trades: sched.TradeCount,
		teaches: sched.TeachCount,
		food:   w.FoodCount(),
		items:  w.ItemCount(),
	}
	totalFit := 0
	totalStress := 0
	for _, npc := range w.NPCs {
		if !npc.Alive() {
			continue
		}
		tp.alive++
		totalFit += npc.Fitness
		tp.gold += npc.Gold
		totalStress += npc.Stress
		if npc.Fitness > tp.bestFit {
			tp.bestFit = npc.Fitness
		}
		if npc.Item != sandbox.ItemNone {
			tp.holders++
		}
		if npc.Item == sandbox.ItemShield || npc.Item == sandbox.ItemCompass {
			tp.crafted++
		}
		if npc.ModSum(sandbox.ModGas) > 0 {
			tp.crystalNPCs++
		}
	}
	if tp.alive > 0 {
		tp.avgFit = totalFit / tp.alive
		tp.avgStress = totalStress / tp.alive
	}
	return tp
}

func sparkline(label string, values []int) string {
	blocks := []rune("▁▂▃▄▅▆▇█")
	n := len(values)
	if n == 0 {
		return ""
	}

	lo, hi := values[0], values[0]
	for _, v := range values {
		if v < lo {
			lo = v
		}
		if v > hi {
			hi = v
		}
	}

	var sb strings.Builder
	fmt.Fprintf(&sb, "%-11s [%d→%d]\t", label, values[0], values[n-1])

	span := hi - lo
	for _, v := range values {
		idx := 0
		if span > 0 {
			idx = (v - lo) * (len(blocks) - 1) / span
		}
		sb.WriteRune(blocks[idx])
	}
	return sb.String()
}

func deltas(values []int) []int {
	if len(values) < 2 {
		return nil
	}
	d := make([]int, len(values)-1)
	for i := 1; i < len(values); i++ {
		d[i-1] = values[i] - values[i-1]
		if d[i-1] < 0 {
			d[i-1] = 0
		}
	}
	return d
}

func extractField(timeline []timePoint, fn func(timePoint) int) []int {
	vals := make([]int, len(timeline))
	for i, tp := range timeline {
		vals[i] = fn(tp)
	}
	return vals
}

func printTimeline(timeline []timePoint, interval int) {
	fmt.Fprintf(os.Stderr, "\n=== Timeline (sampled every %d ticks, %d points) ===\n",
		interval, len(timeline))

	type metric struct {
		label string
		fn    func(timePoint) int
		rate  bool // show delta/interval sparkline too
	}
	metrics := []metric{
		{"alive", func(tp timePoint) int { return tp.alive }, false},
		{"trades", func(tp timePoint) int { return tp.trades }, true},
		{"teaches", func(tp timePoint) int { return tp.teaches }, true},
		{"gold", func(tp timePoint) int { return tp.gold }, false},
		{"stress", func(tp timePoint) int { return tp.avgStress }, false},
		{"food", func(tp timePoint) int { return tp.food }, false},
		{"items", func(tp timePoint) int { return tp.items }, false},
		{"avgFit", func(tp timePoint) int { return tp.avgFit }, false},
		{"bestFit", func(tp timePoint) int { return tp.bestFit }, false},
		{"holders", func(tp timePoint) int { return tp.holders }, false},
		{"crafted", func(tp timePoint) int { return tp.crafted }, false},
		{"crystalNPC", func(tp timePoint) int { return tp.crystalNPCs }, false},
	}

	for _, m := range metrics {
		vals := extractField(timeline, m.fn)
		fmt.Fprintln(os.Stderr, sparkline(m.label, vals))
		if m.rate {
			d := deltas(vals)
			if len(d) > 0 {
				fmt.Fprintln(os.Stderr, sparkline(m.label+"/t", d))
			}
		}
	}
}

func printCSV(timeline []timePoint, w io.Writer) {
	cw := csv.NewWriter(w)
	cw.Write([]string{
		"tick", "alive", "trades", "teaches", "gold", "avg_stress",
		"food", "items", "avg_fit", "best_fit", "holders", "crafted", "crystal_npcs",
	})
	for _, tp := range timeline {
		cw.Write([]string{
			strconv.Itoa(tp.tick),
			strconv.Itoa(tp.alive),
			strconv.Itoa(tp.trades),
			strconv.Itoa(tp.teaches),
			strconv.Itoa(tp.gold),
			strconv.Itoa(tp.avgStress),
			strconv.Itoa(tp.food),
			strconv.Itoa(tp.items),
			strconv.Itoa(tp.avgFit),
			strconv.Itoa(tp.bestFit),
			strconv.Itoa(tp.holders),
			strconv.Itoa(tp.crafted),
			strconv.Itoa(tp.crystalNPCs),
		})
	}
	cw.Flush()
}
