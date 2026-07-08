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

func TestContentSourcesViewRendersInboxAndActions(t *testing.T) {
	tempDir := t.TempDir()
	store, err := db.Open(filepath.Join(tempDir, "content-sources-ui.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer store.Close()

	source, err := store.CreateContentSource(t.Context(), domain.ContentSource{
		Title:     "Launch research",
		Body:      "Audience objections and proof points for the launch.",
		SourceURL: "https://example.com/research",
		Tags:      []string{"launch", "research"},
	})
	if err != nil {
		t.Fatalf("create content source: %v", err)
	}

	srv := Server{Store: store, DataDir: tempDir}
	req := httptest.NewRequest(http.MethodGet, "/?view=sources", nil)
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", w.Code, w.Body.String())
	}

	body := w.Body.String()
	for _, want := range []string{
		"CONTENT SOURCES",
		`id="content-source-form"`,
		"Launch research",
		"Audience objections and proof points",
		`data-content-source-generate="` + source.ID + `"`,
		`data-content-source-archive="` + source.ID + `"`,
		"/content-sources/",
		"generate-angles",
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("expected sources view to contain %q, body=%s", want, body)
		}
	}
}
