package channels

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"strconv"

	// Замени на свой module path из go.mod
	"github.com/yngnoise/vortex/internal/auth"
)

// ────────────────────────────────────────────────────────────
// Handler
// ────────────────────────────────────────────────────────────

type Handler struct {
	service *Service
}

func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

// RegisterRoutes подключает все channel-эндпоинты.
func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	// Channels
	mux.HandleFunc("POST /api/channels", h.handleCreateChannel)
	mux.HandleFunc("GET /api/channels/discover", h.handleDiscover)
	mux.HandleFunc("GET /api/channels/my", h.handleMyChannels)
	mux.HandleFunc("GET /api/channels/{id}", h.handleGetChannel)
	mux.HandleFunc("GET /api/channels/{id}/structure", h.handleGetStructure)

	// Members
	mux.HandleFunc("POST /api/channels/{id}/join", h.handleJoin)
	mux.HandleFunc("POST /api/channels/{id}/leave", h.handleLeave)
	mux.HandleFunc("GET /api/channels/{id}/members", h.handleGetMembers)

	// Rooms
	mux.HandleFunc("POST /api/channels/{id}/rooms", h.handleCreateRoom)

	// Room messages
	mux.HandleFunc("POST /api/rooms/{id}/messages", h.handleSendMessage)
	mux.HandleFunc("GET /api/rooms/{id}/messages", h.handleGetMessages)
}

// ────────────────────────────────────────────────────────────
// Типы запросов
// ────────────────────────────────────────────────────────────

type createChannelRequest struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	IsPublic    bool   `json:"is_public"`
}

type createRoomRequest struct {
	Name       string  `json:"name"`
	Type       string  `json:"type"`        // "text", "voice", "announcement"
	CategoryID *string `json:"category_id"` // в какую категорию
}

type sendMessageRequest struct {
	Content string `json:"content"`
}

// ────────────────────────────────────────────────────────────
// POST /api/channels
// ────────────────────────────────────────────────────────────
// Создать канал. Автоматически создаёт роли, категорию, комнату "general".
//
// Тело: {"name": "Мой канал", "description": "Описание", "is_public": true}

func (h *Handler) handleCreateChannel(w http.ResponseWriter, r *http.Request) {
	claims := auth.GetClaimsFromContext(r.Context())
	if claims == nil {
		writeError(w, http.StatusUnauthorized, "unauthorized", "UNAUTHORIZED")
		return
	}

	var req createChannelRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body", "INVALID_BODY")
		return
	}

	ch, err := h.service.CreateChannel(r.Context(), req.Name, req.Description, req.IsPublic, claims.UserID)
	if err != nil {
		switch {
		case errors.Is(err, ErrEmptyName):
			writeError(w, http.StatusBadRequest, "channel name is required", "EMPTY_NAME")
		case errors.Is(err, ErrSlugTaken):
			writeError(w, http.StatusConflict, "channel slug already taken", "SLUG_TAKEN")
		default:
			log.Printf("create channel error: %v", err)
			writeError(w, http.StatusInternalServerError, "internal error", "INTERNAL")
		}
		return
	}

	writeJSON(w, http.StatusCreated, ch)
}

// ────────────────────────────────────────────────────────────
// GET /api/channels/discover?limit=50&offset=0
// ────────────────────────────────────────────────────────────
// Список публичных каналов (discovery / explore).

func (h *Handler) handleDiscover(w http.ResponseWriter, r *http.Request) {
	claims := auth.GetClaimsFromContext(r.Context())
	if claims == nil {
		writeError(w, http.StatusUnauthorized, "unauthorized", "UNAUTHORIZED")
		return
	}

	limit := queryInt(r, "limit", 50)
	offset := queryInt(r, "offset", 0)

	channels, err := h.service.ListPublicChannels(r.Context(), limit, offset)
	if err != nil {
		log.Printf("discover channels error: %v", err)
		writeError(w, http.StatusInternalServerError, "internal error", "INTERNAL")
		return
	}

	if channels == nil {
		channels = []Channel{}
	}

	writeJSON(w, http.StatusOK, channels)
}

// ────────────────────────────────────────────────────────────
// GET /api/channels/my
// ────────────────────────────────────────────────────────────
// Каналы текущего пользователя.

func (h *Handler) handleMyChannels(w http.ResponseWriter, r *http.Request) {
	claims := auth.GetClaimsFromContext(r.Context())
	if claims == nil {
		writeError(w, http.StatusUnauthorized, "unauthorized", "UNAUTHORIZED")
		return
	}

	previews, err := h.service.GetMyChannels(r.Context(), claims.UserID)
	if err != nil {
		log.Printf("my channels error: %v", err)
		writeError(w, http.StatusInternalServerError, "internal error", "INTERNAL")
		return
	}

	if previews == nil {
		previews = []ChannelPreview{}
	}

	writeJSON(w, http.StatusOK, previews)
}

// ────────────────────────────────────────────────────────────
// GET /api/channels/{id}
// ────────────────────────────────────────────────────────────
// Информация о канале.

