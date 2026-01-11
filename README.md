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

## micro-PSIL: Bytecode VM

micro-PSIL is a minimal bytecode VM designed for Z80/6502 implementation. It uses UTF-8 style encoding for compact programs.

```bash
# Build
go build ./cmd/micro-psil

# Run examples
./micro-psil examples/micro/arithmetic.mpsil
./micro-psil examples/micro/npc-thought.mpsil

# Disassemble
./micro-psil -disasm examples/micro/npc-thought.mpsil

# REPL mode
./micro-psil
```

### Bytecode Format

| Range | Length | Usage |
|-------|--------|-------|
| 0x00-0x1F | 1 byte | Commands (dup, swap, +, -, etc.) |
| 0x20-0x3F | 1 byte | Small numbers (0-31) |
| 0x40-0x5F | 1 byte | Symbols (health, energy, fear) |
| 0x60-0x7F | 1 byte | Quotation refs ([0]-[31]) |
| 0x80-0xBF | 2 bytes | Extended ops (push.b, jmp, jz) |
| 0xC0-0xDF | 3 bytes | Far ops (push.w, far jumps) |
| 0xF0-0xFF | 1 byte | Special (halt, yield) |

### NPC Thought Example

```asm
; "If health < 10 AND enemy nearby, flee"
'health @           ; load health
10 <                ; less than 10?
'enemy @            ; load enemy flag
and                 ; both true?
[flee] [fight]      ; quotation refs
ifte                ; conditional
```

Compiles to **10 bytes**: `45 17 2A 0C 4C 17 0E 60 61 13`

See [micro-PSIL Design Report](reports/2026-01-11-001-micro-psil-bytecode-vm.md) for details.

## Architecture

PSIL is designed with future compilation in mind:

```
Source Code
    |
    v
Parser (Participle v2)
    |
    v
AST (typed structs)
    |
    v
Interpreter ←──────── micro-PSIL VM
    |                      |
    v                      v
REPL / Files         Bytecode execution
                     (Z80/6502 compatible)
```

## Documentation

### Design Reports

See [`reports/`](reports/) for detailed design documentation:

| Report | Description |
|--------|-------------|
| [PSIL Design Rationale](reports/2026-01-10-001-psil-design-rationale.md) | Theoretical foundations and language design |
| [micro-PSIL Bytecode VM](reports/2026-01-11-001-micro-psil-bytecode-vm.md) | Bytecode format, encoding, Z80 implementation |

### Architecture Decision Records

See [`docs/adr/`](docs/adr/) for architectural decisions:

| ADR | Title |
|-----|-------|
| [ADR-001](docs/adr/001-bytecode-encoding.md) | UTF-8 Style Bytecode Encoding |
| [ADR-002](docs/adr/002-stack-format.md) | Tagged Stack Value Format |
| [ADR-003](docs/adr/003-symbol-slots.md) | Fixed Symbol Slots for NPC State |

## License

MIT
