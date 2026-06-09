package messaging

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"strconv"

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

// RegisterRoutes подключает все messaging-эндпоинты.
// Все требуют авторизации (auth middleware применяется в main.go).
func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	// Conversations
	mux.HandleFunc("POST /api/conversations/direct", h.handleCreateDirect)
	mux.HandleFunc("POST /api/conversations/group", h.handleCreateGroup)
	mux.HandleFunc("GET /api/conversations", h.handleListConversations)
	mux.HandleFunc("GET /api/conversations/{id}", h.handleGetConversation)
	mux.HandleFunc("GET /api/conversations/{id}/members", h.handleGetMembers)
	mux.HandleFunc("PATCH /api/conversations/{id}/messages/{msgId}", h.handleEditMessage)
	mux.HandleFunc("DELETE /api/conversations/{id}/messages/{msgId}", h.handleDeleteMessage)

	// Messages
	mux.HandleFunc("POST /api/conversations/{id}/messages", h.handleSendMessage)
	mux.HandleFunc("GET /api/conversations/{id}/messages", h.handleGetMessages)
	mux.HandleFunc("POST /api/conversations/{id}/read", h.handleMarkAsRead)
	mux.HandleFunc("POST /api/conversations/{id}/typing", h.handleTyping)
	mux.HandleFunc("GET /api/conversations/{id}/read-state", h.handleReadState)

	// Group management
	mux.HandleFunc("PATCH /api/conversations/{id}", h.handleRenameGroup)
	mux.HandleFunc("POST /api/conversations/{id}/members", h.handleAddMembers)
	mux.HandleFunc("DELETE /api/conversations/{id}/members/{userId}", h.handleRemoveMember)
	mux.HandleFunc("POST /api/conversations/{id}/leave", h.handleLeaveGroup)
}

// ────────────────────────────────────────────────────────────
// Типы запросов
// ────────────────────────────────────────────────────────────

type createDirectRequest struct {
	OtherUserID string `json:"other_user_id"`
}

type createGroupRequest struct {
	Title     string   `json:"title"`
	MemberIDs []string `json:"member_ids"`
}

type sendMessageRequest struct {
	Content     string            `json:"content"`
	ContentType string            `json:"content_type"` // "text", "image", и т.д.
	ReplyToID   *string           `json:"reply_to_id"`
	Attachments []AttachmentInput `json:"attachments"` // ссылки на загруженные файлы
}

type markAsReadRequest struct {
	MessageID string `json:"message_id"`
}

type editMessageRequest struct {
	Content string `json:"content"`
}

type renameGroupRequest struct {
	Title string `json:"title"`
}

type addMembersRequest struct {
	UserIDs []string `json:"user_ids"`
}

// ────────────────────────────────────────────────────────────
// POST /api/conversations/direct
// ────────────────────────────────────────────────────────────
// Создаёт личный чат с другим пользователем.
// Если чат уже существует — вернёт его.
//
// Тело: {"other_user_id": "uuid-собеседника"}
// Ответ: объект Conversation

func (h *Handler) handleCreateDirect(w http.ResponseWriter, r *http.Request) {
	claims := auth.GetClaimsFromContext(r.Context())
	if claims == nil {
		writeError(w, http.StatusUnauthorized, "unauthorized", "UNAUTHORIZED")
		return
	}

	var req createDirectRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body", "INVALID_BODY")
		return
	}
	if req.OtherUserID == "" {
		writeError(w, http.StatusBadRequest, "other_user_id is required", "MISSING_FIELD")
		return
	}

	conv, err := h.service.CreateDirect(r.Context(), claims.UserID, req.OtherUserID)
	if err != nil {
		switch {
		case errors.Is(err, ErrSelfChat):
			writeError(w, http.StatusBadRequest, "cannot create chat with yourself", "SELF_CHAT")
		default:
			log.Printf("create direct error: %v", err)
			writeError(w, http.StatusInternalServerError, "internal error", "INTERNAL")
		}
		return
	}

	writeJSON(w, http.StatusCreated, conv)
}

// ────────────────────────────────────────────────────────────
// POST /api/conversations/group
// ────────────────────────────────────────────────────────────
// Создаёт групповой чат.
//
// Тело: {"title": "Название", "member_ids": ["uuid1", "uuid2"]}
// Ответ: объект Conversation

