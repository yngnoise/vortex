-- =============================================
-- Vortex Messenger — Migration 002
-- Conversations & Messages (Telegram-style)
-- =============================================

-- ─── Conversations ───────────────────────────
CREATE TABLE conversations (
    id          UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    type        VARCHAR(8) NOT NULL DEFAULT 'direct'
                    CHECK (type IN ('direct', 'group')),
    title       VARCHAR(128),                      -- NULL for direct chats
    avatar_url  VARCHAR(512),
    created_by  UUID       REFERENCES users(id) ON DELETE SET NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- ─── Conversation members ────────────────────
CREATE TABLE conversation_members (
    conversation_id  UUID NOT NULL REFERENCES conversations(id) ON DELETE CASCADE,
    user_id          UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    role             VARCHAR(8)  NOT NULL DEFAULT 'member'
                         CHECK (role IN ('owner', 'admin', 'member')),
    last_read_msg_id UUID,                          -- FK added after messages table
    notifications    VARCHAR(8)  NOT NULL DEFAULT 'all'
                         CHECK (notifications IN ('all', 'mentions', 'none')),
    joined_at        TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    PRIMARY KEY (conversation_id, user_id)
);

-- ─── Messages ────────────────────────────────
CREATE TABLE messages (
    id                 UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    conversation_id    UUID         NOT NULL REFERENCES conversations(id) ON DELETE CASCADE,
    sender_id          UUID         NOT NULL REFERENCES users(id) ON DELETE SET NULL,
    content_encrypted  BYTEA,                       -- E2E encrypted payload
    content_type       VARCHAR(16)  NOT NULL DEFAULT 'text'
                           CHECK (content_type IN ('text', 'image', 'video', 'audio', 'voice', 'file', 'sticker', 'system')),
    reply_to_id        UUID         REFERENCES messages(id) ON DELETE SET NULL,
    forwarded_from     UUID         REFERENCES messages(id) ON DELETE SET NULL,
    created_at         TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    edited_at          TIMESTAMPTZ,
    deleted_at         TIMESTAMPTZ                  -- soft delete
);

-- Now add the FK for last_read_msg_id
ALTER TABLE conversation_members
    ADD CONSTRAINT fk_last_read_msg
    FOREIGN KEY (last_read_msg_id) REFERENCES messages(id) ON DELETE SET NULL;

-- ─── Attachments ─────────────────────────────
CREATE TABLE attachments (
    id             UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    message_id     UUID         NOT NULL REFERENCES messages(id) ON DELETE CASCADE,
    file_type      VARCHAR(16)  NOT NULL CHECK (file_type IN ('image', 'video', 'audio', 'document', 'other')),
    file_url       VARCHAR(512) NOT NULL,
    file_size      BIGINT       NOT NULL DEFAULT 0,
    file_name      VARCHAR(255),
    mime_type      VARCHAR(128),
    thumbnail_url  VARCHAR(512),
    metadata       JSONB        DEFAULT '{}',       -- width, height, duration, etc.
    created_at     TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

-- ─── Reactions ───────────────────────────────
CREATE TABLE message_reactions (
    message_id  UUID        NOT NULL REFERENCES messages(id) ON DELETE CASCADE,
    user_id     UUID        NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    emoji       VARCHAR(32) NOT NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    PRIMARY KEY (message_id, user_id, emoji)
);

-- ─── Indexes ─────────────────────────────────
CREATE INDEX idx_conv_members_user       ON conversation_members (user_id);
CREATE INDEX idx_conv_members_conv       ON conversation_members (conversation_id);

CREATE INDEX idx_messages_conv_time      ON messages (conversation_id, created_at DESC);
CREATE INDEX idx_messages_sender         ON messages (sender_id);
CREATE INDEX idx_messages_not_deleted    ON messages (conversation_id, created_at DESC)
                                         WHERE deleted_at IS NULL;

CREATE INDEX idx_attachments_message     ON attachments (message_id);
CREATE INDEX idx_reactions_message       ON message_reactions (message_id);

-- ─── Triggers ────────────────────────────────
CREATE TRIGGER trg_conversations_updated_at
    BEFORE UPDATE ON conversations
    FOR EACH ROW EXECUTE FUNCTION update_updated_at();
