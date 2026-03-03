# WFC Genome Generation: Structure-Aware Brain Seeding

**Report 009** | 2026-03-03 | psil sandbox

---

## The Problem: Random Genomes Are Mostly Noise

The GA generates new genomes in two ways: random bytes (`RandomGenome()`) and crossover/mutation of existing genomes. Random genomes use a weighted opcode distribution but have no structural awareness. The 4 handcrafted archetypes (trader, forager, crafter, teacher) provide good starting points, but they're static.

How bad are random genomes? We generated 1000 of each kind and measured structural quality:

- **7% of random genomes start with a sensor read.** A genome that doesn't sense the world first is flying blind — it can't make decisions based on what's around it.
- **35% contain a sense→act pattern** (sensor read followed by an action write within 4 instructions). This is the fundamental unit of NPC behavior: read something, do something about it. Random genomes get this right about a third of the time by chance.
- **0% of random genomes have valid branch instructions.** The weighted opcode distribution in `randomOpcode()` never generates 2-byte branch opcodes (jnz, jz) because they require a valid forward offset in the second byte. Every branch in a random genome jumps to garbage.
- **Token diversity averages 6.1 types** out of 10 possible. Random genomes over-index on push constants (16.3 per genome avg) and under-index on everything else.

Meanwhile, the 4 archetypes score 100% on every structural metric. The gap between "random soup" and "functional brain" is enormous, and evolution must bridge it from scratch.

## The Fix: 1D Wave Function Collapse on Token Types

Instead of generating raw random bytes, we:

1. **Classify opcodes into 10 token types** — Sense, Push, Cmp, Branch, Move, Action, Target, Stack, Math, Yield. This abstracts the 256-value opcode space into functional categories.
2. **Mine bigram constraints** from winning genomes — "what token type follows what." A Sense token is usually followed by a Push or Move. A Cmp is usually followed by a Branch. These are the structural rules that make genomes work.
3. **Run 1D WFC** to generate a token sequence that satisfies all bigram constraints. Left-to-right collapse with forward propagation. First token anchored to Sense, last to Yield.
4. **Render tokens to bytecode** — each token maps to concrete opcodes. Branch offsets are computed by scanning forward to the next Yield token, so branches actually jump to meaningful boundaries.

This is an Estimation of Distribution Algorithm (EDA): instead of random generation, we learn the distribution of instruction patterns from winners and sample from it.

### Constraint Sources

Constraints come from two sources, merged via bitwise OR:

- **Archetype constraints** — mined from the 4 handcrafted genomes. These provide a diversity floor: even if evolved genomes converge narrowly, archetype patterns keep alternative instruction sequences available.
- **Evolved constraints** — mined from the top 25% of the population after each evolution round. These capture patterns that evolution actually discovers work — patterns that may not exist in any archetype.

The merge is always permissive: if either source observed a bigram, WFC allows it.

## Metrics Explained

We evaluated 1000 genomes from each generation method against the 4 archetypes as reference:

### Starts with sensor read

Does the genome's first instruction read a Ring0 sensor slot (`r0@ N`)? A genome that starts by sensing the world can make an informed first decision. A genome that starts with `push 3; add; drop` is doing meaningless stack arithmetic.

| Method | Rate |
|--------|------|
| Archetypes | 100% |
| Random | 7% |
| WFC (archetype constraints) | 30% |
| WFC (bootstrapped) | **99%** |

WFC anchors position 0 to TokSense, so the only failures are WFC contradictions that fall back to RandomGenome.

### Sense→act pattern

Does the genome contain at least one sensor read followed by an action write (move/action/target) within 4 instructions? This is the fundamental behavioral unit: perceive, then respond. The trader archetype has `r0@ 13, r1! 0` (read food direction, write move direction) — that's sense→act.

| Method | Rate |
|--------|------|
| Archetypes | 100% |
| Random | 35% |
| WFC (archetype constraints) | 52% |
| WFC (bootstrapped) | **89%** |

Bootstrapped WFC nearly matches archetypes because the mined constraints encode that Sense tokens are typically followed by Move/Action/Push tokens (which lead to actions).

### Ends with yield

Does the genome's last instruction yield control back to the scheduler? Genomes that don't end with yield/halt either run off the end of their bytecode (hitting NOPs from padding) or get killed by the gas limit. Either way, they waste cycles.

| Method | Rate |
|--------|------|
| Archetypes | 100% |
| Random | 88% |
| WFC (archetype constraints) | 90% |
| WFC (bootstrapped) | **97%** |

Random is already high (88%) because `RandomGenome()` forces the last byte to `OpHalt`. WFC anchors the last token to TokYield and gets the remaining 9% from reduced fallback rate.

### Valid branches

Of all branch instructions (jnz, jz) in the batch, what fraction have forward offsets that land within the bytecode? A branch to offset 0 is an infinite loop. A branch past the end of the genome is undefined behavior.

| Method | Rate |
|--------|------|
| Archetypes | 100% |
| Random | **0%** |
| WFC (archetype constraints) | **100%** |
| WFC (bootstrapped) | **100%** |

This is the single biggest structural win. Random genomes never produce valid branches because `randomOpcode()` generates 1-byte opcodes and branches need carefully computed 2-byte sequences. WFC's `RenderTokens()` computes branch offsets by scanning forward to the next Yield token, producing correct if-then patterns like the archetypes use.

### Token diversity

Average number of unique token types (out of 10) per genome. Higher means the genome uses a wider vocabulary of instructions — sensing, comparing, branching, acting — rather than repeating the same category.

