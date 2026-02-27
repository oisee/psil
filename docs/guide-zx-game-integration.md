# Integrating micro-PSIL NPC Brains into ZX Spectrum Games

A practical guide to replacing hardcoded NPC state machines with evolvable, composable bytecode brains — using The Hobbit (1982) and Valhalla (1983) as reference architectures.

## 1. What The Hobbit and Valhalla Did (and Didn't Do)

The Hobbit (1982, Melbourne House) introduced "animaction" — NPCs with independent schedules that acted between player turns. Thorin sits down and sings about gold. Gandalf wanders off. Elrond stays in Rivendell. Each NPC's behavior was a hand-written script: a fixed sequence of actions, selected by a simple priority table and a random number generator. The scripts were frozen at compile time. Philip Mitchell wrote every possible Thorin behavior by hand.

Valhalla (1983, Legend) pushed further. Its NPCs had life cycles — they could be born, age, fight, eat, trade, and die. The 40+ characters moved independently across 200 rooms, each driven by an action table that mapped situations to responses. But the tables, like The Hobbit's scripts, were static. A Valhalla warrior always attacked when an enemy was adjacent. A merchant always traded. Nothing learned. Nothing adapted.

Both games proved that autonomous NPCs could make 48K worlds feel alive. What they couldn't do — because behavior was encoded in Z80 assembly — was let NPCs evolve, compose new behaviors from fragments, or surprise their designers with emergent strategies. The behavior substrate (raw machine code) was the bottleneck: you can't mutate Z80 instructions and expect anything but a crash.

## 2. micro-PSIL NPC Brains in 60 Seconds

micro-PSIL provides the missing layer: a bytecode VM that sits between the game engine and NPC behavior, making behavior *data* that can be executed, composed, and evolved.

| Component | Size | What it does |
|-----------|------|-------------|
| VM core | ~1,550 bytes Z80 | Fetch-decode-execute loop for bytecode |
| NPC genome | 16–64 bytes each | Bytecode program = the NPC's "brain" |
| Ring0 sensors | 11 slots (22 bytes) | Read-only NPC perception (health, food distance, danger...) |
| Ring1 actions | 4 slots (8 bytes) | Writable NPC outputs (move direction, action, target...) |
| Gas counter | 200 steps/tick | Prevents infinite loops, guarantees bounded execution |
| Composition | Concatenate byte arrays | No linker, no symbol resolution — just append |

Reference files:
- VM: [`z80/micro_psil_vm.asm`](../z80/micro_psil_vm.asm)
- Sandbox scheduler: [`z80/sandbox.asm`](../z80/sandbox.asm)
- Go reference: [`pkg/sandbox/`](../pkg/sandbox/)

## 3. The Sense-Think-Act Loop

Every tick, each living NPC runs through three phases: sense the world, think (execute its genome), and act on the results.

### 3.1 Ring0 — What the NPC Sees

The game fills Ring0 with sensor data before running the brain. Ring0 is read-only from the brain's perspective.

| Slot | Name | Sandbox meaning | Hobbit equivalent | Valhalla equivalent |
|------|------|----------------|-------------------|---------------------|
| 0 | self | NPC ID | character identity | character identity |
| 1 | health | 0–100 | hit points | life force |
| 2 | energy | 0–200 | stamina | fatigue |
| 3 | hunger | ticks since ate | — | hunger counter |
| 4 | fear | nearest NPC dist | enemy proximity | threat distance |
| 5 | food | nearest food dist | treasure proximity | food/provisions |
| 6 | danger | danger level | ring influence | combat danger |
| 7 | near | nearest NPC dist | companion distance | NPC proximity |
| 8 | x | X position | room number (low) | room ID (low) |
| 9 | y | Y position | room number (high) | room ID (high) |
| 10 | day | tick mod 256 | turn counter | frame counter |

The `fill_ring0` routine in `z80/sandbox.asm` (lines 215–281) fills these slots from world state:

