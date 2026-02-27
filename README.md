# PSIL - Point-free Stack-based Interpreted Language

PSIL is a concatenative, stack-based, point-free functional language inspired by Joy, designed for VM execution and targeting Z80/6502 compatibility.

## Features

- **Stack-based execution** - all operations work on an implicit stack
- **Quotations as first-class values** - code blocks `[ ... ]` can be passed, stored, and composed
- **Point-free style** - no named variables, only stack transformations
- **Hardware-inspired flags** - Z flag for booleans, C flag for errors, A register for error codes
- **Rich combinator library** - `ifte`, `linrec`, `while`, `map`, `filter`, `fold`
- **Graphics system** - create and render images with shader-style programming
- **Turtle graphics** - Logo-style turtle for L-systems and fractals
- **Math functions** - sin, cos, sqrt, pow, lerp, clamp, smoothstep, etc.
- **micro-PSIL bytecode VM** - compact bytecode for Z80/6502 implementation
- **CPS-compatible semantics** - designed for easy compilation to bytecode

## Quick Start

```bash
# Build
go build ./cmd/psil

# Run REPL
./psil

# Run a file
./psil examples/fibonacci.psil

# Run shader examples
./psil examples/shaders.psil
```

## Language Basics

```psil
% Numbers push to stack
42 3.14 -5

% Strings
"Hello, World!"

% Stack operations
dup     % duplicate top: a -> a a
drop    % remove top: a ->
swap    % swap top two: a b -> b a
over    % copy second: a b -> a b a
rot     % rotate three: a b c -> b c a
roll    % n roll: bring nth item to top
pick    % n pick: copy nth item to top

% Arithmetic
+ - * / mod neg abs

% Math functions
sin cos tan sqrt pow log exp
floor ceil round min max
clamp lerp smoothstep fract

% Comparison (sets Z flag)
< > <= >= = !=

% Quotations (code blocks)
[1 2 +]     % pushes quotation, doesn't execute
i           % execute quotation: [Q] i -> ...
call        % alias for i

% Definitions (three styles)
DEFINE sq == [dup *].       % Joy-style
[dup *] "sq" define         % Point-free with string
[dup *] 'sq define          % Point-free with quoted symbol

5 sq .      % prints 25

% Conditionals
[cond] [then] [else] ifte

% Recursion
[pred] [base] [rec1] [rec2] linrec
```

## Graphics System

PSIL includes a graphics system for creating and rendering images:

```psil
% Create 256x192 image
256 192 img-new

% Fill with color
255 0 0 img-fill            % fill with red

% Set individual pixels
dup 100 50 0 255 0 img-setpixel  % green pixel at (100,50)

% Render with shader quotation
% Shader receives: x y width height
% Shader returns: r g b
[
    drop drop               % x y
    swap 255 * 256 /        % r = x scaled
    swap 255 * 192 /        % g = y scaled
    128                     % b = constant
] img-render

% Save as PNG
"output/image.png" img-save
```

### Shader Examples

The `examples/shaders.psil` file demonstrates various shader effects:

| Gradient | Stripes | Checker | Plasma | Radial | Sphere |
|----------|---------|---------|--------|--------|--------|
| ![Gradient](docs/images/gradient.png) | ![Stripes](docs/images/stripes.png) | ![Checker](docs/images/checker.png) | ![Plasma](docs/images/plasma.png) | ![Radial](docs/images/radial.png) | ![Sphere](docs/images/sphere.png) |

Run them with:
```bash
mkdir -p output
./psil examples/shaders.psil
```

## Turtle Graphics

Logo-style turtle graphics for L-systems, fractals, and generative art:

```psil
256 192 img-new
0 0 0 img-fill
turtle                      % create turtle at center
255 255 0 pencolor          % yellow pen

5 [60 fd 144 rt] times      % draw a star

turtle-img "star.png" img-save
```

### Turtle Examples

