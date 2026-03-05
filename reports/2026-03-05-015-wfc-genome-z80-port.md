# WFC Genome Generation on Z80: Teaching a Spectrum to Dream Better Brains

**Report 015** | 2026-03-05 | psil Z80 sandbox

---

## The Problem: 16 Identical Eat-Loops

The Z80 sandbox (Phase 14) spawns 16 NPCs with identical hardcoded genomes — an 8-byte eat-loop:

```
8A 0D    r0@ food_dir     ; sense food direction
8C 00    r1! move          ; move toward food
21       push 1
8C 01    r1! action        ; eat
F1       yield
F0 F0 F0 F0 F0 F0 F0 F0   ; halt padding
```

All 16 NPCs start with the same brain. Evolution must mutate this single program into diverse specialists — foragers, traders, crafters — from a population of clones. That's a genetic bottleneck. The Go sandbox solved this with WFC genome generation (Report 009), producing structurally diverse brains that evolution can refine. Can we fit the same thing in Z80 assembly?

## The Approach: 8-Type WFC in 470 Lines of Z80

### Token type reduction: 10 → 8

The Go WFC engine uses 10 token types with `uint16` bitmasks. Z80 works in bytes. We merged to 8 types that fit in a single byte bitmask:

| Bit | Type   | Opcodes                      | Bytes |
|-----|--------|------------------------------|-------|
| 0   | Sense  | `r0@ N` (sensor read)        | 2     |
| 1   | Push   | `push 0..7`                  | 1     |
| 2   | Cmp    | `eq`, `lt`, `gt`, `not`      | 1     |
| 3   | Branch | `jnz`, `jz` + offset         | 2     |
| 4   | Move   | `act.move` or `r1! 0`        | 2     |
| 5   | Action | `eat`, `harvest`, `attack`...| 2     |
| 6   | Ops    | `dup`, `+`, `-`, `*`, `swap` | 1     |
| 7   | Yield  | `yield`                      | 1     |

**Merges**: TokStack + TokMath → TokOps (all 1-byte stack/arithmetic); TokTarget → TokAction (both write Ring1 for output).

This means the popcount LUT at $7000 (already used for WFC biome generation), the collapse-random logic, and the constraint propagation all reuse the exact same byte-level patterns from `wfc_biome.asm`. The constraint table is just **8 bytes**.

### Constraint mining: Archetypes, not noise

We initially mined constraints from `warrior999.jsonl` — 200k ticks of evolved genomes. The result was disappointing: nearly every entry was `$FF` (all transitions allowed). Evolved genomes are noisy; 40% is junk DNA. Mining from junk teaches the WFC that anything can follow anything.

Instead, we mined from **4 hand-decoded archetype genomes** (Report 012):
- **Trader** (26 bytes) — sense→compare→branch→move→eat loop with trade sub-routine
- **Forager** (8 bytes) — minimal sense→move→eat→yield
- **Crafter** (30 bytes) — sense items→branch→harvest→craft with fallback foraging
- **Teacher** (40 bytes) — sense→compare→branch→trade→teach with kin-awareness

These produce tight, meaningful constraints:

```
; Bit: 76543210 = Yield,Ops,Action,Move,Branch,Cmp,Push,Sense
Sense(0):   %01110110  → Push, Cmp, Move, Action, Ops
Push(1):    %01100101  → Sense, Cmp, Action, Ops
Cmp(2):     %01001000  → Branch, Ops
Branch(3):  %00110001  → Sense, Move, Action
Move(4):    %10100011  → Sense, Push, Action, Yield
Action(5):  %10010001  → Sense, Move, Yield
Ops(6):     %01000110  → Push, Cmp, Ops
Yield(7):   %00000011  → Sense, Push
```

Each constraint encodes behavioral logic: a comparison must be followed by a branch or another operation (not by a random sensor read). An action leads to another sense-act cycle or a yield. Yield always starts a new cycle with Sense.

### The algorithm: Forward + backward propagation

The 1D WFC engine runs in 5 steps:

1. **Init**: 10 cells, each = `$FF` (all 8 types possible)
2. **Anchor**: cell[0] = bit 0 (Sense), cell[9] = bit 7 (Yield)
3. **Backward propagate**: for each cell N-2 down to 0, intersect with types that can *precede* the next cell
4. **Forward propagate**: for each cell 0 to N-2, intersect with types that can *follow* the current cell
5. **Collapse**: left-to-right, pick random valid bit, re-propagate forward

