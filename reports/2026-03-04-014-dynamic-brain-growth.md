# Dynamic Brain Growth: What Happens When You Uncap the Genome

**Report 014** | 2026-03-04 | psil sandbox

---

## The Problem

By tick ~50k every simulation hits the same wall: 93% of genomes are at the 128-byte cap. Gas is fixed at 200. Evolution has no room to explore longer programs. The genome sparkline flatlines:

```
genomeMax   [40→128]  ▁▂▁▁▁▁▁▁▁▁▁▂▄▆██████████████████████████████████████████████████████████████████
genomeAvg   [24→110]  ▁▁▁▁▁▁▁▁▁▁▂▂▃▃▄▅▆▆▆▅▅▅▅▆▇▇▇▇▇▇▇▆▆▇▆▆▇▇▆▆▇▇▇▆▆▆▆▆▇▆▇▆▇▇▇▆▆▆▆▆▆▆▇▆▆▆▆█▇▅▆▇▇▆▆▇▇▇▆▇
```

We added **dynamic brain growth**: the genome cap and gas limit automatically increase over deep time, simulating an environment where cognitive capacity grows.

---

## The Mechanism

Every 50k ticks, max genome size increases by 64 bytes. Every 70k ticks, gas increases by 10. Both are configurable via flags:

```bash
# Defaults (on by default now)
--genome-grow 64 --genome-grow-every 50000
--gas-grow 10 --gas-grow-every 70000

# Disable growth (reproduce old behavior)
--genome-grow 0 --gas-grow 0
```

At 200k ticks: genome cap 128 → 192 → 256 → 320, gas 200 → 210 → 220.

---

## Seed 999: Growth vs No-Growth

Same seed, same 50 NPCs, same biomes, 200k ticks. The only difference: brain growth on vs off.

| Metric | Growth | No-Growth | Delta |
|--------|--------|-----------|-------|
| alive | 25 | 20 | **+25%** |
| bestFit | 832,249 | 487,199 | **+71%** |
| avgFit | 403,351 | 160,506 | **+151%** |
| trades | 231 | 345 | -33% |
| teaches | 1,461 | 1,841 | -21% |
| harvests | 627,534 | 58,743 | **+968%** |
| terraforms | 30.7M | 24.7M | +24% |
| heals | 9 | 0 | +9 |
| genomeAvg | 312 | 110 | **+184%** |
| genomeMax | 320 | 128 | **+150%** |
| avg stress | 8 | 35 | **-77%** |

The growth run dominates on fitness, survival, and harvesting. The no-growth run has more trades and teaches — brains stuck at 128 bytes rely on social actions, while larger brains discover self-sufficient farming loops.

---

## The Evolved Mega-Brain (320 bytes, Growth Run)

The best NPC (fitness 832,249, age 4,899 ticks) has a 320-byte genome — 2.5x the old cap. Here's what evolution built with the extra room:

```asm
;; SECTION 1: Entry gate (bytes 0-1)
000  push 0
001  jnz +138               ; skip to section 3 (conditional entry)

;; SECTION 2: Terraform-harvest core (bytes 3-48)
003  r0@ ?63                 ; (junk sensor read)
005  act.terraform           ; plant food
007  r1! target
009  r0@ biome               ; check biome
011  act.move →food          ; move toward food
013  act.eat                 ; eat
015  r0@ energy              ; check energy
...
038  quot[3]                 ; (junk)
039  <
040  act.move →food
042  act.eat
044  r0@ energy
046  r0@ tile_type           ; check tile
048  yield                   ; === FIRST YIELD ===

;; SECTION 3: Duplicated farming loop (bytes 49-101)
;; Near-identical to section 2 but with added harvest chains:
068  act.eat
070  act.terraform
072  r0@ near_id             ; read neighbor
074  act.move →food
077  r0@ near_id
079  act.eat
081  act.terraform
083  r0@ near_id
085  r0@ near_id             ; double neighbor read
087  act.eat
089  act.terraform
091  r0@ near_id
093  act.move →food
095  act.eat
097  r0@ energy
099  r0@ tile_type
101  yield                   ; === SECOND YIELD ===

;; SECTION 4: NPC-interaction probe (bytes 102-121)
104  r0@ near_dir            ; locate nearest NPC
106  act.move →food
108  act.eat
110  r0@ ?103                ; (junk sensor)
112  act.terraform
114  r1! action
116  r0@ near_id
118  act.move →food          ; move toward food AND NPC
120  jnz +0                  ; tight loop: no-op branch

;; SECTION 5: Conditional attack gate (bytes 122-163)
122  r0@ energy
124  r0@ tile_type
126  push 0
127  <                       ; tile_type < 0? (never true normally)
128  jnz +138               ; conditional skip
...
152  push 0
153  r0@ near_dir
155  act.move →food
157  act.eat
...
163  yield                   ; === THIRD YIELD ===

;; SECTION 6: Mirror of section 3 (bytes 164-219)
;; Almost byte-for-byte duplication of the farming loop
;; But with act.move →item (byte 240) — explores items too
192  act.move →food
194  dup
195  r0@ near_id
197  act.eat
199  act.terraform
...
219  yield                   ; === FOURTH YIELD ===

;; SECTION 7: Tail — sensor reads, final terraform (bytes 220-317)
;; Reads energy, taught, tile_type, food_dir, self
;; Final act.terraform + act.eat before program ends
```

