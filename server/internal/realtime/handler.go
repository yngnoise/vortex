package realtime

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/yngnoise/vortex/internal/auth"
)

// MemberChecker проверяет членство пользователя в ресурсе.
// Реализуется messaging.Repository и channels.Repository.
type MemberChecker interface {
	IsMember(ctx context.Context, id, userID string) (bool, error)
}

type Handler struct {
	client  *Client
	msgRepo MemberChecker
	chRepo  MemberChecker
}

func NewHandler(client *Client, msgRepo, chRepo MemberChecker) *Handler {
	return &Handler{client: client, msgRepo: msgRepo, chRepo: chRepo}
}

func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/realtime/token", h.handleGetToken)
	mux.HandleFunc("GET /api/realtime/subscribe", h.handleSubscribeToken)
}

// GET /api/realtime/token
func (h *Handler) handleGetToken(w http.ResponseWriter, r *http.Request) {
	claims := auth.GetClaimsFromContext(r.Context())
	if claims == nil {
		writeError(w, http.StatusUnauthorized, "unauthorized", "UNAUTHORIZED")
		return
	}

	token, err := h.client.ConnectionToken(claims.UserID, 1*time.Hour)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal error", "INTERNAL")
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"token": token})
}

// GET /api/realtime/subscribe?channel=chat:uuid
// Проверяет права доступа перед выдачей токена подписки.
func (h *Handler) handleSubscribeToken(w http.ResponseWriter, r *http.Request) {
	claims := auth.GetClaimsFromContext(r.Context())
	if claims == nil {
		writeError(w, http.StatusUnauthorized, "unauthorized", "UNAUTHORIZED")
		return
	}

	channel := r.URL.Query().Get("channel")
	if channel == "" {
		writeError(w, http.StatusBadRequest, "channel query param required", "MISSING_CHANNEL")
		return
	}

	if !h.canSubscribe(r.Context(), claims.UserID, channel) {
		writeError(w, http.StatusForbidden, "access denied", "FORBIDDEN")
		return
	}

	token, err := h.client.SubscriptionToken(claims.UserID, channel, 1*time.Hour)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal error", "INTERNAL")
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"token": token})
}

// canSubscribe проверяет право пользователя подписаться на канал.
// Форматы каналов: user:<id>, chat:<conversationId>, channel:<roomId>
func (h *Handler) canSubscribe(ctx context.Context, userID, channel string) bool {
	parts := strings.SplitN(channel, ":", 2)
	if len(parts) != 2 || parts[1] == "" {
		return false
	}

	channelType, id := parts[0], parts[1]

	switch channelType {
	case "user":
		// Только на свой канал
		return id == userID
	case "chat":
		ok, err := h.msgRepo.IsMember(ctx, id, userID)
		return err == nil && ok
	case "channel":
		ok, err := h.chRepo.IsMember(ctx, id, userID)
		return err == nil && ok
	default:
		return false
	}
}

func writeJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

func writeError(w http.ResponseWriter, status int, message, code string) {
	writeJSON(w, status, map[string]string{"error": message, "code": code})
}