The backward pass was a discovery, not part of the original plan. With forward-only propagation, anchoring Yield at position 9 didn't constrain earlier cells. Result: 7 out of 50 test seeds hit contradictions (a cell reduced to 0 possibilities). Adding backward propagation — "what types can reach Yield?" working backwards — eliminated all contradictions: **50/50 seeds succeed**.

The `get_predecessors` function is the reverse of `get_allowed`: for each type t, if `constraints[t] AND target != 0`, then t can precede the target. This costs 56 bytes of Z80 code and saves the fallback path in ~14% of cases.

### Render dispatch: PUSH DE; RET

Each collapsed token type maps to a concrete opcode emitter. The render function uses a jump table:

```z80
.wr_loop:
    LD A, (IX+0)              ; collapsed cell (single bit)
    CALL bitmask_to_index     ; A = 0-7

    ADD A, A                  ; type * 2
    LD HL, .wr_table
    ADD HL, DE
    LD E, (HL)
    INC HL
    LD D, (HL)                ; DE = handler address

    POP HL                    ; HL = output pointer
    PUSH HL
    PUSH DE                   ; push handler address
    RET                       ; dispatch!
```

8 handlers emit 1-2 bytes each with randomized concrete opcodes. For example, `.wr_sense` picks from 8 useful sensors (food_dir, npc_dir, npc_dist, item_dir, energy, food_dir_sensor, biome, similarity). `.wr_action` picks from 6 actions (eat, harvest, attack, heal, share, craft) with a 25% chance of using Ring1 write instead.

## The Bug Hunt

### Bug 1: Memory overlap

`WFC_WORK` (used by WFC biome generation) and `GA_SCRATCH` (output buffer) were both at `$6600`. The render function was reading cells from the same memory it was writing bytecode to. Fix: separate `wg_cells` buffer (10 bytes) in the code section, independent of both.

### Bug 2: lfsr_next clobbers HL

The LFSR random number generator loads its state via `LD HL, (lfsr_state)`, destroying whatever HL held. Every render handler used HL as the output pointer, called `lfsr_next` for randomization, then tried to write to `(HL)` — which now pointed at the LFSR state, not the genome buffer.

Symptom: first byte correct ($8A for Sense), then all zeros, with genome lengths of 66-213 bytes instead of ~17.

Root cause: the output pointer was corrupted after the very first `lfsr_next` call. Writes scattered across random memory. The "length" was computed as `HL - GA_SCRATCH` where HL had drifted far from the output buffer.

Fix: every handler now saves HL with `PUSH HL` before calling `lfsr_next` and restores with `POP HL` after.

### Bug 3: Missing jump table

The `.wr_table` jump table was referenced but never defined. The dispatch code loaded addresses from whatever bytes happened to follow the `LD HL, .wr_table` instruction — raw code bytes interpreted as addresses. Combined with the HL clobber bug, this made debugging extremely confusing: the first handler would sometimes work (by accident) but subsequent dispatch was random.

## Results

### Z80 test harness output

5 genomes generated from a single LFSR seed:

```
G1: cells: 01 10 01 20 10 80 02 20 10 80
    bytes: 8A 01 8C 00 8A 12 95 00 93 04 F1 26 95 00 93 01 F1 (17B)
         → Sense, Move, Sense, Action, Move, Yield, Cmp, Action, Move, Yield

G2: cells: 01 10 01 20 10 20 10 20 10 80
    bytes: 8A 01 8C 00 8A 07 8C 01 93 03 9B 00 8C 00 9B 00 8C 00 F1 (19B)
         → Sense, Move, Sense, Action, Move, Action, Move, Action, Move, Yield

G3: cells: 01 40 04 08 20 10 02 01 10 80
    bytes: 8A 0D 0E 10 88 04 97 00 8C 00 26 8A 05 93 06 F1 (16B)
         → Sense, Ops, Cmp, Branch, Action, Move, Cmp, Sense, Move, Yield

G4: cells: 01 10 80 02 01 40 02 01 20 80
    bytes: 8A 07 8C 00 F1 25 8A 12 06 25 8A 12 8C 01 F1 (15B)
         → Sense, Move, Yield, Cmp, Sense, Ops, Cmp, Sense, Action, Yield

G5: cells: 01 02 40 02 40 40 40 02 20 80
    bytes: 8A 0D 22 02 24 03 08 03 25 94 00 F1 (12B)
         → Sense, Cmp, Ops, Cmp, Ops, Ops, Ops, Cmp, Action, Yield
```

