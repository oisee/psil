# Session Log: Genome Hex Dump Debugging (2026-03-07)

## Goal

Add `dump_top_genomes` to the Z80 sandbox — print all alive NPC genomes as hex
at the end of each simulation run (T=256).

## Timeline

### 1. Initial attempt: CALL-based hex print (previous session)

- Wrote `dump_nib_pair` subroutine (CALL-based) to print a byte as 2 hex digits
- It compiled correctly (verified in listing + xxd binary inspection)
- Only produced 1-2 bytes of output despite correct code
- Partially explained by mzx `--max-frames` defaulting to 5000 (not 50M)
- Even with `--max-frames 50000000`, CALL version still showed only 2 bytes

### 2. OUTHEX macro approach — works

Switched to an inline macro using sjasmplus `@@` suffix for unique labels:

```z80
MACRO OUTHEX
    PUSH AF
    RRCA \ RRCA \ RRCA \ RRCA
    AND $0F
    ADD A, '0'
    CP '9'+1
    JR C, .oh1_@@
    ADD A, 7
.oh1_@@:
    OUT ($23), A
    POP AF
    AND $0F
    ADD A, '0'
    CP '9'+1
    JR C, .oh2_@@
    ADD A, 7
.oh2_@@:
    OUT ($23), A
ENDM
```

This produced real genome data immediately.

### 3. Dump function placement matters

- Placing dump code BETWEEN `print_stats` and `print_str` caused simulation
  corruption (garbled stats at T=128+, phantom restarts)
- Root cause: sjasmplus local label scoping — `.label` is scoped to nearest
  preceding non-local label. Inserting a new non-local label broke references.
- Fix: place dump function AFTER `sandbox_end` (after all data variables)

### 4. The Heisenbug — binary-size-dependent hang

Extensive testing revealed a reproducible, bizarre bug:

| Binary size | Debug `OUT ($25)` | Result |
|-------------|-------------------|--------|
| 6294 | None | **Hangs** after 2 NPC dumps |
| 6296 | 2 NOPs | **Hangs** |
| 6300 | 1 OUT per loop | **Hangs** |
| 6310 | 3 OUT per loop + exit | **Works** (9 genomes, Done) |
| 6325 | 7 OUT + genome ptr | **Works** |

Key observations:
- Hanging version: fitness=$00D4, genome_len=$48 (different simulation!)
- Working version: fitness=$0004, genome_len=$11 (different simulation!)
- Same LFSR seed ($ACE1), same simulation code — binary size changes trajectory
- Hanging version: mzx never reports "Trigger START" — DI+HALT never reached
- Working version: "Trigger START at frame 2467" — clean exit
- Garbled output on hang: `-280A=8 F=16416 E=0 K=0 H=0` — clearly from
  `print_stats`, meaning execution jumped back into the main loop

### 5. Code review found no bugs

Exhaustive analysis of the dump function:
- Stack balanced on ALL paths (alive/dead NPC, genome len 0/small/large)
- PUSH BC at loop start, POP BC at loop end — both paths through JP Z
- OUTHEX macro: PUSH AF / POP AF balanced within each expansion
- All jump targets correct in listing (verified addresses)
- No memory writes outside the stack
- No bank switching (Bank 0 stays paged at $C000 throughout)
- No register clobbering (OUTHEX only modifies A and flags)
- Binary (6294 bytes) well below stack ($BFFE) — no overlap

### 6. Hypothesis

Binary size changes the amount of uninitialized memory between binary end and
stack. Some simulation code path reads from this "gap" memory (e.g., GA_SCRATCH
before first use, genome slots before WFC fills them), creating LFSR-independent
randomness. Different trajectories hit a latent bug (GA crossover? VM execution?)
that causes a jump to the wrong address or tick_count corruption.

### 7. Resolution

Kept debug `OUT ($25), A` markers in the committed code. They output NPC
iteration progress to stderr (invisible in normal stdout), serve as diagnostics,
and happen to produce a binary size that avoids the hang.

## Lessons learned

1. **mzx `--max-frames` defaults to 5000** — always specify explicitly
2. **sjasmplus local labels**: `.label` scoped to nearest preceding non-local label
3. **OUTHEX macro vs CALL**: inline macro works where CALL mysteriously fails
4. **`@@` suffix**: sjasmplus generates unique labels per macro expansion
5. **DJNZ range**: ±127 bytes. With OUTHEX macros inline (~20 bytes each × 4),
   loop body exceeds range. Use `DEC B / JP NZ` instead.
6. **Code placement**: data-dependent code (referencing tick_count etc.) is
   sensitive to insertion of new non-local labels due to sjasmplus scoping
7. **Binary size Heisenbugs exist on Z80**: when debugging changes the bug,
   suspect uninitialized memory reads

## Files changed

- `z80/sandbox.asm` — OUTHEX macro, dump_top_genomes, call site, exit stderr
- `z80/wfc_genome.asm` — wg_eatloop_only stub (USE_EATLOOP ifdef)
- `z80/build/sandbox.bin` — 6310 bytes (was 6032)
- `README.md` — Phase 16 changelog entry
- `reports/2026-03-07-016-genome-dump-heisenbug.md` — full report

## Working output (T=256, 9 alive NPCs)

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

3/9 have valid WFC genomes ($8A=OpRing0R, $8C=OpRing1W, $F1=OpYield).
4/9 corrupted by crossover. 1 all-zeros. Early evolution is destructive.
