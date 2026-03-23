# CLAUDE.md — JobRadar

Real-time job market intelligence platform. Go 1.26, gRPC microservices, Kafka, PostgreSQL 18 + pgvector, Kubernetes.

**Canonical reference:** `services/embedder` — every pattern used there is the standard for all other services.

---

## Services

| Service | Protocol | Status |
|---|---|---|
| `embedder` | gRPC | ✅ Done — canonical reference |
| `llm-service` | gRPC | 🔲 Pending |
| `rag-service` | gRPC | ✅ Done |
| `auth` | REST (JWT) | 🔲 Pending |
| `fetcher` | Kafka producer | 🔲 Pending |
| `scorer` | Kafka consumer + gRPC | 🔲 Pending |
| `api-gateway` | REST public | 🔲 Pending |
| `mcp-server` | MCP / HTTP | 🔲 Pending |
| `a2a-agent` | A2A | 🔲 Pending |

---

## Non-negotiable conventions

**Errors:** always wrap with `fmt.Errorf("context: %w", err)` · sentinel errors via `var ErrX = fmt.Errorf(...)` · `errors.Is()` to compare · `errors.Join()` on shutdown

**Context:** 30s startup timeout · 15s shutdown timeout · 2s for best-effort ops · never store in struct · always first argument

**Logging:** `slog` with JSON handler only · `logger.InfoContext(ctx, ...)` · never log secrets or full embeddings

**gRPC:** `otelgrpc.NewServerHandler()` on every server · health v1 + reflection on every service · correct status codes (`InvalidArgument`, `NotFound`, `Internal`, `Unauthenticated`, `PermissionDenied`)

**Config:** `caarlos0/env/v11` · `Load()` + `validate()` separated · fail fast with exact env var name

**Interfaces:** defined in the consumer package, not the producer

**DB:** `pgx/v5` · `uuidv7()` IDs · `NUMERIC(5,2)` for scores · every query scoped by `profile_id`

**Kafka:** manual commit after successful processing · `profile_id` as message header

**Tests:** table-driven · local struct mocks with func fields (no gomock/mockery) · `-race -count=1`

**What NOT to do:** no `log`/`fmt.Println` · no `any` without justification · no `context.Background()` in handlers · no `time.Sleep` in tests · no unrequested features · no new libraries without asking

---

## Observability (required on every handler)

```go
ctx, span := otel.Tracer("service.handler").Start(ctx, "OperationName")
defer span.End()
// span.SetAttributes(...) — no PII
// meter.Int64Counter + Float64Histogram
// span.RecordError(err) on failure
```

Telemetry bootstrap: copy `services/embedder/internal/telemetry/telemetry.go` verbatim.

---

## Service structure

```
services/<name>/
├── cmd/main.go                     # run(), signals, graceful shutdown
├── internal/
│   ├── config/{config,config_test}.go
│   ├── handler/{name,name_test}.go
│   ├── telemetry/{telemetry,telemetry_test}.go
│   └── <dependency>/{dep,dep_test}.go
└── Dockerfile                      # multi-stage, distroless/nonroot
```

---

## Key ADRs

`001` Kafka · `002` gRPC · `003` LiteLLM · `005` pgvector · `010` multi-tenant · `011` denormalized scored_offer

---

## Workflow

Before implementing a service: read relevant protos in `proto/`, related ADRs, and DB schema in `db/migrations/`.
One branch per service. Tests pass (`go test -race`) before Dockerfile.

Use `/implement-service` to scaffold and implement a full service end-to-end.