Every genome starts with Sense, ends with Yield, and has structurally valid token sequences. Lengths range from 12-19 bytes — variable because 1-byte tokens (Push, Cmp, Ops, Yield) and 2-byte tokens (Sense, Branch, Move, Action) mix differently.

G3 is especially interesting: `Sense → Ops → Cmp → Branch → Action → Move` — it senses, processes, compares, branches on the result, acts, then moves. This is a decision-making pattern that random genomes almost never produce.

### Go comparison: WFC vs pure random

From the Go test suite (200 trials each):

| Pattern | WFC | Random | Ratio |
|---------|-----|--------|-------|
| sense→cmp→branch | **82** | 14 | **5.9x** |

WFC genomes contain the fundamental decision pattern (sense something, compare it, branch on the result) nearly 6x more often than random genomes.

### Full sandbox performance

```
NPC Sandbox Z80 v3
T=16  A=16 F=126 E=1 K=8 H=48
T=32  A=15 F=162 E=2 K=0 H=80
T=64  A=16 F=194 E=0 K=0 H=96
T=128 A=3  ...                    ← first GA culling
T=192 A=5  F=164 E=0 K=0 H=48   ← recovery with evolved WFC offspring
T=256 A=9  F=228 E=0 K=0 H=64   ← stable and growing
```

16 NPCs start with 16 *different* WFC-generated brains instead of 16 identical eat-loops. The population immediately shows behavioral diversity — some NPCs move toward food, others harvest, others explore. When the first GA culling happens at T=128, the surviving genomes have structural variety for crossover to work with.

## Code Budget

| Component | Z80 bytes | Lines |
|-----------|-----------|-------|
| WFC engine (init, propagate, collapse) | ~180 | 200 |
| Backward propagation | ~56 | 45 |
| Render dispatch + 8 handlers | ~190 | 170 |
| Constraint table | 8 | 9 |
| Render data tables (sensors, ops, actions) | 26 | 10 |
| Work cells buffer | 10 | 2 |
| **Total wfc_genome.asm** | **~470** | **470** |

Full sandbox binary: **6,032 bytes** (up from 5,529 in Phase 14, +503 bytes for WFC genome generation). The WFC genome engine is ~8% of the total binary.

## Implementation

| File | What |
|------|------|
| `z80/wfc_genome.asm` | 1D WFC engine, constraint table, render dispatch |
| `z80/test_wfc_genome.asm` | Standalone test harness, generates 5 genomes with hex output |
| `z80/sandbox.asm` | Integration: init_npcs calls wfc_gen_genome |
| `pkg/sandbox/wfc_genome.go` | Go 8-type WFC engine (reference implementation) |
| `pkg/sandbox/wfc_genome_test.go` | 10 Go tests including WFC8 variants |
| `tools/mine_wfc/main.go` | Constraint mining CLI |

```bash
# Assemble and run test harness
sjasmplus z80/test_wfc_genome.asm --raw=z80/build/test_wfc_genome.bin
mzx --run z80/build/test_wfc_genome.bin@8000 --console-io --frames DI:HALT

# Full sandbox with WFC genomes
sjasmplus z80/sandbox.asm --raw=z80/build/sandbox.bin
mzx --run z80/build/sandbox.bin@8000 --console-io --frames DI:HALT

# Go tests
go test ./pkg/sandbox/ -run TestWFC8 -v
```

## What This Means

The Z80 sandbox now starts with the same structural advantage as the Go sandbox: genomes that "look like programs that worked" rather than identical eat-loops. In 470 lines of assembly and 8 bytes of constraint data, a ZX Spectrum can dream diverse, structurally valid brains for its NPCs.

The journey revealed a pattern: the hardest bugs weren't algorithmic (the WFC collapse logic worked first try) but in the Z80 rendering machinery — register clobbering by utility functions (`lfsr_next` destroying HL), missing data tables, and memory overlaps. The WFC algorithm itself is elegant and portable. The devil is in the dispatch.

Next: compare T=256 fitness distributions between WFC-seeded and eat-loop-seeded runs across multiple seeds. The hypothesis: WFC seeding should produce higher peak fitness earlier because crossover has diverse structural material from tick 0.
