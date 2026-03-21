# ADR-010: Multi-user design with profile_id isolation

- **Status:** Accepted
- **Date:** 2026-03-21
- **Author:** Pau Ferran Grau

---

## Context

JobRadar started as a single-user tool — one CV, one set of alert rules, one scoring pipeline. As the project evolved toward a public deployment and potential open source release, multi-user support became a first-class requirement: multiple users should be able to register, upload their CV, and receive personalized job offer scoring independently.

The key design question is how to enforce data isolation between users across a distributed system with multiple services, a message queue, a vector database, and a cache layer.

Two approaches were evaluated:

- **Row-level isolation** — a `profile_id` column on every table and every Kafka message, enforced at the query and application layer
- **Schema-per-user or database-per-user** — complete physical separation of data per user

---

## Decision

Use **row-level isolation with `profile_id`** as the universal tenant identifier across all data stores, Kafka messages, and Valkey keys. Authentication is handled via JWT in the `auth` service, with the `api-gateway` enforcing `profile_id` on every request.

---

## Rationale

### Row-level isolation is sufficient at JobRadar's scale

Schema-per-user or database-per-user provides stronger isolation but comes with significant operational overhead — connection pool explosion, schema migration complexity, and backup management that scales linearly with user count. This is appropriate for regulated industries (healthcare, finance) where tenant isolation is a compliance requirement.

JobRadar does not have compliance requirements that mandate physical separation. Row-level isolation with proper indexing is sufficient and significantly simpler to operate.

### profile_id as universal tenant identifier

Every entity in the system carries a `profile_id`:

**PostgreSQL:**
```sql
CREATE TABLE offers (
    id          UUID PRIMARY KEY DEFAULT uuidv7(),
    profile_id  UUID NOT NULL REFERENCES profiles(id),
    title       TEXT NOT NULL,
    company     TEXT NOT NULL,
    score       NUMERIC(5,2),
    embedding   VECTOR(1024), -- mxbai-embed-large (Ollama) and gemini-embedding-001 with output_dimensionality=1024 (Gemini)
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX ON offers (profile_id, created_at DESC);
CREATE INDEX ON offers (profile_id, score DESC);
```

**Kafka messages:**
```json
{
  "profile_id": "01950000-0000-7000-8000-000000000001",
  "offer_id": "01950000-0000-7000-8000-000000000002",
  "source": "linkedin",
  "raw_content": "..."
}
```

**Valkey keys:**
```
rate:{profile_id}:{window}
score:{profile_id}:{offer_hash}
```

**pgvector queries:**
```sql
SELECT id, title, embedding <=> $1 AS distance
FROM offers
WHERE profile_id = $2
ORDER BY distance
LIMIT 10;
```

### JWT authentication in a dedicated auth service

Authentication is handled by a dedicated `auth` Go service responsible for registration, login, and JWT issuance. The `api-gateway` validates the JWT on every request and extracts the `profile_id` — no downstream service handles authentication directly.

```
Request → api-gateway
    └── validate JWT → extract profile_id
    └── inject profile_id into gRPC metadata
    └── downstream services trust profile_id from metadata
```

Downstream services (`scorer`, `llm-service`, `rag-service`) receive `profile_id` via gRPC metadata and use it for data access — they never perform authentication themselves. This keeps auth logic in one place.

### uuidv7 for profile_id and all primary keys

PostgreSQL 18 introduces native `uuidv7()` — UUID v7 generates time-ordered UUIDs, which index efficiently on B-tree indexes compared to random UUID v4. All primary keys in JobRadar use `uuidv7()`:

```sql
id UUID PRIMARY KEY DEFAULT uuidv7()
```

This is particularly relevant for the `offers` table which receives continuous high-volume inserts — sequential UUIDs avoid index fragmentation.

### Kafka consumer isolation

The `scorer` service consumes `raw-offers` and processes each message using the `profile_id` embedded in the message. All downstream gRPC calls carry the `profile_id` — the scoring pipeline is naturally isolated per user without any additional coordination.

There is no per-user Kafka topic — a single `raw-offers` topic serves all users. This keeps the Kafka configuration simple and avoids topic proliferation as user count grows.

---

## Consequences

**Positive**
- Simple operational model — one schema, standard PostgreSQL backup and migration tooling
- `profile_id` indexing on all tables ensures query isolation without full table scans
- JWT auth centralized in `auth` service — downstream services are auth-agnostic
- uuidv7 primary keys improve insert performance and index efficiency
- Single Kafka topic per event type regardless of user count

**Negative**
- Row-level isolation relies on correct `profile_id` filtering in every query — a missing `WHERE profile_id = $1` leaks data across users. Mitigated by repository-layer conventions and integration tests that verify isolation
- No physical separation — a bug in data access logic could expose one user's data to another. Acceptable risk for a non-regulated application
- JWT secret rotation requires coordinated redeployment of `auth` and `api-gateway`

---

## Alternatives considered

| Option | Reason rejected |
|---|---|
| Schema-per-user | Connection pool explosion, migration complexity, overkill without compliance requirements |
| Database-per-user | Operational overhead scales with user count, backup complexity |
| Single-user only | Limits open source adoption and real-world deployment |
| OAuth2 only (no JWT) | Adds external provider dependency in v1 — planned for v2 with Google login |

---

## Security considerations

- All PostgreSQL queries in repository layer must include `WHERE profile_id = $1` — enforced by code review and integration tests
- `profile_id` is never derived from user input — always extracted from validated JWT
- Valkey keys are namespaced by `profile_id` — no cross-user cache pollution
- gRPC metadata carrying `profile_id` is only set by `api-gateway` after JWT validation — never by external callers