-- =============================================
-- Vortex Messenger — Migration 001
-- Users & Sessions
-- =============================================

CREATE EXTENSION IF NOT EXISTS "uuid-ossp";
CREATE EXTENSION IF NOT EXISTS "pgcrypto";

-- ─── Users ───────────────────────────────────
CREATE TABLE users (
    id              UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    username        VARCHAR(32)  NOT NULL UNIQUE,
    display_name    VARCHAR(64)  NOT NULL,
    email           VARCHAR(255) UNIQUE,
    phone           VARCHAR(20)  UNIQUE,
    password_hash   BYTEA        NOT NULL,
    avatar_url      VARCHAR(512),
    public_key      TEXT,                          -- X25519 public key for E2E
    status          VARCHAR(16)  NOT NULL DEFAULT 'active'
                        CHECK (status IN ('active', 'banned', 'deleted')),
    bio             TEXT         DEFAULT '',
    last_seen_at    TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    created_at      TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

-- At least one of email or phone must be provided
ALTER TABLE users ADD CONSTRAINT users_contact_check
    CHECK (email IS NOT NULL OR phone IS NOT NULL);

-- ─── Sessions (multi-device support) ─────────
CREATE TABLE sessions (
    id                  UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id             UUID         NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    device_name         VARCHAR(128) NOT NULL DEFAULT 'Unknown',
    device_type         VARCHAR(16)  NOT NULL DEFAULT 'unknown'
                            CHECK (device_type IN ('android', 'ios', 'windows', 'macos', 'linux', 'web', 'unknown')),
    refresh_token_hash  BYTEA        NOT NULL,
    ip_address          INET,
    user_agent          TEXT,
    created_at          TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    expires_at          TIMESTAMPTZ  NOT NULL
);

-- ─── Indexes ─────────────────────────────────
CREATE INDEX idx_users_username     ON users (username);
CREATE INDEX idx_users_email        ON users (email)    WHERE email IS NOT NULL;
CREATE INDEX idx_users_phone        ON users (phone)    WHERE phone IS NOT NULL;
CREATE INDEX idx_users_last_seen    ON users (last_seen_at DESC);

CREATE INDEX idx_sessions_user_id   ON sessions (user_id);
CREATE INDEX idx_sessions_expires   ON sessions (expires_at);

-- ─── Updated_at trigger ──────────────────────
CREATE OR REPLACE FUNCTION update_updated_at()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER trg_users_updated_at
    BEFORE UPDATE ON users
    FOR EACH ROW EXECUTE FUNCTION update_updated_at();
