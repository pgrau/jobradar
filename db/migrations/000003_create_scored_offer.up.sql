CREATE TABLE scored_offer (
                              id             UUID PRIMARY KEY DEFAULT uuidv7(),
                              profile_id     UUID NOT NULL REFERENCES profile(id) ON DELETE CASCADE,
                              offer_id       UUID NOT NULL,
                              embedding      VECTOR(1024),

    -- Denormalized from offer — avoids joins in RAG queries
                              title          TEXT NOT NULL,
                              company        TEXT NOT NULL,
                              location       TEXT,
                              remote         BOOLEAN NOT NULL DEFAULT false,
                              url            TEXT NOT NULL,
                              source         TEXT NOT NULL,
                              salary_min_eur INTEGER,
                              salary_max_eur INTEGER,
                              posted_at      TIMESTAMPTZ,

    -- Scoring
                              score          NUMERIC(5,2) NOT NULL,
                              reasoning      TEXT,
                              skill_matches  TEXT[],
                              skill_gaps     TEXT[],

    -- State
                              reviewed       BOOLEAN NOT NULL DEFAULT false,
                              saved          BOOLEAN NOT NULL DEFAULT false,
                              alerted        BOOLEAN NOT NULL DEFAULT false,
                              scored_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),

                              UNIQUE (profile_id, offer_id)
);

CREATE INDEX idx_scored_offer_profile_score
    ON scored_offer (profile_id, score DESC);

CREATE INDEX idx_scored_offer_profile_date
    ON scored_offer (profile_id, scored_at DESC);

CREATE INDEX idx_scored_offer_profile_unreviewed
    ON scored_offer (profile_id, reviewed, score DESC);

CREATE INDEX idx_scored_offer_embedding
    ON scored_offer USING hnsw (embedding vector_cosine_ops)
    WITH (m = 16, ef_construction = 64);