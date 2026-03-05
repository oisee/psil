// mine_wfc — Mine WFC genome constraints from warrior JSONL files.
//
// Usage: go run tools/mine_wfc/main.go [warrior999.jsonl]
//
// Reads snapshot records, filters top-25% NPCs by fitness, tokenizes
// genomes with 8-type classification, and prints:
//   1. Human-readable constraint matrix
//   2. Z80 assembly DB directives
package main

import (
	"bufio"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"sort"

	"github.com/psilLang/psil/pkg/sandbox"
)

type header struct {
	Type string `json:"type"`
}

type npcRecord struct {
	ID      int    `json:"id"`
	Fitness int    `json:"f"`
	GenLen  int    `json:"gl"`
	Genome  string `json:"gen"`
}

type snapshot struct {
	Type string      `json:"type"`
	Tick int         `json:"t"`
	NPCs []npcRecord `json:"npcs"`
}

func main() {
	path := "warrior999.jsonl"
	if len(os.Args) > 1 {
		path = os.Args[1]
	}

	f, err := os.Open(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "open %s: %v\n", path, err)
		os.Exit(1)
	}
	defer f.Close()

	// Collect all genomes with fitness
	type genomeFit struct {
		genome  []byte
		fitness int
	}
	var all []genomeFit

	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 4*1024*1024), 4*1024*1024) // 4MB buffer for large lines
	lineNo := 0
	for scanner.Scan() {
		lineNo++
		line := scanner.Bytes()

		var h header
		if err := json.Unmarshal(line, &h); err != nil {
			continue
		}
		if h.Type != "full" {
			continue
		}

		var snap snapshot
		if err := json.Unmarshal(line, &snap); err != nil {
			fmt.Fprintf(os.Stderr, "line %d: parse error: %v\n", lineNo, err)
			continue
		}

		for _, npc := range snap.NPCs {
			if npc.Genome == "" || npc.GenLen < 4 {
				continue
			}
			g, err := base64.StdEncoding.DecodeString(npc.Genome)
			if err != nil {
				continue
			}
			if len(g) < 4 {
				continue
			}
			all = append(all, genomeFit{genome: g, fitness: npc.Fitness})
		}
	}
	if err := scanner.Err(); err != nil {
		fmt.Fprintf(os.Stderr, "scan error: %v\n", err)
		os.Exit(1)
	}

	fmt.Fprintf(os.Stderr, "Collected %d genomes from %d lines\n", len(all), lineNo)

	// Sort by fitness descending, take top 25%
	sort.Slice(all, func(i, j int) bool {
		return all[i].fitness > all[j].fitness
	})
	topN := len(all) / 4
	if topN < 10 {
		topN = len(all)
	}
	top := all[:topN]
	fmt.Fprintf(os.Stderr, "Using top %d genomes (fitness >= %d)\n", topN, top[topN-1].fitness)

	// Extract genome bytes
	genomes := make([][]byte, len(top))
	for i, gf := range top {
		genomes[i] = gf.genome
	}

	// Mine 8-type constraints
	mined := sandbox.MineConstraints8(genomes)
	base := sandbox.BaseConstraints8()
	merged := sandbox.MergeConstraints8(mined, base)

	tok8Names := [8]string{"Sense", "Push", "Cmp", "Branch", "Move", "Action", "Ops", "Yield"}

	// Print mined constraint matrix
	fmt.Println("=== Mined constraints (from evolved genomes) ===")
	fmt.Printf("%-8s", "From\\To")
	for _, n := range tok8Names {
		fmt.Printf(" %7s", n)
	}
	fmt.Println()
	for a := 0; a < 8; a++ {
		fmt.Printf("%-8s", tok8Names[a])
		for b := 0; b < 8; b++ {
			if mined[a]&(1<<uint(b)) != 0 {
				fmt.Printf(" %7s", "X")
			} else {
				fmt.Printf(" %7s", ".")
			}
		}
		fmt.Printf("  %%%08b\n", mined[a])
	}

	// Print merged constraint matrix
	fmt.Println("\n=== Merged constraints (mined | base archetypes) ===")
	fmt.Printf("%-8s", "From\\To")
	for _, n := range tok8Names {
		fmt.Printf(" %7s", n)
	}
	fmt.Println()
	for a := 0; a < 8; a++ {
		fmt.Printf("%-8s", tok8Names[a])
		for b := 0; b < 8; b++ {
			if merged[a]&(1<<uint(b)) != 0 {
				fmt.Printf(" %7s", "X")
			} else {
				fmt.Printf(" %7s", ".")
			}
		}
		fmt.Printf("  %%%08b\n", merged[a])
	}

	// Print Z80 assembly DB directives
	fmt.Println("\n; === Z80 WFC genome constraints (8 types, 1 byte each) ===")
	fmt.Println("; Bit: 76543210 = Yield,Ops,Action,Move,Branch,Cmp,Push,Sense")
	fmt.Println("wfc_genome_constraints:")
	for a := 0; a < 8; a++ {
		fmt.Printf("    DB %%%08b    ; %s(%d): ", merged[a], tok8Names[a], a)
		first := true
		for b := 0; b < 8; b++ {
			if merged[a]&(1<<uint(b)) != 0 {
				if !first {
					fmt.Print(", ")
				}
				fmt.Print(tok8Names[b])
				first = false
			}
		}
		fmt.Println()
	}

	// Print token distribution stats
	fmt.Println("\n=== Token distribution in top genomes ===")
	var dist [8]int
	for _, g := range genomes {
		toks := sandbox.TokenizeGenome8(g)
		for _, t := range toks {
			if t < 8 {
				dist[t]++
			}
		}
	}
	total := 0
	for _, c := range dist {
		total += c
	}
	for i := 0; i < 8; i++ {
		pct := float64(dist[i]) / float64(total) * 100
		fmt.Printf("  %s: %d (%.1f%%)\n", tok8Names[i], dist[i], pct)
	}
}
