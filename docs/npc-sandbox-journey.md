# From Bytecode to Brains: Building Evolving NPCs on a Z80

*How a stack-based bytecode VM became a genetic programming platform that runs on both modern hardware and a 1980s microprocessor.*

---

## The Starting Point

PSIL started as a concatenative, stack-based language — a Joy-inspired experiment in point-free functional programming. It had quotations, combinators, turtle graphics, shader rendering. Then came micro-PSIL: a bytecode encoding that compresses the language into a format dense enough for a Z80 processor running at 3.5 MHz with 48K of RAM.

The Z80 VM core compiled to 1,552 bytes. It could run arithmetic, string output, recursive factorial, and conditional NPC thought programs. Four test programs passed identically on both the Go reference VM and the Z80 implementation running under the mzx emulator.

But the original README hinted at something more ambitious: *"A bytecode brain is just a byte array. You can mutate it, crossover two brains, measure fitness."* That was theory. This is the story of making it real.

## The Plan

The goal: a sandbox where NPCs with bytecode genomes live in a tile world, sense their environment, make decisions, and evolve. Go first (for rapid iteration and testing), then Z80 (to prove it runs on real retro hardware).

The design had three layers:

1. **World** — a 32x32 tile grid with food, walls, and water
2. **NPCs** — creatures with health, energy, position, and a bytecode genome
3. **Evolution** — a genetic algorithm that breeds successful NPCs and culls failures

The key insight that made this tractable: the VM already had everything needed. Ring0/Ring1 memory-mapped I/O for sensors and actions. Gas metering to prevent infinite loops. Yield to stop execution cleanly. The NPC brain is just a bytecode program that reads Ring0, computes, writes Ring1, and yields.

## Building the Go Sandbox

### The World

Each tile packs its type (empty, wall, food, water) in the low 4 bits and an occupant ID in the high 4 bits — one byte per tile, 1,024 bytes for the whole 32x32 grid. Food respawns stochastically each tick.

### The NPCs

Each NPC has 11 sensor slots (Ring0) filled by the world before their brain runs:

- Self ID, health, energy, hunger
- Nearest food distance, nearest enemy distance, danger level
- Position (x, y), day counter

After the brain runs, the scheduler reads 4 action slots (Ring1):

- Move direction (none/N/E/S/W)
- Action (idle/eat/attack/share)
- Target NPC ID
- Emotional state

### The Tick Loop

Every tick: sense → think → act → decay. The VM gets reset, Ring0 gets filled, the genome runs with a gas limit of 200, Ring1 gets read, movement and actions apply to the world. Energy drains by 1 each tick. No energy means health drains. No health means death.

### The GA Engine

This is where it gets interesting. The genetic algorithm has six mutation operators:

1. **Point mutation** — swap one byte for a random valid opcode
2. **Insert** — add a random opcode at a random position
3. **Delete** — remove one byte (genome must stay above 16 bytes)
4. **Constant tweak** — find a small number literal and nudge it by +/-1
5. **Block swap** — swap two instruction-aligned segments
6. **Block duplicate** — copy a short segment to another position

Crossover is instruction-aligned: the GA walks both parent genomes opcode-by-opcode (respecting the variable-length encoding) to find valid split points, then concatenates the prefix of one parent with the suffix of the other. This avoids splitting a 2-byte opcode in half — a problem that would produce garbage in most bytecode formats, but micro-PSIL's UTF-8-style encoding makes it possible to find alignment points reliably.

Every 100 ticks, the bottom 25% of the population (ranked by fitness = age + 10x food eaten + health remaining) gets replaced by offspring from the top 50%.

## Porting to the Z80

With the Go version working (9 unit tests passing, CLI running 5,000-tick simulations), it was time for the Z80.

### VM Modifications

The Z80 VM needed three changes:

1. **Library mode** — wrap the entry point in `IFNDEF VM_LIB_MODE` so the sandbox can include the VM as a library
2. **Per-step gas** — add a gas decrement at the top of the main loop (the Go VM had this; the Z80 only decremented on explicit `OpGas` calls)
3. **Mute flag** — NPC brains might accidentally execute print opcodes; a `vm_mute` flag suppresses output during brain execution

The Ring0/Ring1 opcodes were already defined but not wired into the Z80 dispatcher. Adding them required changing several `JR Z` (relative jump) instructions to `JP Z` (absolute jump) because the new handler code pushed the targets beyond the 127-byte range of relative jumps. A classic Z80 constraint — the architecture literally pushes back when you add code.

### The NPC ID Bug

The first Z80 run showed "NPC Sandbox Z80" and then a mysterious "0". Debugging revealed multiple issues stacked on top of each other:

