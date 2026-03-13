-- Add headers JSONB column to persist custom email headers (Reply-To, etc.)
ALTER TABLE emails ADD COLUMN IF NOT EXISTS headers JSONB DEFAULT '{}';
