package middleware

import (
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/yngnoise/vortex/internal/auth"
)

// ── Auth ──────────────────────────────────────────────────────

func Auth(authService *auth.Service) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			header := r.Header.Get("Authorization")
			if header == "" || !strings.HasPrefix(header, "Bearer ") {
				writeJSON(w, http.StatusUnauthorized,
					`{"error":"missing authorization header","code":"UNAUTHORIZED"}`)
				return
			}

			tokenStr := strings.TrimPrefix(header, "Bearer ")

			claims, err := authService.ValidateAccessToken(tokenStr)
			if err != nil {
				writeJSON(w, http.StatusUnauthorized,
					`{"error":"invalid or expired token","code":"INVALID_TOKEN"}`)
				return
			}

			ctx := auth.SetClaimsToContext(r.Context(), claims)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// ── CORS ──────────────────────────────────────────────────────
// allowedOrigins: список разрешённых источников из CORS_ALLOWED_ORIGINS.
// Если список содержит "*" — разрешаются все источники (только для dev).
// В production передавайте конкретные домены: "https://app.example.com".

func CORS(allowedOrigins []string) func(http.Handler) http.Handler {
	allowAll := len(allowedOrigins) == 1 && allowedOrigins[0] == "*"

	allowed := make(map[string]bool, len(allowedOrigins))
	for _, o := range allowedOrigins {
		allowed[strings.TrimSpace(o)] = true
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			origin := r.Header.Get("Origin")

			if allowAll {
				w.Header().Set("Access-Control-Allow-Origin", "*")
			} else if origin != "" && allowed[origin] {
				w.Header().Set("Access-Control-Allow-Origin", origin)
				w.Header().Add("Vary", "Origin")
			}

			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
			w.Header().Set("Access-Control-Max-Age", "86400")

			if r.Method == http.MethodOptions {
				w.WriteHeader(http.StatusNoContent)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// ── Logger ────────────────────────────────────────────────────

func Logger(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

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
