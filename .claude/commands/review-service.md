---
description: Review a JobRadar service against staff-engineer standards
---

Review the `$ARGUMENTS` service using the `code-reviewer` agent.

The agent will check:
- Error handling patterns
- Context usage
- Observability (OTel spans + metrics)
- gRPC conventions
- Config structure
- Test quality
- Kubernetes manifests security

Output: list of CRITICAL / MAJOR / MINOR issues + final verdict.
