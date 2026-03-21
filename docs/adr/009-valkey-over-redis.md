# ADR-009: Valkey over Redis

- **Status:** Accepted
- **Date:** 2026-03-21
- **Author:** Pau Ferran Grau

---

## Context

JobRadar requires an in-memory data store for two specific use cases: rate limiting per user on the `api-gateway` and `mcp-server`, and caching LLM responses to avoid redundant inference calls for identical inputs.

Redis is the industry standard for this type of workload and has been for over a decade. However, in March 2024 Redis Ltd. changed its license from BSD to a dual RSALv2/SSPLv1 license, which is not considered open source by the Open Source Initiative (OSI). This triggered a community fork — **Valkey** — under the Linux Foundation with a permanent BSD license.

---

## Decision

Use **Valkey** instead of Redis.

---

## Rationale

### License — open source matters for a public project

Redis is no longer open source under the OSI definition. For a public GitHub project intended as a portfolio piece and potential open source release, using a non-OSI-compliant dependency sends the wrong signal — and creates ambiguity about redistribution rights for anyone who forks or deploys the project.

Valkey is BSD-licensed — the same license as the original Redis codebase before the change. No legal ambiguity.

### Governance — Linux Foundation vs single vendor

Redis is controlled by Redis Ltd., which demonstrated it can change licensing terms unilaterally. Valkey is governed by the Linux Foundation — a neutral, multi-stakeholder foundation with a track record of stable open source stewardship (Linux, Kubernetes, Kafka, OpenTelemetry).

For a long-lived project, governance stability matters. The risk of a future license change in Valkey is structurally lower than in Redis.

### Drop-in compatibility

Valkey is a fork of Redis 7.2.4 — the last BSD-licensed version. It is fully compatible at the protocol, API, and data format level. Every Redis client works with Valkey without modification. In Go, `go-redis` connects to Valkey with only a URL change:

```go
client := valkey.NewClient(valkey.ClientOption{
    InitAddress: []string{"valkey:6379"},
})
```

Or using `go-redis` (Redis-compatible client):
```go
rdb := redis.NewClient(&redis.Options{
    Addr: "valkey:6379",
})
```

No code changes, no data migration, no retraining required.

### Production adoption

Major cloud providers have already migrated to Valkey:
- AWS ElastiCache Serverless uses Valkey by default
- Google Cloud Memorystore offers Valkey as the primary option
- Aiven, Upstash, and other managed providers support Valkey

This confirms Valkey is production-ready and has long-term infrastructure support.

### Performance

Valkey 8.0 introduced an asynchronous I/O model where the main thread and I/O threads operate concurrently. For JobRadar's rate limiting use case — high-concurrency atomic increments per `profile_id` — this improves throughput under load compared to Redis 7.x's threading model.

---

## Use cases in JobRadar

### Rate limiting

Per-user rate limiting on `api-gateway` and `mcp-server` using atomic increment with TTL:

```go
key := fmt.Sprintf("rate:%s:%s", profileID, window)
count, err := valkeyClient.Incr(ctx, key).Result()
if count == 1 {
    valkeyClient.Expire(ctx, key, time.Minute)
}
if count > limit {
    return ErrRateLimitExceeded
}
```

### LLM response cache

Cache scored offers to avoid redundant Gemini/Ollama calls for identical offer+profile combinations:

```go
cacheKey := fmt.Sprintf("score:%s:%s", offerHash, profileID)
if cached, err := valkeyClient.Get(ctx, cacheKey).Result(); err == nil {
    return cached, nil
}
// ... call llm-service via gRPC
valkeyClient.Set(ctx, cacheKey, result, 24*time.Hour)
```

---

## Consequences

**Positive**
- BSD license — no legal ambiguity for a public open source project
- Linux Foundation governance — stable long-term stewardship
- Full Redis protocol compatibility — existing Go clients work unchanged
- Async I/O in Valkey 8.0 improves throughput under concurrent load
- Adopted by AWS, Google Cloud, and major managed providers

**Negative**
- Smaller community and ecosystem than Redis — fewer third-party integrations and Stack Overflow answers
- Some Redis-specific tooling (RedisInsight) requires configuration to connect to Valkey
- Less name recognition in job descriptions — "Redis experience" still the common phrasing

---

## Alternatives considered

| Option | Reason rejected |
|---|---|
| Redis | Non-OSI license incompatible with open source project goals |
| KeyDB | Fork with different focus (multi-threading), less community momentum than Valkey |
| Dragonfly | Different internal architecture, not a drop-in fork — compatibility gaps possible |
| PostgreSQL (advisory locks + unlogged tables) | Higher latency for rate limiting, not suited for sub-millisecond atomic operations |