| Square | Star | Spiral | Koch Curve |
|--------|------|--------|------------|
| ![Square](docs/images/turtle-square.png) | ![Star](docs/images/turtle-star.png) | ![Spiral](docs/images/turtle-spiral.png) | ![Koch](docs/images/turtle-koch.png) |

| Sierpinski | Tree | Nested Squares | Colorful Circles |
|------------|------|----------------|------------------|
| ![Sierpinski](docs/images/turtle-sierpinski.png) | ![Tree](docs/images/turtle-tree.png) | ![Nested](docs/images/turtle-nested.png) | ![Circles](docs/images/turtle-circles.png) |

Run them with:
```bash
./psil examples/turtle.psil
```

### Turtle Commands

| Command | Effect | Command | Effect |
|---------|--------|---------|--------|
| `fd n` | Forward n pixels | `bk n` | Backward n pixels |
| `rt n` | Turn right n degrees | `lt n` | Turn left n degrees |
| `pu` | Pen up (stop drawing) | `pd` | Pen down (draw) |
| `pencolor r g b` | Set pen color | `setxy x y` | Move to position |
| `setheading n` | Set heading | `home` | Return to center |

## Example: Factorial

```psil
DEFINE fact == [
    [dup 0 =]           % predicate: n == 0?
    [drop 1]            % base case: return 1
    [dup 1 -]           % before recursion: push n-1
    [*]                 % after recursion: multiply
    linrec
].

5 fact .    % prints 120
```

## Example: Fibonacci

```psil
DEFINE fib == [
    [dup 2 <]                         % n < 2?
    []                                % return n
    [dup 1 - fib swap 2 - fib +]     % fib(n-1) + fib(n-2)
    ifte
].

10 fib .    % prints 55
```

## Example: Plasma Shader

```psil
DEFINE plasma-shader == [
    drop drop               % x y
    over 16 / sin           % sin(x/16)
    over 16 / cos +         % + cos(y/16)
    rot 8 / cos +           % + cos(x/8)
    swap 8 / sin +          % + sin(y/8)
    1 + 2 / 255 *           % normalize to 0-255
    dup 1.2 * 255 mod       % r
    swap dup 0.8 * 50 + 255 mod  % g
    swap 1.5 * 100 + 255 mod     % b
].

256 192 img-new
[plasma-shader] img-render
"output/plasma.png" img-save
```

## Error Handling

PSIL uses hardware-inspired flags for error handling:

- **Z flag** - set by boolean operations (true = Z set)
- **C flag** - indicates error condition (true = error)
- **A register** - holds error code when C flag is set

```psil
% Error codes:
% 1 = stack underflow
% 2 = type mismatch
% 3 = division by zero
% 4 = undefined symbol
% 5 = gas exhausted
% 7 = image error
% 8 = file error

% Check for errors
err?        % push C flag as boolean
errcode     % push A register (error code)
clearerr    % clear error state

% Try/catch pattern
[risky-code] [error-handler] try
```

## REPL Commands

```
:help       Show help
:quit       Exit REPL
:stack      Show current stack
:flags      Show Z, C flags and A register
:clear      Clear stack and reset flags
:debug      Toggle debug mode
:words      List defined words
:load file  Load and execute a file
:gas n      Set gas limit (0 = unlimited)
```

## Building for Development

```bash
# Run tests
go test ./...

# Run with debug mode
./psil -debug

# Set gas limit for computation
./psil -gas 10000
```

## Builtins Reference

### Stack Operations
`dup`, `drop`, `swap`, `over`, `rot`, `nip`, `tuck`, `dup2`, `drop2`, `clear`, `depth`, `roll`, `unroll`, `pick`

### Arithmetic
`+`, `-`, `*`, `/`, `mod`, `neg`, `abs`, `inc`, `dec`

### Math Functions
`sin`, `cos`, `tan`, `asin`, `acos`, `atan`, `atan2`, `sqrt`, `pow`, `exp`, `log`, `floor`, `ceil`, `round`, `min`, `max`, `clamp`, `lerp`, `sign`, `fract`, `smoothstep`