```z80
fill_ring0:
    ; Slot 0: self ID
    LD A, (IX+0)
    LD HL, RING0_BUF
    LD (HL), A
    INC HL
    LD (HL), 0
    ; Slot 1: health
    INC HL
    LD A, (IX+3)
    LD (HL), A
    INC HL
    LD (HL), 0
    ; ... slots 2-10 follow the same pattern
```

**How a game replaces this:** Write your own `fill_ring0` that reads from your game's NPC data structure instead of the sandbox's `IX+offset` fields. Map your game's concepts (room number, inventory contents, nearby characters) to the 11 slots. Slots 11–31 are available for game-specific sensors.

### 3.2 Think — Running the Brain

The `run_brain` routine (`z80/sandbox.asm` lines 287–335) executes the genome:

```z80
run_brain:
    ; 1. Mute print output (brains shouldn't print)
    LD A, 1
    LD (vm_mute), A

    ; 2. Reset VM stack
    LD HL, VM_STACK
    LD (vm_sp), HL

    ; 3. Copy Ring0 into VM memory slots 0-10
    LD HL, RING0_BUF
    LD DE, VM_MEM
    LD BC, 22              ; 11 slots × 2 bytes
    LDIR

    ; 4. Clear Ring1 slots in VM memory (slots 64-67)
    LD HL, VM_MEM + 128
    LD (HL), 0
    LD DE, VM_MEM + 129
    LD BC, 7
    LDIR

    ; 5. Set gas limit
    LD HL, GAS_LIMIT       ; 200
    LD (vm_gas), HL

    ; 6. Point PC at genome
    LD L, (IX+12)
    LD H, (IX+13)
    LD (bc_pc), HL

    ; 7. Run
    CALL vm_run

    ; 8. Unmute, copy Ring1 back
    XOR A
    LD (vm_mute), A
    LD HL, VM_MEM + 128
    LD DE, RING1_BUF
    LD BC, 8
    LDIR
    RET
```

**Timing:** 200 gas steps ≈ 2–4 ms per NPC at 3.5 MHz. A game with 4 NPCs spends ~8–16 ms per frame on brain execution — well within a 50 Hz frame budget if you spread NPCs across frames (2 per frame = ~4–8 ms).

### 3.3 Act — What the NPC Does

The `apply_actions` routine (`z80/sandbox.asm` lines 341–422) reads Ring1 and applies effects:

| Ring1 slot | Sandbox action | Hobbit mapping | Valhalla mapping |
|-----------|---------------|----------------|------------------|
| 0: move | 0=none, 1=N, 2=E, 3=S, 4=W | room exits (N/S/E/W/U/D) | compass movement |
| 1: action | 0=idle, 1=eat, 2=attack, 3=share | take/drop/attack/say | eat/attack/trade |
| 2: target | NPC ID | target character | target character |
| 3: emotion | emotional state | — | — |

The sandbox implements movement with bounds-checking and collision detection, plus eating (search current + adjacent tiles for food). A real game would extend `apply_actions` to handle attack, trade, dialogue, and other game-specific verbs.

## 4. Example NPC Brains

### 4.1 Thorin's Brain (Hobbit-style)

Thorin's priorities: flee danger first, pick up treasure second, idle and sing otherwise.

**micro-PSIL source** ([`testdata/sandbox/thorin.mpsil`](../testdata/sandbox/thorin.mpsil)):
```asm
; Check danger
r0@ 6           ; read danger level
5               ; push threshold
>               ; danger > 5?
jnz 7           ; if yes → flee

; Check food (treasure)
r0@ 5           ; read food distance
5               ; push threshold
<               ; food nearby (< 5)?
jnz 5           ; if yes → eat

; Idle: Thorin sits down and sings about gold
yield

; Flee North
1               ; push 1 (North)
r1! 0           ; write move
yield

; Go South toward treasure, eat
3               ; push 3 (South)
r1! 0           ; write move
1               ; push 1 (eat)
r1! 1           ; write action
yield
```

