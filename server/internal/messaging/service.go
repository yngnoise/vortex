package messaging

import (
	"context"
	"errors"
	"strings"
	"unicode/utf8"
)

// ────────────────────────────────────────────────────────────
// Ошибки сервисного слоя
// ────────────────────────────────────────────────────────────

var (
	ErrEmptyMessage       = errors.New("message content cannot be empty")
	ErrSelfChat           = errors.New("cannot create chat with yourself")
	ErrNoTitle            = errors.New("group chat requires a title")
	ErrTooFewMembers      = errors.New("group chat requires at least one other member")
	ErrContentTooLong     = errors.New("message content too long")
	ErrTooManyAttachments = errors.New("too many attachments")
	ErrInvalidAttachment  = errors.New("invalid attachment")
	ErrNotGroup           = errors.New("not a group conversation")
	ErrForbidden          = errors.New("insufficient permissions")
	ErrCannotRemoveOwner  = errors.New("cannot remove the group owner")
	ErrCannotKickSelf     = errors.New("use leave to remove yourself")
	ErrTitleTooLong       = errors.New("group title too long")
)

const (
	maxContentLen  = 8000 // макс. длина текста сообщения (в рунах)
	maxAttachments = 10   // макс. число вложений в одном сообщении
)

// AttachmentInput — ссылка на ранее загруженный файл (POST /api/media/upload).
// Клиент присылает только key и имя файла; размер и MIME-тип сервер
// берёт из хранилища, чтобы не доверять метаданным клиента.
type AttachmentInput struct {
	Key      string `json:"key"`
	FileName string `json:"file_name"`
}

// ────────────────────────────────────────────────────────────
// RTClient — интерфейс для публикации real-time событий.
// ────────────────────────────────────────────────────────────
// Используем интерфейс, а не конкретный тип, чтобы
// messaging не зависел от пакета realtime напрямую.
// Если rt == nil (Centrifugo не настроен) — просто пропускаем.

type RTClient interface {
	Publish(channel string, event interface{}) error
}

// AttachmentStore резолвит загруженные файлы при привязке к сообщению.
// Реализуется media.Storage. Размер и MIME-тип берутся из хранилища —
// данным клиента не доверяем.
type AttachmentStore interface {
	Stat(ctx context.Context, key string) (url string, size int64, contentType string, err error)
}

// ────────────────────────────────────────────────────────────
// Service
// ────────────────────────────────────────────────────────────

type Service struct {
	repo  *Repository
	rt    RTClient
	store AttachmentStore
}

func NewService(repo *Repository, rt RTClient, store AttachmentStore) *Service {
	return &Service{repo: repo, rt: rt, store: store}
}

// ────────────────────────────────────────────────────────────
// Conversations
// ────────────────────────────────────────────────────────────

func (s *Service) CreateDirect(ctx context.Context, userID, otherUserID string) (*Conversation, error) {
	if userID == otherUserID {
		return nil, ErrSelfChat
	}
	return s.repo.CreateDirectConversation(ctx, userID, otherUserID)
}

func (s *Service) CreateGroup(ctx context.Context, creatorID, title string, memberIDs []string) (*Conversation, error) {
	if title == "" {
		return nil, ErrNoTitle
	}
	if len(memberIDs) == 0 {
		return nil, ErrTooFewMembers
	}
	return s.repo.CreateGroupConversation(ctx, creatorID, title, memberIDs)
}

func (s *Service) GetConversations(ctx context.Context, userID string, limit, offset int) ([]ConversationPreview, error) {
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	if offset < 0 {
		offset = 0
	}
	return s.repo.GetUserConversations(ctx, userID, limit, offset)
}

func (s *Service) GetConversation(ctx context.Context, conversationID, userID string) (*Conversation, error) {
	isMember, err := s.repo.IsMember(ctx, conversationID, userID)
	if err != nil {
		return nil, err
	}
	if !isMember {
		return nil, ErrNotMember
	}
	return s.repo.GetConversation(ctx, conversationID)
}

