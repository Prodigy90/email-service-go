-- Add resend_email_id to emails table for mapping webhook events back to emails
ALTER TABLE emails ADD COLUMN resend_email_id VARCHAR(255);
CREATE INDEX idx_emails_resend_email_id ON emails(resend_email_id) WHERE resend_email_id IS NOT NULL;

-- GIN index for campaign metadata filtering
CREATE INDEX idx_emails_metadata_campaign ON emails USING GIN ((metadata->'campaign'));

-- Email events table for tracking Resend webhook events (delivered, opened, clicked, bounced, etc.)
CREATE TABLE email_events (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    email_id UUID NOT NULL REFERENCES emails(id) ON DELETE CASCADE,
    event_type VARCHAR(50) NOT NULL,
    resend_event_id VARCHAR(255),
    payload JSONB DEFAULT '{}',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Webhook event deduplication: Resend retries on non-200, this prevents duplicate events
CREATE UNIQUE INDEX idx_email_events_resend_event_id ON email_events(resend_event_id) WHERE resend_event_id IS NOT NULL;

CREATE INDEX idx_email_events_email_id ON email_events(email_id);
CREATE INDEX idx_email_events_event_type ON email_events(event_type);
CREATE INDEX idx_email_events_created_at ON email_events(created_at DESC);

-- Composite index for campaign stats joins and non-openers NOT EXISTS subquery
CREATE INDEX idx_email_events_email_id_type ON email_events(email_id, event_type);
