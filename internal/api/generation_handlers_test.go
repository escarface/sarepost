package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"strings"
	"testing"

	"golang.org/x/crypto/bcrypt"

	"github.com/escarface/sarepost/internal/db"
)

func newGenerationTestServer(t *testing.T) (Server, http.Handler) {
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

func postJSON(t *testing.T, h http.Handler, path string, body any) *httptest.ResponseRecorder {
	t.Helper()
	payload, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, path, bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	return w
}

func TestGenerateText_RequiresProvider(t *testing.T) {
	_, h := newGenerationTestServer(t)
	w := postJSON(t, h, "/generate/text", map[string]any{"prompt": "hi"})
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d (%s)", w.Code, w.Body.String())
	}
}

func TestGenerateText_MockDriver(t *testing.T) {
	_, h := newGenerationTestServer(t)

	saveW := postJSON(t, h, "/settings/generation/text", map[string]any{
		"provider": "anthropic", "model": "claude-opus-4-8", "api_key": "sk-test",
	})
	if saveW.Code != http.StatusOK {
		t.Fatalf("save provider: %d (%s)", saveW.Code, saveW.Body.String())
	}

	w := postJSON(t, h, "/generate/text", map[string]any{
		"prompt": "Announce launch", "platform": "instagram",
	})
	if w.Code != http.StatusOK {
		t.Fatalf("generate text: %d (%s)", w.Code, w.Body.String())
	}
	var out struct {
		Text          string `json:"text"`
		Provider      string `json:"provider"`
		UsedWebSearch bool   `json:"used_web_search"`
		Sources       []struct {
			Title string `json:"title"`
			URL   string `json:"url"`
		} `json:"sources"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &out); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if !strings.Contains(out.Text, "Announce launch") {
		t.Errorf("unexpected text %q", out.Text)
	}
	if out.UsedWebSearch {
		t.Errorf("expected used_web_search=false, got true")
	}
}

func TestGenerateText_ExplicitWebSearch(t *testing.T) {
	_, h := newGenerationTestServer(t)

	saveW := postJSON(t, h, "/settings/generation/text", map[string]any{
		"provider": "openai", "model": "gpt-4.1-mini", "api_key": "sk-test",
	})
	if saveW.Code != http.StatusOK {
		t.Fatalf("save provider: %d (%s)", saveW.Code, saveW.Body.String())
	}

	w := postJSON(t, h, "/generate/text", map[string]any{
		"prompt":     "Busca noticias de IA y resume lo relevante para LinkedIn",
		"platform":   "linkedin",
		"web_search": true,
	})
	if w.Code != http.StatusOK {
		t.Fatalf("generate text: %d (%s)", w.Code, w.Body.String())
	}
	var out struct {
		UsedWebSearch bool `json:"used_web_search"`
		Sources       []struct {
			Title string `json:"title"`
			URL   string `json:"url"`
		} `json:"sources"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &out); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if !out.UsedWebSearch {
		t.Fatal("expected used_web_search=true")
	}
	if len(out.Sources) != 1 {
		t.Fatalf("expected one source, got %#v", out.Sources)
	}
	if out.Sources[0].URL == "" {
		t.Fatalf("expected source URL, got %#v", out.Sources[0])
	}
}