### What's remarkable:

1. **4 yields per tick** — the brain executes 4 complete action cycles per tick, interleaving terraform/eat/move. The 128-byte brains managed 2-3 yields max.

2. **Gene duplication** — sections 3 and 6 are near-identical 50-byte farming loops. This is textbook gene duplication: copy a working module, let mutations specialize the copy. Section 6 added `act.move →item` — the duplicate explores items while the original stays on food.

3. **968% more harvesting** — the extra gas and genome space let the brain fit more harvest/terraform cycles per tick. The no-growth brain exhausts its gas after 2 yields.

4. **Junk DNA is load-bearing** — sensor reads to invalid slots (r0@ ?63, r0@ ?103) look like junk but push values onto the stack that downstream comparisons consume. Removing them would break the branch logic.

5. **Stress nearly eliminated** — avg stress dropped from 35 to 8. Longer brains with more gas can always find food before energy runs out.

---

## The Capped Brain (128 bytes, No-Growth Run)

The no-growth champion (fitness 487,199) is a different strategy entirely:

```asm
000  act.move 241            ; (junk move direction)
002  act.terraform           ; plant
004  act.eat                 ; eat
006  r0@ energy              ; check energy
008  r0@ energy              ; (redundant read)
010  r0@ biome               ; check biome
012  r0@ ?75                 ; (junk sensor)
014  =                       ; compare
015  act.move →food
017  act.eat
019  nop
020  act.terraform
022  act.eat
...
099  jnz +1                  ; → 102
101  halt                    ; conditional exit
102  act.terraform
104  act.move 152            ; (junk direction)
...
125  nop                     ; end at 126 bytes
```

- **2 yields** (vs 4 in the growth brain)
- **Heavy junk DNA** — 30%+ is nops, redundant sensor reads, and unreachable code
- **No gene duplication** — no room. The 128-byte cap prevents copying working modules.
- **More social** — trades and teaches are higher because the brain can't self-sustain; it needs neighbors for food and items.

---

## Growth Trajectory

```
Tick 50000:  max genome size → 192    (brains start growing past 128)
Tick 70000:  base gas → 210           (more compute per tick)
Tick 100000: max genome size → 256    (gene duplication becomes viable)
Tick 140000: base gas → 220           (3rd yield becomes efficient)
Tick 150000: max genome size → 320    (4-yield brains appear)
```

The genomeAvg sparkline tells the story — a steady climb as evolution fills the new space:

```
genomeAvg   [24→312]  ▁▁▁▁▁▁▂▂▂▂▂▂▂▂▂▂▂▂▁▂▁▂▂▂▃▃▄▄▄▄▄▄▄▄▄▄▄▄▄▄▄▄▅▆▆▆▆▆▆▆▆▅▅▆▆▆▆▆▆▆▆▆▆▅▄▅▅▇▇▇▇▇▇█▇█▇█▇▇
```

Each cap increase triggers a new burst of genome growth. Evolution doesn't waste the space — it fills it with duplicated farming loops that increase per-tick throughput.

---

## The Surprising Balance

The same evolutionary rules, the same world mechanics, the same action opcodes — but different seeds produce qualitatively different civilizations:

- **Seed 999** → warrior culture at 100k, healer transition at 200k, mega-farmer with growth
- **Seed 42** → peaceful traders from the start, compass monoculture
- **Seed 7** → mixed economy, no dominant archetype

The system doesn't favor any strategy. Which one dominates is purely emergent — early stochastic mutations gain fitness advantages, positive feedback loops amplify them into distinct "cultures." Brain growth doesn't change *which* strategy wins; it changes *how far* that strategy can develop.

---

## Reproducibility

All runs are fully deterministic given the same flags:

```bash
# Growth run (new defaults)
go run ./cmd/sandbox --npcs 50 --ticks 200000 --seed 999 --biomes

# Exact reproduction of old behavior
go run ./cmd/sandbox --npcs 50 --ticks 200000 --seed 999 --biomes --genome-grow 0 --gas-grow 0

# Custom growth schedule
go run ./cmd/sandbox --npcs 50 --ticks 500000 --seed 999 --biomes \
    --genome-grow 128 --genome-grow-every 100000 \
    --gas-grow 20 --gas-grow-every 50000
```

Seed 999 with `--genome-grow 0 --gas-grow 0` produces identical output to pre-growth code. The new flags only affect evolution after the first growth event fires.

---

## Flags Reference

| Flag | Default | Description |
|------|---------|-------------|
| `--genome-grow` | 64 | Genome cap increase per step (0=disable) |
| `--genome-grow-every` | 50000 | Ticks between genome cap increases |
| `--gas-grow` | 10 | Gas increase per step (0=disable) |
| `--gas-grow-every` | 70000 | Ticks between gas increases |
| `--snap-every` | 0 | Snapshot interval for reports (e.g. 50000) |
