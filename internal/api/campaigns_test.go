package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	generationapp "github.com/escarface/sarepost/internal/application/generation"
	"github.com/escarface/sarepost/internal/db"
	"github.com/escarface/sarepost/internal/domain"
	"github.com/escarface/sarepost/internal/genai"
)

func TestCreateCampaignResolvesBrandProfileName(t *testing.T) {
	tempDir := t.TempDir()
	store, err := db.Open(filepath.Join(tempDir, "test.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer store.Close()

	srv := Server{Store: store, DataDir: tempDir, DefaultMaxRetries: 3}
	profile, err := srv.generationService().SaveBrandProfile(t.Context(), generationapp.BrandProfileUpdate{Name: "Sare Digital", Tone: "direct"})
	if err != nil {
		t.Fatalf("save brand profile: %v", err)
	}

	payload, _ := json.Marshal(map[string]any{
		"name":          "Q3 launch",
		"brand_profile": "sare digital",
		"visual_style":  "technical-minimal",
		"image_prompt":  "Warm white background, process blueprint",
		"image_size":    "1024x1024",
	})
	req := httptest.NewRequest(http.MethodPost, "/campaigns", bytes.NewReader(payload))
	req.Header.Set("content-type", "application/json")
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d body=%s", w.Code, w.Body.String())
	}
	var out struct {
		ID             string `json:"id"`
		BrandProfileID string `json:"brand_profile_id"`
		VisualStyle    string `json:"visual_style"`
		ImagePrompt    string `json:"image_prompt"`
		ImageSize      string `json:"image_size"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &out); err != nil {
		t.Fatalf("decode campaign: %v", err)
	}
	if out.BrandProfileID != profile.ID {
		t.Fatalf("expected brand profile id %q, got %q", profile.ID, out.BrandProfileID)
	}
	if out.VisualStyle != "technical-minimal" || out.ImagePrompt != "Warm white background, process blueprint" || out.ImageSize != "1024x1024" {
		t.Fatalf("expected visual campaign fields to roundtrip, got %#v", out)
	}
}

func TestCreateCampaignCalendarDraftsReturnsPlannedDrafts(t *testing.T) {
	tempDir := t.TempDir()
	store, err := db.Open(filepath.Join(tempDir, "test.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer store.Close()

	accountID := testAccountID(t, store)
	srv := Server{Store: store, DataDir: tempDir, DefaultMaxRetries: 3, GenerationDriver: string(genai.DriverMock)}
	if _, err := srv.generationService().SaveTextProviderConfig(t.Context(), generationapp.ProviderConfigUpdate{
		Provider: genai.ProviderAnthropic,
		Model:    "mock-text",
	}); err != nil {
		t.Fatalf("save provider: %v", err)
	}
	campaign, err := store.CreateCampaign(t.Context(), domain.Campaign{
		Name:   "Launch week",
		Status: domain.CampaignStatusActive,
	})
	if err != nil {
		t.Fatalf("create campaign: %v", err)
	}

	payload, _ := json.Marshal(map[string]any{
		"account_id": accountID,
		"idea":       "One week of launch education posts",
		"from":       "2026-07-06T09:00:00+02:00",
		"days":       1,
		"slots":      []string{"09:00", "17:00"},
	})
	req := httptest.NewRequest(http.MethodPost, "/campaigns/"+campaign.ID+"/calendar-drafts", bytes.NewReader(payload))
	req.Header.Set("content-type", "application/json")
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d body=%s", w.Code, w.Body.String())
	}

	var out struct {
		CreatedCount int `json:"created_count"`
		Items        []struct {
			PlannedAt string `json:"planned_at"`
			Post      struct {
				ID              string `json:"id"`
				CampaignID      string `json:"campaign_id"`
				Status          string `json:"status"`
				EditorialStatus string `json:"editorial_status"`
			} `json:"post"`
		} `json:"items"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &out); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if out.CreatedCount != 2 || len(out.Items) != 2 {
		t.Fatalf("expected 2 created items, got count=%d len=%d", out.CreatedCount, len(out.Items))
	}
	expectedSlots := []string{"2026-07-06T09:00:00+02:00", "2026-07-06T17:00:00+02:00"}
	for i, expected := range expectedSlots {
		item := out.Items[i]
		if item.PlannedAt != expected {
			t.Fatalf("item %d expected planned_at %q, got %q", i, expected, item.PlannedAt)
		}
		if item.Post.CampaignID != campaign.ID {
			t.Fatalf("item %d expected campaign id %q, got %q", i, campaign.ID, item.Post.CampaignID)
		}
		if item.Post.Status != string(domain.PostStatusDraft) {
			t.Fatalf("item %d expected draft status, got %q", i, item.Post.Status)
		}
		if item.Post.EditorialStatus != string(domain.EditorialStatusNeedsReview) {
			t.Fatalf("item %d expected needs_review editorial status, got %q", i, item.Post.EditorialStatus)
		}
		post, err := store.GetPost(t.Context(), item.Post.ID)
		if err != nil {
			t.Fatalf("get post: %v", err)
		}
		if !post.ScheduledAt.IsZero() {
			t.Fatalf("expected generated calendar draft to remain unscheduled, got %s", post.ScheduledAt)
		}
	}
}

func TestCreateCampaignCalendarDraftsCanGenerateImages(t *testing.T) {
	tempDir := t.TempDir()
	store, err := db.Open(filepath.Join(tempDir, "test.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer store.Close()

	accountID := testAccountID(t, store)
	srv := Server{Store: store, DataDir: tempDir, DefaultMaxRetries: 3, GenerationDriver: string(genai.DriverMock)}
	if _, err := srv.generationService().SaveTextProviderConfig(t.Context(), generationapp.ProviderConfigUpdate{
		Provider: genai.ProviderAnthropic,
		Model:    "mock-text",
	}); err != nil {
		t.Fatalf("save text provider: %v", err)
	}
	if _, err := srv.generationService().SaveImageProviderConfig(t.Context(), generationapp.ProviderConfigUpdate{
		Provider: genai.ProviderOpenAI,
		Model:    "gpt-image-1",
	}); err != nil {
		t.Fatalf("save image provider: %v", err)
	}
	campaign, err := store.CreateCampaign(t.Context(), domain.Campaign{
		Name:   "Launch week",
		Status: domain.CampaignStatusActive,
	})
	if err != nil {
		t.Fatalf("create campaign: %v", err)
	}

	payload, _ := json.Marshal(map[string]any{
		"account_id":      accountID,
		"idea":            "One visual launch post",
		"from":            "2026-07-06T09:00:00+02:00",
		"days":            1,
		"slots":           []string{"09:00"},
		"generate_images": true,
	})
	req := httptest.NewRequest(http.MethodPost, "/campaigns/"+campaign.ID+"/calendar-drafts", bytes.NewReader(payload))
	req.Header.Set("content-type", "application/json")
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d body=%s", w.Code, w.Body.String())
	}

	mediaItems, err := store.ListMedia(t.Context(), 10)
	if err != nil {
		t.Fatalf("list media: %v", err)
	}
	if len(mediaItems) != 1 {
		t.Fatalf("expected one generated media item, got %d", len(mediaItems))
	}
}
