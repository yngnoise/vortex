package media

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"github.com/yngnoise/vortex/internal/auth"
)

// ────────────────────────────────────────────────────────────
// Handler
// ────────────────────────────────────────────────────────────
// Обрабатывает загрузку файлов через multipart/form-data.
//
// Ограничения:
//   - Максимальный размер: 50MB
//   - Разрешённые типы: изображения, видео, аудио, документы
//   - Аватарки: только изображения, максимум 5MB

const (
	maxFileSize   = 50 << 20 // 50MB
	maxAvatarSize = 5 << 20  // 5MB
)

// Разрешённые MIME-типы
var allowedTypes = map[string]bool{
	"image/jpeg":      true,
	"image/png":       true,
	"image/gif":       true,
	"image/webp":      true,
	"video/mp4":       true,
	"video/webm":      true,
	"audio/mpeg":      true,
	"audio/ogg":       true,
	"audio/wav":       true,
	"application/pdf": true,
	"application/zip": true,
	"text/plain":      true,
}

var avatarTypes = map[string]bool{
	"image/jpeg": true,
	"image/png":  true,
	"image/webp": true,
}

type Handler struct {
	storage *Storage
}

func NewHandler(storage *Storage) *Handler {
	return &Handler{storage: storage}
}

func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/media/upload", h.handleUpload)
	mux.HandleFunc("POST /api/media/avatar", h.handleAvatarUpload)
}

// ────────────────────────────────────────────────────────────
// POST /api/media/upload
// ────────────────────────────────────────────────────────────
// Загружает файл (фото, видео, документ) для вложения в сообщение.
//
// Запрос: multipart/form-data с полем "file"
// Ответ: {"key": "messages/2026/03/...", "url": "http://...", "size": 12345}
//
// Пример curl:
// curl -X POST http://localhost:8080/api/media/upload \
//   -H "Authorization: Bearer TOKEN" \
//   -F "file=@photo.jpg"

func (h *Handler) handleUpload(w http.ResponseWriter, r *http.Request) {
	claims := auth.GetClaimsFromContext(r.Context())
	if claims == nil {
		writeError(w, http.StatusUnauthorized, "unauthorized", "UNAUTHORIZED")
		return
	}

	// Ограничиваем размер тела запроса
	r.Body = http.MaxBytesReader(w, r.Body, maxFileSize)

	// Парсим multipart form
	if err := r.ParseMultipartForm(maxFileSize); err != nil {
		writeError(w, http.StatusBadRequest, "file too large (max 50MB)", "FILE_TOO_LARGE")
		return
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		writeError(w, http.StatusBadRequest, "file field 'file' is required", "MISSING_FILE")
		return
	}
	defer file.Close()

	// Проверяем MIME-тип
	contentType := header.Header.Get("Content-Type")
	if !allowedTypes[contentType] {
		writeError(w, http.StatusBadRequest, "file type not allowed", "INVALID_TYPE")
		return
	}

	// Генерируем путь: messages/{year}/{month}/{userID}_{timestamp}_{filename}
	now := time.Now()
	ext := filepath.Ext(header.Filename)
	safeName := sanitizeFilename(header.Filename)
	key := fmt.Sprintf("messages/%d/%02d/%s_%d%s",
		now.Year(), now.Month(),
		claims.UserID[:8], now.UnixMilli(), ext,
	)
	_ = safeName // используем ext от оригинала

	info, err := h.storage.Upload(r.Context(), key, file, header.Size, contentType)
	if err != nil {
		log.Printf("upload error: %v", err)
		writeError(w, http.StatusInternalServerError, "upload failed", "UPLOAD_FAILED")
		return
	}

	writeJSON(w, http.StatusOK, info)
}

// ────────────────────────────────────────────────────────────
// POST /api/media/avatar
// ────────────────────────────────────────────────────────────
// Загружает аватарку пользователя.
// Только изображения, максимум 5MB.
// Перезаписывает предыдущую аватарку.
//
// curl -X POST http://localhost:8080/api/media/avatar \
//   -H "Authorization: Bearer TOKEN" \
//   -F "file=@avatar.jpg"

func (h *Handler) handleAvatarUpload(w http.ResponseWriter, r *http.Request) {
	claims := auth.GetClaimsFromContext(r.Context())
	if claims == nil {
		writeError(w, http.StatusUnauthorized, "unauthorized", "UNAUTHORIZED")
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, maxAvatarSize)

	if err := r.ParseMultipartForm(maxAvatarSize); err != nil {
		writeError(w, http.StatusBadRequest, "file too large (max 5MB)", "FILE_TOO_LARGE")
		return
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		writeError(w, http.StatusBadRequest, "file field 'file' is required", "MISSING_FILE")
		return
	}
	defer file.Close()

	contentType := header.Header.Get("Content-Type")
	if !avatarTypes[contentType] {
		writeError(w, http.StatusBadRequest, "only jpeg, png, webp allowed for avatars", "INVALID_TYPE")
		return
	}

	// Аватарка всегда по одному пути — перезаписывается
	ext := filepath.Ext(header.Filename)
	key := fmt.Sprintf("avatars/%s%s", claims.UserID, ext)

	info, err := h.storage.Upload(r.Context(), key, file, header.Size, contentType)
	if err != nil {
		log.Printf("avatar upload error: %v", err)
		writeError(w, http.StatusInternalServerError, "upload failed", "UPLOAD_FAILED")
		return
	}

	// TODO: обновить avatar_url в таблице users

	writeJSON(w, http.StatusOK, info)
}

// ────────────────────────────────────────────────────────────
// Хелперы
// ────────────────────────────────────────────────────────────

func sanitizeFilename(name string) string {
	name = filepath.Base(name)
	name = strings.ReplaceAll(name, " ", "_")
	var clean strings.Builder
	for _, r := range name {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') ||
			(r >= '0' && r <= '9') || r == '_' || r == '-' || r == '.' {
			clean.WriteRune(r)
		}
	}
	result := clean.String()
	if result == "" {
		result = "file"
	}
	return result
}

func writeJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

func writeError(w http.ResponseWriter, status int, message, code string) {
	writeJSON(w, status, map[string]string{"error": message, "code": code})
}