**Hex bytecode (24 bytes):**
```
8A 06 25 0D 88 07 8A 05 25 0C 88 05 F1 21 8C 00 F1 23 8C 00 21 8C 01 F1
```

**Annotated disassembly:**
```
Offset  Hex        Instruction          Stack effect
0       8A 06      r0@ 6                [...] → [danger]
2       25         push 5               [danger] → [danger 5]
3       0D         >                    [danger 5] → [flag]
4       88 07      jnz +7 → offset 13  pops flag; if nonzero, jump to flee
6       8A 05      r0@ 5                [] → [food_dist]
8       25         push 5               [food_dist] → [food_dist 5]
9       0C         <                    [food_dist 5] → [flag]
10      88 05      jnz +5 → offset 17  pops flag; if nonzero, jump to eat
12      F1         yield                idle path (Ring1 = all zeros)
13      21         push 1               [] → [1]
14      8C 00      r1! 0                pops 1 → Ring1[move] = North
16      F1         yield                flee path
17      23         push 3               [] → [3]
18      8C 00      r1! 0                pops 3 → Ring1[move] = South
20      21         push 1               [] → [1]
21      8C 01      r1! 1                pops 1 → Ring1[action] = eat
23      F1         yield                eat path
```

### 4.2 Valhalla Warrior Brain

A warrior's priorities: eat when hungry, attack when enemy is close, wander otherwise.

**micro-PSIL source** ([`testdata/sandbox/warrior.mpsil`](../testdata/sandbox/warrior.mpsil)):
```asm
; Check hunger
r0@ 3           ; read hunger
20              ; push threshold
>               ; hunger > 20?
jnz 15          ; if yes → eat

; Check enemy proximity
r0@ 4           ; read fear (enemy distance)
3               ; push threshold
<               ; enemy within 3 tiles?
jnz 16          ; if yes → attack

; Wander: direction = (day mod 4) + 1
r0@ 10          ; read day counter
4               ; push 4
mod             ; day mod 4
1               ; push 1
+               ; +1 (valid directions are 1–4)
r1! 0           ; write move direction
yield

; Eat: move South toward food
3               ; push 3 (South)
r1! 0           ; write move
1               ; push 1 (eat)
r1! 1           ; write action
yield

; Attack: charge East toward enemy
2               ; push 2 (East)
r1! 0           ; write move
2               ; push 2 (attack)
r1! 1           ; write action
yield
```

**Hex bytecode (35 bytes):**
```
8A 03 34 0D 88 0F 8A 04 23 0C 88 10 8A 0A 24 0A 21 06 8C 00 F1 23 8C 00 21 8C 01 F1 22 8C 00 22 8C 01 F1
```

### 4.3 Composition Example — Cautious Prefix

The key insight: in a concatenative language, composition is concatenation. A "cautious" behavior modifier is just a byte prefix you prepend to any brain.

**Cautious prefix** ([`testdata/sandbox/cautious.mpsil`](../testdata/sandbox/cautious.mpsil)) — 10 bytes:
```asm
r0@ 1           ; read health
10              ; push 10
<               ; health < 10?
jz 4            ; if health >= 10, skip flee (fall through)
1               ; push 1 (North)
r1! 0           ; write move
yield           ; flee — don't run rest of brain
; ... rest of brain starts here
```

**Hex:** `8A 01 2A 0C 87 04 21 8C 00 F1`

To create "cautious Thorin" or "cautious warrior," concatenate the byte arrays:

```
cautious_thorin  = cautious_prefix ++ thorin_brain     ; 10 + 24 = 34 bytes
cautious_warrior = cautious_prefix ++ warrior_brain    ; 10 + 35 = 45 bytes
```

The `jz 4` in the prefix jumps exactly past the flee block to byte 10 — the start of the appended brain. No symbol tables, no relocation. The stack is empty at the junction point, so any brain works as the suffix.

