package api

import (
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	generationapp "github.com/escarface/sarepost/internal/application/generation"
	"github.com/escarface/sarepost/internal/db"
)

func TestGenerateViewIncludesContentPlanBuilderAndReviewWorkspace(t *testing.T) {
	tempDir := t.TempDir()
	store, err := db.Open(filepath.Join(tempDir, "content-plan-ui.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer store.Close()
	srv := Server{Store: store, DataDir: tempDir}
	account := createTestAccount(t, store)
	profile, err := srv.generationService().SaveBrandProfile(t.Context(), generationapp.BrandProfileUpdate{Name: "Sare Digital"})
	if err != nil {
		t.Fatalf("profile: %v", err)
	}
	req := httptest.NewRequest(http.MethodGet, "/?view=generate", nil)
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
	body := w.Body.String()
	for _, expected := range []string{"content-plan-mode", "content-plan-form", "content-plan-block-template", "content-plan-workspace", profile.ID, account.ID} {
		if !strings.Contains(body, expected) {
			t.Fatalf("expected %q in generate view", expected)
		}
	}
}
