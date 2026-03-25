-- =============================================
-- Vortex Messenger — Migration 003
-- Channels (Discord-style communities)
-- =============================================

-- ─── Channels (= Discord "servers") ──────────
CREATE TABLE channels (
    id            UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    name          VARCHAR(64)  NOT NULL,
    slug          VARCHAR(64)  NOT NULL UNIQUE,      -- URL-friendly name
    description   TEXT         DEFAULT '',
    avatar_url    VARCHAR(512),
    banner_url    VARCHAR(512),
    is_public     BOOLEAN      NOT NULL DEFAULT TRUE,
    invite_code   VARCHAR(16)  UNIQUE,               -- for private channels
    created_by    UUID         REFERENCES users(id) ON DELETE SET NULL,
    member_count  INT          NOT NULL DEFAULT 0,
    created_at    TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at    TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

-- ─── Categories (room groupings) ─────────────
CREATE TABLE channel_categories (
    id          UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    channel_id  UUID        NOT NULL REFERENCES channels(id) ON DELETE CASCADE,
    name        VARCHAR(64) NOT NULL,
    position    INT         NOT NULL DEFAULT 0,

    UNIQUE (channel_id, position)
);

-- ─── Rooms (text / voice / announcements) ────
CREATE TABLE channel_rooms (
    id           UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    channel_id   UUID        NOT NULL REFERENCES channels(id) ON DELETE CASCADE,
    category_id  UUID        REFERENCES channel_categories(id) ON DELETE SET NULL,
    name         VARCHAR(64) NOT NULL,
    topic        VARCHAR(256) DEFAULT '',
    type         VARCHAR(16) NOT NULL DEFAULT 'text'
                     CHECK (type IN ('text', 'voice', 'announcement', 'stage')),
    position     INT         NOT NULL DEFAULT 0,
    is_nsfw      BOOLEAN     NOT NULL DEFAULT FALSE,
    slowmode_sec INT         NOT NULL DEFAULT 0,     -- 0 = off
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- ─── Roles & permissions ─────────────────────
CREATE TABLE channel_roles (
    id           UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    channel_id   UUID         NOT NULL REFERENCES channels(id) ON DELETE CASCADE,
    name         VARCHAR(32)  NOT NULL,
    color        VARCHAR(7)   DEFAULT '#888888',     -- hex color
    permissions  JSONB        NOT NULL DEFAULT '{
        "can_send": true,
        "can_attach": true,
        "can_react": true,
        "can_thread": true,
        "can_voice": true,
        "can_pin": false,
        "can_delete_others": false,
        "can_kick": false,
        "can_ban": false,
        "can_manage_rooms": false,
        "can_manage_roles": false,
        "is_admin": false
    }',
    is_default   BOOLEAN      NOT NULL DEFAULT FALSE, -- auto-assigned on join
    position     INT          NOT NULL DEFAULT 0,      -- higher = more authority

    UNIQUE (channel_id, name)
);

-- ─── Channel members ─────────────────────────
CREATE TABLE channel_members (
    channel_id  UUID NOT NULL REFERENCES channels(id) ON DELETE CASCADE,
    user_id     UUID NOT NULL REFERENCES users(id)    ON DELETE CASCADE,
    role_id     UUID REFERENCES channel_roles(id)     ON DELETE SET NULL,
    nickname    VARCHAR(64),
    joined_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    PRIMARY KEY (channel_id, user_id)
);

-- ─── Channel messages ────────────────────────
CREATE TABLE channel_messages (
    id          UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    room_id     UUID         NOT NULL REFERENCES channel_rooms(id) ON DELETE CASCADE,
    sender_id   UUID         NOT NULL REFERENCES users(id) ON DELETE SET NULL,
    content     TEXT         NOT NULL DEFAULT '',
    thread_id   UUID,                                 -- FK added after threads table
    is_pinned   BOOLEAN      NOT NULL DEFAULT FALSE,
    created_at  TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    edited_at   TIMESTAMPTZ,
    deleted_at  TIMESTAMPTZ
);

-- ─── Threads ─────────────────────────────────
CREATE TABLE threads (
    id               UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    room_id          UUID        NOT NULL REFERENCES channel_rooms(id) ON DELETE CASCADE,
    starter_msg_id   UUID        NOT NULL REFERENCES channel_messages(id) ON DELETE CASCADE,
    name             VARCHAR(128),
    reply_count      INT         NOT NULL DEFAULT 0,
    last_activity_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_at       TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Now add the FK for thread_id
ALTER TABLE channel_messages
    ADD CONSTRAINT fk_channel_msg_thread
    FOREIGN KEY (thread_id) REFERENCES threads(id) ON DELETE SET NULL;

-- ─── Channel message attachments ─────────────
CREATE TABLE channel_attachments (
    id             UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    message_id     UUID         NOT NULL REFERENCES channel_messages(id) ON DELETE CASCADE,
    file_type      VARCHAR(16)  NOT NULL,
    file_url       VARCHAR(512) NOT NULL,
    file_size      BIGINT       NOT NULL DEFAULT 0,
    file_name      VARCHAR(255),
    mime_type      VARCHAR(128),
    thumbnail_url  VARCHAR(512),
    metadata       JSONB        DEFAULT '{}',
    created_at     TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

-- ─── Channel message reactions ───────────────
CREATE TABLE channel_reactions (
    message_id  UUID        NOT NULL REFERENCES channel_messages(id) ON DELETE CASCADE,
    user_id     UUID        NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    emoji       VARCHAR(32) NOT NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    PRIMARY KEY (message_id, user_id, emoji)
);

-- ─── Indexes ─────────────────────────────────
CREATE INDEX idx_channels_slug           ON channels (slug);
CREATE INDEX idx_channels_public         ON channels (is_public) WHERE is_public = TRUE;
CREATE INDEX idx_channel_rooms_channel   ON channel_rooms (channel_id, position);
CREATE INDEX idx_channel_members_user    ON channel_members (user_id);
CREATE INDEX idx_channel_members_channel ON channel_members (channel_id);
CREATE INDEX idx_channel_msgs_room_time  ON channel_messages (room_id, created_at DESC)
                                          WHERE deleted_at IS NULL;
CREATE INDEX idx_channel_msgs_thread     ON channel_messages (thread_id)
                                          WHERE thread_id IS NOT NULL;
CREATE INDEX idx_threads_room            ON threads (room_id, last_activity_at DESC);
CREATE INDEX idx_channel_attach_msg      ON channel_attachments (message_id);

-- ─── Triggers ────────────────────────────────
CREATE TRIGGER trg_channels_updated_at
    BEFORE UPDATE ON channels
    FOR EACH ROW EXECUTE FUNCTION update_updated_at();

-- Auto-increment member_count
CREATE OR REPLACE FUNCTION update_channel_member_count()
RETURNS TRIGGER AS $$
BEGIN
    IF TG_OP = 'INSERT' THEN
        UPDATE channels SET member_count = member_count + 1 WHERE id = NEW.channel_id;
    ELSIF TG_OP = 'DELETE' THEN
        UPDATE channels SET member_count = member_count - 1 WHERE id = OLD.channel_id;
    END IF;
    RETURN NULL;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER trg_channel_member_count
    AFTER INSERT OR DELETE ON channel_members
    FOR EACH ROW EXECUTE FUNCTION update_channel_member_count();
