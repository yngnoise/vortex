package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"

	// Замени на свой путь модуля, если отличается.
	// Посмотри первую строку в server/go.mod — там твой module path.
	"github.com/yngnoise/vortex/pkg/config"
)

// ────────────────────────────────────────────────────────────
// Ошибки сервисного слоя
// ────────────────────────────────────────────────────────────

var (
	ErrInvalidCredentials = errors.New("invalid username or password")
	ErrTokenExpired       = errors.New("token expired")
	ErrInvalidToken       = errors.New("invalid token")
)

// ────────────────────────────────────────────────────────────
// Типы
// ────────────────────────────────────────────────────────────

// TokenPair — пара токенов, которую получает клиент после логина.
//
// AccessToken  — короткоживущий JWT (15 мин). Клиент шлёт его
//
//	в заголовке Authorization: Bearer <token>.
//	Сервер проверяет подпись — если валидный, пропускает.
//
// RefreshToken — долгоживущий случайный токен (30 дней).
//
//	Когда access истекает, клиент шлёт refresh на
//	специальный эндпоинт и получает новую пару.
//	В базе хранится SHA-256 хеш, а не сам токен.
type TokenPair struct {
	AccessToken  string    `json:"access_token"`
	RefreshToken string    `json:"refresh_token"`
	ExpiresAt    time.Time `json:"expires_at"`
}

// Claims — содержимое JWT-токена (payload).
// UserID и SessionID зашиты внутрь токена, поэтому серверу
// не нужно лезть в базу при каждом запросе — достаточно
// проверить подпись.
type Claims struct {
	UserID    string `json:"uid"`
	SessionID string `json:"sid"`
	jwt.RegisteredClaims
}

// ────────────────────────────────────────────────────────────
// Service
// ────────────────────────────────────────────────────────────
// Сервис — слой бизнес-логики. Он знает КАК регистрировать
// и логинить пользователей, но не знает про HTTP.
// Получает данные от handler'а, обрабатывает, дёргает репозиторий.

type Service struct {
	repo *Repository
	cfg  config.JWTConfig
}

func NewService(repo *Repository, cfg config.JWTConfig) *Service {
	return &Service{repo: repo, cfg: cfg}
}

// ────────────────────────────────────────────────────────────
// Публичные методы
// ────────────────────────────────────────────────────────────

// Register создаёт нового пользователя.
//
// Шаги:
//  1. Хешируем пароль через bcrypt (cost=12, ~250мс — достаточно
//     медленно чтобы brute-force был нереален).
//  2. Сохраняем пользователя в базу.
//  3. Создаём сессию и генерируем пару токенов.
func (s *Service) Register(
	ctx context.Context,
	username, displayName, email, password string,
	deviceName, deviceType, ipAddress string,
) (*User, *TokenPair, error) {

	hash, err := bcrypt.GenerateFromPassword([]byte(password), 12)
	if err != nil {
		return nil, nil, fmt.Errorf("hash password: %w", err)
	}

	user, err := s.repo.CreateUser(ctx, username, displayName, email, hash)
	if err != nil {
		return nil, nil, err // ErrUserExists / ErrEmailExists пробросятся наверх
	}

	tokens, err := s.createSession(ctx, user.ID, deviceName, deviceType, ipAddress)
	if err != nil {
		return nil, nil, err
	}

	return user, tokens, nil
}

// Login проверяет учётные данные и возвращает пару токенов.
//
// Шаги:
// 1. Ищем пользователя по username.
// 2. Сравниваем пароль с хешем через bcrypt.
// 3. Если совпало — создаём сессию и токены.
//
// Важно: при неверном пароле возвращаем ту же ошибку,
// что и при несуществующем username — чтобы атакующий
// не мог узнать, существует ли аккаунт.
func (s *Service) Login(
	ctx context.Context,
	username, password string,
	deviceName, deviceType, ipAddress string,
) (*User, *TokenPair, error) {

	user, err := s.repo.GetUserByUsername(ctx, username)
	if err != nil {
		if errors.Is(err, ErrUserNotFound) {
			return nil, nil, ErrInvalidCredentials // не раскрываем что юзера нет
		}
		return nil, nil, err
	}

	// bcrypt.CompareHashAndPassword сам извлекает соль из хеша
	if err := bcrypt.CompareHashAndPassword(user.PasswordHash, []byte(password)); err != nil {
		return nil, nil, ErrInvalidCredentials
	}

	tokens, err := s.createSession(ctx, user.ID, deviceName, deviceType, ipAddress)
	if err != nil {
		return nil, nil, err
	}

	return user, tokens, nil
}