### Comparison
`<`, `>`, `<=`, `>=`, `=`, `!=`

### Logic
`and`, `or`, `not`

### Quotation Operations
`i`, `call`, `x`, `dip`, `concat`, `cons`, `uncons`, `first`, `rest`, `size`, `null?`, `quote`, `unit`

### List Operations
`reverse`, `nth`, `take`, `ldrop`, `split`, `zip`, `zipwith`, `range`, `iota`, `flatten`, `any`, `all`, `find`, `index`, `sort`, `last`

### Combinators
`ifte`, `linrec`, `binrec`, `genrec`, `primrec`, `tailrec`, `while`, `times`, `loop`, `map`, `fold`, `filter`, `each`, `step`, `infra`, `cleave`, `spread`, `apply`

### Graphics
`img-new`, `img-setpixel`, `img-getpixel`, `img-save`, `img-width`, `img-height`, `img-fill`, `img-render`, `image?`

### Turtle Graphics
`turtle`, `fd`, `bk`, `lt`, `rt`, `pu`, `pd`, `pencolor`, `setxy`, `setheading`, `home`, `turtle-img`

### I/O
`.`, `print`, `newline`, `stack`

### Error Handling
`err?`, `errcode`, `clearerr`, `onerr`, `try`

### Definition
`define`, `undefine`

## micro-PSIL: Bytecode VM for Z80

### Why a Bytecode VM on a Z80?

The Z80 runs at 3.5 MHz with 48K of RAM. Every byte matters. A naive interpreter — tokenizing strings, hashing symbol names, walking tree structures — would burn most of that capacity on overhead. But a well-designed bytecode VM inverts the equation: the interpreter becomes a tight fetch-decode-execute loop, and the *programs* compress down to something approaching information-theoretic density.

micro-PSIL's encoding is modeled on UTF-8. The most common operations — stack manipulation, small arithmetic, boolean tests — are single bytes. A complete NPC decision ("if health < 10 and enemy nearby, flee; else fight") compiles to **21 bytes** of bytecode plus quotation bodies. The entire VM core is **1,552 bytes** of Z80 machine code. That leaves ~45K for game data, maps, and hundreds of NPC behavior scripts.

### NPC Brains as Concatenative Programs

The real motivation is AI for NPCs — not the modern neural-net kind, but something closer to what "artificial intelligence" meant in the 1980s: small programs that make creatures seem alive.

In most retro games, NPC behavior is a hardcoded state machine: `IF health < 10 THEN flee`. The transitions are fixed at compile time. The designer writes every possible behavior. Nothing emerges.

micro-PSIL changes this by making behavior *data*. Each NPC carries a bytecode program — its "brain." The VM runs the brain each tick, the NPC reads its sensors (health, enemy distance, hunger), the brain computes a decision, the NPC acts. Different NPCs can carry different programs. A cautious goblin's brain might be:

```
'health @ 10 < 'enemy @ and [flee] [patrol] ifte    ; 12 bytes
```

An aggressive one:

```
'enemy @ [charge] [wander] ifte                      ; 6 bytes
```

The concatenative model makes this unusually powerful because **composition is concatenation**. You don't need a compiler, linker, or symbol resolver to combine behaviors — you literally append bytecode arrays. Want a goblin that checks hunger *before* checking for enemies? Prepend a hunger-check snippet:

```
brain_a = 'hunger @ 20 > [eat] [...] ifte   ; hungry? eat first
brain_b = 'enemy @ [charge] [wander] ifte   ; then fight or wander
brain_ab = brain_a ++ brain_b               ; just concatenate the bytes
```

No variable conflicts. No calling conventions. No scope. The stack is the only interface between the two fragments, and stack effects are local and composable. This is a property unique to concatenative languages — in any applicative language (C, Lisp, Python), combining two code fragments requires managing shared names.

