package auth

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// ────────────────────────────────────────────────────────────
// Ошибки
// ────────────────────────────────────────────────────────────
// Определяем свои ошибки, чтобы handler мог понять ЧТО пошло
// не так и вернуть правильный HTTP-код (409, 404 и т.д.),
// а не универсальный 500.

var (
	ErrUserNotFound   = errors.New("user not found")
	ErrUserExists     = errors.New("username already taken")
	ErrEmailExists    = errors.New("email already registered")
	ErrInvalidSession = errors.New("invalid session")
)

// ────────────────────────────────────────────────────────────
// Модели
// ────────────────────────────────────────────────────────────
// Структуры, которые соответствуют строкам в таблицах.
// json-теги определяют как поля будут выглядеть в JSON-ответах API.
// Тег `json:"-"` означает «никогда не отдавать клиенту».

type User struct {
	ID           string    `json:"id"`
	Username     string    `json:"username"`
	DisplayName  string    `json:"display_name"`
	Email        *string   `json:"email,omitempty"` // указатель, потому что может быть NULL
	Phone        *string   `json:"phone,omitempty"`
	PasswordHash []byte    `json:"-"` // никогда не отдаём наружу
	AvatarURL    *string   `json:"avatar_url,omitempty"`
	PublicKey    *string   `json:"public_key,omitempty"`
	Status       string    `json:"status"`
	Bio          string    `json:"bio"`
	LastSeenAt   time.Time `json:"last_seen_at"`
	CreatedAt    time.Time `json:"created_at"`
}

type Session struct {
	ID               string    `json:"id"`
	UserID           string    `json:"user_id"`
	DeviceName       string    `json:"device_name"`
	DeviceType       string    `json:"device_type"`
	RefreshTokenHash []byte    `json:"-"`
	IPAddress        *string   `json:"ip_address,omitempty"`
	CreatedAt        time.Time `json:"created_at"`
	ExpiresAt        time.Time `json:"expires_at"`
}

// ────────────────────────────────────────────────────────────
// Repository
// ────────────────────────────────────────────────────────────
// Репозиторий — единственное место, где живут SQL-запросы.
// Сервис (бизнес-логика) не знает про SQL, он вызывает
// методы репозитория и получает готовые структуры.

type Repository struct {
	db *pgxpool.Pool
}

func NewRepository(db *pgxpool.Pool) *Repository {
	return &Repository{db: db}
}

// CreateUser вставляет нового пользователя в таблицу users.
// Возвращает созданного пользователя с заполненным ID.
// Если username или email уже заняты — возвращает соответствующую ошибку.
func (r *Repository) CreateUser(ctx context.Context, username, displayName, email string, passwordHash []byte) (*User, error) {
	user := &User{}

	err := r.db.QueryRow(ctx, `
		INSERT INTO users (username, display_name, email, password_hash)
		VALUES ($1, $2, $3, $4)
		RETURNING id, username, display_name, email,
		          status, bio, last_seen_at, created_at
	`, username, displayName, email, passwordHash).Scan(
		&user.ID, &user.Username, &user.DisplayName, &user.Email,
		&user.Status, &user.Bio, &user.LastSeenAt, &user.CreatedAt,
	)

	if err != nil {
		// PostgreSQL возвращает ошибку с именем нарушенного constraint.
		// По имени определяем что именно задублировалось.
		if strings.Contains(err.Error(), "users_username_key") {
			return nil, ErrUserExists
		}
		if strings.Contains(err.Error(), "users_email_key") {
			return nil, ErrEmailExists
		}
		return nil, err
	}

	return user, nil
}

// GetUserByUsername ищет пользователя по username.
// Используется при логине — нужен password_hash для проверки пароля.
func (r *Repository) GetUserByUsername(ctx context.Context, username string) (*User, error) {
	user := &User{}

	err := r.db.QueryRow(ctx, `
		SELECT id, username, display_name, email, phone,
		       password_hash, avatar_url, public_key,
		       status, bio, last_seen_at, created_at
		FROM users
		WHERE username = $1 AND status = 'active'
	`, username).Scan(
		&user.ID, &user.Username, &user.DisplayName,
		&user.Email, &user.Phone, &user.PasswordHash,
		&user.AvatarURL, &user.PublicKey,
		&user.Status, &user.Bio, &user.LastSeenAt, &user.CreatedAt,
	)

	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrUserNotFound
	}
	return user, err
}

