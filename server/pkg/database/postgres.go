package database

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// NewPool создаёт пул соединений к PostgreSQL.
//
// Пул — это набор заранее открытых коннектов к базе.
// Когда сервис хочет сделать запрос, он берёт свободный коннект
// из пула, использует и возвращает обратно. Это намного быстрее,
// чем открывать новое соединение на каждый HTTP-запрос.
//
// Настройки пула:
//   - MaxConns=20:  максимум 20 одновременных соединений
//   - MinConns=5:   держать минимум 5 «горячих» соединений
//   - MaxConnLifetime=1h:  пересоздавать коннект каждый час
//   - MaxConnIdleTime=30m: закрывать простаивающий коннект через 30 мин
//
// Для 50K пользователей 20 коннектов — достаточно на старте.
// PostgreSQL по умолчанию разрешает 100 соединений,
// так что запас есть.
func NewPool(databaseURL string) (*pgxpool.Pool, error) {
	config, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		return nil, fmt.Errorf("parse database URL: %w", err)
	}

	config.MaxConns = 20
	config.MinConns = 5
	config.MaxConnLifetime = 1 * time.Hour
	config.MaxConnIdleTime = 30 * time.Minute

	// Даём 10 секунд на подключение — если база не ответила, падаем
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		return nil, fmt.Errorf("create pool: %w", err)
	}

	// Проверяем что база реально доступна
	if err := pool.Ping(ctx); err != nil {
		return nil, fmt.Errorf("ping database: %w", err)
	}

	return pool, nil
}
