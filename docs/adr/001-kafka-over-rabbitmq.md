# ADR-001: Kafka over RabbitMQ for the ingestion layer

- **Status:** Accepted
- **Date:** 2026-03-21
- **Author:** Pau Ferran Grau

---

## Context

JobRadar needs an async messaging layer to decouple job offer ingestion (fetcher) from AI processing (scorer). The system ingests offers from multiple sources continuously and processes each offer through an AI pipeline that involves multiple gRPC calls (embedder, llm-service, rag-service).

The two main candidates evaluated were **Apache Kafka** and **RabbitMQ**, both well-known and production-proven. RabbitMQ has been used extensively in production at Platinium Group (priority queues, dead letter handling, event versioning across 13+ bounded contexts), so the tradeoffs are based on real operational experience, not theory.

---

## Decision

Use **Apache Kafka** as the messaging backbone for JobRadar.

---

## Rationale

### Where Kafka wins for this use case

**Log retention and replay**
Kafka retains messages for a configurable period regardless of consumption. This is critical for JobRadar: if the scorer is down or a new scoring model is deployed, unprocessed offers can be replayed from the beginning of the retention window. RabbitMQ messages are gone once consumed — reprocessing requires re-fetching from source, which may not always be possible (rate limits, paywalls).

**Multiple independent consumers**
Kafka topics support multiple consumer groups reading the same messages independently. In JobRadar, a single `raw-offers` topic can be consumed simultaneously by the scorer (AI pipeline) and a future analytics consumer (trend aggregation) without any coordination. In RabbitMQ, achieving this requires duplicating messages across exchanges or using fanout exchanges with careful routing configuration.

**Offset-based consumption**
Kafka consumers track their own offset, making it trivial to reprocess a specific time range or replay from a known point. This is directly useful during development — when the scoring logic changes, historical offers can be rescored without touching the fetcher.

**Event sourcing alignment**
The `raw-offers` and `scored-offers` topics act as an immutable event log, which aligns naturally with the append-only nature of job offer ingestion. Kafka's design as a distributed log is a better conceptual fit than RabbitMQ's queue-oriented model.

### Where RabbitMQ wins (and why it doesn't matter here)

**Routing flexibility**
RabbitMQ's exchange/binding model (topic, direct, fanout, headers) is more expressive for complex routing. JobRadar's routing is simple: fetcher → `raw-offers` → scorer → `scored-offers` → api-gateway. No complex routing needed.

**Lower operational complexity**
RabbitMQ is simpler to operate at small scale. Kafka requires ZooKeeper (or KRaft in newer versions), careful partition tuning, and more configuration surface area. For JobRadar this is mitigated by using the Bitnami Helm chart with KRaft mode (no ZooKeeper dependency since Kafka 3.3).

**Lower latency for small messages**
RabbitMQ has lower per-message latency than Kafka for small payloads at low throughput. JobRadar does not have sub-millisecond latency requirements for offer processing — a few seconds from ingestion to scoring is acceptable.

**Better fit for task queues**
RabbitMQ's acknowledgement model and dead letter queues are better suited for task queue patterns (run once, retry on failure). Kafka can replicate this behavior but requires more explicit implementation.

### Why this tradeoff is acceptable

The replay capability alone justifies Kafka for this project. Reprocessing historical offers when scoring logic improves is a first-class requirement, not an edge case. RabbitMQ cannot provide this without significant additional infrastructure.

---

## Consequences

**Positive**
- Full offer history replayable from Kafka retention window
- Multiple consumers (scorer, analytics, alerts) can read `raw-offers` independently
- Natural fit for event-sourcing patterns in the ingestion layer
- KRaft mode eliminates ZooKeeper operational overhead

**Negative**
- Higher operational complexity than RabbitMQ at small scale
- Partition count and replication factor require upfront design decisions
- Consumer group lag monitoring requires explicit tooling (handled by Prometheus + Grafana)

---

## Alternatives considered

| Option | Reason rejected |
|---|---|
| RabbitMQ | No message replay — critical gap for offer reprocessing |
| Redis Streams | Limited ecosystem, less mature tooling for observability |
| Google Pub/Sub | Managed cloud dependency, contradicts self-hosted approach |
| NATS JetStream | Less production-proven than Kafka at the time of decision |