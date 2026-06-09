package channels

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// ────────────────────────────────────────────────────────────
// Ошибки
// ────────────────────────────────────────────────────────────

var (
	ErrChannelNotFound = errors.New("channel not found")
	ErrRoomNotFound    = errors.New("room not found")
	ErrNotMember       = errors.New("user is not a member of this channel")
	ErrAlreadyMember   = errors.New("user is already a member")
	ErrSlugTaken       = errors.New("channel slug is already taken")
	ErrNoPermission    = errors.New("insufficient permissions")
	ErrMessageNotFound = errors.New("message not found")
)

// ────────────────────────────────────────────────────────────
// Модели
// ────────────────────────────────────────────────────────────

// Channel — аналог Discord-сервера.
// Содержит комнаты (текстовые, голосовые), роли, участников.
type Channel struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Slug        string    `json:"slug"`
	Description string    `json:"description"`
	AvatarURL   *string   `json:"avatar_url,omitempty"`
	BannerURL   *string   `json:"banner_url,omitempty"`
	IsPublic    bool      `json:"is_public"`
	InviteCode  *string   `json:"invite_code,omitempty"`
	CreatedBy   *string   `json:"created_by,omitempty"`
	MemberCount int       `json:"member_count"`
	CreatedAt   time.Time `json:"created_at"`
}

// ChannelRoom — комната внутри канала (текст, голос, объявления).
type ChannelRoom struct {
	ID          string  `json:"id"`
	ChannelID   string  `json:"channel_id"`
	CategoryID  *string `json:"category_id,omitempty"`
	Name        string  `json:"name"`
	Topic       string  `json:"topic"`
	Type        string  `json:"type"`
	Position    int     `json:"position"`
	IsNSFW      bool    `json:"is_nsfw"`
	SlowmodeSec int     `json:"slowmode_sec"`
}

// ChannelCategory — группировка комнат ("Общее", "Разработка").
type ChannelCategory struct {
	ID        string        `json:"id"`
	ChannelID string        `json:"channel_id"`
	Name      string        `json:"name"`
	Position  int           `json:"position"`
	Rooms     []ChannelRoom `json:"rooms"`
}

// ChannelMember — участник канала с ролью.
type ChannelMember struct {
	ChannelID   string    `json:"channel_id"`
	UserID      string    `json:"user_id"`
	Username    string    `json:"username"`
	DisplayName string    `json:"display_name"`
	AvatarURL   *string   `json:"avatar_url,omitempty"`
	Nickname    *string   `json:"nickname,omitempty"`
	RoleID      *string   `json:"role_id,omitempty"`
	RoleName    *string   `json:"role_name,omitempty"`
	RoleColor   *string   `json:"role_color,omitempty"`
	JoinedAt    time.Time `json:"joined_at"`
}

// ChannelMessage — сообщение в комнате канала.
type ChannelMessage struct {
	ID             string     `json:"id"`
	RoomID         string     `json:"room_id"`
	SenderID       string     `json:"sender_id"`
	SenderUsername string     `json:"sender_username"`
	SenderName     string     `json:"sender_display_name"`
	SenderAvatar   *string    `json:"sender_avatar,omitempty"`
	Content        string     `json:"content"`
	ThreadID       *string    `json:"thread_id,omitempty"`
	IsPinned       bool       `json:"is_pinned"`
	CreatedAt      time.Time  `json:"created_at"`
	EditedAt       *time.Time `json:"edited_at,omitempty"`
}

// ChannelPreview — элемент списка каналов для пользователя.
type ChannelPreview struct {
	Channel
	MyRoleName *string `json:"my_role_name,omitempty"`
}

// ────────────────────────────────────────────────────────────
// Repository
// ────────────────────────────────────────────────────────────

type Repository struct {
	db *pgxpool.Pool
}

func NewRepository(db *pgxpool.Pool) *Repository {
	return &Repository{db: db}
}

// ────────────────────────────────────────────────────────────
// Channels CRUD
// ────────────────────────────────────────────────────────────

