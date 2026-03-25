package auth

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"strings"
)

// ────────────────────────────────────────────────────────────
// Handler
// ────────────────────────────────────────────────────────────
// Handler — слой, который принимает HTTP-запросы, парсит JSON,
// вызывает сервис и возвращает JSON-ответ.
// Он не содержит бизнес-логики — только «распаковал → вызвал → упаковал».

type Handler struct {
	service *Service
}

func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

// RegisterRoutes подключает все auth-эндпоинты к роутеру.
// Go 1.22+ поддерживает паттерн "METHOD /path" в ServeMux.
func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/auth/register", h.handleRegister)
	mux.HandleFunc("POST /api/auth/login", h.handleLogin)
	mux.HandleFunc("POST /api/auth/logout", h.handleLogout)
	mux.HandleFunc("GET /api/auth/me", h.handleMe)
	mux.HandleFunc("POST /api/auth/refresh", h.handleRefresh)
	mux.HandleFunc("PATCH /api/auth/me", h.handleUpdateProfile)
}

// ────────────────────────────────────────────────────────────
// Типы запросов и ответов
// ────────────────────────────────────────────────────────────

type registerRequest struct {
	Username    string `json:"username"`
	DisplayName string `json:"display_name"`
	Email       string `json:"email"`
	Password    string `json:"password"`
	DeviceName  string `json:"device_name"` // "iPhone 15", "Desktop Chrome"
	DeviceType  string `json:"device_type"` // "ios", "android", "windows"
}

type loginRequest struct {
	Username   string `json:"username"`
	Password   string `json:"password"`
	DeviceName string `json:"device_name"`
	DeviceType string `json:"device_type"`
}

type authResponse struct {
	User   *User      `json:"user"`
	Tokens *TokenPair `json:"tokens"`
}

type errorResponse struct {
	Error string `json:"error"`
	Code  string `json:"code,omitempty"`
}

type updateProfileRequest struct {
	DisplayName string `json:"display_name"`
	Bio         string `json:"bio"`
	AvatarURL   string `json:"avatar_url"` // URL из /api/media/avatar
}

type refreshRequest struct {
	RefreshToken string `json:"refresh_token"`
}

// ────────────────────────────────────────────────────────────
// POST /api/auth/register
// ────────────────────────────────────────────────────────────
// Создаёт аккаунт, возвращает профиль + токены.
//
// Пример запроса:
// curl -X POST http://localhost:8080/api/auth/register \
//   -H "Content-Type: application/json" \
//   -d '{"username":"alice","email":"alice@mail.com","password":"secret123"}'
//
// Ответ 201:
// {"user": {"id":"...","username":"alice",...}, "tokens": {"access_token":"...","refresh_token":"..."}}

func (h *Handler) handleRegister(w http.ResponseWriter, r *http.Request) {
	var req registerRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body", "INVALID_BODY")
		return
	}

	// ── Валидация ────────────────────────────
	if len(req.Username) < 3 || len(req.Username) > 32 {
		writeError(w, http.StatusBadRequest, "username must be 3-32 characters", "INVALID_USERNAME")
		return
	}
	if len(req.Password) < 8 {
		writeError(w, http.StatusBadRequest, "password must be at least 8 characters", "WEAK_PASSWORD")
		return
	}
	if req.Email == "" || !strings.Contains(req.Email, "@") {
		writeError(w, http.StatusBadRequest, "valid email required", "INVALID_EMAIL")
		return
	}

	// Дефолты для необязательных полей
	if req.DisplayName == "" {
		req.DisplayName = req.Username
	}
	if req.DeviceName == "" {
		req.DeviceName = "Unknown Device"
	}
	if req.DeviceType == "" {
		req.DeviceType = "unknown"
	}

	// ── Вызов сервиса ───────────────────────
	user, tokens, err := h.service.Register(
		r.Context(),
		req.Username, req.DisplayName, req.Email, req.Password,
		req.DeviceName, req.DeviceType, extractIP(r),
	)
	if err != nil {
		switch {
		case errors.Is(err, ErrUserExists):
			writeError(w, http.StatusConflict, "username already taken", "USER_EXISTS")
		case errors.Is(err, ErrEmailExists):
			writeError(w, http.StatusConflict, "email already registered", "EMAIL_EXISTS")
		default:
			log.Printf("register error: %v", err)
			writeError(w, http.StatusInternalServerError, "internal error", "INTERNAL")
		}
		return
	}

	writeJSON(w, http.StatusCreated, authResponse{User: user, Tokens: tokens})
}