func (h *Handler) handleCreateGroup(w http.ResponseWriter, r *http.Request) {
	claims := auth.GetClaimsFromContext(r.Context())
	if claims == nil {
		writeError(w, http.StatusUnauthorized, "unauthorized", "UNAUTHORIZED")
		return
	}

	var req createGroupRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body", "INVALID_BODY")
		return
	}

	conv, err := h.service.CreateGroup(r.Context(), claims.UserID, req.Title, req.MemberIDs)
	if err != nil {
		switch {
		case errors.Is(err, ErrNoTitle):
			writeError(w, http.StatusBadRequest, "title is required", "MISSING_TITLE")
		case errors.Is(err, ErrTooFewMembers):
			writeError(w, http.StatusBadRequest, "at least one member required", "TOO_FEW_MEMBERS")
		default:
			log.Printf("create group error: %v", err)
			writeError(w, http.StatusInternalServerError, "internal error", "INTERNAL")
		}
		return
	}

	writeJSON(w, http.StatusCreated, conv)
}

// ────────────────────────────────────────────────────────────
// GET /api/conversations?limit=50&offset=0
// ────────────────────────────────────────────────────────────
// Список чатов текущего пользователя (главный экран мессенджера).
// Каждый чат содержит последнее сообщение и счётчик непрочитанных.

func (h *Handler) handleListConversations(w http.ResponseWriter, r *http.Request) {
	claims := auth.GetClaimsFromContext(r.Context())
	if claims == nil {
		writeError(w, http.StatusUnauthorized, "unauthorized", "UNAUTHORIZED")
		return
	}

	limit := queryInt(r, "limit", 50)
	offset := queryInt(r, "offset", 0)

	previews, err := h.service.GetConversations(r.Context(), claims.UserID, limit, offset)
	if err != nil {
		log.Printf("list conversations error: %v", err)
		writeError(w, http.StatusInternalServerError, "internal error", "INTERNAL")
		return
	}

	// Возвращаем пустой массив, а не null
	if previews == nil {
		previews = []ConversationPreview{}
	}

	writeJSON(w, http.StatusOK, previews)
}

// ────────────────────────────────────────────────────────────
// GET /api/conversations/{id}
// ────────────────────────────────────────────────────────────
// Информация о конкретном чате.

func (h *Handler) handleGetConversation(w http.ResponseWriter, r *http.Request) {
	claims := auth.GetClaimsFromContext(r.Context())
	if claims == nil {
		writeError(w, http.StatusUnauthorized, "unauthorized", "UNAUTHORIZED")
		return
	}

	convID := r.PathValue("id")
	if convID == "" {
		writeError(w, http.StatusBadRequest, "conversation id required", "MISSING_ID")
		return
	}

	conv, err := h.service.GetConversation(r.Context(), convID, claims.UserID)
	if err != nil {
		switch {
		case errors.Is(err, ErrNotMember):
			writeError(w, http.StatusForbidden, "not a member of this conversation", "NOT_MEMBER")
		case errors.Is(err, ErrConversationNotFound):
			writeError(w, http.StatusNotFound, "conversation not found", "NOT_FOUND")
		default:
			log.Printf("get conversation error: %v", err)
			writeError(w, http.StatusInternalServerError, "internal error", "INTERNAL")
		}
		return
	}

	writeJSON(w, http.StatusOK, conv)
}

// ────────────────────────────────────────────────────────────
// GET /api/conversations/{id}/members
// ────────────────────────────────────────────────────────────
// Список участников чата.

func (h *Handler) handleGetMembers(w http.ResponseWriter, r *http.Request) {
	claims := auth.GetClaimsFromContext(r.Context())
	if claims == nil {
		writeError(w, http.StatusUnauthorized, "unauthorized", "UNAUTHORIZED")
		return
	}

	convID := r.PathValue("id")
	members, err := h.service.GetMembers(r.Context(), convID, claims.UserID)
	if err != nil {
		if errors.Is(err, ErrNotMember) {
			writeError(w, http.StatusForbidden, "not a member", "NOT_MEMBER")
			return
		}
		log.Printf("get members error: %v", err)
		writeError(w, http.StatusInternalServerError, "internal error", "INTERNAL")
		return
	}

	writeJSON(w, http.StatusOK, members)
}

// ────────────────────────────────────────────────────────────
// POST /api/conversations/{id}/messages
// ────────────────────────────────────────────────────────────
// Отправить сообщение в чат.
//
// Тело: {"content": "Привет!", "content_type": "text"}
// Ответ: объект Message

