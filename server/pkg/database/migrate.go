package database

import (
	"context"
	"fmt"
	"io/fs"
	"sort"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// RunMigrations применяет все ещё не применённые *.up.sql миграции из files
// по возрастанию номера в имени (формат NNN_name.up.sql). Применённые версии
// фиксируются в таблице schema_migrations, поэтому повторный запуск безопасен.
//
// Каждая миграция выполняется в отдельной транзакции через simple-протокол,
// потому что файлы содержат несколько выражений и тела функций ($$ ... $$).
func RunMigrations(ctx context.Context, pool *pgxpool.Pool, files fs.FS) error {
	if _, err := pool.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS schema_migrations (
			version    INT PRIMARY KEY,
			name       TEXT        NOT NULL,
			applied_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		)
	`); err != nil {
		return fmt.Errorf("ensure schema_migrations: %w", err)
	}

	applied, err := appliedVersions(ctx, pool)
	if err != nil {
		return err
	}

	entries, err := fs.Glob(files, "*.up.sql")
	if err != nil {
		return err
	}
	sort.Strings(entries)

	for _, name := range entries {
		version, err := parseVersion(name)
		if err != nil {
			return fmt.Errorf("bad migration name %q: %w", name, err)
		}
		if applied[version] {
			continue
		}

		sqlBytes, err := fs.ReadFile(files, name)
		if err != nil {
			return err
		}
		if err := applyMigration(ctx, pool, version, name, string(sqlBytes)); err != nil {
			return fmt.Errorf("apply migration %s: %w", name, err)
		}
		fmt.Printf("migrated: %s\n", name)
	}
	return nil
}

func appliedVersions(ctx context.Context, pool *pgxpool.Pool) (map[int]bool, error) {
	rows, err := pool.Query(ctx, `SELECT version FROM schema_migrations`)
	if err != nil {
		return nil, fmt.Errorf("load applied migrations: %w", err)
	}
	defer rows.Close()

	applied := map[int]bool{}
	for rows.Next() {
		var v int
		if err := rows.Scan(&v); err != nil {
			return nil, err
		}
		applied[v] = true
	}
	return applied, rows.Err()
}

func applyMigration(ctx context.Context, pool *pgxpool.Pool, version int, name, sqlText string) error {
	conn, err := pool.Acquire(ctx)
	if err != nil {
		return err
	}
	defer conn.Release()

	tx, err := conn.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	if _, err := tx.Exec(ctx, sqlText, pgx.QueryExecModeSimpleProtocol); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx,
		`INSERT INTO schema_migrations (version, name) VALUES ($1, $2)`,
		version, name,
	); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// parseVersion извлекает числовой префикс из имени файла ("003_channels.up.sql" -> 3).
func parseVersion(name string) (int, error) {
	base := name
	if i := strings.IndexByte(base, '_'); i > 0 {
		base = base[:i]
	}
	return strconv.Atoi(base)
}
