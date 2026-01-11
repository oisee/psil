# micro-PSIL: A Minimal Bytecode VM for Retro Hardware

**Date:** 2026-01-11
**Status:** Implemented
**Related ADRs:** [ADR-001](../docs/adr/001-bytecode-encoding.md), [ADR-002](../docs/adr/002-stack-format.md), [ADR-003](../docs/adr/003-symbol-slots.md)

## Abstract

micro-PSIL is a compact bytecode virtual machine designed for easy implementation on 8-bit processors (Z80, 6502). It uses a UTF-8 inspired variable-length encoding that prioritizes common operations in single bytes while allowing extension for less frequent cases. The primary use case is encoding NPC "thoughts" as compact, evolvable memes in game engines.

## Motivation

### The Problem

Standard PSIL (the full interpreter) is too complex for direct Z80 implementation:
- Dynamic typing requires runtime dispatch
- Quotations as heap objects need garbage collection
- String handling is memory-intensive

We need a simpler representation that:
1. Compiles to compact bytecode (NPC thoughts should be ~20-50 bytes)
2. Can be implemented on Z80 in ~1KB of code
3. Supports the core PSIL semantics (stack, quotations, conditionals)
4. Enables "memetic" operations (copy, mutate, crossover)

### Design Goals

| Goal | Priority | Rationale |
|------|----------|-----------|
| Compact encoding | High | NPC memories are limited |
| Z80 implementable | High | Target platform |
| Fast dispatch | Medium | NPCs run every frame |
| Readable disassembly | Medium | Debugging meme evolution |
| Extensible | Low | Future-proofing |

## Bytecode Encoding

### Overview

The encoding is inspired by UTF-8's variable-length design:

```
Byte Range      Length    Usage
─────────────────────────────────────────────
0x00-0x7F       1 byte    Hot path (128 values)
0x80-0xBF       2 bytes   Main path (64 ops × 256 args)
0xC0-0xDF       3 bytes   Rare path (32 ops × 65536 args)
0xE0-0xEF       Variable  Strings, blobs
0xF0-0xFF       1 byte    Special/control
```

### Single-Byte Operations (0x00-0x7F)

The 128 most common operations fit in a single byte:

```
0x00-0x1F (32): Stack/arithmetic commands
    0x00 nop    0x08 *      0x10 not
    0x01 dup    0x09 /      0x11 neg
    0x02 drop   0x0A mod    0x12 exec
    0x03 swap   0x0B =      0x13 ifte
    0x04 over   0x0C <      0x14 dip
    0x05 rot    0x0D >      0x15 loop
    0x06 +      0x0E and    0x16 ret
    0x07 -      0x0F or     ...

0x20-0x3F (32): Inline small numbers (0-31)
    0x20 = push 0
    0x21 = push 1
    ...
    0x3F = push 31

0x40-0x5F (32): Inline symbols (memory slots)
    0x40 nil      0x48 anger    0x50 safe
    0x41 true     0x49 fear     0x51 near
    0x42 false    0x4A trust    0x52 far
    0x43 self     0x4B hunger   0x53 day
    0x44 target   0x4C enemy    0x54 night
    0x45 health   0x4D friend   ...
    0x46 energy   0x4E food
    0x47 pos      0x4F danger

0x60-0x7F (32): Inline quotation references
    0x60 = [0]    0x70 = [16]
    0x61 = [1]    0x71 = [17]
    ...           ...
    0x6F = [15]   0x7F = [31]
```

### Two-Byte Operations (0x80-0xBF)

Format: `[opcode][argument]`

```
0x80 [n]  push.b    Push byte value n
0x81 [n]  sym.x     Extended symbol (slot n)
0x82 [n]  quot.x    Extended quotation (index n)
0x83 [n]  local     Push local variable n
0x84 [n]  local!    Store to local variable n
0x85 [n]  jmp       Jump forward n bytes
0x86 [n]  jmp-      Jump backward n bytes
0x87 [n]  jz        Jump if zero
0x88 [n]  jnz       Jump if not zero
0x89 [n]  call      Call builtin n
0x8A [n]  r0@       Read ring0 slot n
0x8B [n]  r1@       Read ring1 slot n
0x8C [n]  r1!       Write ring1 slot n
...
```

