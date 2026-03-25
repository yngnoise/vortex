package middleware

import (
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/yngnoise/vortex/internal/auth"
)

// ────────────────────────────────────────────────────────────
// Auth middleware
// ────────────────────────────────────────────────────────────
// Перехватывает каждый запрос к защищённым эндпоинтам.
// Проверяет заголовок Authorization: Bearer <token>.
// Если токен валидный — кладёт Claims в context и пропускает.
// Если нет — возвращает 401.
//
// Паттерн middleware в Go: функция принимает http.Handler
// и возвращает новый http.Handler, который оборачивает оригинал.
// Как матрёшка: Logger(CORS(Auth(handler))).

func Auth(authService *auth.Service) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// 1. Достаём заголовок
			header := r.Header.Get("Authorization")
			if header == "" || !strings.HasPrefix(header, "Bearer ") {
				writeJSON(w, http.StatusUnauthorized,
					`{"error":"missing authorization header","code":"UNAUTHORIZED"}`)
				return
			}

			// 2. Извлекаем токен (убираем "Bearer ")
			tokenStr := strings.TrimPrefix(header, "Bearer ")

			// 3. Проверяем подпись и срок действия
			claims, err := authService.ValidateAccessToken(tokenStr)
			if err != nil {
				writeJSON(w, http.StatusUnauthorized,
					`{"error":"invalid or expired token","code":"INVALID_TOKEN"}`)
				return
			}

			// 4. Кладём claims в context и пропускаем дальше
			ctx := auth.SetClaimsToContext(r.Context(), claims)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// ────────────────────────────────────────────────────────────
// CORS middleware
// ────────────────────────────────────────────────────────────
// Cross-Origin Resource Sharing — без этого браузер и Flutter Web
// не смогут делать запросы к API (браузер блокирует cross-origin).
// В продакшне замени "*" на конкретный домен.

func CORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		w.Header().Set("Access-Control-Max-Age", "86400")

		// Preflight-запрос — браузер сначала шлёт OPTIONS,
		// чтобы узнать разрешён ли настоящий запрос.
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		next.ServeHTTP(w, r)
	})
}

// ────────────────────────────────────────────────────────────
// Logger middleware
// ────────────────────────────────────────────────────────────
// Логирует каждый запрос: метод, путь, статус-код, время выполнения.
// Пример вывода: POST /api/auth/login 200 12ms

func Logger(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		// Оборачиваем ResponseWriter чтобы перехватить статус-код
		wrapped := &responseWriter{ResponseWriter: w, statusCode: http.StatusOK}

		next.ServeHTTP(wrapped, r)

		log.Printf("%s %s %d %s",
			r.Method,
			r.URL.Path,
			wrapped.statusCode,
			time.Since(start).Round(time.Millisecond),
		)
	})
}

// responseWriter — обёртка, которая запоминает статус-код.
// Стандартный http.ResponseWriter не даёт узнать какой код
// был отправлен — приходится перехватывать.
type responseWriter struct {
	http.ResponseWriter
	statusCode int
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}

func writeJSON(w http.ResponseWriter, status int, body string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	w.Write([]byte(body))
}
