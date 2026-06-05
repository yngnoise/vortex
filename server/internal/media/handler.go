package media

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/yngnoise/vortex/internal/auth"
)

const (
	maxFileSize   = 50 << 20 // 50MB
	maxAvatarSize = 5 << 20  // 5MB
)

// Разрешённые MIME-типы для вложений. Тип определяется по magic bytes,
// не по заголовку Content-Type из запроса.
var allowedTypes = map[string]bool{
	"image/jpeg": true,
	"image/png":  true,
	"image/gif":  true,
	"image/webp": true,
	"video/mp4":  true,
	"video/webm": true,
	"audio/mpeg": true,
	"audio/ogg":  true,
	"audio/wav":  true,
	"application/pdf": true,
	"text/plain":      true,
}

var avatarTypes = map[string]bool{
	"image/jpeg": true,
	"image/png":  true,
	"image/webp": true,
}

// mimeToExt возвращает безопасное расширение файла по MIME-типу.
// Исключает использование расширения из имени файла, предоставленного клиентом.
var mimeToExt = map[string]string{
	"image/jpeg": ".jpg",
	"image/png":  ".png",
	"image/gif":  ".gif",
	"image/webp": ".webp",
	"video/mp4":  ".mp4",
	"video/webm": ".webm",
	"audio/mpeg": ".mp3",
	"audio/ogg":  ".ogg",
	"audio/wav":  ".wav",
	"application/pdf": ".pdf",
	"text/plain":      ".txt",
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

// POST /api/media/upload
func (h *Handler) handleUpload(w http.ResponseWriter, r *http.Request) {
	claims := auth.GetClaimsFromContext(r.Context())
	if claims == nil {
		writeError(w, http.StatusUnauthorized, "unauthorized", "UNAUTHORIZED")
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, maxFileSize)

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

	// Определяем реальный тип по magic bytes — Content-Type заголовок клиент может подделать
	detectedType, err := detectContentType(file)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to read file", "INTERNAL")
		return
	}

	if !allowedTypes[detectedType] {
		writeError(w, http.StatusBadRequest, "file type not allowed", "INVALID_TYPE")
		return
	}

	ext := mimeToExt[detectedType]
	now := time.Now()
	key := fmt.Sprintf("messages/%d/%02d/%s_%d%s",
		now.Year(), now.Month(),
		claims.UserID[:8], now.UnixMilli(), ext,
	)
	_ = header // имя файла от клиента не используем в пути

	info, err := h.storage.Upload(r.Context(), key, file, header.Size, detectedType)
	if err != nil {
		log.Printf("upload error: %v", err)
		writeError(w, http.StatusInternalServerError, "upload failed", "UPLOAD_FAILED")
		return
	}

	writeJSON(w, http.StatusOK, info)
}

// POST /api/media/avatar
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
	_ = header

	detectedType, err := detectContentType(file)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to read file", "INTERNAL")
		return
	}

	if !avatarTypes[detectedType] {
		writeError(w, http.StatusBadRequest, "only jpeg, png, webp allowed for avatars", "INVALID_TYPE")
		return
	}

	// Расширение из MIME-типа, не из имени файла — исключает загрузку .exe и т.п.
	ext := mimeToExt[detectedType]
	key := fmt.Sprintf("avatars/%s%s", claims.UserID, ext)

	info, err := h.storage.Upload(r.Context(), key, file, -1, detectedType)
	if err != nil {
		log.Printf("avatar upload error: %v", err)
		writeError(w, http.StatusInternalServerError, "upload failed", "UPLOAD_FAILED")
		return
	}

	writeJSON(w, http.StatusOK, info)
}

// detectContentType читает первые 512 байт файла для определения реального типа,
// затем сбрасывает позицию обратно в начало для последующей загрузки.
func detectContentType(file io.ReadSeeker) (string, error) {
	buf := make([]byte, 512)
	n, err := file.Read(buf)
	if err != nil && err != io.EOF {
		return "", err
	}

	if _, err := file.Seek(0, io.SeekStart); err != nil {
		return "", err
	}

	detected := http.DetectContentType(buf[:n])
	// Убираем параметры вроде "; charset=utf-8"
	return strings.Split(detected, ";")[0], nil
}

func writeJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

func writeError(w http.ResponseWriter, status int, message, code string) {
	writeJSON(w, status, map[string]string{"error": message, "code": code})
}
