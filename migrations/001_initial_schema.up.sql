-- Email service schema
CREATE TABLE IF NOT EXISTS emails (
    id UUID PRIMARY KEY,
    to_address VARCHAR(255) NOT NULL,
    from_address VARCHAR(255),
    subject VARCHAR(500) NOT NULL,
    body TEXT NOT NULL,
    html_body TEXT,
    template VARCHAR(100),
    template_data JSONB DEFAULT '{}',
    status VARCHAR(20) NOT NULL DEFAULT 'pending',
    error_message TEXT,
    retry_count INTEGER NOT NULL DEFAULT 0,
    max_retries INTEGER NOT NULL DEFAULT 3,
    idempotency_id VARCHAR(255) UNIQUE,
    source_service VARCHAR(100),
    metadata JSONB DEFAULT '{}',
    sent_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Indexes for common queries
CREATE INDEX idx_emails_status ON emails(status);
CREATE INDEX idx_emails_to_address ON emails(to_address);
CREATE INDEX idx_emails_created_at ON emails(created_at DESC);
CREATE INDEX idx_emails_source_service ON emails(source_service) WHERE source_service IS NOT NULL;
CREATE INDEX idx_emails_template ON emails(template) WHERE template IS NOT NULL;

-- Partial index for pending emails (most queried)
CREATE INDEX idx_emails_pending ON emails(created_at) WHERE status = 'pending';
CREATE INDEX idx_emails_queued ON emails(created_at) WHERE status = 'queued';