### Genetic Programming on a Z80

This composability opens the door to something that would be absurdly impractical in most languages: **genetic programming on the Z80 itself.**

A bytecode brain is just a byte array. You can:

- **Mutate** it: flip a random byte (change `+` to `*`, change a constant, swap a quotation ref)
- **Crossover** two brains: take the first half of parent A and the second half of parent B
- **Measure fitness**: run the brain in a simulated tick, see if the NPC survived, found food, or died

Because the bytecode is well-formed at every granularity (every byte is either a complete instruction or a prefix that the VM knows how to skip), random mutations produce *valid programs* far more often than in tree-based representations. Single-byte instructions like `dup`, `swap`, `+`, `<` are atomic and self-contained. Even a completely random 20-byte sequence will execute without crashing — it might not do anything useful, but it won't segfault. The VM has a gas counter to prevent infinite loops.

This means you could run a genetic algorithm *in-game, on the Z80*:

1. A population of 20 NPCs, each with a 30-byte brain
2. Every N ticks, score them (survived? found food? killed enemy?)
3. Top 5 reproduce: crossover + mutation → 20 new brains
4. Total memory: 20 × 30 = **600 bytes** for the entire population's genomes

After a few generations, the NPCs evolve behaviors the designer never wrote. The cautious ones learn to flee. The aggressive ones learn to charge. Some discover strategies like "flee when hurt, charge when healthy" — emergent `ifte` patterns that arise from selection pressure, not from a programmer typing `if`.

The entire genetic algorithm (selection, crossover, mutation, fitness evaluation) fits in maybe 200 bytes of Z80 code. The VM is already there. The bytecode programs *are* the genomes. There is no separate representation to maintain.

This is the kind of thing that was theoretically possible in the 1980s but never practical because game behavior was written in assembly — you can't mutate Z80 machine code and expect anything but a crash. A bytecode VM creates exactly the abstraction layer needed: a safe, compact, composable representation that can be both executed and evolved.

### The Inner Loop

The concatenative model maps directly to the Z80's sequential execution. The VM's inner loop is:

```
fetch:  LD A, (bc_pc) / INC bc_pc
decode: CP $20 / JP C, command_table
        CP $40 / JR C, push_small_number
        ...
```

Quotations (code blocks) are stored as separate bytecode arrays referenced by index. `[0]` pushes quotation reference 0 onto the stack. `exec` pops it and runs it. `ifte` pops a condition and two quotation refs, runs one or the other. The Z80 implementation saves/restores the bytecode PC on the machine stack — quotation calls nest naturally using the same hardware stack the CPU already has.

### Building and Running

```bash
# Go reference VM
go build ./cmd/micro-psil
./micro-psil examples/micro/arithmetic.mpsil
./micro-psil -disasm examples/micro/npc-thought.mpsil

# Compile to bytecode
go run tools/compile_mpsil/main.go -o z80/build examples/micro/arithmetic.mpsil

# Run on Z80 (via mzx emulator)
mzx --run z80/build/vm.bin@8000 \
    --load z80/build/arithmetic.bin@9000 \
    --console-io --frames DI:HALT
```

### Bytecode Format

| Range | Length | Usage |
|-------|--------|-------|
| `00-1F` | 1 byte | Commands (dup, swap, +, -, *, <, ifte, exec...) |
| `20-3F` | 1 byte | Small numbers 0-31 |
| `40-5F` | 1 byte | Symbols (health, energy, enemy, fear...) |
| `60-7F` | 1 byte | Quotation refs [0]-[31] |
| `80-BF` | 2 bytes | Extended ops (push.b, jmp, jz, call builtin) |
| `C0-DF` | 3 bytes | Far ops (push.w, far jumps) |
| `F0-FF` | 1 byte | Special (halt, yield, end) |

### NPC Thought Example