func (s *Service) GetMembers(ctx context.Context, conversationID, userID string) ([]ConversationMember, error) {
	isMember, err := s.repo.IsMember(ctx, conversationID, userID)
	if err != nil {
		return nil, err
	}
	if !isMember {
		return nil, ErrNotMember
	}
	return s.repo.GetMembers(ctx, conversationID)
}

// ────────────────────────────────────────────────────────────
// Messages
// ────────────────────────────────────────────────────────────

func (s *Service) SendMessage(ctx context.Context, conversationID, senderID, content, contentType string, replyToID *string, attachments []AttachmentInput) (*Message, error) {
	content = strings.TrimSpace(content)

	if len(attachments) > maxAttachments {
		return nil, ErrTooManyAttachments
	}
	if content == "" && len(attachments) == 0 {
		return nil, ErrEmptyMessage
	}
	if utf8.RuneCountInString(content) > maxContentLen {
		return nil, ErrContentTooLong
	}

	isMember, err := s.repo.IsMember(ctx, conversationID, senderID)
	if err != nil {
		return nil, err
	}
	if !isMember {
		return nil, ErrNotMember
	}

	// Резолвим вложения из хранилища — URL/размер/тип берём оттуда,
	// данным клиента не доверяем. key обязан начинаться с "messages/".
	var resolved []ResolvedAttachment
	for _, in := range attachments {
		if s.store == nil || !strings.HasPrefix(in.Key, "messages/") {
			return nil, ErrInvalidAttachment
		}
		url, size, mime, err := s.store.Stat(ctx, in.Key)
		if err != nil {
			return nil, ErrInvalidAttachment
		}
		resolved = append(resolved, ResolvedAttachment{
			FileType: fileTypeForMime(mime),
			FileURL:  url,
			FileSize: size,
			FileName: sanitizeFileName(in.FileName),
			MimeType: mime,
		})
	}

	if contentType == "" {
		contentType = "text"
	}
	if len(resolved) > 0 {
		// Тип сообщения определяется первым вложением.
		contentType = messageTypeForAttachment(resolved[0].FileType)
	}

	msg, err := s.repo.SendMessage(ctx, conversationID, senderID, content, contentType, replyToID, resolved)
	if err != nil {
		return nil, err
	}

	// Публикуем в Centrifugo — все подписчики чата получат мгновенно.
	// go — запуск в горутине, чтобы не блокировать HTTP-ответ.
	if s.rt != nil {
		go s.rt.Publish("chat:"+conversationID, map[string]interface{}{
			"type": "message",
			"data": msg,
		})
	}

	return msg, nil
}

// fileTypeForMime сопоставляет MIME-тип категории вложения
// (значения совпадают с CHECK в таблице attachments).
func fileTypeForMime(mime string) string {
	switch {
	case strings.HasPrefix(mime, "image/"):
		return "image"
	case strings.HasPrefix(mime, "video/"):
		return "video"
	case strings.HasPrefix(mime, "audio/"):
		return "audio"
	case mime == "application/pdf" || strings.HasPrefix(mime, "text/"):
		return "document"
	default:
		return "other"
	}
}

// messageTypeForAttachment определяет content_type сообщения
// (значения совпадают с CHECK в таблице messages).
func messageTypeForAttachment(fileType string) string {
	switch fileType {
	case "image", "video", "audio":
		return fileType
	default:
		return "file"
	}
}

// sanitizeFileName убирает разделители путей и ограничивает длину.
func sanitizeFileName(name string) string {
	name = strings.TrimSpace(name)
	name = strings.ReplaceAll(name, "/", "_")
	name = strings.ReplaceAll(name, "\\", "_")
	if len(name) > 255 {
		name = name[:255]
	}
	return name
}

func (s *Service) GetMessages(ctx context.Context, conversationID, userID string, limit int, before *string) ([]Message, error) {
	if limit <= 0 || limit > 100 {
		limit = 50
	}

	isMember, err := s.repo.IsMember(ctx, conversationID, userID)
	if err != nil {
		return nil, err
	}
	if !isMember {
		return nil, ErrNotMember
	}

	return s.repo.GetMessages(ctx, conversationID, limit, before)
}

