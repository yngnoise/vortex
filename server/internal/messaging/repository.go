package messaging

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// ────────────────────────────────────────────────────────────
// Ошибки
// ────────────────────────────────────────────────────────────

var (
	ErrConversationNotFound = errors.New("conversation not found")
	ErrNotMember            = errors.New("user is not a member of this conversation")
	ErrMessageNotFound      = errors.New("message not found")
	ErrDirectExists         = errors.New("direct conversation already exists")
)

// ────────────────────────────────────────────────────────────
// Модели
// ────────────────────────────────────────────────────────────

// Conversation — чат (личный или групповой).
// Для личных чатов Title = nil, тип = "direct".
// Для групповых — Title задаёт название группы.
type Conversation struct {
	ID        string    `json:"id"`
	Type      string    `json:"type"`  // "direct" или "group"
	Title     *string   `json:"title"` // nil для direct-чатов
	AvatarURL *string   `json:"avatar_url"`
	CreatedBy *string   `json:"created_by"`
	CreatedAt time.Time `json:"created_at"`
}

// ConversationMember — участник чата с его ролью.
type ConversationMember struct {
	ConversationID string    `json:"conversation_id"`
	UserID         string    `json:"user_id"`
	Username       string    `json:"username"`
	DisplayName    string    `json:"display_name"`
	AvatarURL      *string   `json:"avatar_url,omitempty"`
	Role           string    `json:"role"` // "owner", "admin", "member"
	JoinedAt       time.Time `json:"joined_at"`
}

// Message — одно сообщение в чате.
// ContentEncrypted хранит зашифрованные байты (E2E).
// На данном этапе мы шлём обычный текст, E2E добавим позже.
type Message struct {
	ID               string     `json:"id"`
	ConversationID   string     `json:"conversation_id"`
	SenderID         string     `json:"sender_id"`
	SenderUsername   string     `json:"sender_username"`
	SenderName       string     `json:"sender_display_name"`
	ContentEncrypted []byte     `json:"-"`       // сырые байты — не отдаём
	Content          string     `json:"content"` // расшифрованный текст для ответа
	ContentType      string     `json:"content_type"`
	ReplyToID        *string    `json:"reply_to_id,omitempty"`
	CreatedAt        time.Time    `json:"created_at"`
	EditedAt         *time.Time   `json:"edited_at,omitempty"`
	Attachments      []Attachment `json:"attachments,omitempty"`
}

// Attachment — вложение сообщения (строка таблицы attachments).
type Attachment struct {
	ID           string    `json:"id"`
	MessageID    string    `json:"message_id"`
	FileType     string    `json:"file_type"`
	FileURL      string    `json:"file_url"`
	FileSize     int64     `json:"file_size"`
	FileName     *string   `json:"file_name,omitempty"`
	MimeType     *string   `json:"mime_type,omitempty"`
	ThumbnailURL *string   `json:"thumbnail_url,omitempty"`
	CreatedAt    time.Time `json:"created_at"`
}

// ResolvedAttachment — вложение, уже проверенное сервисом
// (метаданные взяты из хранилища). Передаётся в репозиторий для вставки.
type ResolvedAttachment struct {
	FileType string
	FileURL  string
	FileSize int64
	FileName string // "" → NULL
	MimeType string // "" → NULL
}