This is the property unique to concatenative bytecode: you can compose behaviors by splicing byte arrays, and the result is always a valid program with well-defined semantics.

## 5. Adapting Ring0/Ring1 for Different Genres

### 5.1 Text Adventures

For a Hobbit-style parser adventure, reinterpret the coordinate slots:

| Slot | Meaning |
|------|---------|
| 8 (x) | Current room ID |
| 9 (y) | Number of exits from room |
| 5 (food) | Interesting item nearby (0 = none) |
| 4 (fear) | Hostile NPC in room (0 = none, 1 = present) |
| Ring1[0] move | Exit index (1 = first exit, 2 = second...) |
| Ring1[1] action | Verb (0=wait, 1=take, 2=drop, 3=attack, 4=say) |

The brain doesn't know it's in a text adventure. It reads sensors, computes, writes actions. The game's `fill_ring0` and `apply_actions` handle the translation.

### 5.2 Real-Time Adventures

For a Valhalla-style real-time game, Ring0/Ring1 map directly to screen coordinates and compass directions. The key concern is frame budgeting:

- At 200 gas, each brain takes ~2–4 ms at 3.5 MHz
- A 50 Hz frame is 20 ms
- Budget: 2–4 NPCs per frame
- With 16 NPCs: update 4 per frame, each NPC runs every 4th frame
- Perception is still responsive — NPCs react within 80 ms

### 5.3 Custom Slots

Ring0 slots 11–31 and Ring1 slots 4–31 are available for game-specific data:

```z80
; Example: add "inventory count" as Ring0 slot 11
; In your fill_ring0:
    LD A, (npc_inventory_count)
    LD HL, RING0_BUF + 22     ; slot 11 = byte offset 22
    LD (HL), A
    INC HL
    LD (HL), 0
```

The brain reads custom slots with `r0@ 11`, writes with `r1! 4` through `r1! 31`. No VM changes required.

## 6. In-Game Evolution

### 6.1 When to Evolve

Evolution is expensive — it requires comparing fitness scores and copying genomes. Schedule it at natural pause points:

- **The Hobbit style:** Between turns. After each player command, run one evolution step on the NPC population. The player never notices the ~1 ms overhead.
- **Valhalla style:** Scene transitions. When the player enters a new room, evolve NPCs in the rooms they left. Off-screen evolution is free time.
- **Chapter boundaries:** Every N turns/ticks, run a full generation. The sandbox uses every 128 ticks.

### 6.2 The GA on Z80

The `evolve_step` routine (`z80/ga.asm`, ~146 bytes of code + 9 bytes state) implements tournament-2 selection with point mutation:

1. Scan all living NPCs, track best and worst fitness
2. Copy the best NPC's genome over the worst's
3. Pick a random position, replace with a random byte (0x00–0x7F)
4. Reset the worst NPC's stats (health=100, energy=100, age=0)

```z80
evolve_step:
    ; Find best and worst living NPCs by fitness
    ; ...scan loop...

    ; Copy best genome to worst genome slot
    LD C, A             ; genome length
    LD B, 0
    LDIR

    ; Point mutation: random position, random opcode
    CALL lfsr_next      ; random byte
    ; ... mod genome_length ...
    CALL lfsr_next      ; random opcode
    AND $7F             ; keep in 1-byte range
    LD (HL), A          ; write mutation
```

The entire GA is ~155 bytes of Z80 code. Combined with the 16-bit LFSR random generator (~20 bytes), evolution adds under 200 bytes to the binary.

### 6.3 Custom Fitness Functions

Fitness determines which brains survive. The sandbox uses `age + food_eaten×10 + health`. For specific games:

- **Hobbit-style:** `turns_survived + items_collected×5 + quests_helped` — rewards NPCs that assist the player
- **Valhalla-style:** `frames_alive + kills×10 + trades×3` — rewards combat skill and social interaction
- **Cooperative:** `team_total_health + shared_food×5` — rewards NPCs that help each other

