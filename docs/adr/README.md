# Architecture Decision Records

This directory contains Architecture Decision Records (ADRs) for the PSIL project.

## What is an ADR?

An ADR is a document that captures an important architectural decision made along with its context and consequences. ADRs help team members understand why certain decisions were made and provide a reference for future development.

## Index

| ADR | Title | Status | Date |
|-----|-------|--------|------|
| [001](./001-bytecode-encoding.md) | UTF-8 Style Bytecode Encoding | Accepted | 2026-01-11 |
| [002](./002-stack-format.md) | Tagged Stack Value Format | Accepted | 2026-01-11 |
| [003](./003-symbol-slots.md) | Fixed Symbol Slots for NPC State | Accepted | 2026-01-11 |

## ADR Status Definitions

- **Proposed:** Under discussion
- **Accepted:** Decision made and implemented
- **Deprecated:** No longer applies (superseded or project changed)
- **Superseded:** Replaced by another ADR

## Template

New ADRs should follow this structure:

```markdown
# ADR-NNN: Title

**Date:** YYYY-MM-DD
**Status:** Proposed | Accepted | Deprecated | Superseded
**Deciders:** Who made this decision
**Related:** Links to related ADRs or documents

## Context

What is the issue that we're seeing that is motivating this decision?

## Decision

What is the change that we're proposing and/or doing?

### Alternatives Considered

What other options were evaluated?

## Consequences

What becomes easier or more difficult because of this change?

## References

Links to relevant resources.
```

## Related Documentation

- [micro-PSIL Design Report](../reports/2026-01-11-001-micro-psil-bytecode-vm.md)
- [PSIL Design Rationale](../reports/2026-01-10-001-psil-design-rationale.md)
