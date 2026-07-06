package config

import (
	"fmt"
	"os"
	"sort"
	"strings"
	"time"
)

// Значения по умолчанию для dev-окружения. В production запуск с ними запрещён
// (см. Validate) — секреты обязаны приходить из окружения.
const (
	devJWTSecret        = "dev_secret_change_in_production"
	devCentrifugoKey    = "vortex-centrifugo-api-key-change-in-production"
	devCentrifugoSecret = "vortex-centrifugo-secret-change-in-production"
	devDBURL            = "postgres://vortex:vortex_dev_2024@127.0.0.1:5433/vortex?sslmode=disable"
	devMinIOSecret      = "vortex_minio_dev"
)

type Config struct {
	Env        string
	Server     ServerConfig
	Database   DatabaseConfig
	Redis      RedisConfig
	JWT        JWTConfig
	Centrifugo CentrifugoConfig
	MinIO      MinIOConfig
}

type ServerConfig struct {
	Port           string
	AllowedOrigins []string
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
	APIURL     string
	APIKey     string
	HMACSecret string
}

type MinIOConfig struct {
	Endpoint  string
	AccessKey string
	SecretKey string
	Bucket    string
	UseSSL    bool
	PublicURL string // базовый URL, по которому клиенты качают файлы
}

func Load() *Config {
	return &Config{
		Env: getEnv("APP_ENV", "development"),
		Server: ServerConfig{
			Port:           getEnv("PORT", "8080"),
			AllowedOrigins: strings.Split(getEnv("CORS_ALLOWED_ORIGINS", "*"), ","),
		},
		Database: DatabaseConfig{
			URL: getEnv("DATABASE_URL", devDBURL),
		},
		Redis: RedisConfig{
			URL: getEnv("REDIS_URL", "redis://default:vortex_redis_dev@127.0.0.1:6379"),
		},
		JWT: JWTConfig{
			Secret:         getEnv("JWT_SECRET", devJWTSecret),
			AccessExpires:  15 * time.Minute,
			RefreshExpires: 30 * 24 * time.Hour,
		},
		Centrifugo: CentrifugoConfig{
			APIURL:     getEnv("CENTRIFUGO_API_URL", "http://localhost:8001/api"),
			APIKey:     getEnv("CENTRIFUGO_API_KEY", devCentrifugoKey),
			HMACSecret: getEnv("CENTRIFUGO_SECRET", devCentrifugoSecret),
		},
		MinIO: MinIOConfig{
			Endpoint:  getEnv("MINIO_ENDPOINT", "127.0.0.1:9000"),
			AccessKey: getEnv("MINIO_ACCESS_KEY", "vortex_minio"),
			SecretKey: getEnv("MINIO_SECRET_KEY", devMinIOSecret),
			Bucket:    getEnv("MINIO_BUCKET", "vortex-media"),
			UseSSL:    getEnvBool("MINIO_USE_SSL", false),
			PublicURL: getEnv("MEDIA_PUBLIC_URL", "http://localhost:9000"),
		},
	}
}

// Validate запрещает старт в production с дефолтными dev-секретами.
// В dev-режиме всегда возвращает nil.
func (c *Config) Validate() error {
	if !strings.EqualFold(c.Env, "production") {
		return nil
	}
	usesDefault := map[string]bool{
		"JWT_SECRET":         c.JWT.Secret == devJWTSecret,
		"CENTRIFUGO_API_KEY": c.Centrifugo.APIKey == devCentrifugoKey,
		"CENTRIFUGO_SECRET":  c.Centrifugo.HMACSecret == devCentrifugoSecret,
		"DATABASE_URL":       c.Database.URL == devDBURL,
		"MINIO_SECRET_KEY":   c.MinIO.SecretKey == devMinIOSecret,
	}
	var bad []string
	for name, isDefault := range usesDefault {
		if isDefault {
			bad = append(bad, name)
		}
	}
	if len(bad) > 0 {
		sort.Strings(bad)
		return fmt.Errorf("refusing to start in production with default secrets: %s", strings.Join(bad, ", "))
	}
	return nil
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func getEnvBool(key string, fallback bool) bool {
	if v := os.Getenv(key); v != "" {
		return v == "1" || strings.EqualFold(v, "true")
	}
	return fallback
}