func (s *Service) MarkAsRead(ctx context.Context, conversationID, userID, messageID string) error {
	isMember, err := s.repo.IsMember(ctx, conversationID, userID)
	if err != nil {
		return err
	}
	if !isMember {
		return ErrNotMember
	}

	readAt, err := s.repo.MarkAsRead(ctx, conversationID, userID, messageID)
	if err != nil {
		return err
	}

	// Сообщаем отправителям, что их сообщения прочитаны (галочки прочтения).
	if s.rt != nil && !readAt.IsZero() {
		go s.rt.Publish("chat:"+conversationID, map[string]interface{}{
			"type": "read",
			"data": map[string]interface{}{
				"user_id":         userID,
				"conversation_id": conversationID,
				"last_read_at":    readAt,
			},
		})
	}
	return nil
}

// GetReadState возвращает позиции прочтения участников чата.
func (s *Service) GetReadState(ctx context.Context, conversationID, userID string) ([]MemberReadState, error) {
	isMember, err := s.repo.IsMember(ctx, conversationID, userID)
	if err != nil {
		return nil, err
	}
	if !isMember {
		return nil, ErrNotMember
	}
	return s.repo.GetReadState(ctx, conversationID)
}

// ──────────────────────────────────────────────────────
// ДОБАВЬ ЭТИ МЕТОДЫ В КОНЕЦ messaging/service.go
// (после метода MarkAsRead)
// ──────────────────────────────────────────────────────

// EditMessage редактирует сообщение.
// Только автор может редактировать своё сообщение.
func (s *Service) EditMessage(ctx context.Context, conversationID, messageID, userID, newContent string) (*Message, error) {
	if newContent == "" {
		return nil, ErrEmptyMessage
	}

	isMember, err := s.repo.IsMember(ctx, conversationID, userID)
	if err != nil {
		return nil, err
	}
	if !isMember {
		return nil, ErrNotMember
	}

	msg, err := s.repo.EditMessage(ctx, messageID, userID, newContent)
	if err != nil {
		return nil, err
	}

	// Уведомляем подписчиков об изменении
	if s.rt != nil {
		go s.rt.Publish("chat:"+conversationID, map[string]interface{}{
			"type": "message_edited",
			"data": msg,
		})
	}

	return msg, nil
}

// DeleteMessage мягко удаляет сообщение.
// Только автор может удалить своё сообщение.
func (s *Service) DeleteMessage(ctx context.Context, conversationID, messageID, userID string) error {
	isMember, err := s.repo.IsMember(ctx, conversationID, userID)
	if err != nil {
		return err
	}
	if !isMember {
		return ErrNotMember
	}

	if err := s.repo.DeleteMessage(ctx, messageID, userID); err != nil {
		return err
	}

	// Уведомляем подписчиков об удалении
	if s.rt != nil {
		go s.rt.Publish("chat:"+conversationID, map[string]interface{}{
			"type": "message_deleted",
			"data": map[string]string{
				"message_id":      messageID,
				"conversation_id": conversationID,
			},
		})
	}

	return nil
}

// NotifyTyping публикует эфемерное событие «печатает» в канал чата.
// Ничего не сохраняет; уведомлять может только участник чата.
func (s *Service) NotifyTyping(ctx context.Context, conversationID, userID string) error {
	isMember, err := s.repo.IsMember(ctx, conversationID, userID)
	if err != nil {
		return err
	}
	if !isMember {
		return ErrNotMember
	}

	if s.rt != nil {
		go s.rt.Publish("chat:"+conversationID, map[string]interface{}{
			"type": "typing",
			"data": map[string]string{"user_id": userID},
		})
	}
	return nil
}

// ────────────────────────────────────────────────────────────
// Group management
// ────────────────────────────────────────────────────────────

const maxTitleLen = 128

func isManager(role string) bool { return role == "owner" || role == "admin" }

