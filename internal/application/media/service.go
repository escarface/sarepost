package media

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"

	"github.com/escarface/sarepost/internal/db"
	"github.com/escarface/sarepost/internal/domain"
)

const (
	DefaultListLimit = 200
	MaxListLimit     = 500
)

var (
	ErrMediaIDRequired = errors.New("media id is required")
)

type Store interface {
	ListMedia(ctx context.Context, limit int) ([]db.MediaWithUsage, error)
	ListMediaFiltered(ctx context.Context, filter domain.MediaListFilter) ([]db.MediaWithUsage, error)
	DeleteMediaIfUnused(ctx context.Context, mediaID string) (domain.Media, error)
	DeleteMediaUnusedByPendingPosts(ctx context.Context) ([]domain.Media, error)
}

type GeneratedStore interface {
	CreateMedia(ctx context.Context, media domain.Media) (domain.Media, error)
}

type RemoveFileFunc func(path string) error

type Service struct {
	Store          Store
	GeneratedStore GeneratedStore
	DataDir        string
	RemoveFile     RemoveFileFunc
}

func (s Service) PersistGeneratedMedia(ctx context.Context, data []byte, mimeType string, tags []string) (domain.Media, error) {
	if len(data) == 0 {
		return domain.Media{}, errors.New("generated image is empty")
	}
	if s.GeneratedStore == nil {
		return domain.Media{}, errors.New("generated media store is not configured")
	}
	mediaID, err := db.NewID("med")
	if err != nil {
		return domain.Media{}, err
	}
	storageDir := filepath.Join(s.DataDir, "media")
	if err := os.MkdirAll(storageDir, 0o755); err != nil {
		return domain.Media{}, err
	}
	ext := ".png"
	if strings.EqualFold(strings.TrimSpace(mimeType), "image/jpeg") {
		ext = ".jpg"
	}
	storagePath := filepath.Join(storageDir, mediaID+"_generated"+ext)
	if err := os.WriteFile(storagePath, data, 0o644); err != nil {
		return domain.Media{}, err
	}
	media, err := s.GeneratedStore.CreateMedia(ctx, domain.Media{
		ID: mediaID, Kind: "image", OriginalName: "generated" + ext,
		StoragePath: storagePath, MimeType: mimeType, SizeBytes: int64(len(data)), Tags: append([]string(nil), tags...),
	})
	if err != nil {
		_ = os.Remove(storagePath)
		return domain.Media{}, err
	}
	return media, nil
}

func ClampListLimit(limit int) int {
	if limit <= 0 {
		return DefaultListLimit
	}
	if limit > MaxListLimit {
		return MaxListLimit
	}
	return limit
}

func (s Service) List(ctx context.Context, limit int) ([]db.MediaWithUsage, error) {
	return s.Store.ListMedia(ctx, ClampListLimit(limit))
}

func (s Service) ListFiltered(ctx context.Context, filter domain.MediaListFilter) ([]db.MediaWithUsage, error) {
	filter.Limit = ClampListLimit(filter.Limit)
	filter.Tag = strings.TrimSpace(filter.Tag)
	if filter.Tag == "" {
		return s.List(ctx, filter.Limit)
	}
	return s.Store.ListMediaFiltered(ctx, filter)
}

func (s Service) Delete(ctx context.Context, mediaID string) (domain.Media, error) {
	mediaID = strings.TrimSpace(mediaID)
	if mediaID == "" {
		return domain.Media{}, ErrMediaIDRequired
	}

	deleted, err := s.Store.DeleteMediaIfUnused(ctx, mediaID)
	if err != nil {
		return domain.Media{}, err
	}

	s.removeFile(strings.TrimSpace(deleted.StoragePath))
	return deleted, nil
}

func (s Service) PurgeUnusedByPendingPosts(ctx context.Context) ([]domain.Media, error) {
	deleted, err := s.Store.DeleteMediaUnusedByPendingPosts(ctx)
	if err != nil {
		return nil, err
	}
	for _, item := range deleted {
		s.removeFile(strings.TrimSpace(item.StoragePath))
	}
	return deleted, nil
}

func (s Service) removeFile(path string) {
	if path == "" {
		return
	}
	remove := s.RemoveFile
	if remove == nil {
		remove = os.Remove
	}
	err := remove(path)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
	}
}