// CreateChannel создаёт новый канал с дефолтной ролью и комнатой.
//
// В одной транзакции:
// 1. Создаём канал
// 2. Создаём дефолтную роль "member" (автоназначается при вступлении)
// 3. Создаём admin-роль для создателя
// 4. Создаём категорию "Общее" с комнатой "general"
// 5. Добавляем создателя с admin-ролью
func (r *Repository) CreateChannel(ctx context.Context, name, description string, isPublic bool, creatorID string) (*Channel, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	slug := generateSlug(name)
	var inviteCode *string
	if !isPublic {
		code := generateInviteCode()
		inviteCode = &code
	}

	ch := &Channel{}
	err = tx.QueryRow(ctx, `
		INSERT INTO channels (name, slug, description, is_public, invite_code, created_by)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id, name, slug, description, avatar_url, banner_url,
		          is_public, invite_code, created_by, member_count, created_at
	`, name, slug, description, isPublic, inviteCode, creatorID).Scan(
		&ch.ID, &ch.Name, &ch.Slug, &ch.Description,
		&ch.AvatarURL, &ch.BannerURL, &ch.IsPublic,
		&ch.InviteCode, &ch.CreatedBy, &ch.MemberCount, &ch.CreatedAt,
	)
	if err != nil {
		if strings.Contains(err.Error(), "channels_slug_key") {
			return nil, ErrSlugTaken
		}
		return nil, err
	}

	var defaultRoleID string
	err = tx.QueryRow(ctx, `
		INSERT INTO channel_roles (channel_id, name, color, is_default, position)
		VALUES ($1, 'member', '#888888', true, 0)
		RETURNING id
	`, ch.ID).Scan(&defaultRoleID)
	if err != nil {
		return nil, err
	}

	var adminRoleID string
	err = tx.QueryRow(ctx, `
		INSERT INTO channel_roles (channel_id, name, color, is_default, position, permissions)
		VALUES ($1, 'admin', '#E24B4A', false, 100, $2)
		RETURNING id
	`, ch.ID, `{
		"can_send": true, "can_attach": true, "can_react": true,
		"can_thread": true, "can_voice": true, "can_pin": true,
		"can_delete_others": true, "can_kick": true, "can_ban": true,
		"can_manage_rooms": true, "can_manage_roles": true, "is_admin": true
	}`).Scan(&adminRoleID)
	if err != nil {
		return nil, err
	}

	var categoryID string
	err = tx.QueryRow(ctx, `
		INSERT INTO channel_categories (channel_id, name, position)
		VALUES ($1, 'General', 0)
		RETURNING id
	`, ch.ID).Scan(&categoryID)
	if err != nil {
		return nil, err
	}

	_, err = tx.Exec(ctx, `
		INSERT INTO channel_rooms (channel_id, category_id, name, type, position)
		VALUES ($1, $2, 'general', 'text', 0)
	`, ch.ID, categoryID)
	if err != nil {
		return nil, err
	}

	_, err = tx.Exec(ctx, `
		INSERT INTO channel_members (channel_id, user_id, role_id)
		VALUES ($1, $2, $3)
	`, ch.ID, creatorID, adminRoleID)
	if err != nil {
		return nil, err
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}

	return r.GetChannel(ctx, ch.ID)
}

// GetChannel возвращает канал по ID.
func (r *Repository) GetChannel(ctx context.Context, channelID string) (*Channel, error) {
	ch := &Channel{}
	err := r.db.QueryRow(ctx, `
		SELECT id, name, slug, description, avatar_url, banner_url,
		       is_public, invite_code, created_by, member_count, created_at
		FROM channels WHERE id = $1
	`, channelID).Scan(
		&ch.ID, &ch.Name, &ch.Slug, &ch.Description,
		&ch.AvatarURL, &ch.BannerURL, &ch.IsPublic,
		&ch.InviteCode, &ch.CreatedBy, &ch.MemberCount, &ch.CreatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrChannelNotFound
	}
	return ch, err
}

