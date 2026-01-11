# ADR-003: Fixed Symbol Slots for NPC State

**Date:** 2026-01-11
**Status:** Accepted
**Deciders:** Architecture Team
**Related:** [ADR-001](./001-bytecode-encoding.md), [micro-PSIL Report](../../reports/2026-01-11-001-micro-psil-bytecode-vm.md)

## Context

NPCs need to access their state (health, position, emotions) efficiently. In full PSIL, this uses a dictionary with string keys. For micro-PSIL, we need something faster and more compact.

Requirements:
1. Single-byte access for common properties
2. No string comparison at runtime
3. Shared vocabulary across all NPCs
4. Support for both read-only and read-write values

## Decision

We use **fixed memory slots** addressed by numeric indices, with predefined symbolic names compiled to slot numbers:

```
Slot    Symbol      Description
────────────────────────────────
0       nil         Null/empty
1       true        Boolean true
2       false       Boolean false
3       self        Current entity ID
4       target      Target entity ID
5       health      Health points
6       energy      Energy/stamina
7       pos         Position (encoded)
8       anger       Anger level (0-255)
9       fear        Fear level (0-255)
10      trust       Trust level (0-255)
11      hunger      Hunger level (0-255)
12      enemy       Enemy nearby flag
13      friend      Friend nearby flag
14      food        Food nearby flag
15      danger      Danger level
...
```

The first 32 slots (0-31) are accessible via single-byte inline symbols (0x40-0x5F).

### Memory Layout

```
Ring 0 (Read-only, set by engine):    slots 0-31
Ring 1 (Read-write, NPC accessible):  slots 32-63
Ring 2 (Scratch/temp):                slots 64-127
Extended (via 2-byte ops):            slots 128-255
```

### Alternatives Considered

#### A. String-Keyed Dictionary

```
"health" get
"health" 100 set
```

**Pros:**
- Human readable
- Unlimited symbols
- Self-documenting

**Cons:**
- String comparison is slow
- Strings consume memory
- Multi-byte per access

**Rejected:** Too slow and memory-heavy for Z80.

#### B. Hash Table

```
hash("health") → slot lookup
```

**Pros:**
- O(1) average lookup
- Dynamic symbol set

**Cons:**
- Hash computation overhead
- Collision handling
- Complex implementation

**Rejected:** Overkill for 32 common symbols.

#### C. Enum-Based (Chosen)

```
'health → 5 → memory[5]
```

**Pros:**
- Single byte instruction (0x45)
- O(1) direct indexing
- No string storage
- Trivial Z80 implementation

**Cons:**
- Fixed vocabulary
- Slot numbers are arbitrary
- Need documentation

## Consequences

### Positive

1. **Single-byte access:** `'health` compiles to `0x45`
2. **Instant lookup:** `memory[slot]` with no search
3. **Shared meaning:** All NPCs use same slot numbers
4. **Ring protection:** Slots 0-31 read-only by convention

### Negative

1. **Fixed vocabulary:** Can't add symbols at runtime
2. **Magic numbers:** Slot 5 means "health" only by convention
3. **Limited inline:** Only 32 single-byte symbols

### Mitigations

1. **Extended symbols:** 2-byte form `[0x81][n]` accesses any slot
2. **Documentation:** Symbol table is well-documented
3. **Assembler:** Human-readable names in source

## Implementation

### Bytecode Generation

```go
var symbols = map[string]byte{
    "nil":     0x40,
    "health":  0x45,
    "energy":  0x46,
    "enemy":   0x4C,
    // ...
}

func assembleSymbol(name string) []byte {
    if op, ok := symbols[name]; ok {
        return []byte{op}  // inline
    }
    // Extended form
    slot := lookupExtended(name)
    return []byte{0x81, slot}
}
```

### Runtime Access

```go
// Inline symbol (0x40-0x5F)
case IsInlineSym(op):
    slot := op - 0x40
    vm.PushInt(int(slot))  // push slot number

// Load operation
case OpLoad:
    slot := byte(vm.PopInt())
    value := vm.MemRead(slot)
    vm.PushWord(value)

// Store operation
case OpStore:
    slot := byte(vm.PopInt())
    value := vm.PopWord()
    vm.MemWrite(slot, value)
```

### Z80 Implementation

```asm
; Symbol access: 'health @ (load health)
; Bytecode: 45 17

sym_health:         ; opcode 0x45
    ld a, 5         ; slot number
    jr push_byte    ; push to stack

op_load:            ; opcode 0x17 (@)
    call pop_byte   ; get slot in A
    ld l, a
    ld h, high(MEMORY)
    ld e, (hl)
    inc hl
    ld d, (hl)      ; DE = memory[slot]
    ex de, hl
    jr push_word    ; push value
```

## Symbol Categories

### Ring 0: Engine-Set (Read-Only)

| Slot | Symbol | Description |
|------|--------|-------------|
| 0 | nil | Null value |
| 1 | true | Boolean true (1) |
| 2 | false | Boolean false (0) |
| 3 | self | This NPC's entity ID |
| 4 | target | Current target ID |
| 5 | health | Current health |
| 6 | energy | Current energy |
| 7 | pos | Encoded position |

### Ring 0: Perception (Read-Only)

| Slot | Symbol | Description |
|------|--------|-------------|
| 12 | enemy | Enemy nearby (0/1) |
| 13 | friend | Friend nearby (0/1) |
| 14 | food | Food nearby (0/1) |
| 15 | danger | Danger level (0-255) |
| 16 | safe | Safety level (0-255) |
| 17 | near | Something near (0/1) |
| 18 | far | Something far (0/1) |
| 19 | day | Is daytime (0/1) |
| 20 | night | Is nighttime (0/1) |

### Ring 1: Emotions (Read-Write)

| Slot | Symbol | Description |
|------|--------|-------------|
| 8 | anger | Anger level |
| 9 | fear | Fear level |
| 10 | trust | Trust level |
| 11 | hunger | Hunger level |

### Ring 2: Scratch

| Slot | Symbol | Description |
|------|--------|-------------|
| 21 | result | Last operation result |
| 22 | count | Counter variable |
| 23 | temp | Temporary storage |
| 24 | x | X coordinate |
| 25 | y | Y coordinate |

## Example Usage

```asm
; Check if should flee
'health @           ; get health (slot 5)
10 <                ; less than 10?
'fear @             ; get fear level (slot 9)
50 >                ; greater than 50?
or                  ; either condition?
[flee] [stay]       ; quotations
ifte                ; decide
```

Compiles to:
```
45 17 2A 0C 49 17 32 0D 0F 60 61 13
```

12 bytes of bytecode for a complex emotional decision.

## References

- [Game Programming Patterns: Component](https://gameprogrammingpatterns.com/component.html)
- [Entity Component Systems](https://en.wikipedia.org/wiki/Entity_component_system)
- [Memory-Mapped I/O](https://en.wikipedia.org/wiki/Memory-mapped_I/O)
