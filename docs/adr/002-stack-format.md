# ADR-002: Tagged Stack Value Format

**Date:** 2026-01-11
**Status:** Accepted
**Deciders:** Architecture Team
**Related:** [ADR-001](./001-bytecode-encoding.md), [micro-PSIL Report](../../reports/2026-01-11-001-micro-psil-bytecode-vm.md)

## Context

The stack needs to support values of different sizes (bytes, words, potentially larger). Operations like `dup` need to know how many bytes to copy. We must balance:

1. **Type safety:** Prevent operations on wrong types
2. **Compactness:** Minimize memory overhead
3. **Speed:** Fast push/pop on Z80
4. **Simplicity:** Easy to implement

## Decision

We use a **minimal size-tagged format** with 1-byte size prefix:

```
Stack Layout:
[size][data...][size][data...]...

Byte value (2 bytes total):
[01][value]

Word value (3 bytes total):
[02][lo][hi]
```

The size byte indicates data length (1-255 bytes). Operations read the size to determine how many bytes to manipulate.

### Alternatives Considered

#### A. Untagged Stack (Pure Forth)

```
Stack: [data][data][data]...
```

All values are fixed width (e.g., 16-bit cells).

**Pros:**
- Simplest possible
- Fastest operations
- No overhead

**Cons:**
- `dup` can only copy fixed size
- Bytes waste 50% space
- Can't mix sizes

**Rejected:** Too inflexible for our needs.

#### B. Full Type Tags

```
[type][size][data...]

Types: 0=nil, 1=bool, 2=i8, 3=i16, 4=i32, 5=str, 6=quot...
```

**Pros:**
- Full type information
- Enables runtime type checking
- Supports introspection

**Cons:**
- 2 bytes overhead minimum
- Complex dispatch logic
- Overkill for simple VM

**Rejected:** Too heavy for micro-PSIL goals.

#### C. Nibble Tag (Type + Size in one byte)

```
[TTTTSSSS][data...]

T = type (4 bits, 16 types)
S = size (4 bits, 1-15 bytes)
```

**Pros:**
- Single byte overhead
- Type and size together
- Supports 16 types

**Cons:**
- Limited to 15-byte values
- Complex bit manipulation
- Still more than we need

**Rejected:** Unnecessary complexity.

#### D. Size-Only Tag (Chosen)

```
[size][data...]

size = 1-255 bytes
```

**Pros:**
- Minimal overhead (1 byte)
- `dup` works: read size, copy size+1 bytes
- Simple arithmetic: assumes both are same size
- Z80 friendly: `ld a, (sp)` gets size

**Cons:**
- No type information (arithmetic on strings = garbage)
- Caller must know what they're operating on

## Consequences

### Positive

1. **`dup` works generically:**
   ```asm
   dup:
       ld a, (sp)      ; size
       inc a           ; size + 1 for tag
       ; copy 'a' bytes
   ```

2. **Low overhead:** 1 byte per value vs 0 for untagged

3. **Variable sizes:** Can push 1, 2, or N byte values

### Negative

1. **Type confusion possible:** `"hello" 5 +` produces garbage
2. **Operations assume matching sizes:** `byte + word` needs care

### Mitigations

1. **Consistent promotion:** All arithmetic promotes to word (2 bytes)
2. **Convention:** Quotation indices use high bit (0x8000+)
3. **Debug mode:** Full interpreter has type checking

## Implementation

### Push Operations

```go
func (vm *VM) PushByte(v byte) {
    vm.Stack[vm.SP] = 1    // size
    vm.Stack[vm.SP+1] = v  // value
    vm.SP += 2
}

func (vm *VM) PushWord(v int16) {
    vm.Stack[vm.SP] = 2           // size
    vm.Stack[vm.SP+1] = byte(v)   // lo
    vm.Stack[vm.SP+2] = byte(v>>8) // hi
    vm.SP += 3
}
```

### Pop Operations

```go
func (vm *VM) PopWord() int16 {
    // Try word first (most common)
    if vm.SP >= 3 && vm.Stack[vm.SP-3] == 2 {
        lo := vm.Stack[vm.SP-2]
        hi := vm.Stack[vm.SP-1]
        vm.SP -= 3
        return int16(lo) | int16(hi)<<8
    }
    // Fall back to byte promotion
    if vm.SP >= 2 && vm.Stack[vm.SP-2] == 1 {
        v := vm.Stack[vm.SP-1]
        vm.SP -= 2
        return int16(v)
    }
    // Error
    return 0
}
```

### Z80 Implementation

```asm
; pop_word - pop 16-bit value from stack
; Returns: HL = value
; Destroys: A
pop_word:
    ld a, (sp)
    cp 2
    jr z, .is_word
    cp 1
    jr z, .is_byte
    ; error
    ld hl, 0
    ret

.is_word:
    inc sp              ; skip size
    pop hl              ; get value
    ret

.is_byte:
    inc sp              ; skip size
    ld l, (sp)          ; get byte
    inc sp
    ld h, 0             ; zero extend
    ret
```

## Memory Layout Example

```
Stack growing upward:

SP → [free space]
     ─────────────
     [02]         ← size (word)
     [05]         ← lo byte (value = 5)
     [00]         ← hi byte
     ─────────────
     [01]         ← size (byte)
     [03]         ← value (3)
     ─────────────
     [bottom]
```

After `5 3`:
- SP = 5 (2 bytes for "3", 3 bytes for "5")
- Total: 5 bytes for two values

## References

- [Forth Cell Size Considerations](https://forth-standard.org/)
- [Tagged Architectures](https://en.wikipedia.org/wiki/Tagged_architecture)
- [Z80 Stack Operations](http://z80-heaven.wikidot.com/instructions-set:push)