Change the fitness function by modifying the scoring block in the tick loop. In Z80:
```z80
    ; Score fitness = age + food_eaten*10 + health
    LD L, (IX+5)
    LD H, (IX+6)           ; HL = age
    ; ... multiply food_eaten by 10, add health ...
    LD (IX+9), L
    LD (IX+10), H
```

### 6.4 Go GA vs Z80 GA

The Go implementation (`pkg/sandbox/ga.go`) has six mutation operators: point mutation, insert, delete, constant tweak, block swap, and block duplicate. It also performs instruction-aligned crossover.

The Z80 implementation uses only point mutation. Why is this enough?

- Point mutation can reach any genome in the search space (it's ergodic)
- The other operators accelerate convergence but aren't required
- On Z80, the simpler GA saves ~150 bytes of code
- The population runs for thousands of ticks — there's time to converge

If you need faster convergence on Z80, add insertion/deletion (~40 bytes each). Instruction-aligned crossover (~80 bytes) gives the biggest payoff but is the most complex to implement.

## 7. Memory Budget

### 7.1 System Cost

| Component | Bytes | Notes |
|-----------|-------|-------|
| VM core (library mode) | ~1,500 | Fetch-decode-execute, math, stack ops |
| Scheduler (sense/think/act) | ~500 | fill_ring0, run_brain, apply_actions |
| GA (evolve_step + LFSR) | ~200 | Tournament-2, point mutation |
| Tick loop + init + helpers | ~600 | Main loop, food spawn, tile ops, printing |
| **Total code** | **~2,800** | Matches actual sandbox binary (2,818 bytes) |

| Data structure | Bytes | Notes |
|---------------|-------|-------|
| VM stack | 256 | 128 × 16-bit entries |
| VM memory slots | 128 | 64 × 16-bit (Ring0 + Ring1 + locals) |
| Quotation table | 64 | 32 × 16-bit pointers (unused if genomes are flat) |
| Ring0/Ring1 buffers | 30 | 11 + 4 slots × 2 bytes |
| NPC table | 224 | 16 NPCs × 14 bytes |
| World grid | 1,024 | 32×32 tiles |
| Genome bank | 1,024 | 16 × 64 bytes (max) |
| **Total data** | **~2,750** | |

**Grand total: ~5,550 bytes** for 16 NPCs in a 32×32 world with full evolution.

### 7.2 What's Left

On a 48K Spectrum: 49,152 – 5,550 ≈ **43,600 bytes free** for the game itself — graphics, maps, parser, sound, and game logic. For context, The Hobbit used about 40K total.

### 7.3 Shrinking It

If memory is tight, scale down:

| Parameter | Default | Minimum | Savings |
|-----------|---------|---------|---------|
| NPCs | 16 | 8 | 112 bytes (NPC table) + 512 bytes (genomes) |
| World grid | 32×32 | 16×16 | 768 bytes |
| Max genome | 64 bytes | 32 bytes | 512 bytes (genome bank) |
| VM stack | 128 entries | 32 entries | 192 bytes |

Minimum configuration (8 NPCs, 16×16 world, 32-byte genomes): **~3,200 bytes total**.

### 7.4 128K Options

On a 128K Spectrum, use bank switching for genome storage:

- Page 1 ($C000): 256 genomes × 64 bytes = 16,384 bytes
- Page 3 ($C000): Another 256 genomes
- Main RAM: VM + scheduler + NPC table + world grid

This supports 256+ NPCs across a large world, with genomes paged in during brain execution.

## 8. Integration Checklist

1. **Include VM as library.** Add `DEFINE VM_LIB_MODE` before `INCLUDE "micro_psil_vm.asm"`. This skips the standalone entry point and ORG directive.

2. **Define NPC table.** Allocate 14 bytes per NPC. Set genome pointer (bytes 12–13) to each NPC's bytecode location.

3. **Write `fill_ring0`.** Map your game's NPC state to Ring0 slots 0–10. Use slots 11–31 for game-specific sensors.

4. **Write `apply_actions`.** Read Ring1 slots 0–3 after brain execution. Map move direction and action type to your game's movement and verb systems.

5. **Compile seed genomes.** Write .mpsil files for initial NPC behaviors. Compile with `go run tools/compile_mpsil/main.go` or hand-assemble (genomes are short enough).

6. **Hook into game loop.** For each living NPC per tick/turn:
   ```
   CALL fill_ring0
   CALL run_brain
   CALL apply_actions
   ```

7. **Add evolution trigger.** Call `evolve_step` every N ticks. Start with N=128 and tune based on how fast you want adaptation.

8. **Test.** Cross-validate genomes between Go and Z80 VMs using `go test ./testdata/sandbox/...`. Both must produce identical Ring1 outputs.

## 9. Historical Speculation

The Hobbit shipped in 1982 with 40K of code and data. The micro-PSIL VM adds ~1,500 bytes. A minimal NPC brain system — 8 NPCs, no world grid, point mutation only — would have cost perhaps 2,500 bytes. It would have fit.

What if Thorin had carried a 24-byte genome instead of a hardcoded script? After a few hundred player turns, evolution would have selected for brains that responded to the player's behavior. A Thorin who learned that following the player leads to survival (because the player fights off goblins) would evolve to follow. A Thorin who discovered that picking up treasure and hoarding it increased his fitness score would become a rival instead of an ally. The "Thorin sits down and starts singing about gold" jokes would never have existed — because Thorin would have *learned* to do something useful, or died trying.

What if Valhalla's 40 NPCs had each carried a 30-byte evolvable brain? The warriors who attacked wisely (when healthy, when the enemy was weak) would have survived and reproduced. The merchants who traded food for weapons at the right moment would have thrived. After a few generations, the simulation would have produced emergent social structures — alliances, predator-prey dynamics, specialization — that no designer wrote.

The technology was available. A Z80 at 3.5 MHz can run the VM. The memory budget works. The missing insight was that concatenative bytecode is the right substrate for genetic programming: every mutation produces a valid program, every concatenation produces a valid composition, and the gas counter prevents any brain from crashing the system.

## 10. File Reference

| Concept | Go implementation | Z80 implementation |
|---------|------------------|-------------------|
| VM core | `pkg/micro/` | `z80/micro_psil_vm.asm` |
| Opcode table | `pkg/micro/opcodes.go` | `z80/micro_psil_vm.asm:403` (op_tbl) |
| Ring0/Ring1 slots | `pkg/sandbox/npc.go` | `z80/sandbox.asm:49–53` |
| Sense (fill Ring0) | `pkg/sandbox/scheduler.go:82` | `z80/sandbox.asm:215` |
| Think (run brain) | `pkg/sandbox/scheduler.go:100` | `z80/sandbox.asm:287` |
| Act (apply Ring1) | `pkg/sandbox/scheduler.go:119` | `z80/sandbox.asm:341` |
| Tick loop | `pkg/sandbox/scheduler.go:32` | `z80/sandbox.asm:95` |
| GA engine | `pkg/sandbox/ga.go` | `z80/ga.asm` |
| Mutation operators | `pkg/sandbox/ga.go:157` (6 ops) | `z80/ga.asm:98` (point only) |
| Crossover | `pkg/sandbox/ga.go:90` | — (Z80 uses copy+mutate) |
| Fitness scoring | `pkg/sandbox/scheduler.go:75` | `z80/sandbox.asm:150` |
| Bytecode compiler | `tools/compile_mpsil/main.go` | — |
| Seed genomes | `testdata/sandbox/*.mpsil` | — (compiled to .bin) |
| Cross-validation | `testdata/sandbox/crossval_test.go` | — |
