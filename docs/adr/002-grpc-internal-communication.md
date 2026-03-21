# ADR-002: gRPC for internal service communication

- **Status:** Accepted
- **Date:** 2026-03-21
- **Author:** Pau Ferran Grau

---

## Context

JobRadar is composed of multiple Go microservices that communicate synchronously for specific operations: the `scorer` calls `embedder`, `llm-service`, and `rag-service` to process each offer; the `api-gateway` calls `rag-service` and `llm-service` to serve frontend queries; the `mcp-server` and `a2a-agent` call internal services to fulfill tool requests.

The communication protocol for these internal synchronous calls needs to be decided. The main candidates are **gRPC** and **REST over HTTP/JSON**.

REST is already used for the public API (`api-gateway` → frontend) where broad client compatibility is required. The question is whether internal service-to-service communication should use the same protocol or a different one.

---

## Decision

Use **gRPC + Protocol Buffers** for all internal service-to-service communication.

---

## Rationale

### Strong typing via Protocol Buffers

gRPC contracts are defined in `.proto` files — the schema is the source of truth, not documentation or convention. Any breaking change in a service interface is caught at compile time, not at runtime. For a project with 8 Go services sharing internal contracts, this eliminates an entire class of integration bugs.

REST + JSON requires either OpenAPI specs (which can drift from implementation) or runtime discovery of contract mismatches.

### Performance

Protocol Buffers serialization is significantly more compact and faster than JSON. For JobRadar's scoring pipeline — where a single offer triggers sequential gRPC calls to `embedder`, `llm-service`, and `rag-service` — reduced serialization overhead compounds across the pipeline.

Benchmarks consistently show Protobuf serialization at 3-10x faster than JSON with 2-5x smaller payload size for equivalent data structures.

### Native Go code generation

`protoc` with `protoc-gen-go` and `protoc-gen-go-grpc` generates type-safe client and server stubs directly from `.proto` files. Internal service clients are generated, not hand-written — reducing boilerplate and ensuring consistency across all services.

### Streaming support

gRPC supports four communication patterns: unary, server streaming, client streaming, and bidirectional streaming. JobRadar uses server streaming for the `llm-service` — LLM token responses are streamed back to the caller rather than waiting for the full response. This is not possible with standard REST without SSE or WebSocket workarounds.

### OpenTelemetry integration

`otelgrpc` interceptors provide automatic trace propagation across gRPC calls with zero per-call instrumentation. A single request from `api-gateway` through `scorer` to `llm-service` appears as one continuous trace in Grafana Tempo without any manual span creation.

### gRPC-gateway for public API

`grpc-gateway` generates a REST/HTTP JSON proxy from `.proto` annotations. The `api-gateway` exposes a standard REST API to the frontend while internally using gRPC — one protocol definition serves both internal and external contracts.

---

## Consequences

**Positive**
- Compile-time contract validation across all internal services
- Automatic client/server code generation from `.proto` files
- Native server streaming for LLM token responses
- Automatic OTel trace propagation via interceptors
- Single `.proto` source of truth for internal contracts

**Negative**
- Initial setup overhead — protoc, plugins, generated code workflow
- `.proto` files require a shared `proto/` directory and consistent generation tooling
- Debugging requires tooling (grpcurl, grpcui) — not as simple as curl for REST
- Learning curve for engineers unfamiliar with Protocol Buffers

---

## Alternatives considered

| Option | Reason rejected |
|---|---|
| REST/JSON (internal) | No compile-time contract validation, slower serialization, no native streaming |
| GraphQL | Overkill for service-to-service, adds resolver complexity with no benefit |
| Thrift | Less ecosystem support in Go, gRPC is the industry standard |
| Message passing only (Kafka) | Async-only — some operations require synchronous request/response semantics |

---

## Proto structure

All `.proto` files live in `proto/` at the root of the monorepo, versioned by service:

```
proto/
  llm/v1/llm.proto
  rag/v1/rag.proto
  embedder/v1/embedder.proto
```

Versioning (`v1`) allows non-breaking evolution — a `v2` can be introduced alongside `v1` without forcing simultaneous updates across all consumers.