// ────────────────────────────────────────────────────────────
// PATCH /api/auth/me
// ────────────────────────────────────────────────────────────
// Обновить профиль текущего пользователя.
// Обновляет только переданные непустые поля.
//
// Тело: {"display_name": "Новое имя", "bio": "Обо мне"}
// Ответ: обновлённый объект User
//
// Пример:
// curl -X PATCH http://localhost:8080/api/auth/me \
//   -H "Authorization: Bearer TOKEN" \
//   -H "Content-Type: application/json" \
//   -d '{"display_name": "Alice W.", "bio": "Flutter dev"}'

func (h *Handler) handleUpdateProfile(w http.ResponseWriter, r *http.Request) {
	claims := GetClaimsFromContext(r.Context())
	if claims == nil {
		writeError(w, http.StatusUnauthorized, "unauthorized", "UNAUTHORIZED")
		return
	}

	var req updateProfileRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body", "INVALID_BODY")
		return
	}

	// Валидация
	if req.DisplayName != "" && len(req.DisplayName) > 64 {
		writeError(w, http.StatusBadRequest, "display_name too long (max 64)", "INVALID_NAME")
		return
	}
	if len(req.Bio) > 500 {
		writeError(w, http.StatusBadRequest, "bio too long (max 500)", "INVALID_BIO")
		return
	}

	user, err := h.service.repo.UpdateProfile(r.Context(), claims.UserID, req.DisplayName, req.Bio, req.AvatarURL)
	if err != nil {
		if errors.Is(err, ErrUserNotFound) {
			writeError(w, http.StatusNotFound, "user not found", "NOT_FOUND")
			return
		}
		writeError(w, http.StatusInternalServerError, "internal error", "INTERNAL")
		return
	}

	writeJSON(w, http.StatusOK, user)
}

// ────────────────────────────────────────────────────────────
// POST /api/auth/login
// ────────────────────────────────────────────────────────────
// Проверяет логин/пароль, возвращает профиль + токены.
//
// Пример:
// curl -X POST http://localhost:8080/api/auth/login \
//   -H "Content-Type: application/json" \
//   -d '{"username":"alice","password":"secret123"}'

func (h *Handler) handleLogin(w http.ResponseWriter, r *http.Request) {
	var req loginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body", "INVALID_BODY")
		return
	}

	if req.Username == "" || req.Password == "" {
		writeError(w, http.StatusBadRequest, "username and password required", "MISSING_FIELDS")
		return
	}
	if req.DeviceName == "" {
		req.DeviceName = "Unknown Device"
	}
	if req.DeviceType == "" {
		req.DeviceType = "unknown"
	}

	user, tokens, err := h.service.Login(
		r.Context(),
		req.Username, req.Password,
		req.DeviceName, req.DeviceType, extractIP(r),
	)
	if err != nil {
		if errors.Is(err, ErrInvalidCredentials) {
			writeError(w, http.StatusUnauthorized, "invalid username or password", "INVALID_CREDENTIALS")
			return
		}
		writeError(w, http.StatusInternalServerError, "internal error", "INTERNAL")
		return
	}

	writeJSON(w, http.StatusOK, authResponse{User: user, Tokens: tokens})
}

// ────────────────────────────────────────────────────────────
// POST /api/auth/logout         (требует auth)
// ────────────────────────────────────────────────────────────
// Удаляет текущую сессию. Токен перестаёт работать.

