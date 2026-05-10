CREATE TABLE IF NOT EXISTS users (
    id UUID PRIMARY KEY,
    display_name TEXT,
    email TEXT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    last_seen_at TIMESTAMPTZ
);

CREATE TABLE IF NOT EXISTS ssh_identities (
    id UUID PRIMARY KEY,
    user_id UUID NOT NULL REFERENCES users(id),
    public_key_fingerprint TEXT NOT NULL UNIQUE,
    public_key TEXT NOT NULL,
    label TEXT,
    first_seen_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    last_seen_at TIMESTAMPTZ,
    revoked_at TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS idx_ssh_identities_user_id ON ssh_identities(user_id);

CREATE TABLE IF NOT EXISTS oauth_accounts (
    id UUID PRIMARY KEY,
    user_id UUID NOT NULL REFERENCES users(id),
    provider TEXT NOT NULL,
    provider_user_id TEXT,
    encrypted_access_token TEXT NOT NULL,
    token_expires_at TIMESTAMPTZ,
    scopes TEXT[],
    status TEXT NOT NULL DEFAULT 'active',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE(user_id, provider),
    CONSTRAINT oauth_accounts_status_check CHECK (status IN ('active', 'expired', 'reconnect_required', 'revoked'))
);

CREATE INDEX IF NOT EXISTS idx_oauth_accounts_user_id ON oauth_accounts(user_id);

CREATE TABLE IF NOT EXISTS terminal_sessions (
    id UUID PRIMARY KEY,
    user_id UUID REFERENCES users(id),
    ssh_identity_id UUID REFERENCES ssh_identities(id),
    ssh_fingerprint TEXT,
    current_screen TEXT,
    selected_address_id TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    last_seen_at TIMESTAMPTZ,
    ended_at TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS idx_terminal_sessions_user_id ON terminal_sessions(user_id);
CREATE INDEX IF NOT EXISTS idx_terminal_sessions_ssh_identity_id ON terminal_sessions(ssh_identity_id);

CREATE TABLE IF NOT EXISTS audit_events (
    id UUID PRIMARY KEY,
    user_id UUID NULL REFERENCES users(id),
    terminal_session_id UUID NULL REFERENCES terminal_sessions(id),
    event_name TEXT NOT NULL,
    provider TEXT,
    status TEXT NOT NULL,
    error_code TEXT NULL,
    metadata JSONB,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_audit_events_user_id ON audit_events(user_id);
CREATE INDEX IF NOT EXISTS idx_audit_events_terminal_session_id ON audit_events(terminal_session_id);
