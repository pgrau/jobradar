CREATE TABLE alert_rule (
                            id          UUID PRIMARY KEY DEFAULT uuidv7(),
                            profile_id  UUID NOT NULL REFERENCES profile(id) ON DELETE CASCADE,
                            min_score   INTEGER NOT NULL DEFAULT 80,
                            remote_only BOOLEAN NOT NULL DEFAULT false,
                            locations   TEXT[],
                            companies   TEXT[],
                            keywords    TEXT[],
                            active      BOOLEAN NOT NULL DEFAULT true,
                            created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_alert_rule_profile_active ON alert_rule (profile_id, active);