package media

import (
	"context"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

// ────────────────────────────────────────────────────────────
// Storage
// ────────────────────────────────────────────────────────────
// Обёртка над MinIO (S3-совместимое хранилище).
// Все файлы (аватарки, фото, видео, документы) хранятся здесь.
//
// Структура хранения:
//   vortex-media/           ← бакет
//     avatars/              ← аватарки пользователей и каналов
//       {userID}.jpg
//     messages/             ← вложения в сообщениях
//       {year}/{month}/{messageID}/{filename}
//     channels/             ← файлы в комнатах каналов
//       {year}/{month}/{messageID}/{filename}

type Storage struct {
	client     *minio.Client
	bucketName string
	publicURL  string
}

// FileInfo — метаданные загруженного файла.
type FileInfo struct {
	Key         string `json:"key"`          // путь в хранилище
	URL         string `json:"url"`          // URL для скачивания
	Size        int64  `json:"size"`         // размер в байтах
	ContentType string `json:"content_type"` // MIME-тип
}

// NewStorage создаёт клиент MinIO и инициализирует бакет.
// Если бакет не существует — создаёт его.
func NewStorage(endpoint, accessKey, secretKey, bucketName string, useSSL bool, publicURL string) (*Storage, error) {
	client, err := minio.New(endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(accessKey, secretKey, ""),
		Secure: useSSL,
	})
	if err != nil {
		return nil, fmt.Errorf("create minio client: %w", err)
	}

	s := &Storage{
		client:     client,
		bucketName: bucketName,
		publicURL:  publicURL,
	}

	// Создаём бакет если не существует
	if err := s.ensureBucket(context.Background()); err != nil {
		return nil, err
	}

	return s, nil
}

// Upload загружает файл в хранилище.
//
// key — путь внутри бакета (например "messages/2026/03/uuid/photo.jpg").
// reader — поток данных файла.
// size — размер файла в байтах (-1 если неизвестен).
// contentType — MIME-тип ("image/jpeg", "application/pdf" и т.д.).
func (s *Storage) Upload(ctx context.Context, key string, reader io.Reader, size int64, contentType string) (*FileInfo, error) {
	_, err := s.client.PutObject(ctx, s.bucketName, key, reader, size, minio.PutObjectOptions{
		ContentType: contentType,
	})
	if err != nil {
		return nil, fmt.Errorf("upload file: %w", err)
	}

	return &FileInfo{
		Key:         key,
		URL:         s.GetPublicURL(key),
		Size:        size,
		ContentType: contentType,
	}, nil
}

// GetPresignedURL генерирует временную ссылку на файл.
// Ссылка работает указанное время, потом истекает.
// Используется для приватных файлов — не хранишь файлы публично,
// а выдаёшь временную ссылку авторизованным пользователям.
func (s *Storage) GetPresignedURL(ctx context.Context, key string, ttl time.Duration) (string, error) {
	url, err := s.client.PresignedGetObject(ctx, s.bucketName, key, ttl, nil)
	if err != nil {
		return "", fmt.Errorf("presign url: %w", err)
	}
	return url.String(), nil
}

// GetPublicURL возвращает прямой URL к файлу.
// Работает только если бакет настроен как публичный.
// Для dev-окружения используем это, для prod — presigned URLs.
func (s *Storage) GetPublicURL(key string) string {
	return fmt.Sprintf("%s/%s/%s", strings.TrimRight(s.publicURL, "/"), s.bucketName, key)
}

// Stat возвращает публичный URL, размер и MIME-тип объекта.
// Используется messaging при привязке вложений к сообщениям:
// размер и тип берутся из хранилища, а не из данных клиента.
// Если объект не существует — возвращает ошибку.
func (s *Storage) Stat(ctx context.Context, key string) (url string, size int64, contentType string, err error) {
	info, err := s.client.StatObject(ctx, s.bucketName, key, minio.StatObjectOptions{})
	if err != nil {
		return "", 0, "", fmt.Errorf("stat object: %w", err)
	}
	return s.GetPublicURL(key), info.Size, info.ContentType, nil
}

// Delete удаляет файл из хранилища.
func (s *Storage) Delete(ctx context.Context, key string) error {
	return s.client.RemoveObject(ctx, s.bucketName, key, minio.RemoveObjectOptions{})
}

// ensureBucket создаёт бакет если его нет.
// Также устанавливает публичную политику для dev-окружения.
func (s *Storage) ensureBucket(ctx context.Context) error {
	exists, err := s.client.BucketExists(ctx, s.bucketName)
	if err != nil {
		return fmt.Errorf("check bucket: %w", err)
	}
	if exists {
		return nil
	}

	if err := s.client.MakeBucket(ctx, s.bucketName, minio.MakeBucketOptions{}); err != nil {
		return fmt.Errorf("create bucket: %w", err)
	}

	// Делаем бакет публичным для dev (в prod убрать)
	policy := fmt.Sprintf(`{
		"Version": "2012-10-17",
		"Statement": [{
			"Effect": "Allow",
			"Principal": {"AWS": ["*"]},
			"Action": ["s3:GetObject"],
			"Resource": ["arn:aws:s3:::%s/*"]
		}]
	}`, s.bucketName)

	if err := s.client.SetBucketPolicy(ctx, s.bucketName, policy); err != nil {
		return fmt.Errorf("set bucket policy: %w", err)
	}

	return nil
}
