package realtime

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/yngnoise/vortex/internal/auth"
)

type Handler struct {
	client *Client
}

func NewHandler(client *Client) *Handler {
	return &Handler{client: client}
}

func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/realtime/token", h.handleGetToken)
	mux.HandleFunc("GET /api/realtime/subscribe", h.handleSubscribeToken)
}

// GET /api/realtime/token
// Возвращает JWT для подключения к Centrifugo WebSocket.
func (h *Handler) handleGetToken(w http.ResponseWriter, r *http.Request) {
	claims := auth.GetClaimsFromContext(r.Context())
	if claims == nil {
		writeError(w, http.StatusUnauthorized, "unauthorized", "UNAUTHORIZED")
		return
	}

	token, err := h.client.ConnectionToken(claims.UserID, 12*time.Hour)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal error", "INTERNAL")
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{
		"token": token,
	})
}

// GET /api/realtime/subscribe?channel=chat:uuid
// Возвращает JWT для подписки на конкретный канал Centrifugo.
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

	token, err := h.client.SubscriptionToken(claims.UserID, channel, 12*time.Hour)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal error", "INTERNAL")
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{
		"token": token,
	})
}

func writeJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

func writeError(w http.ResponseWriter, status int, message, code string) {
	writeJSON(w, status, map[string]string{"error": message, "code": code})
}
