---
description: Implement a JobRadar microservice end-to-end following staff-engineer patterns
---

Implement the `$ARGUMENTS` service for JobRadar.

1. Use the `service-implementer` agent to build the full service (code + initial tests + K8s manifests)
2. Use the `test-writer` agent to review test coverage and fill any gaps
3. Use the `code-reviewer` agent to validate conventions and Go patterns
4. Use the `qa` agent to validate behaviour against the proto contract, ADRs, and domain rules
5. Fix any CRITICAL or MAJOR issues found
6. Confirm `make test-service SVC=$ARGUMENTS` passes
7. Use the `doc-writer` agent to update README.md and create grpcurl examples