// GetUserByID ищет пользователя по ID.
// Используется для получения профиля (GET /api/auth/me).
// Не возвращает password_hash — он здесь не нужен.
func (r *Repository) GetUserByID(ctx context.Context, id string) (*User, error) {
	user := &User{}

	err := r.db.QueryRow(ctx, `
		SELECT id, username, display_name, email, phone,
		       avatar_url, public_key,
		       status, bio, last_seen_at, created_at
		FROM users
		WHERE id = $1
	`, id).Scan(
		&user.ID, &user.Username, &user.DisplayName,
		&user.Email, &user.Phone,
		&user.AvatarURL, &user.PublicKey,
		&user.Status, &user.Bio, &user.LastSeenAt, &user.CreatedAt,
	)

	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrUserNotFound
	}
	return user, err
}

// CreateSession сохраняет новую сессию устройства.
// Каждый логин с нового устройства = новая сессия.
// Хранится хеш refresh-токена, а не сам токен (безопасность).
func (r *Repository) CreateSession(
	ctx context.Context,
	userID, deviceName, deviceType string,
	refreshTokenHash []byte,
	ipAddress string,
	expiresAt time.Time,
) (*Session, error) {
	session := &Session{}

	err := r.db.QueryRow(ctx, `
		INSERT INTO sessions (user_id, device_name, device_type,
		                      refresh_token_hash, ip_address, expires_at)
		VALUES ($1, $2, $3, $4, $5::inet, $6)
		RETURNING id, user_id, device_name, device_type, created_at, expires_at
	`, userID, deviceName, deviceType, refreshTokenHash, ipAddress, expiresAt).Scan(
		&session.ID, &session.UserID, &session.DeviceName,
		&session.DeviceType, &session.CreatedAt, &session.ExpiresAt,
	)

	return session, err
}

// DeleteSession удаляет одну сессию (logout с одного устройства).
func (r *Repository) DeleteSession(ctx context.Context, sessionID, userID string) error {
	tag, err := r.db.Exec(ctx, `
		DELETE FROM sessions WHERE id = $1 AND user_id = $2
	`, sessionID, userID)

	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrInvalidSession
	}
	return nil
}

// DeleteAllUserSessions удаляет ВСЕ сессии пользователя (logout everywhere).
// Полезно если подозреваешь что аккаунт скомпрометирован.
func (r *Repository) DeleteAllUserSessions(ctx context.Context, userID string) error {
	_, err := r.db.Exec(ctx, `DELETE FROM sessions WHERE user_id = $1`, userID)
	return err
}

// UpdateLastSeen обновляет время последней активности.
// Вызывается из middleware при каждом запросе авторизованного пользователя.
func (r *Repository) UpdateLastSeen(ctx context.Context, userID string) error {
	_, err := r.db.Exec(ctx, `UPDATE users SET last_seen_at = NOW() WHERE id = $1`, userID)
	return err
}

