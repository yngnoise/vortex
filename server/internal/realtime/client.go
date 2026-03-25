package realtime

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// ────────────────────────────────────────────────────────────
// Client
// ────────────────────────────────────────────────────────────
// Клиент для взаимодействия с Centrifugo Server API.
//
// Как работает real-time в Vortex:
//
// 1. Пользователь подключается к Centrifugo по WebSocket
//    (ws://localhost:8001/connection/websocket) с JWT-токеном.
//
// 2. Centrifugo проверяет токен и подписывает пользователя
//    на его персональный канал (user:UUID).
//
// 3. Когда кто-то отправляет сообщение через Go API,
//    Go публикует событие в Centrifugo через HTTP API.
//
// 4. Centrifugo мгновенно доставляет событие всем подписчикам
//    через WebSocket — без polling, без задержек.
//
// Пользователь подписывается на:
//   - "user:<userID>"           — личные уведомления, новые чаты
//   - "chat:<conversationID>"   — сообщения в конкретном чате
//   - "channel:<roomID>"        — сообщения в комнате канала

type Client struct {
	apiURL     string
	apiKey     string
	hmacSecret string
}

func NewClient(apiURL, apiKey, hmacSecret string) *Client {
	return &Client{
		apiURL:     apiURL,
		apiKey:     apiKey,
		hmacSecret: hmacSecret,
	}
}

// ────────────────────────────────────────────────────────────
// Генерация токенов
// ────────────────────────────────────────────────────────────

// ConnectionToken генерирует JWT для подключения к Centrifugo.
// sub — ID пользователя.
func (c *Client) ConnectionToken(userID string, ttl time.Duration) (string, error) {
	claims := jwt.MapClaims{
		"sub": userID,
		"exp": time.Now().Add(ttl).Unix(),
		"iat": time.Now().Unix(),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(c.hmacSecret))
}

// SubscriptionToken генерирует JWT для подписки на конкретный канал.
func (c *Client) SubscriptionToken(userID, channel string, ttl time.Duration) (string, error) {
	claims := jwt.MapClaims{
		"sub":     userID,
		"channel": channel,
		"exp":     time.Now().Add(ttl).Unix(),
		"iat":     time.Now().Unix(),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(c.hmacSecret))
}

// ────────────────────────────────────────────────────────────
// Публикация событий
// ────────────────────────────────────────────────────────────

// Publish отправляет событие в канал Centrifugo.
// Все подписчики канала мгновенно получат это событие.
//
// channel — имя канала Centrifugo:
//   - "chat:<conversationID>" — для личных/групповых чатов
//   - "channel:<roomID>"      — для комнат в каналах
//   - "user:<userID>"         — для персональных уведомлений
//
// event — любые данные (Message, уведомление и т.д.)
func (c *Client) Publish(channel string, event interface{}) error {
	payload, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("marshal event: %w", err)
	}

	body := map[string]interface{}{
		"method": "publish",
		"params": map[string]interface{}{
			"channel": channel,
			"data":    json.RawMessage(payload),
		},
	}

	return c.apiCall(body)
}

// PublishToUsers отправляет событие нескольким пользователям.
func (c *Client) PublishToUsers(userIDs []string, event interface{}) error {
	for _, userID := range userIDs {
		if err := c.Publish("user:"+userID, event); err != nil {
			return err
		}
	}
	return nil
}

// ────────────────────────────────────────────────────────────
// HTTP-вызов к Centrifugo API
// ────────────────────────────────────────────────────────────

func (c *Client) apiCall(body interface{}) error {
	jsonBody, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("marshal body: %w", err)
	}

	req, err := http.NewRequest("POST", c.apiURL, bytes.NewReader(jsonBody))
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "apikey "+c.apiKey)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("centrifugo request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("centrifugo returned status %d", resp.StatusCode)
	}

	return nil
}