// Logout удаляет одну сессию (выход с конкретного устройства).
func (s *Service) Logout(ctx context.Context, sessionID, userID string) error {
	return s.repo.DeleteSession(ctx, sessionID, userID)
}

// LogoutAll удаляет все сессии пользователя (выход отовсюду).
func (s *Service) LogoutAll(ctx context.Context, userID string) error {
	return s.repo.DeleteAllUserSessions(ctx, userID)
}

// GetProfile возвращает профиль пользователя по ID.
func (s *Service) GetProfile(ctx context.Context, userID string) (*User, error) {
	return s.repo.GetUserByID(ctx, userID)
}

// ValidateAccessToken проверяет подпись и срок действия JWT.
// Возвращает Claims с UserID и SessionID если токен валиден.
func (s *Service) ValidateAccessToken(tokenStr string) (*Claims, error) {
	token, err := jwt.ParseWithClaims(tokenStr, &Claims{}, func(t *jwt.Token) (interface{}, error) {
		// Проверяем что алгоритм подписи — HMAC (а не RSA и т.д.)
		// Это защита от атаки "algorithm confusion"
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return []byte(s.cfg.Secret), nil
	})

	if err != nil {
		if errors.Is(err, jwt.ErrTokenExpired) {
			return nil, ErrTokenExpired
		}
		return nil, ErrInvalidToken
	}

	claims, ok := token.Claims.(*Claims)
	if !ok || !token.Valid {
		return nil, ErrInvalidToken
	}

	return claims, nil
}

// ────────────────────────────────────────────────────────────
// Приватные методы
// ────────────────────────────────────────────────────────────

// createSession генерирует пару токенов и сохраняет сессию в базу.
//
// Как работает refresh token:
//  1. Генерируем 32 случайных байта → кодируем в hex (64 символа).
//  2. Считаем SHA-256 хеш от этого токена.
//  3. В базу сохраняем ХЕШ, клиенту отдаём ТОКЕН.
//  4. При обновлении клиент пришлёт токен, мы снова посчитаем
//     хеш и сравним с тем что в базе.
//
// Зачем: если базу украдут, атакующий получит хеши,
// а не настоящие токены — использовать их невозможно.
func (s *Service) createSession(
	ctx context.Context,
	userID, deviceName, deviceType, ipAddress string,
) (*TokenPair, error) {

	// 1. Генерируем refresh token
	refreshBytes := make([]byte, 32)
	if _, err := rand.Read(refreshBytes); err != nil {
		return nil, fmt.Errorf("generate refresh token: %w", err)
	}
	refreshToken := hex.EncodeToString(refreshBytes)

	// 2. Хешируем для хранения в базе
	hash := sha256.Sum256([]byte(refreshToken))
	expiresAt := time.Now().Add(s.cfg.RefreshExpires)

	// 3. Сохраняем сессию
	session, err := s.repo.CreateSession(
		ctx, userID, deviceName, deviceType,
		hash[:], ipAddress, expiresAt,
	)
	if err != nil {
		return nil, fmt.Errorf("create session: %w", err)
	}

	// 4. Генерируем JWT access token
	accessExpiresAt := time.Now().Add(s.cfg.AccessExpires)
	claims := &Claims{
		UserID:    userID,
		SessionID: session.ID,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(accessExpiresAt),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			Issuer:    "vortex",
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	accessToken, err := token.SignedString([]byte(s.cfg.Secret))
	if err != nil {
		return nil, fmt.Errorf("sign access token: %w", err)
	}

	return &TokenPair{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		ExpiresAt:    accessExpiresAt,
	}, nil
}