// SearchUsers ищет пользователей по username или display_name.
// Использует ILIKE для case-insensitive поиска.
// Поддерживает частичное совпадение: "ali" найдёт "alice", "Alina".
// Исключает текущего пользователя из результатов.
func (r *Repository) SearchUsers(ctx context.Context, query string, currentUserID string, limit int) ([]User, error) {
	pattern := "%" + query + "%"

	rows, err := r.db.Query(ctx, `
		SELECT id, username, display_name, avatar_url,
		       public_key, status, bio, last_seen_at, created_at
		FROM users
		WHERE (username ILIKE $1 OR display_name ILIKE $1)
		  AND id != $2
		  AND status = 'active'
		ORDER BY
			CASE WHEN username ILIKE $3 THEN 0 ELSE 1 END,
			username
		LIMIT $4
	`, pattern, currentUserID, query+"%", limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var users []User
	for rows.Next() {
		var u User
		err := rows.Scan(
			&u.ID, &u.Username, &u.DisplayName, &u.AvatarURL,
			&u.PublicKey, &u.Status, &u.Bio, &u.LastSeenAt, &u.CreatedAt,
		)
		if err != nil {
			return nil, err
		}
		users = append(users, u)
	}
	return users, rows.Err()
}

// GetUserPublicProfile возвращает публичный профиль пользователя.
// Не содержит email и phone — они приватные.
func (r *Repository) GetUserPublicProfile(ctx context.Context, userID string) (*User, error) {
	user := &User{}
	err := r.db.QueryRow(ctx, `
		SELECT id, username, display_name, avatar_url,
		       public_key, status, bio, last_seen_at, created_at
		FROM users WHERE id = $1 AND status = 'active'
	`, userID).Scan(
		&user.ID, &user.Username, &user.DisplayName, &user.AvatarURL,
		&user.PublicKey, &user.Status, &user.Bio, &user.LastSeenAt, &user.CreatedAt,
	)
	if err != nil {
		return nil, ErrUserNotFound
	}
	return user, nil
}

// ──────────────────────────────────────────────────────
// ДОБАВЬ ЭТОТ МЕТОД В КОНЕЦ auth/repository.go
// ──────────────────────────────────────────────────────

// FindSessionByToken ищет сессию по хешу refresh-токена.
// Проверяет что сессия не истекла.
// Используется при обновлении токенов (POST /api/auth/refresh).
func (r *Repository) FindSessionByToken(ctx context.Context, tokenHash []byte) (*Session, error) {
	session := &Session{}
	err := r.db.QueryRow(ctx, `
		SELECT id, user_id, device_name, device_type,
		       refresh_token_hash, created_at, expires_at
		FROM sessions
		WHERE refresh_token_hash = $1
		  AND expires_at > NOW()
	`, tokenHash).Scan(
		&session.ID, &session.UserID, &session.DeviceName,
		&session.DeviceType, &session.RefreshTokenHash,
		&session.CreatedAt, &session.ExpiresAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrInvalidSession
	}
	return session, err
}

// UpdateSessionToken обновляет refresh-токен существующей сессии.
// Старый токен перестаёт работать (rotation).
// Это защита: если refresh-токен украли, при первом использовании
// оригинальным пользователем старый токен инвалидируется.
func (r *Repository) UpdateSessionToken(ctx context.Context, sessionID string, newTokenHash []byte, newExpiresAt time.Time) error {
	tag, err := r.db.Exec(ctx, `
		UPDATE sessions
		SET refresh_token_hash = $2, expires_at = $3
		WHERE id = $1
	`, sessionID, newTokenHash, newExpiresAt)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrInvalidSession
	}
	return nil
}

// UpdateProfile обновляет профиль пользователя.
// Обновляет только непустые поля — если display_name пустой,
// оставляет старое значение.
func (r *Repository) UpdateProfile(ctx context.Context, userID, displayName, bio, avatarURL string) (*User, error) {
	user := &User{}

	err := r.db.QueryRow(ctx, `
		UPDATE users SET
			display_name = CASE WHEN $2 = '' THEN display_name ELSE $2 END,
			bio = CASE WHEN $3 = '' THEN bio ELSE $3 END,
			avatar_url = CASE WHEN $4 = '' THEN avatar_url ELSE $4 END,
			updated_at = NOW()
		WHERE id = $1 AND status = 'active'
		RETURNING id, username, display_name, email, phone,
		          avatar_url, public_key, status, bio, last_seen_at, created_at
	`, userID, displayName, bio, avatarURL).Scan(
		&user.ID, &user.Username, &user.DisplayName,
		&user.Email, &user.Phone,
		&user.AvatarURL, &user.PublicKey,
		&user.Status, &user.Bio, &user.LastSeenAt, &user.CreatedAt,
	)
	if err != nil {
		return nil, ErrUserNotFound
	}
	return user, nil
}