// requireGroupManager проверяет, что чат групповой, а вызывающий — owner/admin.
func (s *Service) requireGroupManager(ctx context.Context, conversationID, userID string) (*Conversation, error) {
	conv, err := s.repo.GetConversation(ctx, conversationID)
	if err != nil {
		return nil, err
	}
	if conv.Type != "group" {
		return nil, ErrNotGroup
	}
	role, err := s.repo.GetMemberRole(ctx, conversationID, userID)
	if err != nil {
		return nil, err
	}
	if !isManager(role) {
		return nil, ErrForbidden
	}
	return conv, nil
}

// RenameGroup меняет название группы (owner/admin).
func (s *Service) RenameGroup(ctx context.Context, conversationID, userID, title string) (*Conversation, error) {
	title = strings.TrimSpace(title)
	if title == "" {
		return nil, ErrNoTitle
	}
	if utf8.RuneCountInString(title) > maxTitleLen {
		return nil, ErrTitleTooLong
	}
	if _, err := s.requireGroupManager(ctx, conversationID, userID); err != nil {
		return nil, err
	}

	conv, err := s.repo.RenameGroup(ctx, conversationID, title)
	if err != nil {
		return nil, err
	}

	if s.rt != nil {
		go s.rt.Publish("chat:"+conversationID, map[string]interface{}{
			"type": "group_renamed",
			"data": map[string]interface{}{"conversation_id": conversationID, "title": title},
		})
	}
	return conv, nil
}

// AddMembers добавляет участников в группу (owner/admin).
func (s *Service) AddMembers(ctx context.Context, conversationID, userID string, memberIDs []string) error {
	if len(memberIDs) == 0 {
		return ErrTooFewMembers
	}
	if _, err := s.requireGroupManager(ctx, conversationID, userID); err != nil {
		return err
	}
	if err := s.repo.AddMembers(ctx, conversationID, memberIDs); err != nil {
		return err
	}

	if s.rt != nil {
		for _, uid := range memberIDs {
			go s.rt.Publish("chat:"+conversationID, map[string]interface{}{
				"type": "member_added",
				"data": map[string]interface{}{"conversation_id": conversationID, "user_id": uid},
			})
			go s.rt.Publish("user:"+uid, map[string]interface{}{
				"type": "added_to_conversation",
				"data": map[string]interface{}{"conversation_id": conversationID},
			})
		}
	}
	return nil
}

// RemoveMember исключает участника (owner/admin; нельзя владельца и себя).
func (s *Service) RemoveMember(ctx context.Context, conversationID, callerID, targetID string) error {
	if callerID == targetID {
		return ErrCannotKickSelf
	}
	if _, err := s.requireGroupManager(ctx, conversationID, callerID); err != nil {
		return err
	}
	targetRole, err := s.repo.GetMemberRole(ctx, conversationID, targetID)
	if err != nil {
		return err
	}
	if targetRole == "owner" {
		return ErrCannotRemoveOwner
	}
	if err := s.repo.RemoveMember(ctx, conversationID, targetID); err != nil {
		return err
	}

	if s.rt != nil {
		go s.rt.Publish("chat:"+conversationID, map[string]interface{}{
			"type": "member_removed",
			"data": map[string]interface{}{"conversation_id": conversationID, "user_id": targetID},
		})
		go s.rt.Publish("user:"+targetID, map[string]interface{}{
			"type": "removed_from_conversation",
			"data": map[string]interface{}{"conversation_id": conversationID},
		})
	}
	return nil
}

// LeaveGroup — выход участника. Если уходит owner, владельцем
// становится самый давний из оставшихся участников.
func (s *Service) LeaveGroup(ctx context.Context, conversationID, userID string) error {
	conv, err := s.repo.GetConversation(ctx, conversationID)
	if err != nil {
		return err
	}
	if conv.Type != "group" {
		return ErrNotGroup
	}
	if err := s.repo.LeaveConversation(ctx, conversationID, userID); err != nil {
		return err
	}

	if s.rt != nil {
		go s.rt.Publish("chat:"+conversationID, map[string]interface{}{
			"type": "member_removed",
			"data": map[string]interface{}{"conversation_id": conversationID, "user_id": userID},
		})
	}
	return nil
}