| Method | Diversity |
|--------|-----------|
| Archetypes | 7.0 |
| Random | 6.1 |
| WFC (archetype constraints) | 6.2 |
| WFC (bootstrapped) | **7.2** |

Bootstrapped WFC slightly exceeds archetype diversity because constraint mining from 1000+ genomes discovers more token transitions than 4 archetypes alone can demonstrate.

### Token distribution

Average count of each token type per genome:

| Token | Archetypes | Random | WFC | WFC-boot |
|-------|-----------|--------|-----|----------|
| Sense | 3.75 | 1.98 | 2.59 | **2.86** |
| Push | 3.50 | **16.29** | 13.09 | 2.20 |
| Cmp | 1.25 | 0.96 | 0.90 | **1.49** |
| Branch | 1.25 | 0.00 | 0.18 | 0.16 |
| Move | 2.00 | 0.02 | 0.40 | **0.47** |
| Action | 2.00 | 1.95 | 1.76 | 1.56 |
| Target | 0.50 | 0.02 | 0.43 | **0.90** |
| Stack | 0.00 | 1.54 | 1.16 | 1.73 |
| Math | 0.00 | 2.49 | 1.95 | 1.52 |
| Yield | 2.00 | 2.17 | 2.26 | **2.90** |

The random distribution is dominated by Push tokens (16.3 per genome) because the 0x20-0x3F range is 32 opcodes out of 256 and gets 30% weight. WFC-bootstrapped corrects toward archetype proportions: more sensor reads, more move/target writes, fewer wasted push constants.

## Simulation Results

100 NPCs, 5000 ticks, seed 42, 40x40 world. Both modes use identical archetype seeding for the first 65% of the population — the difference is only in the remaining 35% (random vs WFC) and in refill after evolution rounds (60% WFC / 40% archetype vs 100% archetype).

| Metric | Random-only | WFC-genome | Delta |
|--------|------------|------------|-------|
| Alive | 35 | **37** | +6% |
| Avg fitness | 577 | **626** | +8% |
| Best fitness | 3,942 | **7,269** | **+84%** |
| Trades | 208 | **788** | **+279%** |
| Teaches | 218 | 84 | -62% |

**Best fitness nearly doubles.** The WFC-generated refill NPCs contribute structurally valid brains that evolution can refine rather than discard. The trade explosion (+279%) suggests WFC genomes more often contain the sense→move→trade sequences that the trader archetype demonstrates.

The teach count drops because WFC genomes that successfully trade don't need memetic transfer — they already have good structure. Teaching is most valuable when the recipient genome is bad enough to benefit from overwriting.

## Bootstrapping Effect

The most interesting result is the gap between "WFC with archetype constraints" and "WFC with bootstrapped constraints":

| Metric | Archetype-only | Bootstrapped |
|--------|---------------|--------------|
| Starts with sense | 30% | **99%** |
| Sense→act | 52% | **89%** |
| Token diversity | 6.2 | **7.2** |

Archetype-only WFC uses constraints mined from just 4 genomes. The constraint matrix is sparse — many token transitions were never observed, so WFC falls back to unconstrained random choices. Bootstrapping feeds 100 first-generation WFC genomes back into the miner, filling in the constraint matrix. This is the EDA feedback loop: generate → evaluate structure → mine better constraints → generate better.

In the live simulation, this happens automatically: `GA.Evolve()` mines constraints from the top 25% every evolution round, so WFC constraints improve as the population improves.

## Implementation

| File | What |
|------|------|
| `pkg/sandbox/wfc_genome.go` | Token types, ClassifyOpcode, WFC1D engine, constraint mining, rendering |
| `pkg/sandbox/wfc_genome_test.go` | 10 unit tests + stress test (1000 genomes x 50 seeds) |
| `pkg/sandbox/wfc_genome_eval_test.go` | Structural evaluation + sim fitness comparison |
| `pkg/sandbox/ga.go` | WFCEnabled/Archetypes/MinedConstraints fields, WFCGenome(), UpdateConstraints() |
| `cmd/sandbox/main.go` | `--wfc-genome` flag, 60/40 WFC/archetype refill split |

```bash
# Structural evaluation (1000 genomes, fast)
go test ./pkg/sandbox/... -v -run TestEvalWFCvsRandom

# Sim fitness comparison (100 NPCs, 5000 ticks)
go test ./pkg/sandbox/... -v -run TestEvalWFCSimFitness

# Live run with WFC genome generation
go run ./cmd/sandbox --npcs 200 --ticks 100000 --seed 42 --biomes --wfc-genome --verbose

# Compare WFC vs standard
go run ./cmd/sandbox --npcs 200 --ticks 100000 --seed 42 --biomes --wfc-genome 2>&1 | tail -5
go run ./cmd/sandbox --npcs 200 --ticks 100000 --seed 42 --biomes 2>&1 | tail -5
```

## What This Means

Random genome generation is a cold start problem. Evolution can find good genomes eventually, but most random genomes are structurally defective — no valid branches, no sense→act patterns, too many meaningless push constants. WFC genome generation solves this by encoding structural knowledge (from archetypes and evolved winners) into the generation process itself.

The cost is minimal: WFC generation adds ~2μs per genome (vs ~0.5μs for random). The benefit compounds: structurally valid genomes survive longer, breed more, and give crossover better material to work with. The EDA feedback loop means WFC constraints improve automatically as evolution discovers new patterns.

This is the difference between "generate random programs and hope" and "generate programs that look like programs that worked."