func (h *Handler) handleGetChannel(w http.ResponseWriter, r *http.Request) {
	claims := auth.GetClaimsFromContext(r.Context())
	if claims == nil {
		writeError(w, http.StatusUnauthorized, "unauthorized", "UNAUTHORIZED")
		return
	}

	channelID := r.PathValue("id")
	ch, err := h.service.GetChannel(r.Context(), channelID, claims.UserID)
	if err != nil {
		switch {
		case errors.Is(err, ErrChannelNotFound):
			writeError(w, http.StatusNotFound, "channel not found", "NOT_FOUND")
		case errors.Is(err, ErrNotMember):
			writeError(w, http.StatusForbidden, "not a member", "NOT_MEMBER")
		default:
			log.Printf("get channel error: %v", err)
			writeError(w, http.StatusInternalServerError, "internal error", "INTERNAL")
		}
		return
	}

	writeJSON(w, http.StatusOK, ch)
}

// ────────────────────────────────────────────────────────────
// GET /api/channels/{id}/structure
// ────────────────────────────────────────────────────────────
// Категории с комнатами — боковая панель Discord.

func (h *Handler) handleGetStructure(w http.ResponseWriter, r *http.Request) {
	claims := auth.GetClaimsFromContext(r.Context())
	if claims == nil {
		writeError(w, http.StatusUnauthorized, "unauthorized", "UNAUTHORIZED")
		return
	}

	channelID := r.PathValue("id")
	structure, err := h.service.GetStructure(r.Context(), channelID, claims.UserID)
	if err != nil {
		if errors.Is(err, ErrNotMember) {
			writeError(w, http.StatusForbidden, "not a member", "NOT_MEMBER")
			return
		}
		log.Printf("get structure error: %v", err)
		writeError(w, http.StatusInternalServerError, "internal error", "INTERNAL")
		return
	}

	if structure == nil {
		structure = []ChannelCategory{}
	}

	writeJSON(w, http.StatusOK, structure)
}

// ────────────────────────────────────────────────────────────
// POST /api/channels/{id}/join
// ────────────────────────────────────────────────────────────
// Вступить в публичный канал.

func (h *Handler) handleJoin(w http.ResponseWriter, r *http.Request) {
	claims := auth.GetClaimsFromContext(r.Context())
	if claims == nil {
		writeError(w, http.StatusUnauthorized, "unauthorized", "UNAUTHORIZED")
		return
	}

	channelID := r.PathValue("id")
	err := h.service.JoinChannel(r.Context(), channelID, claims.UserID)
	if err != nil {
		switch {
		case errors.Is(err, ErrChannelNotFound):
			writeError(w, http.StatusNotFound, "channel not found", "NOT_FOUND")
		case errors.Is(err, ErrAlreadyMember):
			writeError(w, http.StatusConflict, "already a member", "ALREADY_MEMBER")
		case errors.Is(err, ErrNoPermission):
			writeError(w, http.StatusForbidden, "channel is private", "PRIVATE_CHANNEL")
		default:
			log.Printf("join channel error: %v", err)
			writeError(w, http.StatusInternalServerError, "internal error", "INTERNAL")
		}
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "joined"})
}

// ────────────────────────────────────────────────────────────
// POST /api/channels/{id}/leave
// ────────────────────────────────────────────────────────────
// Покинуть канал.

func (h *Handler) handleLeave(w http.ResponseWriter, r *http.Request) {
	claims := auth.GetClaimsFromContext(r.Context())
	if claims == nil {
		writeError(w, http.StatusUnauthorized, "unauthorized", "UNAUTHORIZED")
		return
	}

	channelID := r.PathValue("id")
	err := h.service.LeaveChannel(r.Context(), channelID, claims.UserID)
	if err != nil {
		if errors.Is(err, ErrNotMember) {
			writeError(w, http.StatusBadRequest, "not a member", "NOT_MEMBER")
			return
		}
		log.Printf("leave channel error: %v", err)
		writeError(w, http.StatusInternalServerError, "internal error", "INTERNAL")
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "left"})
}

// ────────────────────────────────────────────────────────────
// GET /api/channels/{id}/members?limit=50&offset=0
// ────────────────────────────────────────────────────────────
// Список участников канала.

func (h *Handler) handleGetMembers(w http.ResponseWriter, r *http.Request) {
	claims := auth.GetClaimsFromContext(r.Context())
	if claims == nil {
		writeError(w, http.StatusUnauthorized, "unauthorized", "UNAUTHORIZED")
		return
	}

	channelID := r.PathValue("id")
	limit := queryInt(r, "limit", 50)
	offset := queryInt(r, "offset", 0)

	members, err := h.service.GetMembers(r.Context(), channelID, claims.UserID, limit, offset)
	if err != nil {
		if errors.Is(err, ErrNotMember) {
			writeError(w, http.StatusForbidden, "not a member", "NOT_MEMBER")
			return
		}
		log.Printf("get members error: %v", err)
		writeError(w, http.StatusInternalServerError, "internal error", "INTERNAL")
		return
	}

	if members == nil {
		members = []ChannelMember{}
	}

	writeJSON(w, http.StatusOK, members)
}

