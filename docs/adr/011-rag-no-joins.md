# ADR-011: Denormalized scored_offer table to avoid joins in RAG queries

- **Status:** Accepted
- **Date:** 2026-03-22
- **Author:** Pau Ferran Grau

---

## Context

The RAG pipeline in JobRadar performs semantic similarity searches over the `scored_offer` corpus to find relevant offers for a given user profile. These searches happen on the hot path — every LLM scoring request and every frontend query triggers a pgvector similarity search.

The naive normalized schema would store offer metadata in the `offer` table and scoring results in `scored_offer`, requiring a JOIN to retrieve the full offer context:

```sql
-- Normalized — requires JOIN
SELECT so.score, so.reasoning, o.title, o.company, o.location, o.url
FROM scored_offer so
INNER JOIN offer o ON o.id = so.offer_id
WHERE so.profile_id = $1
ORDER BY so.embedding <=> $2::vector
LIMIT 10;
```

---

## Decision

Denormalize `scored_offer` to include all offer fields needed by the RAG pipeline directly, eliminating the JOIN entirely.

---

## Rationale

### JOINs on the RAG hot path are expensive

Every similarity search in the RAG pipeline would require a JOIN between `scored_offer` and `offer`. At query time, PostgreSQL must:

1. Execute the HNSW index scan on `scored_offer.embedding`
2. For each result, fetch the corresponding row from `offer` by primary key
3. Merge the result sets

Step 2 introduces random I/O for each row returned — the HNSW index returns results in similarity order, not storage order, so the PK lookups are not sequential. This defeats the I/O optimization benefits of PostgreSQL 18's async I/O subsystem.

### The RAG query becomes a single table scan

With denormalization, the RAG query is:

```sql
SELECT id, title, company, location, url, source, score, reasoning,
       skill_matches, skill_gaps,
       1 - (embedding <=> $1::vector) AS similarity
FROM scored_offer
WHERE profile_id = $2
ORDER BY embedding <=> $1::vector
LIMIT 10;
```

One table, one HNSW index scan, no random I/O for lookups. The query plan is predictable and fast regardless of corpus size.

### Denormalization is appropriate for this access pattern

`scored_offer` represents a **snapshot** of an offer at the time of scoring. This is semantically correct — if an offer changes on the source after being scored, the RAG context should reflect what was scored, not the current state. Denormalization enforces this immutability naturally.

The fields copied into `scored_offer` are exactly those needed by the RAG pipeline: `title`, `company`, `location`, `remote`, `url`, `source`, `salary_min_eur`, `salary_max_eur`, `posted_at`. No more, no less.

### Storage cost is acceptable

Duplicating offer metadata into `scored_offer` increases storage. At JobRadar's scale — thousands of offers scored per user — the additional storage is measured in megabytes, not gigabytes. The query performance gain outweighs the storage cost by a significant margin.

### The offer table is still the source of truth for ingestion

The `offer` table is not removed — it remains the source of truth for the fetcher and deduplication (`UNIQUE(source, external_id)`). Only the RAG read path is affected by this decision.

---

## Consequences

**Positive**
- RAG similarity search is a single table scan — no JOIN overhead
- HNSW index scan benefits fully from PostgreSQL 18 async I/O
- `scored_offer` naturally represents an immutable snapshot of the scored offer
- Query plan is simple and predictable — easier to reason about and optimize

**Negative**
- Offer metadata is duplicated between `offer` and `scored_offer`
- If offer metadata needs to be updated after scoring, `scored_offer` must be updated separately
- Slightly higher storage usage per scored offer

---

## Alternatives considered

| Option | Reason rejected |
|---|---|
| Normalized schema with JOIN | Random I/O on HNSW results defeats pgvector performance |
| Materialized view | Adds complexity, requires refresh strategy, still reads from two tables internally |
| Covering index | Cannot include VECTOR columns — pgvector HNSW indexes do not support INCLUDE |