```asm
; "If health < 10 AND enemy nearby, flee; otherwise fight"
8 5 !               ; health = 8
1 12 !              ; enemy = 1

'health @           ; load health       → 45 17
10 <                ; less than 10?     → 2A 0C
'enemy @            ; load enemy flag   → 4C 17
and                 ; both true?        → 0E
[0] [1]             ; [flee] [fight]    → 60 61
ifte                ; conditional       → 13
halt                ;                   → F0
```

The decision logic compiles to **10 bytes**. The VM executes it, prints `Flee!`.

### Z80 VM Architecture

```
Memory Map:
  $8000-$8FFF  VM code (1,552 bytes)
  $9000-$91FF  Bytecode program (loaded)
  $9200-$97FF  Quotation blob (loaded)
  $B000-$B0FF  VM value stack (128 × 16-bit entries)
  $B100-$B17F  Memory slots (64 × 16-bit)
  $B180-$B1BF  Quotation pointer table (32 entries)

I/O: OUT ($23), A via mzx --console-io (no ROM needed)
```

### Test Results

All programs verified against the Go reference VM:

| Program | Bytecode | Output | What it tests |
|---------|----------|--------|---------------|
| arithmetic | 49 bytes | `5 6 56 20 45 25 4` | Stack ops, +, -, *, /, dup, swap |
| hello | 51 bytes | `Hello World!` | Character output via builtins |
| factorial | 7 + 14 bytes | `120` | Recursive quotation, loop, dec, * |
| npc-thought | 21 + 76 bytes | `Flee!` | Memory, ifte, 3 quotations |

### Prebuilt Binaries

The `z80/build/` directory contains ready-to-run binaries:

| File | Size | Description |
|------|------|-------------|
| `vm.bin` | 1,552 B | Z80 micro-PSIL VM (load at $8000) |
| `arithmetic.bin` | 49 B | Arithmetic test (load at $9000) |
| `hello.bin` | 51 B | Hello World (load at $9000) |
| `factorial.bin` | 7 B | Factorial main (load at $9000) |
| `factorial_quots.bin` | 14 B | Factorial quotations (load at $9200) |
| `npc-thought.bin` | 21 B | NPC thought main (load at $9000) |
| `npc-thought_quots.bin` | 76 B | NPC thought quotations (load at $9200) |

See [micro-PSIL Design Report](reports/2026-01-11-001-micro-psil-bytecode-vm.md) and [MinZ Feasibility Analysis](reports/2026-01-11-002-micro-psil-on-minz-feasibility.md) for details.

## Architecture

```
Source Code (.psil)          micro-PSIL (.mpsil)
    |                              |
    v                              v
Parser (Participle v2)       Assembler (pkg/micro)
    |                              |
    v                              v
AST (typed structs)          Bytecode (.bin)
    |                              |
    v                              v
Go Interpreter               Z80 VM (1,552 bytes)
    |                              |
    v                              v
REPL / Files                 mzx emulator / real hardware
```

## Documentation

### Design Reports

See [`reports/`](reports/) for detailed design documentation:

| Report | Description |
|--------|-------------|
| [PSIL Design Rationale](reports/2026-01-10-001-psil-design-rationale.md) | Theoretical foundations and language design |
| [micro-PSIL Bytecode VM](reports/2026-01-11-001-micro-psil-bytecode-vm.md) | Bytecode format, encoding, Z80 implementation |
| [micro-PSIL on MinZ](reports/2026-01-11-002-micro-psil-on-minz-feasibility.md) | Feasibility analysis for MinZ compiler/VM |

### Architecture Decision Records

See [`docs/adr/`](docs/adr/) for architectural decisions:

| ADR | Title |
|-----|-------|
| [ADR-001](docs/adr/001-bytecode-encoding.md) | UTF-8 Style Bytecode Encoding |
| [ADR-002](docs/adr/002-stack-format.md) | Tagged Stack Value Format |
| [ADR-003](docs/adr/003-symbol-slots.md) | Fixed Symbol Slots for NPC State |

## License

MIT
