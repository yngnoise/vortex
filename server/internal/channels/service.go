package channels

import (
	"context"
	"errors"
)

// ────────────────────────────────────────────────────────────
// Ошибки сервисного слоя
// ────────────────────────────────────────────────────────────

var (
	ErrEmptyName    = errors.New("channel name cannot be empty")
	ErrEmptyContent = errors.New("message content cannot be empty")
	ErrEmptyRoom    = errors.New("room name cannot be empty")
	ErrBadRoomType  = errors.New("room type must be text, voice, or announcement")
)

// ────────────────────────────────────────────────────────────
// RTClient — интерфейс для real-time событий.
// ────────────────────────────────────────────────────────────

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
// Channels
// ────────────────────────────────────────────────────────────

func (s *Service) CreateChannel(ctx context.Context, name, description string, isPublic bool, creatorID string) (*Channel, error) {
	if name == "" {
		return nil, ErrEmptyName
	}
	return s.repo.CreateChannel(ctx, name, description, isPublic, creatorID)
}

func (s *Service) GetChannel(ctx context.Context, channelID, userID string) (*Channel, error) {
	ch, err := s.repo.GetChannel(ctx, channelID)
	if err != nil {
		return nil, err
	}

	if !ch.IsPublic {
		isMember, err := s.repo.IsMember(ctx, channelID, userID)
		if err != nil {
			return nil, err
		}
		if !isMember {
			return nil, ErrNotMember
		}
	}

	return ch, nil
}

func (s *Service) ListPublicChannels(ctx context.Context, limit, offset int) ([]Channel, error) {
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	if offset < 0 {
		offset = 0
	}
	return s.repo.ListPublicChannels(ctx, limit, offset)
}

func (s *Service) GetMyChannels(ctx context.Context, userID string) ([]ChannelPreview, error) {
	return s.repo.GetUserChannels(ctx, userID)
}

// ────────────────────────────────────────────────────────────
// Members
// ────────────────────────────────────────────────────────────

func (s *Service) JoinChannel(ctx context.Context, channelID, userID string) error {
	ch, err := s.repo.GetChannel(ctx, channelID)
	if err != nil {
		return err
	}

	if !ch.IsPublic {
		return ErrNoPermission
	}

	return s.repo.JoinChannel(ctx, channelID, userID)
}

func (s *Service) LeaveChannel(ctx context.Context, channelID, userID string) error {
	return s.repo.LeaveChannel(ctx, channelID, userID)
}

func (s *Service) GetMembers(ctx context.Context, channelID, userID string, limit, offset int) ([]ChannelMember, error) {
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	if offset < 0 {
		offset = 0
	}

	isMember, err := s.repo.IsMember(ctx, channelID, userID)
	if err != nil {
		return nil, err
	}
	if !isMember {
		return nil, ErrNotMember
	}

	return s.repo.GetMembers(ctx, channelID, limit, offset)
}

// ────────────────────────────────────────────────────────────
// Rooms
// ────────────────────────────────────────────────────────────

func (s *Service) GetStructure(ctx context.Context, channelID, userID string) ([]ChannelCategory, error) {
	isMember, err := s.repo.IsMember(ctx, channelID, userID)
	if err != nil {
		return nil, err
	}
	if !isMember {
		return nil, ErrNotMember
	}
	return s.repo.GetChannelStructure(ctx, channelID)
}

func (s *Service) CreateRoom(ctx context.Context, channelID, userID string, categoryID *string, name, roomType string) (*ChannelRoom, error) {
	if name == "" {
		return nil, ErrEmptyRoom
	}
	if roomType != "text" && roomType != "voice" && roomType != "announcement" {
		return nil, ErrBadRoomType
	}

	isMember, err := s.repo.IsMember(ctx, channelID, userID)
	if err != nil {
		return nil, err
	}
	if !isMember {
		return nil, ErrNotMember
	}

	return s.repo.CreateRoom(ctx, channelID, categoryID, name, roomType)
}

// ────────────────────────────────────────────────────────────
// Messages in rooms
// ────────────────────────────────────────────────────────────

func (s *Service) SendMessage(ctx context.Context, roomID, userID, content string) (*ChannelMessage, error) {
	if content == "" {
		return nil, ErrEmptyContent
	}

	room, err := s.repo.GetRoom(ctx, roomID)
	if err != nil {
		return nil, err
	}

	isMember, err := s.repo.IsMember(ctx, room.ChannelID, userID)
	if err != nil {
		return nil, err
	}
	if !isMember {
		return nil, ErrNotMember
	}

	msg, err := s.repo.SendRoomMessage(ctx, roomID, userID, content)
	if err != nil {
		return nil, err
	}

	// Публикуем в Centrifugo — все в комнате получат мгновенно
	if s.rt != nil {
		go s.rt.Publish("channel:"+roomID, map[string]interface{}{
			"type": "message",
			"data": msg,
		})
	}

	return msg, nil
}

func (s *Service) GetMessages(ctx context.Context, roomID, userID string, limit int, before *string) ([]ChannelMessage, error) {
	if limit <= 0 || limit > 100 {
		limit = 50
	}

	room, err := s.repo.GetRoom(ctx, roomID)
	if err != nil {
		return nil, err
	}

	isMember, err := s.repo.IsMember(ctx, room.ChannelID, userID)
	if err != nil {
		return nil, err
	}
	if !isMember {
		return nil, ErrNotMember
	}

	return s.repo.GetRoomMessages(ctx, roomID, limit, before)
}
