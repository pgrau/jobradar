# ADR-006: Grafana LGTM stack for observability

- **Status:** Accepted
- **Date:** 2026-03-21
- **Author:** Pau Ferran Grau

---

## Context

JobRadar is composed of 8 Go services communicating via Kafka and gRPC. Understanding system behavior in production requires three observability signals: traces (request flow across services), logs (structured events per service), and metrics (throughput, latency, error rates, Kafka consumer lag).

The options evaluated were assembling individual best-of-breed tools (Jaeger for traces, ELK for logs, Prometheus + Grafana for metrics) versus adopting the **Grafana LGTM stack** (Loki + Grafana + Tempo + Mimir/Prometheus) as an integrated suite.

---

## Decision

Use the **Grafana LGTM stack** — Grafana Alloy as OTel collector, Tempo for traces, Loki for logs, Prometheus for metrics, and Grafana as the unified dashboard.

---

## Rationale

### Unified correlation in a single UI

Grafana natively correlates traces, logs, and metrics. From a slow trace in Tempo, one click opens the correlated logs in Loki for the same `trace_id`. From a Prometheus alert, one click opens the relevant traces in Tempo. This correlation is built into Grafana's data source linking — no custom configuration required.

With a fragmented stack (Jaeger + ELK + Grafana), cross-signal correlation requires manual `trace_id` copying or custom plugins.

### Grafana Alloy as OTel collector

Grafana Alloy replaces the standalone OpenTelemetry Collector in the Grafana ecosystem. It receives OTLP (traces, logs, metrics) from all services via a single endpoint and routes each signal to the appropriate backend. One collector, one configuration, one Helm chart.

All Go services export to Alloy via OTLP — the same exporter regardless of which backend stores the signal. Switching from Tempo to Jaeger would require only an Alloy config change, not service code changes.

### OpenTelemetry as the instrumentation standard

All services are instrumented with the **OpenTelemetry SDK for Go** — vendor-neutral, not tied to Grafana. `otelgrpc` interceptors propagate trace context automatically across gRPC calls. `otelhttp` middleware instruments HTTP handlers. The instrumentation investment is portable — it works with any OTel-compatible backend.

```go
// Automatic trace propagation across gRPC — zero per-call instrumentation
grpc.NewServer(
    grpc.UnaryInterceptor(otelgrpc.UnaryServerInterceptor()),
    grpc.StreamInterceptor(otelgrpc.StreamServerInterceptor()),
)
```

### Langfuse → Tempo integration

Langfuse exports LLM traces in OTel format. Alloy ingests them alongside application traces — LLM calls (prompt, response, latency, token count) appear in the same Grafana Tempo view as the Go service traces that triggered them. A single trace from `api-gateway` through `scorer` to `llm-service` includes the Gemini/Ollama call as a child span.

### Self-hosted, all components under Helm

Every component in the LGTM stack has an official Helm chart from the Grafana Labs repository. The entire observability stack deploys with:

```bash
helm repo add grafana https://grafana.github.io/helm-charts
helm upgrade --install alloy grafana/alloy -f k8s/helm/observability/alloy/values.yaml
helm upgrade --install tempo grafana/tempo -f k8s/helm/observability/tempo/values.yaml
helm upgrade --install loki grafana/loki -f k8s/helm/observability/loki/values.yaml
helm upgrade --install prometheus prometheus-community/prometheus ...
helm upgrade --install grafana grafana/grafana -f k8s/helm/observability/grafana/values.yaml
```

No external SaaS dependencies — observability data stays within the cluster.

---

## Consequences

**Positive**
- Single Grafana UI for traces, logs, and metrics with native cross-signal correlation
- OpenTelemetry instrumentation is vendor-neutral — not locked to Grafana
- Grafana Alloy as single OTel ingestion point — one exporter config per service
- Langfuse LLM traces appear alongside application traces in Tempo
- All components self-hosted via official Helm charts

**Negative**
- Higher resource footprint than a minimal setup — Alloy + Tempo + Loki + Prometheus + Grafana running concurrently on a 16GB RAM M2 requires careful resource limits in Helm values
- More components to operate than a single observability SaaS (Datadog, New Relic)
- Loki's log querying (LogQL) has a learning curve compared to Elasticsearch

---

## Alternatives considered

| Option | Reason rejected |
|---|---|
| Jaeger + ELK + Grafana | Three separate stacks, manual cross-signal correlation, higher operational complexity |
| Datadog / New Relic | SaaS cost at scale, data leaves the cluster, contradicts self-hosted approach |
| Zipkin | Traces only — no logs or metrics integration, less active development |
| Grafana Cloud | SaaS dependency, data leaves the cluster |

---

## Resource configuration for local development

Running the full LGTM stack locally on a 16GB M2 requires conservative resource limits:

```yaml
# Recommended limits for local kind cluster
tempo:
  resources:
    requests: { memory: 256Mi, cpu: 100m }
    limits:   { memory: 512Mi, cpu: 500m }

loki:
  resources:
    requests: { memory: 256Mi, cpu: 100m }
    limits:   { memory: 512Mi, cpu: 500m }

grafana:
  resources:
    requests: { memory: 128Mi, cpu: 50m }
    limits:   { memory: 256Mi, cpu: 200m }
```

On Hetzner production, limits can be relaxed based on actual usage observed in Grafana.