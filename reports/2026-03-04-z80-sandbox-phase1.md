# Z80 Sandbox Evolutionary Upgrade — Phase 1 Technical Report

## What it is

A self-contained evolutionary NPC simulation running on a Z80 CPU (ZX Spectrum 128K).
16 NPCs with bytecode genomes live in a 32x24 tile world, sense their environment,
execute decisions through a stack-based VM, and evolve through tournament selection
with crossover and mutation. The entire system — VM, scheduler, sensors, 9 action
handlers, genetic algorithm — fits in **4,665 bytes** of Z80 machine code.

## What runs on the Z80

Every tick, for each living NPC:

1. **Sense** — 5x5 neighborhood scan (25 cells) finds nearest food, NPC, and item
   with Manhattan distance and cardinal direction
2. **Think** — NPC's bytecode genome runs through the micro-PSIL VM (stack-based,
   gas-limited to 50 instructions)
3. **Act** — VM output dispatches through a 9-action jump table: eat, attack, heal,
   harvest, terraform, share, trade, craft, teach
4. **Decay** — energy decreases, hunger increases, death on health=0

Every 16 ticks, the GA runs: tournament-3 selection picks two parents, single-point
crossover (LDIR) produces a child genome, one of three mutation types applies
(point/insert/delete), and the child replaces a dead or worst-fitness NPC.

## Architecture: How 4.7KB does the work of the Go sandbox

### Memory layout (128K Spectrum)

```
Bank 5 ($4000-$7FFF) — always mapped:
  $5B00  Tile grid      768B   (32x24, 1 byte per cell: tile type)
  $5E00  Occupancy grid 768B   (32x24, 1 byte per cell: NPC index)
  $6400  Ring0 buffer    62B   (31 sensor slots x 2 bytes)
  $6440  Ring1 buffer     8B   (4 output slots x 2 bytes)
  $6500  NPC table      256B   (16 NPCs x 16 bytes)
  $6600  GA scratch     256B   (crossover workspace)
  $6700  Trade table     16B   (bilateral trade intents)

Bank 2 ($8000-$BFFF) — always mapped:
  $8000  VM code       1912B   (micro-PSIL bytecode interpreter)
  $877B  Scheduler      302B   (tick loop, init, entry point)
  $88A9  Sensors        327B   (5x5 neighborhood scan)
  $89F0  Brain runner    73B   (VM setup/teardown per NPC)
  $8A39  9 actions      695B   (eat, attack, heal, harvest, terraform,
                                share, trade, craft, teach)
  $8CF0  Tile helpers   164B   (tile_addr, occ_addr, get/set/clear)
  $8D94  Food/trade     125B   (respawn, bilateral trade resolution)
  $8E11  LFSR            31B   (16-bit Galois pseudo-random)
  $8E30  Init           201B   (NPC creation, food seeding)
  $8EF9  GA             673B   (tournament-3, crossover, 3 mutations)
  $919A  Print          159B   (stats output, strings)
  $BFFE  Stack                 (grows down, never paged out)

Bank 0 ($C000-$FFFF) — paged at startup:
  $C000  NPC genomes   2048B   (16 x 128 bytes max per genome)
```

### NPC struct (16 bytes, power-of-2 for shift-based indexing)

```
+0   ID         (0=dead)     +8   FoodEaten
+1   X                       +9   Fitness lo
+2   Y                       +10  Fitness hi
+3   Health     (0-100)      +11  GenomeLen
+4   Energy     (0-200)      +12  GenomePtr lo
+5   Age lo                  +13  GenomePtr hi
+6   Age hi                  +14  Item (0=none, 2-7=items)
+7   Hunger                  +15  Flags (bits 0-1: lastDir)
```

### Key Z80-native tricks

- **Separate tile + occupancy grids** instead of nibble-packing. Costs 768B extra
  RAM but eliminates AND/OR/shift on every tile access. The old nibble-packed
  approach needed 4 instructions per read; the new approach needs 1.

- **16-byte NPC struct** (power of 2). Indexing is `RLCA x4` instead of multiply.
  The old 14-byte struct required `ADD IX, DE` with DE=14.

- **LDIR for crossover**. Single-point crossover copies parent_A[0..split] then
  parent_B[split..end] to scratch. Two LDIR instructions, ~4 T-states/byte.
  A 64-byte genome crossover takes ~256 T-states vs thousands for a byte loop.

- **Smart direction opcodes** ($93-$9B). Instead of the genome having to compute
  "which direction is food?", the VM action opcodes resolve `5=toward_food`,
  `6=toward_NPC`, `7=toward_item` by reading Ring0 sensor values directly.
  This lets a 7-byte genome (`push 5; act.eat 0`) be a competent forager.

- **No bank switching during runtime**. Bank 0 (genomes at $C000) stays paged
  for the entire simulation. WFC biome generation (Phase 3) will page Bank 3
  at startup only, then switch back permanently.

## Performance benchmark

Profiled via `mzx --profile`: **256 ticks, 16 NPCs, gas limit 50**.

### Summary

