CREATE TABLE offer (
                       id             UUID PRIMARY KEY DEFAULT uuidv7(),
                       source         TEXT NOT NULL,
                       external_id    TEXT NOT NULL,
                       url            TEXT NOT NULL,
                       title          TEXT NOT NULL,
                       company        TEXT NOT NULL,
                       location       TEXT,
                       remote         BOOLEAN NOT NULL DEFAULT false,
                       salary_min_eur INTEGER,
                       salary_max_eur INTEGER,
                       raw_text       TEXT NOT NULL,
                       embedding      VECTOR(1024),
                       posted_at      TIMESTAMPTZ,
                       ingested_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
                       UNIQUE (source, external_id)
);

CREATE INDEX idx_offer_source_ingested ON offer (source, ingested_at DESC);
CREATE INDEX idx_offer_company ON offer (company);
CREATE INDEX idx_offer_embedding ON offer
    USING hnsw (embedding vector_cosine_ops)
    WITH (m = 16, ef_construction = 64);