// ListPublicChannels возвращает публичные каналы (discovery).
func (r *Repository) ListPublicChannels(ctx context.Context, limit, offset int) ([]Channel, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id, name, slug, description, avatar_url, banner_url,
		       is_public, invite_code, created_by, member_count, created_at
		FROM channels
		WHERE is_public = true
		ORDER BY member_count DESC, created_at DESC
		LIMIT $1 OFFSET $2
	`, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanChannels(rows)
}

// GetUserChannels возвращает каналы пользователя.
func (r *Repository) GetUserChannels(ctx context.Context, userID string) ([]ChannelPreview, error) {
	rows, err := r.db.Query(ctx, `
		SELECT c.id, c.name, c.slug, c.description, c.avatar_url, c.banner_url,
		       c.is_public, c.invite_code, c.created_by, c.member_count, c.created_at,
		       cr.name
		FROM channel_members cm
		JOIN channels c ON c.id = cm.channel_id
		LEFT JOIN channel_roles cr ON cr.id = cm.role_id
		WHERE cm.user_id = $1
		ORDER BY c.name
	`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var previews []ChannelPreview
	for rows.Next() {
		var p ChannelPreview
		err := rows.Scan(
			&p.ID, &p.Name, &p.Slug, &p.Description,
			&p.AvatarURL, &p.BannerURL, &p.IsPublic,
			&p.InviteCode, &p.CreatedBy, &p.MemberCount, &p.CreatedAt,
			&p.MyRoleName,
		)
		if err != nil {
			return nil, err
		}
		previews = append(previews, p)
	}
	return previews, rows.Err()
}

// ────────────────────────────────────────────────────────────
// Members
// ────────────────────────────────────────────────────────────

// JoinChannel добавляет пользователя с дефолтной ролью.
func (r *Repository) JoinChannel(ctx context.Context, channelID, userID string) error {
	var defaultRoleID string
	err := r.db.QueryRow(ctx, `
		SELECT id FROM channel_roles
		WHERE channel_id = $1 AND is_default = true
		LIMIT 1
	`, channelID).Scan(&defaultRoleID)
	if err != nil {
		return err
	}

	_, err = r.db.Exec(ctx, `
		INSERT INTO channel_members (channel_id, user_id, role_id)
		VALUES ($1, $2, $3)
	`, channelID, userID, defaultRoleID)
	if err != nil {
		if strings.Contains(err.Error(), "channel_members_pkey") {
			return ErrAlreadyMember
		}
		return err
	}
	return nil
}

// LeaveChannel удаляет пользователя из канала.
func (r *Repository) LeaveChannel(ctx context.Context, channelID, userID string) error {
	tag, err := r.db.Exec(ctx, `
		DELETE FROM channel_members WHERE channel_id = $1 AND user_id = $2
	`, channelID, userID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotMember
	}
	return nil
}

// IsMember проверяет участие пользователя в канале.
func (r *Repository) IsMember(ctx context.Context, channelID, userID string) (bool, error) {
	var exists bool
	err := r.db.QueryRow(ctx, `
		SELECT EXISTS(
			SELECT 1 FROM channel_members
			WHERE channel_id = $1 AND user_id = $2
		)
	`, channelID, userID).Scan(&exists)
	return exists, err
}

// IsRoomMember проверяет доступ к комнате по roomID — то есть членство
// в родительском канале этой комнаты. Используется realtime-подпиской
// на канал "channel:<roomID>" (имена комнат, а не каналов).
func (r *Repository) IsRoomMember(ctx context.Context, roomID, userID string) (bool, error) {
	var exists bool
	err := r.db.QueryRow(ctx, `
		SELECT EXISTS(
			SELECT 1
			FROM channel_rooms cr
			JOIN channel_members cm ON cm.channel_id = cr.channel_id
			WHERE cr.id = $1 AND cm.user_id = $2
		)
	`, roomID, userID).Scan(&exists)
	return exists, err
}

// GetMembers возвращает список участников канала.
func (r *Repository) GetMembers(ctx context.Context, channelID string, limit, offset int) ([]ChannelMember, error) {
	rows, err := r.db.Query(ctx, `
		SELECT cm.channel_id, cm.user_id, u.username, u.display_name,
		       u.avatar_url, cm.nickname, cm.role_id,
		       cr.name, cr.color
		FROM channel_members cm
		JOIN users u ON u.id = cm.user_id
		LEFT JOIN channel_roles cr ON cr.id = cm.role_id
		WHERE cm.channel_id = $1
		ORDER BY cr.position DESC, u.username
		LIMIT $2 OFFSET $3
	`, channelID, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var members []ChannelMember
	for rows.Next() {
		var m ChannelMember
		err := rows.Scan(
			&m.ChannelID, &m.UserID, &m.Username, &m.DisplayName,
			&m.AvatarURL, &m.Nickname, &m.RoleID,
			&m.RoleName, &m.RoleColor,
		)
		if err != nil {
			return nil, err
		}
		members = append(members, m)
	}
	return members, rows.Err()
}

// ────────────────────────────────────────────────────────────
// Rooms & Structure
// ────────────────────────────────────────────────────────────

// GetChannelStructure возвращает категории с комнатами (боковая панель).
func (r *Repository) GetChannelStructure(ctx context.Context, channelID string) ([]ChannelCategory, error) {
	catRows, err := r.db.Query(ctx, `
		SELECT id, channel_id, name, position
		FROM channel_categories
		WHERE channel_id = $1 ORDER BY position
	`, channelID)
	if err != nil {
		return nil, err
	}
	defer catRows.Close()

	catMap := make(map[string]*ChannelCategory)
	var categories []ChannelCategory
	for catRows.Next() {
		var c ChannelCategory
		if err := catRows.Scan(&c.ID, &c.ChannelID, &c.Name, &c.Position); err != nil {
			return nil, err
		}
		c.Rooms = []ChannelRoom{}
		categories = append(categories, c)
		catMap[c.ID] = &categories[len(categories)-1]
	}

	roomRows, err := r.db.Query(ctx, `
		SELECT id, channel_id, category_id, name, topic,
		       type, position, is_nsfw, slowmode_sec
		FROM channel_rooms
		WHERE channel_id = $1 ORDER BY position
	`, channelID)
	if err != nil {
		return nil, err
	}
	defer roomRows.Close()

	for roomRows.Next() {
		var room ChannelRoom
		if err := roomRows.Scan(
			&room.ID, &room.ChannelID, &room.CategoryID, &room.Name,
			&room.Topic, &room.Type, &room.Position,
			&room.IsNSFW, &room.SlowmodeSec,
		); err != nil {
			return nil, err
		}
		if room.CategoryID != nil {
			if cat, ok := catMap[*room.CategoryID]; ok {
				cat.Rooms = append(cat.Rooms, room)
			}
		}
	}

	return categories, nil
}

// CreateRoom создаёт новую комнату в канале.
func (r *Repository) CreateRoom(ctx context.Context, channelID string, categoryID *string, name, roomType string) (*ChannelRoom, error) {
	var maxPos int
	r.db.QueryRow(ctx, `
		SELECT COALESCE(MAX(position), -1)
		FROM channel_rooms WHERE channel_id = $1
	`, channelID).Scan(&maxPos)

	room := &ChannelRoom{}
	err := r.db.QueryRow(ctx, `
		INSERT INTO channel_rooms (channel_id, category_id, name, type, position)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id, channel_id, category_id, name, topic, type,
		          position, is_nsfw, slowmode_sec
	`, channelID, categoryID, name, roomType, maxPos+1).Scan(
		&room.ID, &room.ChannelID, &room.CategoryID, &room.Name,
		&room.Topic, &room.Type, &room.Position,
		&room.IsNSFW, &room.SlowmodeSec,
	)
	return room, err
}

// GetRoom возвращает комнату по ID.
func (r *Repository) GetRoom(ctx context.Context, roomID string) (*ChannelRoom, error) {
	room := &ChannelRoom{}
	err := r.db.QueryRow(ctx, `
		SELECT id, channel_id, category_id, name, topic, type,
		       position, is_nsfw, slowmode_sec
		FROM channel_rooms WHERE id = $1
	`, roomID).Scan(
		&room.ID, &room.ChannelID, &room.CategoryID, &room.Name,
		&room.Topic, &room.Type, &room.Position,
		&room.IsNSFW, &room.SlowmodeSec,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrRoomNotFound
	}
	return room, err
}

// ────────────────────────────────────────────────────────────
// Messages in rooms
// ────────────────────────────────────────────────────────────

// SendRoomMessage сохраняет сообщение в комнату.
func (r *Repository) SendRoomMessage(ctx context.Context, roomID, senderID, content string) (*ChannelMessage, error) {
	msg := &ChannelMessage{}
	err := r.db.QueryRow(ctx, `
		WITH inserted AS (
			INSERT INTO channel_messages (room_id, sender_id, content)
			VALUES ($1, $2, $3)
			RETURNING id, room_id, sender_id, content, thread_id,
			          is_pinned, created_at, edited_at
		)
		SELECT i.id, i.room_id, i.sender_id,
		       u.username, u.display_name, u.avatar_url,
		       i.content, i.thread_id, i.is_pinned,
		       i.created_at, i.edited_at
		FROM inserted i
		JOIN users u ON u.id = i.sender_id
	`, roomID, senderID, content).Scan(
		&msg.ID, &msg.RoomID, &msg.SenderID,
		&msg.SenderUsername, &msg.SenderName, &msg.SenderAvatar,
		&msg.Content, &msg.ThreadID, &msg.IsPinned,
		&msg.CreatedAt, &msg.EditedAt,
	)
	return msg, err
}

// GetRoomMessages возвращает историю сообщений в комнате.
func (r *Repository) GetRoomMessages(ctx context.Context, roomID string, limit int, before *string) ([]ChannelMessage, error) {
	var rows pgx.Rows
	var err error

	if before != nil && *before != "" {
		rows, err = r.db.Query(ctx, `
			SELECT m.id, m.room_id, m.sender_id,
			       u.username, u.display_name, u.avatar_url,
			       m.content, m.thread_id, m.is_pinned,
			       m.created_at, m.edited_at
			FROM channel_messages m
			JOIN users u ON u.id = m.sender_id
			WHERE m.room_id = $1 AND m.deleted_at IS NULL
			  AND m.created_at < (SELECT created_at FROM channel_messages WHERE id = $2)
			ORDER BY m.created_at DESC
			LIMIT $3
		`, roomID, *before, limit)
	} else {
		rows, err = r.db.Query(ctx, `
			SELECT m.id, m.room_id, m.sender_id,
			       u.username, u.display_name, u.avatar_url,
			       m.content, m.thread_id, m.is_pinned,
			       m.created_at, m.edited_at
			FROM channel_messages m
			JOIN users u ON u.id = m.sender_id
			WHERE m.room_id = $1 AND m.deleted_at IS NULL
			ORDER BY m.created_at DESC
			LIMIT $2
		`, roomID, limit)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var messages []ChannelMessage
	for rows.Next() {
		var m ChannelMessage
		if err := rows.Scan(
			&m.ID, &m.RoomID, &m.SenderID,
			&m.SenderUsername, &m.SenderName, &m.SenderAvatar,
			&m.Content, &m.ThreadID, &m.IsPinned,
			&m.CreatedAt, &m.EditedAt,
		); err != nil {
			return nil, err
		}
		messages = append(messages, m)
	}
	return messages, rows.Err()
}

// ────────────────────────────────────────────────────────────
// Хелперы
// ────────────────────────────────────────────────────────────

func generateSlug(name string) string {
	slug := strings.ToLower(name)
	slug = strings.ReplaceAll(slug, " ", "-")
	var clean strings.Builder
	for _, r := range slug {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' ||
			(r >= 0x400 && r <= 0x4FF) {
			clean.WriteRune(r)
		}
	}
	result := clean.String()
	if result == "" {
		result = "channel"
	}
	suffix := make([]byte, 3)
	rand.Read(suffix)
	return result + "-" + hex.EncodeToString(suffix)
}

func generateInviteCode() string {
	b := make([]byte, 4)
	rand.Read(b)
	return hex.EncodeToString(b)
}

func scanChannels(rows pgx.Rows) ([]Channel, error) {
	var channels []Channel
	for rows.Next() {
		var ch Channel
		err := rows.Scan(
			&ch.ID, &ch.Name, &ch.Slug, &ch.Description,
			&ch.AvatarURL, &ch.BannerURL, &ch.IsPublic,
			&ch.InviteCode, &ch.CreatedBy, &ch.MemberCount, &ch.CreatedAt,
		)
		if err != nil {
			return nil, err
		}
		channels = append(channels, ch)
	}
	return channels, rows.Err()
}