The NPC initialization loop used register C as an ID counter. But PUSH BC / POP BC around the inner loop body saved and restored C, so `INC C` at the end of each iteration was effectively a no-op. All 16 NPCs got ID 1. The fix: use a memory variable (`npc_id_ctr`) instead of a register.

### The Infinite Loop Bug

With IDs fixed, the sandbox still hung. The problem: NPC brains with accidental loops (a random genome might contain a backward jump equivalent) ran forever because the Z80 VM only consumed gas on explicit `OpGas` opcodes, not per-step like the Go VM. Adding a gas decrement at the top of `vm_run` fixed this — after 200 steps, the brain is forcibly stopped regardless of what it's doing.

### The Spurious Print Bug

With gas limiting working, NPCs started printing garbage to the console. Random genomes sometimes contain `0x19` (the print opcode). In the Go sandbox, `vm.Output = io.Discard` handles this. For Z80, a `vm_mute` flag was added — set to 1 before running a brain, cleared after. The print and builtin handlers check the flag and skip output when muted.

### The HL Corruption Bug

The tick loop counted ticks in HL, but `evolve_step` and `print_stats` both clobbered HL. After returning from these calls, the tick limit comparison used stale values. The fix: reload `LD HL, (tick_count)` from memory before the limit check.

### The Power-of-Two Bug

`EVOLVE_EVERY` was set to 100, but the tick check used `AND EVOLVE_EVERY-1` — a bitmask that only works for powers of 2. Changed to 128.

Five bugs, each masked by the others. Classic Z80 debugging.

### The Final Build

The complete Z80 sandbox — VM, scheduler, GA, world grid, NPC table, random number generator — assembles to **2,818 bytes**. It runs 16 NPCs on a 32x32 grid for 500 ticks with evolution every 128 ticks. The GA uses tournament-2 selection and point mutation (simplified from the Go version's six operators to fit in Z80 constraints).

Output:
```
NPC Sandbox Z80
T=128 A=2
T=256 A=0
T=384 A=0
Done
```

Two NPCs alive at tick 128, then population dies off. The Z80's simplified GA doesn't have enough genetic diversity to sustain the population long-term — but it proves the concept: genetic programming running on a Z80.

## Cross-Validation

The seed genomes (forager, flee, random walker) are tested on both VMs. The forager genome `8A 05 23 8C 00 21 8C 01 F1` — read food distance, push 3 (South), write move, push 1 (eat), write action, yield — produces identical Ring1 outputs on Go and Z80: move=3, action=1.

The random walker genome, given day=7, computes `7 mod 4 = 3, 3 + 1 = 4` (West) on both VMs. Bytecode compatibility is byte-exact.

## What Emerged

After 5,000 ticks with 20 NPCs on the Go sandbox, the population stabilizes around 5-10 survivors. The best genomes are typically 16-30 bytes — compact programs that found some combination of movement and eating that keeps them alive longer than their competitors.

The genomes aren't human-readable strategies. They're evolved artifacts — tangled sequences of stack operations, sensor reads, and action writes that happen to produce fitness-positive behavior. Some contain dead code. Some rely on stack underflow defaulting to zero. Some do arithmetic on sensor values in ways that accidentally produce useful directions.

This is exactly what genetic programming predicts: the search finds solutions in the space of all programs, not in the space of programs a human would write. The concatenative representation makes this work because there's no syntax to get wrong — every byte sequence is a valid (if possibly useless) program.

## The Numbers

| Component | Go | Z80 |
|-----------|-----|-----|
| VM core | ~800 LOC | 1,552 bytes |
| Sandbox (scheduler + world + NPC) | ~400 LOC | ~1,200 bytes |
| GA engine | ~250 LOC | ~300 bytes |
| Total sandbox binary | — | 2,818 bytes |
| NPCs per run | 20 | 16 |
| World size | 32x32 | 32x32 |
| Gas per brain | 200 | 200 |
| Mutation operators | 6 | 1 (point) |
| Unit tests | 11 | — |

## What's Next

The Z80 sandbox proves that genetic programming can run on an 8-bit microprocessor. The obvious extensions:

- **More mutation operators on Z80** — instruction insert/delete would fit in ~100 bytes each
- **Larger populations** — use 128K paging to bank-switch genome storage
- **Visual output** — render the 32x32 world to the Spectrum's screen (it maps naturally to the 256x192 display)
- **Inter-NPC communication** — the Ring1 "share" action could pass stack values between NPCs
- **Co-evolution** — predator/prey dynamics where both populations evolve simultaneously

But the core result stands: a concatenative bytecode VM is an almost suspiciously good substrate for genetic programming. The programs are compact, mutation-safe, composition-friendly, and run identically on hardware separated by 40 years.

---

*Built with PSIL, micro-PSIL, sjasmplus, and the mzx ZX Spectrum emulator.*