func (h *Handler) handleLogout(w http.ResponseWriter, r *http.Request) {
	claims := GetClaimsFromContext(r.Context())
	if claims == nil {
		writeError(w, http.StatusUnauthorized, "unauthorized", "UNAUTHORIZED")
		return
	}

	if err := h.service.Logout(r.Context(), claims.SessionID, claims.UserID); err != nil {
		log.Printf("login error: %v", err)
		writeError(w, http.StatusInternalServerError, "internal error", "INTERNAL")
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// ────────────────────────────────────────────────────────────
// GET /api/auth/me              (требует auth)
// ────────────────────────────────────────────────────────────
// Возвращает профиль текущего пользователя.
//
// Пример:
// curl http://localhost:8080/api/auth/me \
//   -H "Authorization: Bearer <access_token>"

func (h *Handler) handleMe(w http.ResponseWriter, r *http.Request) {
	claims := GetClaimsFromContext(r.Context())
	if claims == nil {
		writeError(w, http.StatusUnauthorized, "unauthorized", "UNAUTHORIZED")
		return
	}

	user, err := h.service.GetProfile(r.Context(), claims.UserID)
	if err != nil {
		if errors.Is(err, ErrUserNotFound) {
			writeError(w, http.StatusNotFound, "user not found", "NOT_FOUND")
			return
		}
		writeError(w, http.StatusInternalServerError, "internal error", "INTERNAL")
		return
	}

	writeJSON(w, http.StatusOK, user)
}

// ────────────────────────────────────────────────────────────
// Хелперы
// ────────────────────────────────────────────────────────────

// ────────────────────────────────────────────────────────────
// POST /api/auth/refresh
// ────────────────────────────────────────────────────────────
// Обновляет пару токенов. Клиент шлёт refresh_token,
// получает новые access + refresh.
//
// Этот эндпоинт НЕ требует access token в заголовке
// (он может быть уже истёкшим — для того и refresh).
// Но требует валидный refresh_token в теле.
//
// Пример:
// curl -X POST http://localhost:8080/api/auth/refresh \
//   -H "Content-Type: application/json" \
//   -d '{"refresh_token": "abc123..."}'
//
// Ответ: {"access_token": "...", "refresh_token": "новый...", "expires_at": "..."}

func (h *Handler) handleRefresh(w http.ResponseWriter, r *http.Request) {
	var req refreshRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body", "INVALID_BODY")
		return
	}

	if req.RefreshToken == "" {
		writeError(w, http.StatusBadRequest, "refresh_token is required", "MISSING_TOKEN")
		return
	}

	tokens, err := h.service.RefreshTokens(r.Context(), req.RefreshToken)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "invalid or expired refresh token", "INVALID_REFRESH")
		return
	}

	writeJSON(w, http.StatusOK, tokens)
}

func writeJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

func writeError(w http.ResponseWriter, status int, message, code string) {
	writeJSON(w, status, errorResponse{Error: message, Code: code})
}

// extractIP достаёт IP-адрес клиента.
// Сначала проверяет proxy-заголовки (если сервер за nginx),
// потом fallback на прямой адрес.
func extractIP(r *http.Request) string {
	if ip := r.Header.Get("X-Forwarded-For"); ip != "" {
		return strings.Split(ip, ",")[0]
	}
	if ip := r.Header.Get("X-Real-IP"); ip != "" {
		return ip
	}
	// RemoteAddr = "127.0.0.1:54321" или "[::1]:54321"
	host := r.RemoteAddr
	if idx := strings.LastIndex(host, ":"); idx != -1 {
		host = host[:idx]
	}
	// Убираем квадратные скобки IPv6: [::1] → ::1
	host = strings.TrimPrefix(host, "[")
	host = strings.TrimSuffix(host, "]")
	if host == "" {
		host = "127.0.0.1"
	}
	return host
}
