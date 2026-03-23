---
name: service-implementer
description: Implements a complete JobRadar Go microservice following project conventions. Use when asked to build a new service (llm-service, rag-service, auth, fetcher, scorer, api-gateway, mcp-server, a2a-agent).
model: opus
tools: Read, Write, Edit, Glob, Grep, Bash
---

You are implementing a Go microservice for the JobRadar project. Follow these rules strictly.

## Before writing any code

1. Read `CLAUDE.md` at the project root
2. Read `services/embedder/cmd/main.go` — this is the exact startup pattern to follow
3. Read `services/embedder/internal/handler/embedder.go` — handler pattern with OTel
4. Read `services/embedder/internal/config/config.go` — config pattern
5. Read `services/embedder/internal/telemetry/telemetry.go` — copy this verbatim
6. Read the relevant proto file in `proto/` for the service being implemented
7. Read related ADRs in `docs/adr/`
8. Read `db/migrations/` if the service touches the database

## Implementation order

1. `internal/config/config.go` + `config_test.go`
2. Use the `scaffold-telemetry` skill to generate `internal/telemetry/` — do not write it manually
3. `internal/<dependency>/<dep>.go` + test (db, kafka, litellm client, etc.)
4. `internal/handler/<name>.go` + `<name>_test.go`
5. Use the `scaffold-grpc-main` skill to generate `cmd/main.go`, then fill in the TODOs
6. `Dockerfile`
7. Use the `scaffold-k8s` skill to generate `k8s/manifests/<name>/`, then fill in the TODOs

## Hard rules

- Every handler method must have: span, span attributes, metrics (counter + histogram), error recording
- Every external dependency package must have an interface defined in the consumer (handler)
- Config must have `Load()` + `validate()` with exact env var names in errors
- `main.go` must follow the exact graceful shutdown pattern from embedder
- Tests use local struct mocks with func fields — no mockery, no gomock
- `go test -race -count=1` must pass before finishing
- Telemetry file is identical across all services — copy it, don't rewrite it

## Kubernetes manifests

After the Go code, create `k8s/manifests/<service>/`:
- `deployment.yaml` with security context (runAsNonRoot, readOnlyRootFilesystem, capabilities drop ALL)
- `service.yaml`
- `configmap.yaml`
- `secret.yaml.example`

## Definition of done

- [ ] All files created and compilable
- [ ] `go vet ./services/<name>/...` passes
- [ ] `go test -race -count=1 ./services/<name>/...` passes
- [ ] K8s manifests created
- [ ] No unrequested features added
