package db

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"github.com/escarface/sarepost/internal/domain"
)

func TestContentSourceStoreCRUDAndArchiveFiltering(t *testing.T) {
	store, err := Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer store.Close()
	ctx := context.Background()

	source, err := store.CreateContentSource(ctx, domain.ContentSource{
		Title:          "  Raw launch notes  ",
		Body:           "  Customer insight  ",
		SourceURL:      " https://example.com/notes ",
		CampaignID:     " cmp_123 ",
		BrandProfileID: " bp_123 ",
		Tags:           []string{"launch", " launch ", "", "customer"},
	})
	if err != nil {
		t.Fatalf("create source: %v", err)
	}
	if !strings.HasPrefix(source.ID, "src_") {
		t.Fatalf("expected src id, got %q", source.ID)
	}
	if source.Status != domain.ContentSourceStatusNew {
		t.Fatalf("expected default new status, got %q", source.Status)
	}
	if got := strings.Join(source.Tags, ","); got != "launch,customer" {
		t.Fatalf("unexpected tags %q", got)
	}
	if source.CreatedAt.IsZero() || source.UpdatedAt.IsZero() {
		t.Fatalf("expected timestamps, got %#v", source)
	}

	source.Title = "Updated"
	source.Status = domain.ContentSourceStatusProcessed
	source.Tags = []string{"updated"}
	updated, err := store.UpdateContentSource(ctx, source)
	if err != nil {
		t.Fatalf("update source: %v", err)
	}
	if updated.Title != "Updated" || updated.Status != domain.ContentSourceStatusProcessed {
		t.Fatalf("unexpected updated source %#v", updated)
	}

	listed, err := store.ListContentSources(ctx, domain.ContentSourceListFilter{})
	if err != nil {
		t.Fatalf("list sources: %v", err)
	}
	if len(listed) != 1 || listed[0].ID != source.ID {
		t.Fatalf("expected one listed source, got %#v", listed)
	}

	updated.Status = domain.ContentSourceStatusArchived
	if _, err := store.UpdateContentSource(ctx, updated); err != nil {
		t.Fatalf("archive update: %v", err)
	}
	listed, err = store.ListContentSources(ctx, domain.ContentSourceListFilter{})
	if err != nil {
		t.Fatalf("list default after archive: %v", err)
	}
	if len(listed) != 0 {
		t.Fatalf("expected archived source excluded, got %#v", listed)
	}
	listed, err = store.ListContentSources(ctx, domain.ContentSourceListFilter{IncludeArchived: true})
	if err != nil {
		t.Fatalf("list include archived: %v", err)
	}
	if len(listed) != 1 || listed[0].Status != domain.ContentSourceStatusArchived {
		t.Fatalf("expected archived source included, got %#v", listed)
	}
}

func TestContentSourceStoreMissingRows(t *testing.T) {
	store, err := Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer store.Close()

	if _, err := store.GetContentSource(context.Background(), "src_missing"); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("expected sql.ErrNoRows, got %v", err)
	}
}