### Three-Byte Operations (0xC0-0xDF)

Format: `[opcode][hi][lo]`

```
0xC0 [h][l]  push.w    Push 16-bit value
0xC1 [h][l]  sym.16    Extended symbol (16-bit)
0xC2 [h][l]  quot.16   Extended quotation (16-bit)
0xC3 [h][l]  jmp.far   Far jump
...
```

### Variable-Length Operations (0xE0-0xEF)

Format: `[opcode][length][data...]`

```
0xE0 [len][bytes...]  String literal
0xE1 [len][bytes...]  Raw bytes
0xE2 [len][items...]  Vector
0xE3 [len][code...]   Inline quotation body
```

### Special Operations (0xF0-0xFF)

```
0xF0  halt     Stop execution
0xF1  yield    Yield to scheduler
0xF2  break    Debugger breakpoint
0xF3  debug    Debug print
0xF4  error    Set error flag
0xF5  clrerr   Clear error flag
0xF6  err?     Check error flag
0xFE  extend   Extended opcode (future)
0xFF  end      End marker
```

## Stack Format

The stack uses tagged values for type safety while remaining compact:

```
Stack Layout: [size][data...][size][data...]...
              └─────────────┘└─────────────┘
                 value 1        value 2

Byte value:   [1][xx]           (2 bytes)
Word value:   [2][lo][hi]       (3 bytes)
```

This allows `dup` to know how many bytes to copy without full type information.

### Z80 Implementation

```asm
; dup - duplicate top of stack
dup:
    ld a, (sp+0)      ; get size byte
    cp 1
    jr z, .dup_byte
    cp 2
    jr z, .dup_word
    ret               ; error: unknown size

.dup_byte:
    ld a, (sp+1)      ; get value
    push af           ; push size=1, value
    ret

.dup_word:
    ld hl, (sp+1)     ; get 16-bit value
    ld a, 2
    push af           ; push size=2
    push hl           ; push value
    ret
```

## Example: NPC Thought

### High-Level Intent

"If my health is low AND there's an enemy nearby, flee. Otherwise, fight."

### micro-PSIL Assembly

```asm
; Setup memory (in real use, set by game engine)
8 5 !               ; health = 8 (low)
1 12 !              ; enemy = 1 (present)

; The "thought" - decision logic
'health @           ; load health value
10 <                ; health < 10?
'enemy @            ; load enemy presence
and                 ; both conditions true?
[0] [1]             ; [flee] [fight] quotations
ifte                ; conditional execution
```

### Compiled Bytecode

```
Offset  Bytes       Disassembly
──────────────────────────────────
0000    28          8           (push 8)
0001    25          5           (push 5)
0002    18          !           (store)
0003    21          1           (push 1)
0004    2C          12          (push 12)
0005    18          !           (store)
0006    45          'health     (symbol slot 5)
0007    17          @           (load)
0008    2A          10          (push 10)
0009    0C          <           (less than)
000A    4C          'enemy      (symbol slot 12)
000B    17          @           (load)
000C    0E          and         (logical and)
000D    60          [0]         (quotation 0)
000E    61          [1]         (quotation 1)
000F    13          ifte        (conditional)
0010    F0          halt
```

**Total: 17 bytes** for setup + decision logic.

The core "thought" (lines 0006-000F) is just **10 bytes**.

### Bytecode Visualization

```
    ┌─────────────────────────────────────────┐
    │  NPC "Thought" Bytecode (10 bytes)      │
    ├─────────────────────────────────────────┤
    │ 45 17 │ 2A 0C │ 4C 17 │ 0E │ 60 61 13  │
    │ ├──┬──┤ ├──┬──┤ ├──┬──┤ ├──┤ ├────┬────┤
    │ │  │  │ │  │  │ │  │  │ │  │ │    │    │
    │ │  │  │ │  │  │ │  │  │ │  │ │    └─ ifte
    │ │  │  │ │  │  │ │  │  │ │  │ └─ [flee][fight]
    │ │  │  │ │  │  │ │  │  │ └─ and
    │ │  │  │ │  │  │ └──┴─ 'enemy @
    │ │  │  │ └──┴─ 10 <
    │ └──┴─ 'health @
    └─────────────────────────────────────────┘
```

