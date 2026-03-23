---
description: Create a new Architecture Decision Record
---

Create a new ADR for: $ARGUMENTS

1. Find the next available number by checking `docs/adr/` directory
2. Create `docs/adr/<NNN>-<slug>.md` following this structure:

```markdown
# ADR-<NNN>: <Title>

## Status
Accepted

## Context
[What problem are we solving? What forces are at play?]

## Decision
[What did we decide?]

## Rationale
[Why this decision over the alternatives?]

## Consequences
[What becomes easier? What becomes harder?]

## Alternatives considered
[What else did we evaluate and why we rejected it]
```

3. Add the new ADR to the list in `README.md` under "Architecture Decision Records"
