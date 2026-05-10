ALTER TABLE terminal_sessions
  DROP COLUMN IF EXISTS client,
  DROP COLUMN IF EXISTS client_session_id;