func (h *Handler) handleSendMessage(w http.ResponseWriter, r *http.Request) {
	claims := auth.GetClaimsFromContext(r.Context())
	if claims == nil {
		writeError(w, http.StatusUnauthorized, "unauthorized", "UNAUTHORIZED")
		return
	}

	convID := r.PathValue("id")
	var req sendMessageRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body", "INVALID_BODY")
		return
	}

	msg, err := h.service.SendMessage(
		r.Context(), convID, claims.UserID,
		req.Content, req.ContentType, req.ReplyToID, req.Attachments,
	)
	if err != nil {
		switch {
		case errors.Is(err, ErrEmptyMessage):
			writeError(w, http.StatusBadRequest, "message cannot be empty", "EMPTY_MESSAGE")
		case errors.Is(err, ErrContentTooLong):
			writeError(w, http.StatusBadRequest, "message too long (max 8000 chars)", "CONTENT_TOO_LONG")
		case errors.Is(err, ErrTooManyAttachments):
			writeError(w, http.StatusBadRequest, "too many attachments (max 10)", "TOO_MANY_ATTACHMENTS")
		case errors.Is(err, ErrInvalidAttachment):
			writeError(w, http.StatusBadRequest, "invalid attachment", "INVALID_ATTACHMENT")
		case errors.Is(err, ErrNotMember):
			writeError(w, http.StatusForbidden, "not a member", "NOT_MEMBER")
		default:
			log.Printf("send message error: %v", err)
			writeError(w, http.StatusInternalServerError, "internal error", "INTERNAL")
		}
		return
	}

	writeJSON(w, http.StatusCreated, msg)
}

// ────────────────────────────────────────────────────────────
// GET /api/conversations/{id}/messages?limit=50&before=uuid
// ────────────────────────────────────────────────────────────
// История сообщений с cursor-пагинацией.
// before — ID сообщения, от которого листаем назад.
// Без before — последние сообщения.

func (h *Handler) handleGetMessages(w http.ResponseWriter, r *http.Request) {
	claims := auth.GetClaimsFromContext(r.Context())
	if claims == nil {
		writeError(w, http.StatusUnauthorized, "unauthorized", "UNAUTHORIZED")
		return
	}

	convID := r.PathValue("id")
	limit := queryInt(r, "limit", 50)

	var before *string
	if b := r.URL.Query().Get("before"); b != "" {
		before = &b
	}

	messages, err := h.service.GetMessages(r.Context(), convID, claims.UserID, limit, before)
	if err != nil {
		if errors.Is(err, ErrNotMember) {
			writeError(w, http.StatusForbidden, "not a member", "NOT_MEMBER")
			return
		}
		log.Printf("get messages error: %v", err)
		writeError(w, http.StatusInternalServerError, "internal error", "INTERNAL")
		return
	}

	if messages == nil {
		messages = []Message{}
	}

	writeJSON(w, http.StatusOK, messages)
}

// ────────────────────────────────────────────────────────────
// POST /api/conversations/{id}/read
// ────────────────────────────────────────────────────────────
// Отметить сообщения прочитанными.
// Тело: {"message_id": "uuid-последнего-прочитанного"}

