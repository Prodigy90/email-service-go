-- Match an address case-insensitively without scanning emails: the daily
-- marketing cap and /recipients/state count by lower(to_address), because
-- the backend sends the address as user_auth holds it (may be mixed case)
-- and the campaign server lower-cases its lists. ~123K rows at 2026-11-21:
-- a plain build takes well under a second.
CREATE INDEX IF NOT EXISTS idx_emails_to_address_lower ON emails (lower(to_address));
