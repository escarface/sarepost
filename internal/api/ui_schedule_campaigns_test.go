package api

import (
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/escarface/sarepost/internal/db"
	"github.com/escarface/sarepost/internal/domain"
)

func TestBacklogViewRendersEditorialBacklogItems(t *testing.T) {
	tempDir := t.TempDir()
	store, err := db.Open(filepath.Join(tempDir, "test.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer store.Close()

	accountID := testAccountID(t, store)
	campaign, err := store.CreateCampaign(t.Context(), domain.Campaign{
		Name:   "Weekend backlog",
		Status: domain.CampaignStatusActive,
	})
	if err != nil {
		t.Fatalf("create campaign: %v", err)
	}
	post, err := store.CreatePost(t.Context(), db.CreatePostParams{Post: domain.Post{
		AccountID: accountID,
		Text:      "Weekend backlog copy",
		Status:    domain.PostStatusDraft,
	}})
	if err != nil {
		t.Fatalf("create post: %v", err)
	}
	if err := store.AddPostToCampaign(t.Context(), post.Post.ID, campaign.ID, domain.EditorialStatusNeedsReview, true, []string{"weekend", "review"}); err != nil {
		t.Fatalf("add post to campaign: %v", err)
	}

	srv := Server{Store: store, DataDir: tempDir, DefaultMaxRetries: 3}
	req := httptest.NewRequest(http.MethodGet, "/?view=backlog", nil)
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", w.Code, w.Body.String())
	}

	body := w.Body.String()
	for _, want := range []string{"Weekend backlog", "needs_review", "approval required"} {
		if !strings.Contains(body, want) {
			t.Fatalf("expected backlog view to contain %q, body=%s", want, body)
		}
	}
	if strings.Contains(body, "No editorial backlog items match the current filters.") {
		t.Fatalf("expected backlog view to render items, body=%s", body)
	}
}

func TestCampaignsViewHidesArchivedCampaigns(t *testing.T) {
	tempDir := t.TempDir()
	store, err := db.Open(filepath.Join(tempDir, "test.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer store.Close()

	if _, err := store.CreateCampaign(t.Context(), domain.Campaign{
		Name:   "Visible campaign",
		Status: domain.CampaignStatusActive,
	}); err != nil {
		t.Fatalf("create active campaign: %v", err)
	}
	if _, err := store.CreateCampaign(t.Context(), domain.Campaign{
		Name:   "Archived campaign",
		Status: domain.CampaignStatusArchived,
	}); err != nil {
		t.Fatalf("create archived campaign: %v", err)
	}

	srv := Server{Store: store, DataDir: tempDir, DefaultMaxRetries: 3}
	req := httptest.NewRequest(http.MethodGet, "/?view=campaigns", nil)
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", w.Code, w.Body.String())
	}

	body := w.Body.String()
	if !strings.Contains(body, "Visible campaign") {
		t.Fatalf("expected active campaign to be visible, body=%s", body)
	}
	if strings.Contains(body, "Archived campaign") {
		t.Fatalf("expected archived campaign to be hidden, body=%s", body)
	}
}
