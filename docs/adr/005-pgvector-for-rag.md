# ADR-005: pgvector for RAG over dedicated vector database

- **Status:** Accepted
- **Date:** 2026-03-21
- **Author:** Pau Ferran Grau

---

## Context

JobRadar requires vector similarity search for two operations: scoring job offers against a user CV (semantic similarity between offer embeddings and CV embeddings) and RAG-augmented context retrieval (finding historically similar offers when generating LLM responses).

This requires storing and querying high-dimensional vectors efficiently. The options evaluated were **pgvector** (PostgreSQL extension) and dedicated vector databases such as Pinecone, Qdrant, or Weaviate.

JobRadar already uses PostgreSQL 18 as its primary datastore for users, offers, scores, and profiles. The question is whether vector search justifies adding a dedicated vector database to the stack.

---

## Decision

Use **pgvector** as a PostgreSQL 18 extension for all vector storage and similarity search.

---

## Rationale

### Single datastore, transactional consistency

With pgvector, offer metadata and its embedding live in the same row, in the same transaction. Inserting a scored offer and its embedding is atomic — no risk of metadata and vector being out of sync across two separate systems.

```sql
INSERT INTO offers (id, profile_id, title, company, score, embedding)
VALUES ($1, $2, $3, $4, $5, $6::vector);
```

With a dedicated vector DB, every write requires two operations across two systems. Partial failures leave data inconsistent.

### Simplified operations

Adding pgvector to PostgreSQL 18 is a single SQL statement:

```sql
CREATE EXTENSION IF NOT EXISTS vector;
```

No additional service to deploy, monitor, scale, or back up. The existing PostgreSQL Helm chart, backup strategy, and observability cover vectors automatically.

### PostgreSQL 18 async I/O performance

PostgreSQL 18 introduces a new asynchronous I/O subsystem that significantly improves read throughput for large sequential scans — directly relevant for HNSW index traversal during vector similarity search. The performance gap between pgvector and dedicated vector databases has narrowed considerably with PG18.

### HNSW indexing

pgvector supports HNSW (Hierarchical Navigable Small World) indexes since version 0.5.0 — the same algorithm used by dedicated vector databases like Qdrant and Weaviate. Query performance for approximate nearest neighbor search is comparable for the data volumes JobRadar handles.

```sql
CREATE INDEX ON offers
USING hnsw (embedding vector_cosine_ops)
WITH (m = 16, ef_construction = 64);
```

### Scale considerations

JobRadar ingests job offers from multiple sources continuously. At realistic ingestion rates (hundreds to low thousands of offers per day), the vector corpus will reach millions of rows over months — well within pgvector's proven operating range. Dedicated vector databases become relevant at hundreds of millions of vectors or with requirements for real-time vector updates at very high throughput. Neither applies here.

### Self-hosted alignment

Dedicated vector databases like Pinecone are cloud-only SaaS. Qdrant and Weaviate are self-hostable but add operational complexity. pgvector requires no additional infrastructure decisions.

---

## Consequences

**Positive**
- Atomic writes — offer metadata and embedding in one transaction
- No additional service to operate, monitor, or back up
- HNSW indexing for approximate nearest neighbor search
- Full SQL expressiveness — filter by `profile_id`, `score`, `created_at` alongside vector similarity
- PostgreSQL 18 async I/O improves vector scan performance

**Negative**
- Less specialized than dedicated vector databases for pure vector workloads
- HNSW index build time increases with corpus size — acceptable at JobRadar's ingestion rate (hundreds of offers/day), revisit if corpus exceeds tens of millions of vectors
- No built-in vector-specific UI (mitigated by pgAdmin or standard SQL tooling)

---

## Alternatives considered

| Option | Reason rejected |
|---|---|
| Pinecone | Cloud-only SaaS, external dependency, cost at scale, contradicts self-hosted approach |
| Qdrant | Additional service to operate, no transactional consistency with relational data |
| Weaviate | High resource consumption, complex configuration, overkill for this scale |
| Elasticsearch (dense_vector) | Heavy operational footprint, better suited for full-text search than pure vector workloads |

---

## Query pattern

Semantic search for RAG context retrieval:

```sql
SELECT id, title, company, score, summary,
       1 - (embedding <=> $1::vector) AS similarity
FROM offers
WHERE profile_id = $2
  AND created_at > NOW() - INTERVAL '90 days'
ORDER BY embedding <=> $1::vector
LIMIT 10;
```

The `<=>` operator (cosine distance) combined with standard SQL filters (`profile_id`, `created_at`) in a single query is the primary advantage over dedicated vector databases — no application-level join between two systems.