## Memetic Operations

The bytecode format enables evolutionary operations on NPC behaviors:

### Copy (Inheritance)

```go
child := make([]byte, len(parent))
copy(child, parent)
```

### Mutation

```go
// Change "flee" to "hide" (different quotation)
if meme[13] == 0x60 {  // [flee]
    meme[13] = 0x62    // [hide]
}
```

### Crossover

```go
// Combine two thoughts
crosspoint := len(parent1) / 2
child := append(parent1[:crosspoint], parent2[crosspoint:]...)
```

### Inspection

```go
// What does this NPC do when it sees an enemy?
for i, op := range meme {
    if op == 0x4C {  // 'enemy symbol
        // Found enemy reference at position i
        // Analyze surrounding context
    }
}
```

## Z80 Implementation Sketch

### Fetch-Decode-Execute Loop

```asm
vm_loop:
    ld a, (pc)          ; fetch opcode
    inc pc

    bit 7, a            ; check high bit
    jr nz, .multi_byte  ; >= 0x80

.single_byte:
    ; Single-byte dispatch (0x00-0x7F)
    ld l, a
    ld h, high(jump_table)
    jp (hl)

.multi_byte:
    bit 6, a
    jr nz, .three_plus  ; >= 0xC0

.two_byte:
    ; Two-byte: opcode in A, fetch arg
    and 0x3F            ; op = 0-63
    ld b, a
    ld a, (pc)
    inc pc              ; arg in A
    ; dispatch by B
    jp two_byte_dispatch

.three_plus:
    ; Handle 0xC0+ opcodes
    ...
```

### Memory Layout

```
0x0000-0x00FF: Zero page (stack, locals)
0x0100-0x01FF: Symbol slots (256 bytes = 128 slots)
0x0200-0x03FF: Quotation table (pointers)
0x0400-0x07FF: Quotation bodies
0x0800+:       Program code
```

## Performance Characteristics

| Operation | Bytes | Z80 Cycles (est.) |
|-----------|-------|-------------------|
| push 5    | 1     | 10                |
| dup       | 1     | 20                |
| +         | 1     | 30                |
| push.b 100| 2     | 15                |
| 'health @ | 2     | 25                |
| ifte      | 1     | 40+               |

Typical NPC thought (10 bytes): ~250 cycles = 62μs @ 4MHz

## Comparison with Alternatives

| Approach | Size | Speed | Implement | Evolvable |
|----------|------|-------|-----------|-----------|
| micro-PSIL | ★★★★ | ★★★ | ★★★★ | ★★★★★ |
| Forth tokens | ★★★★ | ★★★★ | ★★★ | ★★★ |
| Lua bytecode | ★★ | ★★★★ | ★ | ★★ |
| Native Z80 | ★★★ | ★★★★★ | ★★ | ★ |
| JSON behavior | ★ | ★★ | ★★★★★ | ★★★★ |

## Future Work

1. **Ring-based memory protection** - separate NPC-writable and read-only slots
2. **Energy/gas metering** - limit computation per tick
3. **Inter-NPC communication** - message passing primitives
4. **Serialization format** - save/load memes to disk
5. **JIT compilation** - hot paths to native Z80

## Conclusion

micro-PSIL achieves its design goals:
- **Compact**: 10-50 bytes per thought
- **Implementable**: ~500 lines of Z80 assembly
- **Fast**: Sub-millisecond execution
- **Evolvable**: Simple binary format for genetic operations

The UTF-8 inspired encoding proves effective: common operations (stack manipulation, small numbers, common symbols) fit in single bytes, while the extension mechanism handles edge cases gracefully.

## References

- [PSIL Design Rationale](./2026-01-10-001-psil-design-rationale.md)
- [ADR-001: Bytecode Encoding](../docs/adr/001-bytecode-encoding.md)
- [ADR-002: Stack Format](../docs/adr/002-stack-format.md)
- [ADR-003: Symbol Slots](../docs/adr/003-symbol-slots.md)
- [Forth Threading Techniques](https://www.bradrodriguez.com/papers/moving1.htm)
- [UTF-8 Encoding](https://en.wikipedia.org/wiki/UTF-8)