// ConversationPreview — элемент списка чатов.
// Содержит инфу о чате + последнее сообщение + счётчик непрочитанных.
type ConversationPreview struct {
	Conversation
	LastMessage *Message `json:"last_message,omitempty"`
	UnreadCount int      `json:"unread_count"`
	MemberCount int      `json:"member_count"`
	// Для direct-чатов: данные собеседника
	OtherUserID   *string    `json:"other_user_id,omitempty"`
	OtherUsername *string    `json:"other_username,omitempty"`
	OtherName     *string    `json:"other_display_name,omitempty"`
	OtherAvatar   *string    `json:"other_avatar_url,omitempty"`
	OtherLastSeen *time.Time `json:"other_last_seen,omitempty"`
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
// Conversations
// ────────────────────────────────────────────────────────────

// CreateDirectConversation создаёт личный чат между двумя пользователями.
// Сначала проверяет нет ли уже такого чата — если есть, возвращает его.
//
// Логика:
// 1. Ищем существующий direct-чат между userID и otherUserID.
// 2. Если нашли — возвращаем его (не создаём дубль).
// 3. Если нет — создаём conversation + добавляем обоих как members.
// Всё в одной транзакции, чтобы не было гонки.
func (r *Repository) CreateDirectConversation(ctx context.Context, userID, otherUserID string) (*Conversation, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	// Проверяем существующий direct-чат
	var existingID string
	err = tx.QueryRow(ctx, `
		SELECT cm1.conversation_id
		FROM conversation_members cm1
		JOIN conversation_members cm2 ON cm1.conversation_id = cm2.conversation_id
		JOIN conversations c ON c.id = cm1.conversation_id
		WHERE cm1.user_id = $1 AND cm2.user_id = $2 AND c.type = 'direct'
		LIMIT 1
	`, userID, otherUserID).Scan(&existingID)

	if err == nil {
		// Чат уже существует — возвращаем его
		tx.Rollback(ctx)
		return r.GetConversation(ctx, existingID)
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return nil, err
	}

	// Создаём новый direct-чат
	conv := &Conversation{}
	err = tx.QueryRow(ctx, `
		INSERT INTO conversations (type, created_by)
		VALUES ('direct', $1)
		RETURNING id, type, title, avatar_url, created_by, created_at
	`, userID).Scan(
		&conv.ID, &conv.Type, &conv.Title,
		&conv.AvatarURL, &conv.CreatedBy, &conv.CreatedAt,
	)
	if err != nil {
		return nil, err
	}

	// Добавляем обоих участников
	_, err = tx.Exec(ctx, `
		INSERT INTO conversation_members (conversation_id, user_id, role)
		VALUES ($1, $2, 'owner'), ($1, $3, 'member')
	`, conv.ID, userID, otherUserID)
	if err != nil {
		return nil, err
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}

	return conv, nil
}

// CreateGroupConversation создаёт групповой чат.
// creatorID становится owner, memberIDs — обычными members.
func (r *Repository) CreateGroupConversation(ctx context.Context, creatorID, title string, memberIDs []string) (*Conversation, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	conv := &Conversation{}
	err = tx.QueryRow(ctx, `
		INSERT INTO conversations (type, title, created_by)
		VALUES ('group', $1, $2)
		RETURNING id, type, title, avatar_url, created_by, created_at
	`, title, creatorID).Scan(
		&conv.ID, &conv.Type, &conv.Title,
		&conv.AvatarURL, &conv.CreatedBy, &conv.CreatedAt,
	)
	if err != nil {
		return nil, err
	}

	// Добавляем создателя как owner
	_, err = tx.Exec(ctx, `
		INSERT INTO conversation_members (conversation_id, user_id, role)
		VALUES ($1, $2, 'owner')
	`, conv.ID, creatorID)
	if err != nil {
		return nil, err
	}

	// Добавляем остальных участников
	for _, memberID := range memberIDs {
		if memberID == creatorID {
			continue // не добавляем создателя дважды
		}
		_, err = tx.Exec(ctx, `
			INSERT INTO conversation_members (conversation_id, user_id, role)
			VALUES ($1, $2, 'member')
		`, conv.ID, memberID)
		if err != nil {
			return nil, err
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}

	return conv, nil
}

// GetConversation возвращает чат по ID.
func (r *Repository) GetConversation(ctx context.Context, conversationID string) (*Conversation, error) {
	conv := &Conversation{}
	err := r.db.QueryRow(ctx, `
		SELECT id, type, title, avatar_url, created_by, created_at
		FROM conversations WHERE id = $1
	`, conversationID).Scan(
		&conv.ID, &conv.Type, &conv.Title,
		&conv.AvatarURL, &conv.CreatedBy, &conv.CreatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrConversationNotFound
	}
	return conv, err
}

// GetUserConversations возвращает список чатов пользователя
// с превью последнего сообщения и счётчиком непрочитанных.
// Это то, что показывается на главном экране мессенджера.
func (r *Repository) GetUserConversations(ctx context.Context, userID string, limit, offset int) ([]ConversationPreview, error) {
	rows, err := r.db.Query(ctx, `
		SELECT
			c.id, c.type, c.title, c.avatar_url, c.created_by, c.created_at,
			-- Последнее сообщение
			m.id, m.sender_id, u_sender.username, u_sender.display_name,
			m.content_encrypted, m.content_type, m.created_at,
			-- Количество участников
			(SELECT COUNT(*) FROM conversation_members WHERE conversation_id = c.id),
			-- Непрочитанные (сообщения после last_read_msg_id)
			(SELECT COUNT(*) FROM messages
			 WHERE conversation_id = c.id
			   AND deleted_at IS NULL
			   AND created_at > COALESCE(
			       (SELECT m2.created_at FROM messages m2 WHERE m2.id = cm.last_read_msg_id),
			       cm.joined_at
			   )
			   AND sender_id != $1
			),
			-- Для direct-чатов: собеседник
			u_other.id, u_other.username, u_other.display_name, u_other.avatar_url, u_other.last_seen_at
		FROM conversation_members cm
		JOIN conversations c ON c.id = cm.conversation_id
		-- Последнее сообщение (LEFT JOIN — чат может быть пустым)
		LEFT JOIN LATERAL (
			SELECT id, sender_id, content_encrypted, content_type, created_at
			FROM messages
			WHERE conversation_id = c.id AND deleted_at IS NULL
			ORDER BY created_at DESC
			LIMIT 1
		) m ON true
		LEFT JOIN users u_sender ON u_sender.id = m.sender_id
		-- Собеседник в direct-чате
		LEFT JOIN LATERAL (
			SELECT u.id, u.username, u.display_name, u.avatar_url, u.last_seen_at
			FROM conversation_members cm2
			JOIN users u ON u.id = cm2.user_id
			WHERE cm2.conversation_id = c.id
			  AND cm2.user_id != $1
			  AND c.type = 'direct'
			LIMIT 1
		) u_other ON true
		WHERE cm.user_id = $1
		ORDER BY COALESCE(m.created_at, c.created_at) DESC
		LIMIT $2 OFFSET $3
	`, userID, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var previews []ConversationPreview
	for rows.Next() {
		var p ConversationPreview
		var msgID, msgSenderID, msgSenderUsername, msgSenderName *string
		var msgContent []byte
		var msgContentType *string
		var msgCreatedAt *time.Time

		err := rows.Scan(
			&p.ID, &p.Type, &p.Title, &p.AvatarURL, &p.CreatedBy, &p.CreatedAt,
			&msgID, &msgSenderID, &msgSenderUsername, &msgSenderName,
			&msgContent, &msgContentType, &msgCreatedAt,
			&p.MemberCount,
			&p.UnreadCount,
			&p.OtherUserID, &p.OtherUsername, &p.OtherName, &p.OtherAvatar,
			&p.OtherLastSeen,
		)
		if err != nil {
			return nil, err
		}

		// Собираем последнее сообщение если оно есть
		if msgID != nil {
			p.LastMessage = &Message{
				ID:             *msgID,
				ConversationID: p.ID,
				SenderID:       *msgSenderID,
				SenderUsername: *msgSenderUsername,
				SenderName:     *msgSenderName,
				Content:        string(msgContent), // пока без E2E — просто текст
				ContentType:    *msgContentType,
				CreatedAt:      *msgCreatedAt,
			}
		}

		previews = append(previews, p)
	}

	return previews, rows.Err()
}

// IsMember проверяет что пользователь — участник чата.
// Вызывается перед отправкой сообщений и чтением истории.
func (r *Repository) IsMember(ctx context.Context, conversationID, userID string) (bool, error) {
	var exists bool
	err := r.db.QueryRow(ctx, `
		SELECT EXISTS(
			SELECT 1 FROM conversation_members
			WHERE conversation_id = $1 AND user_id = $2
		)
	`, conversationID, userID).Scan(&exists)
	return exists, err
}

// GetMembers возвращает список участников чата.
func (r *Repository) GetMembers(ctx context.Context, conversationID string) ([]ConversationMember, error) {
	rows, err := r.db.Query(ctx, `
		SELECT cm.conversation_id, cm.user_id, u.username, u.display_name,
		       u.avatar_url, cm.role, cm.joined_at
		FROM conversation_members cm
		JOIN users u ON u.id = cm.user_id
		WHERE cm.conversation_id = $1
		ORDER BY cm.joined_at
	`, conversationID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var members []ConversationMember
	for rows.Next() {
		var m ConversationMember
		err := rows.Scan(
			&m.ConversationID, &m.UserID, &m.Username,
			&m.DisplayName, &m.AvatarURL, &m.Role, &m.JoinedAt,
		)
		if err != nil {
			return nil, err
		}
		members = append(members, m)
	}
	return members, rows.Err()
}

// ────────────────────────────────────────────────────────────
// Messages
// ────────────────────────────────────────────────────────────

// SendMessage сохраняет новое сообщение в базу.
// content приходит как строка, мы конвертируем в []byte.
// Когда добавим E2E — клиент будет слать уже зашифрованные байты.
func (r *Repository) SendMessage(ctx context.Context, conversationID, senderID, content, contentType string, replyToID *string, atts []ResolvedAttachment) (*Message, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	msg := &Message{}
	var senderUsername, senderName string

	err = tx.QueryRow(ctx, `
		WITH inserted AS (
			INSERT INTO messages (conversation_id, sender_id, content_encrypted, content_type, reply_to_id)
			VALUES ($1, $2, $3, $4, $5)
			RETURNING id, conversation_id, sender_id, content_encrypted,
			          content_type, reply_to_id, created_at, edited_at
		)
		SELECT i.id, i.conversation_id, i.sender_id,
		       u.username, u.display_name,
		       i.content_encrypted, i.content_type,
		       i.reply_to_id, i.created_at, i.edited_at
		FROM inserted i
		JOIN users u ON u.id = i.sender_id
	`, conversationID, senderID, []byte(content), contentType, replyToID).Scan(
		&msg.ID, &msg.ConversationID, &msg.SenderID,
		&senderUsername, &senderName,
		&msg.ContentEncrypted, &msg.ContentType,
		&msg.ReplyToID, &msg.CreatedAt, &msg.EditedAt,
	)
	if err != nil {
		return nil, err
	}

	// Вложения вставляем в той же транзакции — либо сообщение со всеми
	// вложениями, либо ничего.
	for _, a := range atts {
		var att Attachment
		err = tx.QueryRow(ctx, `
			INSERT INTO attachments (message_id, file_type, file_url, file_size, file_name, mime_type)
			VALUES ($1, $2, $3, $4, NULLIF($5, ''), NULLIF($6, ''))
			RETURNING id, message_id, file_type, file_url, file_size,
			          file_name, mime_type, thumbnail_url, created_at
		`, msg.ID, a.FileType, a.FileURL, a.FileSize, a.FileName, a.MimeType).Scan(
			&att.ID, &att.MessageID, &att.FileType, &att.FileURL, &att.FileSize,
			&att.FileName, &att.MimeType, &att.ThumbnailURL, &att.CreatedAt,
		)
		if err != nil {
			return nil, err
		}
		msg.Attachments = append(msg.Attachments, att)
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}

	msg.SenderUsername = senderUsername
	msg.SenderName = senderName
	msg.Content = string(msg.ContentEncrypted)

	return msg, nil
}

// GetMessages возвращает историю сообщений чата с пагинацией.
// before — UUID сообщения, от которого листаем назад (cursor-based pagination).
// Если before пустой — возвращаем последние сообщения.
func (r *Repository) GetMessages(ctx context.Context, conversationID string, limit int, before *string) ([]Message, error) {
	var rows pgx.Rows
	var err error

	if before != nil && *before != "" {
		// Загружаем сообщения старше указанного
		rows, err = r.db.Query(ctx, `
			SELECT m.id, m.conversation_id, m.sender_id,
			       u.username, u.display_name,
			       m.content_encrypted, m.content_type,
			       m.reply_to_id, m.created_at, m.edited_at
			FROM messages m
			JOIN users u ON u.id = m.sender_id
			WHERE m.conversation_id = $1
			  AND m.deleted_at IS NULL
			  AND m.created_at < (SELECT created_at FROM messages WHERE id = $2)
			ORDER BY m.created_at DESC
			LIMIT $3
		`, conversationID, *before, limit)
	} else {
		// Загружаем последние сообщения
		rows, err = r.db.Query(ctx, `
			SELECT m.id, m.conversation_id, m.sender_id,
			       u.username, u.display_name,
			       m.content_encrypted, m.content_type,
			       m.reply_to_id, m.created_at, m.edited_at
			FROM messages m
			JOIN users u ON u.id = m.sender_id
			WHERE m.conversation_id = $1 AND m.deleted_at IS NULL
			ORDER BY m.created_at DESC
			LIMIT $2
		`, conversationID, limit)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var messages []Message
	for rows.Next() {
		var m Message
		err := rows.Scan(
			&m.ID, &m.ConversationID, &m.SenderID,
			&m.SenderUsername, &m.SenderName,
			&m.ContentEncrypted, &m.ContentType,
			&m.ReplyToID, &m.CreatedAt, &m.EditedAt,
		)
		if err != nil {
			return nil, err
		}
		m.Content = string(m.ContentEncrypted)
		messages = append(messages, m)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	rows.Close() // освобождаем соединение перед догрузкой вложений

	if err := r.attachToMessages(ctx, messages); err != nil {
		return nil, err
	}
	return messages, nil
}

// attachToMessages догружает вложения для набора сообщений одним запросом
// и раскладывает их по соответствующим сообщениям (без N+1).
func (r *Repository) attachToMessages(ctx context.Context, msgs []Message) error {
	if len(msgs) == 0 {
		return nil
	}

	ids := make([]string, len(msgs))
	idx := make(map[string]int, len(msgs))
	for i, m := range msgs {
		ids[i] = m.ID
		idx[m.ID] = i
	}

	rows, err := r.db.Query(ctx, `
		SELECT id, message_id, file_type, file_url, file_size,
		       file_name, mime_type, thumbnail_url, created_at
		FROM attachments
		WHERE message_id = ANY($1::uuid[])
		ORDER BY created_at
	`, ids)
	if err != nil {
		return err
	}
	defer rows.Close()

	for rows.Next() {
		var a Attachment
		if err := rows.Scan(
			&a.ID, &a.MessageID, &a.FileType, &a.FileURL, &a.FileSize,
			&a.FileName, &a.MimeType, &a.ThumbnailURL, &a.CreatedAt,
		); err != nil {
			return err
		}
		if i, ok := idx[a.MessageID]; ok {
			msgs[i].Attachments = append(msgs[i].Attachments, a)
		}
	}
	return rows.Err()
}

// MarkAsRead обновляет last_read_msg_id для пользователя в чате.
// Это как "синие галочки" — сервер знает до какого сообщения
// пользователь долистал.
// Возвращает время создания прочитанного сообщения (для галочек прочтения).
// Джойн с messages гарантирует, что сообщение принадлежит этому чату.
func (r *Repository) MarkAsRead(ctx context.Context, conversationID, userID, messageID string) (time.Time, error) {
	var readAt time.Time
	err := r.db.QueryRow(ctx, `
		UPDATE conversation_members cm
		SET last_read_msg_id = m.id
		FROM messages m
		WHERE cm.conversation_id = $1 AND cm.user_id = $2
		  AND m.id = $3 AND m.conversation_id = $1
		RETURNING m.created_at
	`, conversationID, userID, messageID).Scan(&readAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return time.Time{}, nil // невалидный message_id для этого чата — тихо игнорируем
	}
	return readAt, err
}

// MemberReadState — позиция прочтения участника (для галочек прочтения).
type MemberReadState struct {
	UserID     string     `json:"user_id"`
	LastReadAt *time.Time `json:"last_read_at"`
}

// GetReadState возвращает для каждого участника время последнего
// прочитанного сообщения (NULL, если ещё ничего не читал).
func (r *Repository) GetReadState(ctx context.Context, conversationID string) ([]MemberReadState, error) {
	rows, err := r.db.Query(ctx, `
		SELECT cm.user_id, m.created_at
		FROM conversation_members cm
		LEFT JOIN messages m ON m.id = cm.last_read_msg_id
		WHERE cm.conversation_id = $1
	`, conversationID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var states []MemberReadState
	for rows.Next() {
		var s MemberReadState
		if err := rows.Scan(&s.UserID, &s.LastReadAt); err != nil {
			return nil, err
		}
		states = append(states, s)
	}
	return states, rows.Err()
}

// ──────────────────────────────────────────────────────
// ДОБАВЬ ЭТИ МЕТОДЫ В КОНЕЦ messaging/repository.go
// (после метода MarkAsRead)
// ──────────────────────────────────────────────────────

// EditMessage обновляет текст сообщения.
// Проверяет что сообщение принадлежит отправителю.
// Устанавливает edited_at — клиент покажет "(изменено)".
func (r *Repository) EditMessage(ctx context.Context, messageID, senderID, newContent string) (*Message, error) {
	msg := &Message{}
	var senderUsername, senderName string

	err := r.db.QueryRow(ctx, `
		UPDATE messages
		SET content_encrypted = $3, edited_at = NOW()
		WHERE id = $1 AND sender_id = $2 AND deleted_at IS NULL
		RETURNING id, conversation_id, sender_id,
		          content_encrypted, content_type,
		          reply_to_id, created_at, edited_at
	`, messageID, senderID, []byte(newContent)).Scan(
		&msg.ID, &msg.ConversationID, &msg.SenderID,
		&msg.ContentEncrypted, &msg.ContentType,
		&msg.ReplyToID, &msg.CreatedAt, &msg.EditedAt,
	)
	if err != nil {
		return nil, ErrMessageNotFound
	}

	// Достаём имя отправителя
	r.db.QueryRow(ctx, `
		SELECT username, display_name FROM users WHERE id = $1
	`, senderID).Scan(&senderUsername, &senderName)

	msg.SenderUsername = senderUsername
	msg.SenderName = senderName
	msg.Content = string(msg.ContentEncrypted)

	return msg, nil
}

// DeleteMessage мягко удаляет сообщение (устанавливает deleted_at).
// Сообщение остаётся в базе, но не показывается в истории.
// Проверяет что сообщение принадлежит отправителю.
func (r *Repository) DeleteMessage(ctx context.Context, messageID, senderID string) error {
	tag, err := r.db.Exec(ctx, `
		UPDATE messages
		SET deleted_at = NOW()
		WHERE id = $1 AND sender_id = $2 AND deleted_at IS NULL
	`, messageID, senderID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrMessageNotFound
	}
	return nil
}