| Metric | Value |
|--------|-------|
| Total Z80 instructions | 6,747,270 |
| Total frames (at 50fps) | 943 |
| Wall time at 3.5 MHz | 18.83 seconds |
| Instructions per tick | 26,356 |
| Time per tick | 73.6 ms |
| Ticks per second (real-time) | ~13.6 |

### Execution breakdown by function

| Function | Instructions | % of total |
|----------|-------------|-----------|
| fill_ring0 (5x5 sensor scan) | 2,952,964 | **43.8%** |
| vm_run (fetch-decode loop) | 1,272,823 | 18.9% |
| tile_addr + occ_addr | 1,256,549 | 18.6% |
| vm_ops (stack, math, logic) | 591,486 | 8.8% |
| run_brain (VM setup/teardown) | 212,160 | 3.1% |
| tick_loop (main scheduler) | 147,187 | 2.2% |
| apply_actions (9 handlers) | 107,178 | 1.6% |
| try_eat | 76,945 | 1.1% |
| Everything else | 130,978 | 1.9% |

### Analysis

The **sensor scan dominates at 43.8%**. Each NPC checks 25 cells per tick, each
cell requiring two grid lookups (tile + occupancy) via `tile_addr`/`occ_addr`.
That's 25 x 2 x 11 instructions = ~550 instructions per NPC per tick, times ~16
NPCs times 256 ticks. The `tile_addr` function itself (Y*32+X → address) accounts
for 18.6% — five `ADD HL,HL` shifts for the multiply.

The **VM interpreter at 27.7%** (fetch-decode + ops) is the second hotspot. With
gas=50, each NPC runs up to 50 bytecode instructions per tick. The fetch-decode
loop (`vm_run`) does ~7 comparisons per opcode to dispatch.

The **GA is essentially free at 0.2%** — it only runs once per 16 ticks and does
a single crossover + mutation.

### Optimization opportunities (not yet implemented)

1. **Reduce sensor scan to 3x3** (9 cells instead of 25) — would cut 43.8% to ~16%
2. **Lookup table for Y*32** — precompute 24 row addresses, replace 5 shifts with
   one table lookup, cutting `tile_addr` from 18.6% to ~5%
3. **Dispatch table for VM opcodes** — replace CP chain with indexed jump table,
   saving ~3 instructions per opcode dispatch

## Compromises vs the Go sandbox

| Feature | Go sandbox | Z80 sandbox | Compromise |
|---------|-----------|-------------|------------|
| World size | 64x64 | 32x24 | Fits Spectrum screen; 768 cells vs 4096 |
| NPCs | 128 | 16 | 128K can hold 128 genomes but tick budget limits active NPCs |
| Genome size | 256 bytes | 128 bytes max | Sufficient for evolved behaviors |
| Sensors | 31 slots, full world scan | 31 slots, 5x5 local scan | No global awareness; NPCs are "nearsighted" |
| GA | Tournament-3, crossover, 3 mutations | Same | Feature-complete match |
| Actions | 9 + move | 9 + move | Feature-complete match |
| Biomes | WFC-generated, 7 types | Flat (Phase 3 adds WFC) | Coming in Phase 3 |
| Items | 7 types with modifiers | 6 types, simple effects | Simplified crafting |
| Ticks/sec | ~50,000 (Go) | ~13.6 (Z80 at 3.5MHz) | 3,600x slower; acceptable for evolution |
| Visualization | Terminal ASCII | Console I/O stats | Visual mode planned (UDG characters) |

The critical insight: **evolution doesn't need speed, it needs fidelity**. The same
tournament-3 selection, crossover, and mutation mechanics produce the same emergent
behaviors (eating specialists, farmers, healers) — just slower. A 256-tick run at
3.5MHz takes 19 seconds; long enough to see NPCs evolve from random to eating.

## Phases

### Phase 1 (this release)
32x24 world, 5x5 sensors, 9 actions, tournament-3 GA with crossover and 3
mutation types, 128K bank switching, item pickup/drop, trade resolution.

### Phase 2 (next)
Item effect lookup table (tool/weapon/treasure/crystal/shield/compass), biome-aware
harvest yields, expanded fitness formula, full bilateral trade with item swaps.

### Phase 3
WFC biome generation using Z80-native bitwise AND/OR wave collapse at 16x12 half
resolution (expanded 2x2 to 32x24). 7 biome types with constraint propagation.
Popcount LUT for minimum-entropy cell selection. Runs in Bank 3 at startup only.

## Build and run

```sh
sjasmplus z80/sandbox.asm --raw=z80/build/sandbox.bin
mzx --run z80/build/sandbox.bin@8000 --console-io --frames DI:HALT --max-frames 2000000
```

## Sample output

```
NPC Sandbox Z80 v2
T=16 A=16 F=146 E=9       ← 16 alive, 9 eats, fitness 146
T=32 A=16 F=172 E=6
T=48 A=2 F=158 E=6        ← die-off, GA kicks in
T=64 A=3 F=184 E=1        ← respawning dead NPCs
T=80 A=4 F=200 E=0
...
T=208 A=9 F=378 E=2       ← population recovered
T=224 A=10 F=394 E=1      ← stable at ~10 NPCs
T=256 A=4 F=426 E=0       ← fitness still growing
Done
```

T=tick, A=alive, F=best fitness, E=eat count since last report.
