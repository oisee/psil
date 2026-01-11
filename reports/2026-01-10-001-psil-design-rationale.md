# PSIL: Design Rationale and Theoretical Foundations

**Date:** 2026-01-10
**Document:** 001
**Topic:** Language Design and Theory

## Executive Summary

PSIL (Point-free Stack-based Interpreted Language) is a concatenative, stack-based, point-free functional language with quotations as first-class values. It is designed for VM execution with a path to Z80/6502 native compilation, and intended as a scripting language for emergent NPC behavior in games.

## Theoretical Position

PSIL sits at the intersection of several paradigms:

| Aspect | Influence |
|--------|-----------|
| Theory | Joy (concatenative calculus) |
| Compilation | CPS / SSA |
| Execution | Forth (stack machine) |
| Expressiveness | Lambda calculus |
| Effects | Hardware flags (Z80) |

### Key Classification

PSIL is a:
- **Concatenative functional language** - programs are compositions
- **Point-free** - no named variables
- **Stack-based** - implicit data stack
- **Quotation-centric** - code blocks are first-class values
- **CPS-compatible** - control flow via combinators
- **SSA-friendly** - suitable for optimization
- **VM-oriented** - designed for bytecode execution
- **Hardware-aware** - flags map to CPU semantics

## Stack vs Register Machines

### Equivalence
Both stack and register machines are Turing-complete and can emulate each other. The choice is about representation, not capability.

### Trade-offs

| Criterion | Stack | Register |
|-----------|-------|----------|
| Code density | Excellent | Moderate |
| Hardware speed | Moderate | Excellent |
| AST mapping | Direct | Indirect |
| IR simplicity | High | Low |
| Optimization | Limited | Extensive |

PSIL chooses stack-based as primary representation because:
1. Direct mapping from source to execution
2. Compact bytecode for constrained targets (Z80)
3. Natural fit for quotations and combinators
4. Later compilation to registers via SSA

## Lambda Calculus Relationship

Lambda calculus naturally maps to stack machines:
- Application = push + call
- Arguments = stack positions
- Reduction = stack computation

Historical precedents:
- SECD machine (Landin, 1964)
- Krivine machine
- G-machine (Haskell)

PSIL is structurally closer to these theoretical machines than to register-based VMs.

## CPS (Continuation-Passing Style)

### What It Is
CPS makes "what happens next" explicit as a function argument:

```
f(x) = g(x) + 1

// Becomes:
f(x, k) = g(x, lambda v: k(v + 1))
```

### Why It Matters for PSIL
PSIL is already CPS-like:
- `ifte` selects continuation based on condition
- `linrec` is explicit recursive continuation
- Quotations ARE continuation objects
- No implicit return - everything is explicit

This means PSIL doesn't need a separate CPS transformation for compilation.

## SSA (Static Single Assignment)

### Definition
Each variable is assigned exactly once:
```
x = x + 1  ->  x1 = x0 + 1
```

### Connection to PSIL
The compilation path:
```
PSIL -> CPS (semantic) -> SSA -> Register Allocation -> Native
```

PSIL's stack operations map cleanly to SSA because:
- No mutable variables
- Explicit data flow through stack
- Quotations as labeled blocks

## Point-Free Style

### Definition
Programs without explicit arguments - only compositions:
```
f(x) = g(h(x))  ->  f = g . h
```

### PSIL as Point-Free
```psil
dup 1 - fib
```
The argument is never named - it's implicitly on the stack. This is pure point-free.

## Comparison: Joy, PSIL, Forth

### Joy vs PSIL

| Aspect | Joy | PSIL |
|--------|-----|------|
| Goal | Theory | Execution |
| Purity | Absolute | Practical |
| Effects | None | Flags, errors |
| Hardware | Abstract | Direct |
| VM | No | Yes |

**Joy** is a language of ideas.
**PSIL** is a language of machines.

### Forth vs PSIL

| Aspect | Forth | PSIL |
|--------|-------|------|
| Paradigm | Imperative | Functional |
| Quotations | Limited | First-class |
| Control flow | Syntax | Combinators |
| CPS compatibility | No | Yes |
| SSA friendly | No | Yes |

**Forth** is procedural with a stack.
**PSIL** is an algebra executed on a stack.

## Error Handling: Hardware-Inspired Design

PSIL uses CPU-style flags for effects:

| Flag | Purpose | Z80 Equivalent |
|------|---------|----------------|
| Z | Boolean result | Zero flag |
| C | Error condition | Carry flag |
| A | Error code | Accumulator |

Benefits:
- Zero-cost propagation
- CPS-compatible (errors are continuations)
- Direct hardware mapping
- Similar to Result/Either types

### Auto-propagation
When C flag is set, most operations become no-ops. This provides automatic error propagation without explicit checks at each step.

## Named Quotations

The `DEFINE name == [quotation].` syntax is a legitimate extension:
- Values, not procedures
- Fully compatible with point-free
- Equivalent to let-binding in lambda calculus

## Protocols vs Objects

### Objects: No
OOP would violate:
- Point-free style
- Referential transparency
- SSA compatibility

### Protocols: Yes (Functionally)
A protocol is a stack-effect contract:
```
( a b -- flag )  % comparison protocol
```
Any quotation satisfying the contract is an implementation.

This is typeclass-like without objects.

## VM Design Considerations

### Opcode Mapping

Each PSIL operation should map 1:1 to bytecode:

| PSIL | Opcode | Z80 |
|------|--------|-----|
| `dup` | 0x01 | `LD A,(HL); PUSH AF` |
| `drop` | 0x02 | `POP AF` |
| `+` | 0x10 | `ADD A,B` |
| `<` | 0x20 | `CP; sets Z,C` |
| `ifte` | 0x30 | `JP Z,addr` |

### Internal Representation

The AST converts to a flat IR:
```go
type Opcode byte
const (
    OpPushNum   Opcode = 0x40
    OpDup       Opcode = 0x01
    OpAdd       Opcode = 0x10
    OpIfte      Opcode = 0x30
)
```

## Application: NPC Mind Scripting

PSIL is designed as a scripting language for NPC behavior where:
- Quotations = thoughts/intentions
- Composition = influence
- Stack = focus of attention
- Code is data (memetic engineering)

### Ring-Based Protection (Future)
```
Ring 0 (ROM): Instincts - immutable
Ring 1: Core personality - protected
Ring 2: Skills/memory - writable
Ring 3: Current thoughts - open
```

### Gas/Energy Limits (Future)
Computation costs energy, enabling:
- Infinite loop protection
- "Computational exhaustion" gameplay
- Resource-limited AI

## Conclusion

PSIL occupies a unique position:
- **Theoretically grounded** in Joy and lambda calculus
- **Practically executable** on constrained hardware
- **Compilation-ready** via CPS/SSA path
- **Application-specific** for emergent AI

It is neither purely academic (like Joy) nor purely pragmatic (like Forth), but a synthesis designed for a specific purpose: programmable, composable intelligence.

## References

- Manfred von Thun, "Joy: Forth's Functional Cousin"
- Peter Landin, "The Mechanical Evaluation of Expressions" (SECD)
- Andrew Appel, "Compiling with Continuations"
- Keith Cooper, "Engineering a Compiler" (SSA)

---

*Document generated during PSIL implementation, 2026-01-10*
