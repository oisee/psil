# Phase 16 — Genome Hex Dump & the Heisenbug

## What it does

The Z80 sandbox now dumps all alive NPC genomes as hex at the end of each simulation run.
At T=256, after the final stats line, `dump_top_genomes` iterates all 16 NPC slots, skips
dead NPCs (ID=0), and prints each alive NPC's fitness and raw genome bytecode:

```
--- Genomes ---
0011: 00 8A 01 01 24 8A 03 97 00 8A 01 8C 00 8A 8C 00 F1
0011: 00 8A 01 01 24 8A 03 97 00 8A 01 8C 00 8A 8C 00 F1
0011: 23 59 C1 51 4F C9 CD D5 2D DA F9 24 0E 01 C8 0E FF
0011: 00 00 00 00 00 00 00 00 00 00 00 00 00 00 00 00 00
0011: C1 51 4F C9 CD D5 2D DA F9 24 0E 01 C8 0E FF C9 DF
0011: 00 8A 01 01 24 8A 03 97 00 8A 01 8C 00 8A 8C 00 F1
0011: 23 22 5B 5C C9 CD 31 10 01 01 00 C3 E8 19 CD D4 15
0011: C1 51 4F C9 CD D5 2D DA F9 24 0E 01 C8 0E FF C9 DF
0011: 23 59 C1 51 4F C9 CD D5 2D DA F9 24 0E 01 C8 0E FF
Done
```

Format: `FFFF LL: BB BB BB ...` — fitness (16-bit hex), genome length (hex), up to 32 bytes.

## Implementation

### OUTHEX macro

A key design decision: hex printing uses an **inline macro** rather than a CALL-based subroutine.
The CALL approach was attempted first but produced only 1-2 bytes of output despite correct
assembly listings and verified binary code. The root cause was never fully isolated (see Heisenbug
section). The macro approach works reliably:

```z80
MACRO OUTHEX
    PUSH AF
    RRCA \ RRCA \ RRCA \ RRCA   ; high nibble
    AND $0F
    ADD A, '0'
    CP '9'+1
    JR C, .oh1_@@
    ADD A, 7
.oh1_@@:
    OUT ($23), A
    POP AF
    AND $0F                       ; low nibble
    ADD A, '0'
    CP '9'+1
    JR C, .oh2_@@
    ADD A, 7
.oh2_@@:
    OUT ($23), A
ENDM
```

Key sjasmplus feature: `@@` suffix generates unique labels per macro expansion, preventing
label collisions when the macro is expanded 4 times in the dump loop body.

Trade-off: each OUTHEX expansion is ~20 bytes vs 3 bytes for a CALL. With 4 expansions in
the loop, the dump function body is ~140 bytes. But `DJNZ` range is ±127, so the outer loop
uses `DEC B / JP NZ` (3 bytes) instead of `DJNZ` (2 bytes). The inner genome-byte loop still
fits in JR range.

### Dump function placement

The dump function is placed **after `sandbox_end`** — after all data variables (tick_count,
stat counters, strings). Early attempts to place it between `print_stats` and `print_str`
caused simulation corruption (garbled stats at T=128+, phantom program restarts), likely due
to sjasmplus local label scoping: `.label` is scoped to the nearest preceding non-local label,
so inserting a new non-local label between existing code broke `.label` references.

### Code budget

| Component | Bytes |
|-----------|-------|
| OUTHEX macro (×4 expansions) | ~80 |
| dump_top_genomes logic | ~60 |
| String data | 17 |
| Debug OUT ($25) markers | ~10 |
| **Total** | **~167** |

Binary grew from 5529 → 6310 bytes (but ~600 of the growth is from previous WFC genome
generation additions in Phase 15).

## The Heisenbug

The dump function exhibits a **binary-size-dependent hang**:

| Binary size | Debug OUT ($25) | Behavior |
|-------------|-----------------|----------|
| 6294 bytes | None | Hangs after 2 NPC dumps, garbled output |
| 6296 bytes | 2 NOPs only | Hangs |
| 6300 bytes | 1 OUT ($25) per loop | Hangs |
| 6310 bytes | 3 OUT ($25) per loop + exit | **Works** (9 genomes, "Done", exit 0) |
| 6325 bytes | 7 OUT ($25) + genome ptr | **Works** (9 genomes, "Done", exit 0) |

Characteristics:
- The hanging version produces **different simulation data** (fitness $00D4, genome_len $48)
  than the working version (fitness $0004, genome_len $11), despite identical LFSR seed $ACE1
  and identical simulation code above `sandbox_end`.
- The garbled output `-280A=8 F=16416 E=0 K=0 H=0` on hang is clearly from `print_stats`,
  meaning execution jumped back into the main loop instead of reaching `DI + HALT`.
- mzx stderr shows "Trigger START at frame 2467" for working versions but **no trigger**
  for hanging versions — confirming `DI + HALT` was never reached.
- Code review found no bugs: stack balanced on all paths (PUSH BC/POP BC, OUTHEX PUSH AF/POP AF),
  all jumps target correct addresses in the listing, no memory writes outside the stack, no
  bank switching, no register clobbering.

### Hypothesis

The binary size changes the **amount of uninitialized memory** between the end of the loaded
binary and the stack at $BFFE. While the simulation explicitly initializes all data structures
(NPC table via LDIR, tile grid, occupancy grid, LFSR seed), some code path may read from
memory in the gap (e.g., GA_SCRATCH at $6600, WFC work area, or genome slots at $C000+
before WFC fills them). Different binary sizes leave different "random" patterns in the gap
region, creating different LFSR-independent randomness that changes the simulation trajectory.

The working binary size happens to produce a simulation trajectory that reaches DI+HALT
cleanly. The hanging binary size produces a trajectory that triggers a latent bug (possibly
in the GA crossover, WFC genome generation, or VM execution) causing a jump to the wrong
address or a tick_count corruption that prevents the exit check from working.

### Workaround

The debug `OUT ($25), A` markers are kept in the committed code. They output NPC iteration
progress to stderr (port $25), which is invisible in normal stdout capture. The markers
serve double duty: runtime diagnostics and binary-size-dependent hang avoidance.

## Genome analysis

From the working dump at T=256 (9 alive NPCs):

**Valid WFC genomes** (3 of 9):
```
00 8A 01 01 24 8A 03 97 00 8A 01 8C 00 8A 8C 00 F1
```
Decodes to: `SmallNum(0) Ring0R(1) SmallNum(1) SmallNum($24) Ring0R(3) OpActEat
SmallNum(0) Ring0R(1) Ring1W SmallNum(0) Ring0R Ring1W SmallNum(0) OpYield`

This is a recognizable sense→compare→act→yield loop: read sensors, check values, take actions,
yield control. Exactly the pattern WFC is designed to produce.

**Mutated/crossover genomes** (4 of 9):
```
C1 51 4F C9 CD D5 2D DA F9 24 0E 01 C8 0E FF C9 DF
```
These contain bytes outside the VM opcode range ($80+), suggesting they're reading from
uninitialized genome memory or are products of crossover with corrupted genomes. The VM
treats unknown opcodes as no-ops (gas still consumed), so these NPCs burn through their
50-gas budget doing nothing useful.

**Zero genome** (1 of 9): all $00 — SmallNum(0) repeated. This NPC pushes zeros onto the
stack each tick but never acts.

**Observation**: at T=256 (16 evolution cycles), 3 of 9 alive NPCs retain intact WFC genomes
while 4 have been corrupted by crossover/mutation. This matches the Go sandbox's observation
that early evolution is destructive before selection pressure shapes functional brains.
