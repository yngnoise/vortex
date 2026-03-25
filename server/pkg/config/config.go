package config

import (
	"os"
	"time"
)

type Config struct {
	Server     ServerConfig
	Database   DatabaseConfig
	Redis      RedisConfig
	JWT        JWTConfig
	Centrifugo CentrifugoConfig
}

type ServerConfig struct {
	Port string
}

type DatabaseConfig struct {
	URL string
}

type RedisConfig struct {
	URL string
}

type JWTConfig struct {
	Secret         string
	AccessExpires  time.Duration
	RefreshExpires time.Duration
}

type CentrifugoConfig struct {
	APIURL     string // URL для Server API (Go → Centrifugo)
	APIKey     string // ключ авторизации для Server API
	HMACSecret string // секрет для подписи клиентских JWT
}

func Load() *Config {
	return &Config{
		Server: ServerConfig{
			Port: getEnv("PORT", "8080"),
		},
		Database: DatabaseConfig{
			URL: getEnv("DATABASE_URL",
				"postgres://vortex:vortex_dev_2024@127.0.0.1:5433/vortex?sslmode=disable"),
		},
		Redis: RedisConfig{
			URL: getEnv("REDIS_URL",
				"redis://default:vortex_redis_dev@127.0.0.1:6379"),
		},
		JWT: JWTConfig{
			Secret:         getEnv("JWT_SECRET", "dev_secret_change_in_production"),
			AccessExpires:  15 * time.Minute,
			RefreshExpires: 30 * 24 * time.Hour,
		},
		Centrifugo: CentrifugoConfig{
			APIURL:     getEnv("CENTRIFUGO_API_URL", "http://localhost:8001/api"),
			APIKey:     getEnv("CENTRIFUGO_API_KEY", "vortex-centrifugo-api-key-change-in-production"),
			HMACSecret: getEnv("CENTRIFUGO_SECRET", "vortex-centrifugo-secret-change-in-production"),
		},
	}
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