func (h *Handler) handleMarkAsRead(w http.ResponseWriter, r *http.Request) {
	claims := auth.GetClaimsFromContext(r.Context())
	if claims == nil {
		writeError(w, http.StatusUnauthorized, "unauthorized", "UNAUTHORIZED")
		return
	}

	convID := r.PathValue("id")
	var req markAsReadRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body", "INVALID_BODY")
		return
	}
	if req.MessageID == "" {
		writeError(w, http.StatusBadRequest, "message_id required", "MISSING_FIELD")
		return
	}

	if err := h.service.MarkAsRead(r.Context(), convID, claims.UserID, req.MessageID); err != nil {
		if errors.Is(err, ErrNotMember) {
			writeError(w, http.StatusForbidden, "not a member", "NOT_MEMBER")
			return
		}
		log.Printf("mark as read error: %v", err)
		writeError(w, http.StatusInternalServerError, "internal error", "INTERNAL")
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// ────────────────────────────────────────────────────────────
// POST /api/conversations/{id}/typing
// ────────────────────────────────────────────────────────────
// Эфемерное уведомление «печатает». Тело не требуется,
// ничего не сохраняется — только публикация в realtime-канал.

func (h *Handler) handleTyping(w http.ResponseWriter, r *http.Request) {
	claims := auth.GetClaimsFromContext(r.Context())
	if claims == nil {
		writeError(w, http.StatusUnauthorized, "unauthorized", "UNAUTHORIZED")
		return
	}

	convID := r.PathValue("id")
	if err := h.service.NotifyTyping(r.Context(), convID, claims.UserID); err != nil {
		if errors.Is(err, ErrNotMember) {
			writeError(w, http.StatusForbidden, "not a member", "NOT_MEMBER")
			return
		}
		log.Printf("typing error: %v", err)
		writeError(w, http.StatusInternalServerError, "internal error", "INTERNAL")
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// ────────────────────────────────────────────────────────────
// Хелперы
// ────────────────────────────────────────────────────────────

// ────────────────────────────────────────────────────────────
// PATCH /api/conversations/{id}/messages/{msgId}
// ────────────────────────────────────────────────────────────
// Редактировать своё сообщение.
// Тело: {"content": "Новый текст"}

func (h *Handler) handleEditMessage(w http.ResponseWriter, r *http.Request) {
	claims := auth.GetClaimsFromContext(r.Context())
	if claims == nil {
		writeError(w, http.StatusUnauthorized, "unauthorized", "UNAUTHORIZED")
		return
	}

	convID := r.PathValue("id")
	msgID := r.PathValue("msgId")

	var req editMessageRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body", "INVALID_BODY")
		return
	}

	msg, err := h.service.EditMessage(r.Context(), convID, msgID, claims.UserID, req.Content)
	if err != nil {
		switch {
		case errors.Is(err, ErrEmptyMessage):
			writeError(w, http.StatusBadRequest, "content cannot be empty", "EMPTY_MESSAGE")
		case errors.Is(err, ErrNotMember):
			writeError(w, http.StatusForbidden, "not a member", "NOT_MEMBER")
		case errors.Is(err, ErrMessageNotFound):
			writeError(w, http.StatusNotFound, "message not found or not yours", "NOT_FOUND")
		default:
			log.Printf("edit message error: %v", err)
			writeError(w, http.StatusInternalServerError, "internal error", "INTERNAL")
		}
		return
	}

	writeJSON(w, http.StatusOK, msg)
}

// ────────────────────────────────────────────────────────────
// DELETE /api/conversations/{id}/messages/{msgId}
// ────────────────────────────────────────────────────────────
// Удалить своё сообщение (мягкое удаление).

func (h *Handler) handleDeleteMessage(w http.ResponseWriter, r *http.Request) {
	claims := auth.GetClaimsFromContext(r.Context())
	if claims == nil {
		writeError(w, http.StatusUnauthorized, "unauthorized", "UNAUTHORIZED")
		return
	}

	convID := r.PathValue("id")
	msgID := r.PathValue("msgId")

	err := h.service.DeleteMessage(r.Context(), convID, msgID, claims.UserID)
	if err != nil {
		switch {
		case errors.Is(err, ErrNotMember):
			writeError(w, http.StatusForbidden, "not a member", "NOT_MEMBER")
		case errors.Is(err, ErrMessageNotFound):
			writeError(w, http.StatusNotFound, "message not found or not yours", "NOT_FOUND")
		default:
			log.Printf("delete message error: %v", err)
			writeError(w, http.StatusInternalServerError, "internal error", "INTERNAL")
		}
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

func writeJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

func writeError(w http.ResponseWriter, status int, message, code string) {
	writeJSON(w, status, map[string]string{"error": message, "code": code})
}

// queryInt извлекает int query-параметр с дефолтом.
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

// ────────────────────────────────────────────────────────────
// GET /api/conversations/{id}/read-state
// ────────────────────────────────────────────────────────────
// Позиции прочтения участников — для отрисовки галочек при открытии чата.

func (h *Handler) handleReadState(w http.ResponseWriter, r *http.Request) {
	claims := auth.GetClaimsFromContext(r.Context())
	if claims == nil {
		writeError(w, http.StatusUnauthorized, "unauthorized", "UNAUTHORIZED")
		return
	}

	convID := r.PathValue("id")
	states, err := h.service.GetReadState(r.Context(), convID, claims.UserID)
	if err != nil {
		if errors.Is(err, ErrNotMember) {
			writeError(w, http.StatusForbidden, "not a member", "NOT_MEMBER")
			return
		}
		log.Printf("read state error: %v", err)
		writeError(w, http.StatusInternalServerError, "internal error", "INTERNAL")
		return
	}

	if states == nil {
		states = []MemberReadState{}
	}
	writeJSON(w, http.StatusOK, states)
}

// ────────────────────────────────────────────────────────────
// Group management
// ────────────────────────────────────────────────────────────

// PATCH /api/conversations/{id} — переименовать группу (owner/admin).
func (h *Handler) handleRenameGroup(w http.ResponseWriter, r *http.Request) {
	claims := auth.GetClaimsFromContext(r.Context())
	if claims == nil {
		writeError(w, http.StatusUnauthorized, "unauthorized", "UNAUTHORIZED")
		return
	}
	convID := r.PathValue("id")
	var req renameGroupRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body", "INVALID_BODY")
		return
	}
	conv, err := h.service.RenameGroup(r.Context(), convID, claims.UserID, req.Title)
	if err != nil {
		writeGroupError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, conv)
}

// POST /api/conversations/{id}/members — добавить участников (owner/admin).
func (h *Handler) handleAddMembers(w http.ResponseWriter, r *http.Request) {
	claims := auth.GetClaimsFromContext(r.Context())
	if claims == nil {
		writeError(w, http.StatusUnauthorized, "unauthorized", "UNAUTHORIZED")
		return
	}
	convID := r.PathValue("id")
	var req addMembersRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body", "INVALID_BODY")
		return
	}
	if err := h.service.AddMembers(r.Context(), convID, claims.UserID, req.UserIDs); err != nil {
		writeGroupError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// DELETE /api/conversations/{id}/members/{userId} — исключить участника.
func (h *Handler) handleRemoveMember(w http.ResponseWriter, r *http.Request) {
	claims := auth.GetClaimsFromContext(r.Context())
	if claims == nil {
		writeError(w, http.StatusUnauthorized, "unauthorized", "UNAUTHORIZED")
		return
	}
	convID := r.PathValue("id")
	targetID := r.PathValue("userId")
	if err := h.service.RemoveMember(r.Context(), convID, claims.UserID, targetID); err != nil {
		writeGroupError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "removed"})
}

// POST /api/conversations/{id}/leave — покинуть группу.
func (h *Handler) handleLeaveGroup(w http.ResponseWriter, r *http.Request) {
	claims := auth.GetClaimsFromContext(r.Context())
	if claims == nil {
		writeError(w, http.StatusUnauthorized, "unauthorized", "UNAUTHORIZED")
		return
	}
	convID := r.PathValue("id")
	if err := h.service.LeaveGroup(r.Context(), convID, claims.UserID); err != nil {
		writeGroupError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "left"})
}

// writeGroupError маппит ошибки управления группой в HTTP-коды.
func writeGroupError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, ErrNoTitle):
		writeError(w, http.StatusBadRequest, "title is required", "MISSING_TITLE")
	case errors.Is(err, ErrTitleTooLong):
		writeError(w, http.StatusBadRequest, "title too long (max 128)", "TITLE_TOO_LONG")
	case errors.Is(err, ErrTooFewMembers):
		writeError(w, http.StatusBadRequest, "no members to add", "TOO_FEW_MEMBERS")
	case errors.Is(err, ErrNotGroup):
		writeError(w, http.StatusBadRequest, "not a group conversation", "NOT_GROUP")
	case errors.Is(err, ErrForbidden):
		writeError(w, http.StatusForbidden, "insufficient permissions", "FORBIDDEN")
	case errors.Is(err, ErrCannotRemoveOwner):
		writeError(w, http.StatusBadRequest, "cannot remove the group owner", "CANNOT_REMOVE_OWNER")
	case errors.Is(err, ErrCannotKickSelf):
		writeError(w, http.StatusBadRequest, "use leave to remove yourself", "USE_LEAVE")
	case errors.Is(err, ErrNotMember):
		writeError(w, http.StatusForbidden, "not a member", "NOT_MEMBER")
	case errors.Is(err, ErrConversationNotFound):
		writeError(w, http.StatusNotFound, "conversation not found", "NOT_FOUND")
	default:
		log.Printf("group management error: %v", err)
		writeError(w, http.StatusInternalServerError, "internal error", "INTERNAL")
	}
}
