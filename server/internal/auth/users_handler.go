package auth

import (
	"errors"
	"log"
	"net/http"
)

// ────────────────────────────────────────────────────────────
// UsersHandler
// ────────────────────────────────────────────────────────────
// Отдельный handler для пользовательских эндпоинтов:
// поиск, просмотр профиля. Использует тот же репозиторий
// что и auth, но предоставляет другие маршруты.

type UsersHandler struct {
	repo *Repository
}

func NewUsersHandler(repo *Repository) *UsersHandler {
	return &UsersHandler{repo: repo}
}

func (h *UsersHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/users/search", h.handleSearch)
	mux.HandleFunc("GET /api/users/{id}", h.handleGetProfile)
}

// ────────────────────────────────────────────────────────────
// GET /api/users/search?q=alice&limit=20
// ────────────────────────────────────────────────────────────
// Поиск пользователей по username или display_name.
// Case-insensitive, частичное совпадение.
//
// Пример: /api/users/search?q=ali → найдёт alice, alina, ali123
//
// Результаты отсортированы: сначала те, у кого username
// начинается с запроса, потом остальные.
// Текущий пользователь исключён из результатов.

func (h *UsersHandler) handleSearch(w http.ResponseWriter, r *http.Request) {
	claims := GetClaimsFromContext(r.Context())
	if claims == nil {
		writeError(w, http.StatusUnauthorized, "unauthorized", "UNAUTHORIZED")
		return
	}

	query := r.URL.Query().Get("q")
	if query == "" {
		writeError(w, http.StatusBadRequest, "query parameter 'q' is required", "MISSING_QUERY")
		return
	}
	if len(query) < 2 {
		writeError(w, http.StatusBadRequest, "query must be at least 2 characters", "SHORT_QUERY")
		return
	}

	limit := 20
	if l := r.URL.Query().Get("limit"); l != "" {
		if v := parseInt(l); v > 0 && v <= 50 {
			limit = v
		}
	}

	users, err := h.repo.SearchUsers(r.Context(), query, claims.UserID, limit)
	if err != nil {
		log.Printf("search users error: %v", err)
		writeError(w, http.StatusInternalServerError, "internal error", "INTERNAL")
		return
	}

	if users == nil {
		users = []User{}
	}

	writeJSON(w, http.StatusOK, users)
}

// ────────────────────────────────────────────────────────────
// GET /api/users/{id}
// ────────────────────────────────────────────────────────────
// Публичный профиль пользователя.
// Возвращает username, display_name, avatar, bio, last_seen.
// НЕ возвращает email и phone — они приватные.

func (h *UsersHandler) handleGetProfile(w http.ResponseWriter, r *http.Request) {
	claims := GetClaimsFromContext(r.Context())
	if claims == nil {
		writeError(w, http.StatusUnauthorized, "unauthorized", "UNAUTHORIZED")
		return
	}

	userID := r.PathValue("id")
	if userID == "" {
		writeError(w, http.StatusBadRequest, "user id required", "MISSING_ID")
		return
	}

	user, err := h.repo.GetUserPublicProfile(r.Context(), userID)
	if err != nil {
		if errors.Is(err, ErrUserNotFound) {
			writeError(w, http.StatusNotFound, "user not found", "NOT_FOUND")
			return
		}
		log.Printf("get user profile error: %v", err)
		writeError(w, http.StatusInternalServerError, "internal error", "INTERNAL")
		return
	}

	writeJSON(w, http.StatusOK, user)
}

// parseInt — простой парсер строки в int.
func parseInt(s string) int {
	v := 0
	for _, c := range s {
		if c >= '0' && c <= '9' {
			v = v*10 + int(c-'0')
		} else {
			return 0
		}
	}
	return v
}
