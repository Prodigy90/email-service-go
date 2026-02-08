DROP TABLE IF EXISTS email_events;

DROP INDEX IF EXISTS idx_emails_metadata_campaign;
DROP INDEX IF EXISTS idx_emails_resend_email_id;

ALTER TABLE emails DROP COLUMN IF EXISTS resend_email_id;
