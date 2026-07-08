package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	generationapp "github.com/escarface/sarepost/internal/application/generation"
	"github.com/escarface/sarepost/internal/db"
	"github.com/escarface/sarepost/internal/domain"
)

func newContentSourceTestServer(t *testing.T) (Server, http.Handler) {
	t.Helper()
	tempDir := t.TempDir()
	store, err := db.Open(filepath.Join(tempDir, "test.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })
	srv := Server{Store: store, DataDir: tempDir, GenerationDriver: "mock"}
	return srv, srv.Handler()
}

func TestContentSourceHTTPCreateListGetArchive(t *testing.T) {
	_, h := newContentSourceTestServer(t)
	createW := postJSON(t, h, "/content-sources", map[string]any{
		"title": "Call notes", "body": "Customer wants faster onboarding.", "tags": []string{"sales"},
	})
	if createW.Code != http.StatusCreated {
		t.Fatalf("create status=%d body=%s", createW.Code, createW.Body.String())
	}
	var created domain.ContentSource
	if err := json.Unmarshal(createW.Body.Bytes(), &created); err != nil {
		t.Fatalf("decode created: %v", err)
	}
	if created.ID == "" || created.Status != domain.ContentSourceStatusNew {
		t.Fatalf("unexpected created source %#v", created)
	}

	listW := httptest.NewRecorder()
	h.ServeHTTP(listW, httptest.NewRequest(http.MethodGet, "/content-sources", nil))
	if listW.Code != http.StatusOK {
		t.Fatalf("list status=%d body=%s", listW.Code, listW.Body.String())
	}
	if !strings.Contains(listW.Body.String(), created.ID) {
		t.Fatalf("list did not include created source: %s", listW.Body.String())
	}

	getW := httptest.NewRecorder()
	h.ServeHTTP(getW, httptest.NewRequest(http.MethodGet, "/content-sources/"+created.ID, nil))
	if getW.Code != http.StatusOK {
		t.Fatalf("get status=%d body=%s", getW.Code, getW.Body.String())
	}

	archiveW := postJSON(t, h, "/content-sources/"+created.ID+"/archive", map[string]any{})
	if archiveW.Code != http.StatusOK {
		t.Fatalf("archive status=%d body=%s", archiveW.Code, archiveW.Body.String())
	}
	var archived domain.ContentSource
	if err := json.Unmarshal(archiveW.Body.Bytes(), &archived); err != nil {
		t.Fatalf("decode archived: %v", err)
	}
	if archived.Status != domain.ContentSourceStatusArchived {
		t.Fatalf("expected archived status, got %#v", archived)
	}

	defaultList := httptest.NewRecorder()
	h.ServeHTTP(defaultList, httptest.NewRequest(http.MethodGet, "/content-sources", nil))
	if strings.Contains(defaultList.Body.String(), created.ID) {
		t.Fatalf("default list should exclude archived source: %s", defaultList.Body.String())
	}
	includeArchived := httptest.NewRecorder()
	h.ServeHTTP(includeArchived, httptest.NewRequest(http.MethodGet, "/content-sources?include_archived=true", nil))
	if !strings.Contains(includeArchived.Body.String(), created.ID) {
		t.Fatalf("include_archived list should include source: %s", includeArchived.Body.String())
	}
}

func TestContentSourceHTTPRejectsInvalidCreate(t *testing.T) {
	_, h := newContentSourceTestServer(t)
	w := postJSON(t, h, "/content-sources", map[string]any{"title": "Only title"})
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d (%s)", w.Code, w.Body.String())
	}
}

func TestContentSourceHTTPGenerateAngles(t *testing.T) {
	srv, h := newContentSourceTestServer(t)
	if _, err := srv.generationService().SaveTextProviderConfig(t.Context(), generationapp.ProviderConfigUpdate{
		Provider: "anthropic", Model: "claude-opus-4-8", APIKey: "sk-test",
	}); err != nil {
		t.Fatalf("save provider: %v", err)
	}
	createW := postJSON(t, h, "/content-sources", map[string]any{
		"title": "Research", "body": "AI agents are changing content workflows.",
	})
	if createW.Code != http.StatusCreated {
		t.Fatalf("create status=%d body=%s", createW.Code, createW.Body.String())
	}
	var created domain.ContentSource
	if err := json.Unmarshal(createW.Body.Bytes(), &created); err != nil {
		t.Fatalf("decode created: %v", err)
	}
	body, _ := json.Marshal(map[string]any{"count": 3, "instructions": "LinkedIn angles"})
	req := httptest.NewRequest(http.MethodPost, "/content-sources/"+created.ID+"/generate-angles", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("generate status=%d body=%s", w.Code, w.Body.String())
	}
	var out struct {
		SourceID string `json:"source_id"`
		Angles   string `json:"angles"`
		Provider string `json:"provider"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &out); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if out.SourceID != created.ID || out.Angles == "" || out.Provider == "" {
		t.Fatalf("unexpected output %#v", out)
	}
}