// ────────────────────────────────────────────────────────────
// POST /api/channels/{id}/rooms
// ────────────────────────────────────────────────────────────
// Создать комнату в канале.
//
// Тело: {"name": "dev-chat", "type": "text", "category_id": "uuid"}

func (h *Handler) handleCreateRoom(w http.ResponseWriter, r *http.Request) {
	claims := auth.GetClaimsFromContext(r.Context())
	if claims == nil {
		writeError(w, http.StatusUnauthorized, "unauthorized", "UNAUTHORIZED")
		return
	}

	channelID := r.PathValue("id")
	var req createRoomRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body", "INVALID_BODY")
		return
	}
	if req.Type == "" {
		req.Type = "text"
	}

	room, err := h.service.CreateRoom(r.Context(), channelID, claims.UserID, req.CategoryID, req.Name, req.Type)
	if err != nil {
		switch {
		case errors.Is(err, ErrEmptyRoom):
			writeError(w, http.StatusBadRequest, "room name is required", "EMPTY_NAME")
		case errors.Is(err, ErrBadRoomType):
			writeError(w, http.StatusBadRequest, "type must be text, voice, or announcement", "BAD_TYPE")
		case errors.Is(err, ErrNotMember):
			writeError(w, http.StatusForbidden, "not a member", "NOT_MEMBER")
		default:
			log.Printf("create room error: %v", err)
			writeError(w, http.StatusInternalServerError, "internal error", "INTERNAL")
		}
		return
	}

	writeJSON(w, http.StatusCreated, room)
}

// ────────────────────────────────────────────────────────────
// POST /api/rooms/{id}/messages
// ────────────────────────────────────────────────────────────
// Отправить сообщение в комнату.
//
// Тело: {"content": "Привет!"}

func (h *Handler) handleSendMessage(w http.ResponseWriter, r *http.Request) {
	claims := auth.GetClaimsFromContext(r.Context())
	if claims == nil {
		writeError(w, http.StatusUnauthorized, "unauthorized", "UNAUTHORIZED")
		return
	}

	roomID := r.PathValue("id")
	var req sendMessageRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body", "INVALID_BODY")
		return
	}

	msg, err := h.service.SendMessage(r.Context(), roomID, claims.UserID, req.Content)
	if err != nil {
		switch {
		case errors.Is(err, ErrEmptyContent):
			writeError(w, http.StatusBadRequest, "message cannot be empty", "EMPTY_MESSAGE")
		case errors.Is(err, ErrNotMember):
			writeError(w, http.StatusForbidden, "not a member of this channel", "NOT_MEMBER")
		case errors.Is(err, ErrRoomNotFound):
			writeError(w, http.StatusNotFound, "room not found", "NOT_FOUND")
		default:
			log.Printf("send room message error: %v", err)
			writeError(w, http.StatusInternalServerError, "internal error", "INTERNAL")
		}
		return
	}

	writeJSON(w, http.StatusCreated, msg)
}

// ────────────────────────────────────────────────────────────
// GET /api/rooms/{id}/messages?limit=50&before=uuid
// ────────────────────────────────────────────────────────────
// История сообщений комнаты.

func (h *Handler) handleGetMessages(w http.ResponseWriter, r *http.Request) {
	claims := auth.GetClaimsFromContext(r.Context())
	if claims == nil {
		writeError(w, http.StatusUnauthorized, "unauthorized", "UNAUTHORIZED")
		return
	}

	roomID := r.PathValue("id")
	limit := queryInt(r, "limit", 50)
	var before *string
	if b := r.URL.Query().Get("before"); b != "" {
		before = &b
	}

	messages, err := h.service.GetMessages(r.Context(), roomID, claims.UserID, limit, before)
	if err != nil {
		switch {
		case errors.Is(err, ErrNotMember):
			writeError(w, http.StatusForbidden, "not a member", "NOT_MEMBER")
		case errors.Is(err, ErrRoomNotFound):
			writeError(w, http.StatusNotFound, "room not found", "NOT_FOUND")
		default:
			log.Printf("get room messages error: %v", err)
			writeError(w, http.StatusInternalServerError, "internal error", "INTERNAL")
		}
		return
	}

	if messages == nil {
		messages = []ChannelMessage{}
	}

	writeJSON(w, http.StatusOK, messages)
}

// ────────────────────────────────────────────────────────────
// Хелперы
// ────────────────────────────────────────────────────────────

func writeJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

func writeError(w http.ResponseWriter, status int, message, code string) {
	writeJSON(w, status, map[string]string{"error": message, "code": code})
}

func queryInt(r *http.Request, key string, defaultVal int) int {
	s := r.URL.Query().Get(key)
	if s == "" {
		return defaultVal
	}
	v, err := strconv.Atoi(s)
	if err != nil {
		return defaultVal
	}
	return v
}