func TestGenerateText_WithLocalSessionCookie(t *testing.T) {
	tempDir := t.TempDir()
	store, err := db.Open(filepath.Join(tempDir, "test.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })

	passwordHash, err := bcrypt.GenerateFromPassword([]byte("super-secret"), bcrypt.MinCost)
	if err != nil {
		t.Fatalf("hash password: %v", err)
	}
	if _, err := store.UpsertLocalOwnerBootstrap(t.Context(), "owner@example.com", string(passwordHash)); err != nil {
		t.Fatalf("bootstrap owner: %v", err)
	}

	srv := Server{Store: store, DataDir: tempDir, GenerationDriver: "mock", LocalAuthEnabled: true}
	h := srv.Handler()

	form := url.Values{}
	form.Set("email", "owner@example.com")
	form.Set("password", "super-secret")
	loginReq := httptest.NewRequest(http.MethodPost, "/login", strings.NewReader(form.Encode()))
	loginReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	loginReq.Header.Set("Accept", "text/html")
	loginW := httptest.NewRecorder()
	h.ServeHTTP(loginW, loginReq)
	sessionCookie := cookieValue(t, loginW.Result().Cookies(), localSessionCookieName)
	if sessionCookie == "" {
		t.Fatalf("expected session cookie after login")
	}

	payload, _ := json.Marshal(map[string]any{"prompt": "Announce launch"})
	req := httptest.NewRequest(http.MethodPost, "/generate/text", bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(&http.Cookie{Name: localSessionCookieName, Value: sessionCookie})
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected session-authenticated request to reach the handler (400 no provider), got %d (%s)", w.Code, w.Body.String())
	}
}

func TestGenerateImage_PersistsMedia(t *testing.T) {
	srv, h := newGenerationTestServer(t)

	saveW := postJSON(t, h, "/settings/generation/image", map[string]any{
		"provider": "openai", "model": "gpt-image-1", "api_key": "sk-test",
	})
	if saveW.Code != http.StatusOK {
		t.Fatalf("save provider: %d (%s)", saveW.Code, saveW.Body.String())
	}

	w := postJSON(t, h, "/generate/image", map[string]any{"prompt": "a logo"})
	if w.Code != http.StatusCreated {
		t.Fatalf("generate image: %d (%s)", w.Code, w.Body.String())
	}
	var out struct {
		MediaID    string `json:"media_id"`
		PreviewURL string `json:"preview_url"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &out); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if out.MediaID == "" {
		t.Fatal("expected media_id")
	}
	media, err := srv.Store.GetMediaByIDs(t.Context(), []string{out.MediaID})
	if err != nil || len(media) != 1 {
		t.Fatalf("media not persisted: %v len=%d", err, len(media))
	}
	if media[0].Kind != "image" {
		t.Errorf("expected image kind, got %q", media[0].Kind)
	}
}

func TestGenerateView_Renders(t *testing.T) {
	_, h := newGenerationTestServer(t)
	req := httptest.NewRequest(http.MethodGet, "/?view=generate", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	body := w.Body.String()
	if !strings.Contains(body, "generate-view") {
		t.Errorf("generate view not rendered")
	}
}

func TestBrandProfile_FormCreateAndHandoff(t *testing.T) {
	srv, h := newGenerationTestServer(t)

	form := "name=Sare&system_prompt=We+automate&tone=confident&return_to=%2F%3Fview%3Dsettings"
	req := httptest.NewRequest(http.MethodPost, "/settings/brand-profiles", strings.NewReader(form))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusSeeOther {
		t.Fatalf("expected redirect, got %d (%s)", w.Code, w.Body.String())
	}

	profiles, err := srv.generationService().ListBrandProfiles(t.Context())
	if err != nil || len(profiles) != 1 {
		t.Fatalf("brand profiles: %v len=%d", err, len(profiles))
	}

	// Verify the create view accepts text + media_id prefill query params.
	saveW := postJSON(t, h, "/settings/generation/image", map[string]any{
		"provider": "openai", "model": "gpt-image-1", "api_key": "sk-test",
	})
	if saveW.Code != http.StatusOK {
		t.Fatalf("save image provider: %d", saveW.Code)
	}
	imgW := postJSON(t, h, "/generate/image", map[string]any{"prompt": "logo"})
	var img struct {
		MediaID string `json:"media_id"`
	}
	_ = json.Unmarshal(imgW.Body.Bytes(), &img)

	req = httptest.NewRequest(http.MethodGet, "/?view=create&text=hello+world&media_id="+img.MediaID, nil)
	w = httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("create view: %d", w.Code)
	}
	if !strings.Contains(w.Body.String(), "hello world") {
		t.Errorf("create view did not prefill generated text")
	}
}
