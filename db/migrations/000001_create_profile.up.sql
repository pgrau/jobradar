CREATE TABLE profile (
                         id              UUID PRIMARY KEY DEFAULT uuidv7(),
                         email           TEXT NOT NULL UNIQUE,
                         password_hash   TEXT NOT NULL,
                         full_name       TEXT,
                         cv_text         TEXT,
                         cv_embedding    VECTOR(1024),
                         cv_bucket       TEXT,
                         skills          TEXT[],
                         seniority       TEXT,
                         location        TEXT,
                         remote_only     BOOLEAN NOT NULL DEFAULT false,
                         min_salary_eur  INTEGER,
                         alert_threshold INTEGER NOT NULL DEFAULT 80,
                         created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
                         updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_profile_email ON profile (email);