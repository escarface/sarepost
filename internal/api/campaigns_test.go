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
	}
	if err := json.Unmarshal(w.Body.Bytes(), &out); err != nil {
		t.Fatalf("decode campaign: %v", err)
	}
	if out.BrandProfileID != profile.ID {
		t.Fatalf("expected brand profile id %q, got %q", profile.ID, out.BrandProfileID)
	}
}
