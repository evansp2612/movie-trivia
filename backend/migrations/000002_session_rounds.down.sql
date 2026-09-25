-- Reverse of 000002_session_rounds.
ALTER TABLE sessions DROP COLUMN IF EXISTS rounds;
