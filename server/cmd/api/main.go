package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/yngnoise/vortex/internal/auth"
	"github.com/yngnoise/vortex/internal/channels"
	"github.com/yngnoise/vortex/internal/media"
	"github.com/yngnoise/vortex/internal/messaging"
	"github.com/yngnoise/vortex/internal/middleware"
	"github.com/yngnoise/vortex/internal/realtime"
	"github.com/yngnoise/vortex/pkg/config"
	"github.com/yngnoise/vortex/pkg/database"
)

func main() {
	// ── 1. Конфиг ────────────────────────────
	cfg := config.Load()

	// ── 2. PostgreSQL ────────────────────────
	db, err := database.NewPool(cfg.Database.URL)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer db.Close()
	log.Println("Connected to PostgreSQL")

	// ── 3. Centrifugo (real-time) ────────────
	rtClient := realtime.NewClient(
		cfg.Centrifugo.APIURL,
		cfg.Centrifugo.APIKey,
		cfg.Centrifugo.HMACSecret,
	)
	rtHandler := realtime.NewHandler(rtClient)

	// ── Media (файловое хранилище) ───────────
	storage, err := media.NewStorage(
		cfg.MinIO.Endpoint, cfg.MinIO.AccessKey,
		cfg.MinIO.SecretKey, cfg.MinIO.Bucket, cfg.MinIO.UseSSL,
	)
	if err != nil {
		log.Fatalf("Failed to connect to MinIO: %v", err)
	}
	log.Println("Connected to MinIO")
	mediaHandler := media.NewHandler(storage)

	// ── 4. Auth модуль ───────────────────────
	authRepo := auth.NewRepository(db)
	authService := auth.NewService(authRepo, cfg.JWT)
	authHandler := auth.NewHandler(authService)
	usersHandler := auth.NewUsersHandler(authRepo)

	// ── 5. Messaging модуль ──────────────────
	msgRepo := messaging.NewRepository(db)
	msgService := messaging.NewService(msgRepo, rtClient)
	msgHandler := messaging.NewHandler(msgService)

	// ── 6. Channels модуль ───────────────────
	chRepo := channels.NewRepository(db)
	chService := channels.NewService(chRepo, rtClient)
	chHandler := channels.NewHandler(chService)

	// ── 7. Роутер ────────────────────────────
	mux := http.NewServeMux()

	mux.HandleFunc("GET /api/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"status":"ok","service":"vortex"}`))
	})

	authHandler.RegisterRoutes(mux)
	msgHandler.RegisterRoutes(mux)
	chHandler.RegisterRoutes(mux)
	rtHandler.RegisterRoutes(mux)
	usersHandler.RegisterRoutes(mux)
	mediaHandler.RegisterRoutes(mux)

	// ── 8. Middleware ─────────────────────────
	authMW := middleware.Auth(authService)

	handler := middleware.Logger(
		middleware.CORS(
			authRouter(mux, authMW),
		),
	)

	// ── 9. HTTP-сервер ───────────────────────
	server := &http.Server{
		Addr:         ":" + cfg.Server.Port,
		Handler:      handler,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	go func() {
		log.Printf("Vortex API starting on http://localhost:%s", cfg.Server.Port)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Server error: %v", err)
		}
	}()

	// ── 10. Graceful shutdown ────────────────
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("Shutting down server...")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		log.Fatalf("Server forced to shutdown: %v", err)
	}
	log.Println("Server stopped")
}

// authRouter — решает какие пути публичные, а какие требуют JWT.
func authRouter(mux *http.ServeMux, authMW func(http.Handler) http.Handler) http.Handler {
	publicPaths := map[string]bool{
		"POST /api/auth/register": true,
		"POST /api/auth/login":    true,
		"GET /api/health":         true,
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		routeKey := r.Method + " " + r.URL.Path

		if publicPaths[routeKey] {
			mux.ServeHTTP(w, r)
			return
		}

		authMW(mux).ServeHTTP(w, r)
	})
}
