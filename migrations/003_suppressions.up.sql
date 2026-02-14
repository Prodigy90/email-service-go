-- Suppression list for email compliance (unsubscribes, bounces, complaints)
CREATE TABLE IF NOT EXISTS suppressions (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    email      TEXT NOT NULL,
    reason     TEXT NOT NULL CHECK (reason IN ('unsubscribe', 'bounce', 'complaint')),
    source     TEXT NOT NULL DEFAULT '',
    campaign_tag TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- Unique constraint on email to prevent duplicates
CREATE UNIQUE INDEX IF NOT EXISTS idx_suppressions_email ON suppressions(email);

-- Index for lookups by reason
CREATE INDEX IF NOT EXISTS idx_suppressions_reason ON suppressions(reason);
