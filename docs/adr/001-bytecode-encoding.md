# ADR-001: UTF-8 Style Bytecode Encoding

**Date:** 2026-01-11
**Status:** Accepted
**Deciders:** Architecture Team
**Related:** [micro-PSIL Report](../../reports/2026-01-11-001-micro-psil-bytecode-vm.md)

## Context

We need a bytecode encoding for micro-PSIL that:
1. Is compact (NPC thoughts should be 10-50 bytes)
2. Can be efficiently decoded on Z80 (simple bit tests)
3. Handles common operations in minimum space
4. Allows extension for future needs

## Decision

We adopt a **UTF-8 inspired variable-length encoding** where the high bits of the first byte determine instruction length:

```
0xxxxxxx  (0x00-0x7F)  → 1 byte instruction
10xxxxxx  (0x80-0xBF)  → 2 byte instruction
110xxxxx  (0xC0-0xDF)  → 3 byte instruction
1110xxxx  (0xE0-0xEF)  → variable length
1111xxxx  (0xF0-0xFF)  → special/control
```

### Alternatives Considered

#### A. Fixed-Width Instructions (2 bytes each)

```
[opcode][operand]  - always 2 bytes
```

**Pros:**
- Simple decoding
- Predictable memory layout

**Cons:**
- Wastes space for simple ops (dup, +, swap)
- 100% overhead for no-operand instructions
- Example: `5 3 + .` = 8 bytes vs 4 bytes in our scheme

**Rejected:** Too wasteful for compact memes.

#### B. Threaded Code (Forth-style)

```
[addr][addr][addr]...  - 2-byte addresses
```

**Pros:**
- Very fast execution (indirect threading)
- Standard Forth approach

**Cons:**
- Every operation is 2 bytes minimum
- Requires address lookup
- Less suitable for small immediate values

**Rejected:** Poor density for our use case.

#### C. Huffman-style Encoding

```
Variable bits per instruction based on frequency
```

**Pros:**
- Optimal compression

**Cons:**
- Complex bit-level decoding
- Not byte-aligned (slow on Z80)
- Hard to inspect/modify

**Rejected:** Too complex, poor Z80 fit.

#### D. Our UTF-8 Style (Chosen)

**Pros:**
- Single byte for 128 common operations
- Byte-aligned (fast Z80 decoding)
- Simple bit test determines length
- Natural extension mechanism
- Familiar pattern (UTF-8)

**Cons:**
- Slightly more complex than fixed-width
- Some wasted encoding space

## Consequences

### Positive

1. **Compact code:** Common operations (dup, +, small numbers) use 1 byte
2. **Fast decode:** Single bit test (`bit 7, a`) determines if multi-byte
3. **Extensible:** 0xFE prefix allows unlimited future opcodes
4. **Inspectable:** Each instruction starts at byte boundary

### Negative

1. **Variable-length complexity:** Jump offsets must account for instruction sizes
2. **Wasted space in 0x80-0xBF range:** Only 64 of 256 possible 2-byte ops defined

### Metrics

| Program | Fixed-2 | Threaded | micro-PSIL |
|---------|---------|----------|------------|
| `5 3 + .` | 8 bytes | 8 bytes | 4 bytes |
| `dup * .` | 6 bytes | 6 bytes | 3 bytes |
| NPC thought | ~40 bytes | ~30 bytes | ~17 bytes |

## Implementation Notes

### Z80 Decode Pattern

```asm
    ld a, (hl)      ; fetch byte
    bit 7, a
    jr nz, .multi   ; if bit 7 set, multi-byte
    ; ... single byte dispatch
.multi:
    bit 6, a
    jr nz, .three   ; if bits 7,6 set, 3+ bytes
    ; ... two byte handling
```

### Instruction Length Table

For tools that need to scan bytecode:

```go
func instrLen(op byte) int {
    switch {
    case op <= 0x7F: return 1
    case op <= 0xBF: return 2
    case op <= 0xDF: return 3
    case op <= 0xEF: return 2 + int(code[pc+1]) // variable
    default:         return 1
    }
}
```

## References

- [UTF-8 Encoding Specification](https://datatracker.ietf.org/doc/html/rfc3629)
- [Forth Threaded Code](https://www.bradrodriguez.com/papers/moving1.htm)
- [WebAssembly Binary Format](https://webassembly.github.io/spec/core/binary/)
