package media

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/escarface/sarepost/internal/db"
	"github.com/escarface/sarepost/internal/domain"
)

type generatedMediaStore struct{ created domain.Media }

func (s *generatedMediaStore) CreateMedia(_ context.Context, media domain.Media) (domain.Media, error) {
	s.created = media
	return media, nil
}

func TestPersistGeneratedMediaWritesFileAndMetadata(t *testing.T) {
	store := &generatedMediaStore{}
	service := Service{GeneratedStore: store, DataDir: t.TempDir()}
	media, err := service.PersistGeneratedMedia(t.Context(), []byte("png-data"), "image/png", []string{"content-plan", "plan_1"})
	if err != nil {
		t.Fatalf("persist generated media: %v", err)
	}
	if media.ID == "" || media.Tags[0] != "content-plan" || store.created.StoragePath == "" {
		t.Fatalf("unexpected media: %#v", media)
	}
	if _, err := os.Stat(media.StoragePath); err != nil {
		t.Fatalf("expected generated file: %v", err)
	}
}

type fakeStore struct {
	lastListLimit int
	lastFilter    domain.MediaListFilter
	listItems     []db.MediaWithUsage
	deleteInput   string
	deleteOutput  domain.Media
	deleteErr     error
	purgeOutput   []domain.Media
	purgeErr      error
}

func (f *fakeStore) ListMedia(_ context.Context, limit int) ([]db.MediaWithUsage, error) {
	f.lastListLimit = limit
	return f.listItems, nil
}

func (f *fakeStore) ListMediaFiltered(_ context.Context, filter domain.MediaListFilter) ([]db.MediaWithUsage, error) {
	f.lastFilter = filter
	return f.listItems, nil
}

func (f *fakeStore) DeleteMediaIfUnused(_ context.Context, mediaID string) (domain.Media, error) {
	f.deleteInput = mediaID
	if f.deleteErr != nil {
		return domain.Media{}, f.deleteErr
	}
	return f.deleteOutput, nil
}

func (f *fakeStore) DeleteMediaUnusedByPendingPosts(_ context.Context) ([]domain.Media, error) {
	if f.purgeErr != nil {
		return nil, f.purgeErr
	}
	return f.purgeOutput, nil
}

func TestClampListLimit(t *testing.T) {
	if got := ClampListLimit(0); got != DefaultListLimit {
		t.Fatalf("expected default limit, got %d", got)
	}
	if got := ClampListLimit(-10); got != DefaultListLimit {
		t.Fatalf("expected default limit for negative value, got %d", got)
	}
	if got := ClampListLimit(MaxListLimit + 10); got != MaxListLimit {
		t.Fatalf("expected max limit cap, got %d", got)
	}
	if got := ClampListLimit(123); got != 123 {
		t.Fatalf("expected explicit limit to be preserved, got %d", got)
	}
}

func TestListUsesClampedLimit(t *testing.T) {
	store := &fakeStore{}
	svc := Service{Store: store}

	if _, err := svc.List(t.Context(), MaxListLimit+1000); err != nil {
		t.Fatalf("list failed: %v", err)
	}
	if store.lastListLimit != MaxListLimit {
		t.Fatalf("expected clamped limit %d, got %d", MaxListLimit, store.lastListLimit)
	}
}

func TestListFilteredUsesTagAndClampedLimit(t *testing.T) {
	store := &fakeStore{}
	svc := Service{Store: store}

	if _, err := svc.ListFiltered(t.Context(), domain.MediaListFilter{Tag: " launch ", Limit: MaxListLimit + 10}); err != nil {
		t.Fatalf("list filtered failed: %v", err)
	}
	if store.lastFilter.Tag != "launch" || store.lastFilter.Limit != MaxListLimit {
		t.Fatalf("unexpected filter: %#v", store.lastFilter)
	}
}

func TestDeleteValidatesInput(t *testing.T) {
	svc := Service{Store: &fakeStore{}}
	if _, err := svc.Delete(t.Context(), "   "); !errors.Is(err, ErrMediaIDRequired) {
		t.Fatalf("expected ErrMediaIDRequired, got %v", err)
	}
}

func TestDeleteRemovesFileWhenPresent(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "clip.mp4")
	if err := os.WriteFile(path, []byte("test"), 0o644); err != nil {
		t.Fatalf("write temp file: %v", err)
	}

	store := &fakeStore{
		deleteOutput: domain.Media{
			ID:          "med_1",
			StoragePath: path,
		},
	}
	svc := Service{Store: store}

	deleted, err := svc.Delete(t.Context(), "med_1")
	if err != nil {
		t.Fatalf("delete failed: %v", err)
	}
	if deleted.ID != "med_1" {
		t.Fatalf("unexpected deleted media: %+v", deleted)
	}
	if _, statErr := os.Stat(path); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("expected file to be removed, stat err=%v", statErr)
	}
}

func TestDeleteIgnoresMissingFile(t *testing.T) {
	store := &fakeStore{
		deleteOutput: domain.Media{
			ID:          "med_2",
			StoragePath: filepath.Join(t.TempDir(), "missing.png"),
		},
	}
	svc := Service{Store: store}

	if _, err := svc.Delete(t.Context(), "med_2"); err != nil {
		t.Fatalf("delete failed when file missing: %v", err)
	}
}

func TestPurgeUnusedByPendingPostsRemovesFiles(t *testing.T) {
	tmp := t.TempDir()
	keptPath := filepath.Join(tmp, "kept-missing.mp4")
	purgePath := filepath.Join(tmp, "purge.mp4")
	if err := os.WriteFile(purgePath, []byte("to-purge"), 0o644); err != nil {
		t.Fatalf("write purge file: %v", err)
	}

	store := &fakeStore{
		purgeOutput: []domain.Media{
			{ID: "med_1", StoragePath: purgePath},
			{ID: "med_2", StoragePath: keptPath},
		},
	}
	svc := Service{Store: store}

	deleted, err := svc.PurgeUnusedByPendingPosts(t.Context())
	if err != nil {
		t.Fatalf("purge failed: %v", err)
	}
	if len(deleted) != 2 {
		t.Fatalf("expected 2 purged media records, got %d", len(deleted))
	}
	if _, statErr := os.Stat(purgePath); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("expected purge file to be removed, stat err=%v", statErr)
	}
}
