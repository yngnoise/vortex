package messaging

import (
	"context"
	"errors"
)

// ────────────────────────────────────────────────────────────
// Ошибки сервисного слоя
// ────────────────────────────────────────────────────────────

var (
	ErrEmptyMessage  = errors.New("message content cannot be empty")
	ErrSelfChat      = errors.New("cannot create chat with yourself")
	ErrNoTitle       = errors.New("group chat requires a title")
	ErrTooFewMembers = errors.New("group chat requires at least one other member")
)

// ────────────────────────────────────────────────────────────
// RTClient — интерфейс для публикации real-time событий.
// ────────────────────────────────────────────────────────────
// Используем интерфейс, а не конкретный тип, чтобы
// messaging не зависел от пакета realtime напрямую.
// Если rt == nil (Centrifugo не настроен) — просто пропускаем.

type RTClient interface {
	Publish(channel string, event interface{}) error
}

// ────────────────────────────────────────────────────────────
// Service
// ────────────────────────────────────────────────────────────

type Service struct {
	repo *Repository
	rt   RTClient
}

func NewService(repo *Repository, rt RTClient) *Service {
	return &Service{repo: repo, rt: rt}
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

func (s *Service) SendMessage(ctx context.Context, conversationID, senderID, content, contentType string, replyToID *string) (*Message, error) {
	if content == "" {
		return nil, ErrEmptyMessage
	}
	if contentType == "" {
		contentType = "text"
	}

	isMember, err := s.repo.IsMember(ctx, conversationID, senderID)
	if err != nil {
		return nil, err
	}
	if !isMember {
		return nil, ErrNotMember
	}

	msg, err := s.repo.SendMessage(ctx, conversationID, senderID, content, contentType, replyToID)
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
	return s.repo.MarkAsRead(ctx, conversationID, userID, messageID)
}
