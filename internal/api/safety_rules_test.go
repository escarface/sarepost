package api

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/escarface/sarepost/internal/db"
	"github.com/escarface/sarepost/internal/domain"
)

func newSafetyTestServer(t *testing.T) (*db.Store, *httptest.Server, string) {
	t.Helper()
	tempDir := t.TempDir()
	store, err := db.Open(filepath.Join(tempDir, "safety_api.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })
	token := "tok_safety_api"
	srv := Server{
		Store:             store,
		DataDir:           tempDir,
		DefaultMaxRetries: 3,
		APIToken:          token,
	}
	httpServer := httptest.NewServer(srv.Handler())
	t.Cleanup(httpServer.Close)
	return store, httpServer, token
}

func safetyDo(t *testing.T, server *httptest.Server, token, method, path string, body any) ([]byte, int) {
	t.Helper()
	var reader io.Reader
	if body != nil {
		raw, err := json.Marshal(body)
		if err != nil {
			t.Fatalf("marshal body: %v", err)
		}
		reader = bytes.NewReader(raw)
	}
	req, err := http.NewRequest(method, server.URL+path, reader)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("do request: %v", err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	return raw, resp.StatusCode
}

func TestSafetyRulesAPIListSeeded(t *testing.T) {
	_, server, token := newSafetyTestServer(t)
	raw, status := safetyDo(t, server, token, http.MethodGet, "/safety-rules", nil)
	if status != http.StatusOK {
		t.Fatalf("list status=%d body=%s", status, string(raw))
	}
	var out struct {
		Count int `json:"count"`
		Items []struct {
			ID   string `json:"id"`
			Kind string `json:"kind"`
		} `json:"items"`
	}
	json.Unmarshal(raw, &out)
	if out.Count != 10 || len(out.Items) != 10 {
		t.Fatalf("expected 10 rules, got count=%d items=%d", out.Count, len(out.Items))
	}
}

func TestSafetyRulesAPIUpsertCreatesRule(t *testing.T) {
	_, server, token := newSafetyTestServer(t)
	body := map[string]any{
		"name":     "custom banned api",
		"kind":     "banned_terms",
		"params":   map[string]any{"banned_patterns": []string{"scam\\b"}},
		"scope":    "global",
		"platform": "x",
		"severity": "block",
		"enabled":  true,
	}
	raw, status := safetyDo(t, server, token, http.MethodPost, "/safety-rules", body)
	if status != http.StatusOK && status != http.StatusCreated {
		t.Fatalf("upsert status=%d body=%s", status, string(raw))
	}
	var out struct {
		ID       string `json:"id"`
		Kind     string `json:"kind"`
		Enabled  bool   `json:"enabled"`
		Platform string `json:"platform"`
	}
	json.Unmarshal(raw, &out)
	if !strings.HasPrefix(out.ID, "sft_") {
		t.Fatalf("expected sft_ id, got %q", out.ID)
	}
	if out.Kind != "banned_terms" || !out.Enabled {
		t.Fatalf("upsert returned wrong rule: %+v", out)
	}
	if out.Platform != "x" {
		t.Fatalf("expected platform x, got %q", out.Platform)
	}
}

func TestSafetyRulesAPIGetNotFound(t *testing.T) {
	_, server, token := newSafetyTestServer(t)
	raw, status := safetyDo(t, server, token, http.MethodGet, "/safety-rules/sft_missing", nil)
	if status != http.StatusNotFound {
		t.Fatalf("expected 404, got %d body=%s", status, string(raw))
	}
}

func TestSafetyRulesAPIDeleteAndIdempotent404(t *testing.T) {
	_, server, token := newSafetyTestServer(t)
	body := map[string]any{"name": "tmp", "kind": "link_max", "params": map[string]any{"link_max": 1}, "severity": "block", "enabled": true}
	raw, _ := safetyDo(t, server, token, http.MethodPost, "/safety-rules", body)
	var created struct {
		ID string `json:"id"`
	}
	json.Unmarshal(raw, &created)

	if _, status := safetyDo(t, server, token, http.MethodDelete, "/safety-rules/"+created.ID, nil); status != http.StatusNoContent && status != http.StatusOK {
		t.Fatalf("delete status=%d", status)
	}
	raw2, status := safetyDo(t, server, token, http.MethodDelete, "/safety-rules/"+created.ID, nil)
	if status != http.StatusNotFound {
		t.Fatalf("second delete expected 404, got %d body=%s", status, string(raw2))
	}
}

func TestPostsAutoApproveAPIPromotesEligible(t *testing.T) {
	store, server, token := newSafetyTestServer(t)
	accountID := testAccountID(t, store)
	campaign, err := store.CreateCampaign(t.Context(), domain.Campaign{Name: "auto approve api", Status: domain.CampaignStatusActive})
	if err != nil {
		t.Fatalf("create campaign: %v", err)
	}
	created, err := store.CreatePost(t.Context(), db.CreatePostParams{
		Post: domain.Post{
			AccountID:       accountID,
			Platform:        domain.PlatformX,
			Text:            "clean api post",
			Status:          domain.PostStatusScheduled,
			ScheduledAt:     time.Now().UTC().Add(time.Hour),
			MaxAttempts:     3,
			EditorialStatus: domain.EditorialStatusNeedsReview,
		},
		CampaignID:       campaign.ID,
		EditorialStatus:  domain.EditorialStatusNeedsReview,
		RequiresApproval: true,
	})
	if err != nil {
		t.Fatalf("seed post: %v", err)
	}

	raw, status := safetyDo(t, server, token, http.MethodPost, "/posts/auto-approve", map[string]any{})
	if status != http.StatusOK {
		t.Fatalf("auto-approve status=%d body=%s", status, string(raw))
	}
	var out struct {
		Evaluated int `json:"evaluated"`
		Approved  int `json:"approved"`
		Blocked   int `json:"blocked"`
	}
	json.Unmarshal(raw, &out)
	if out.Approved < 1 {
		t.Fatalf("expected >=1 approved, got %+v", out)
	}

	post, err := store.GetPost(t.Context(), created.Post.ID)
	if err != nil {
		t.Fatalf("get post: %v", err)
	}
	if post.EditorialStatus != domain.EditorialStatusApproved {
		t.Fatalf("expected post approved, got %s", post.EditorialStatus)
	}
	if post.AutoApprovedReason == "" {
		t.Fatalf("expected auto_approved_reason set")
	}
}

func TestPostsAutoApproveAPIResponseHasNoSkippedField(t *testing.T) {
	// R3-W-R1: the Skipped field is always 0 (VerdictSkipped is never produced)
	// and must not be exposed in the response. Assert it is absent from the JSON.
	_, server, token := newSafetyTestServer(t)
	raw, status := safetyDo(t, server, token, http.MethodPost, "/posts/auto-approve", map[string]any{})
	if status != http.StatusOK {
		t.Fatalf("auto-approve status=%d body=%s", status, string(raw))
	}
	var generic map[string]any
	if err := json.Unmarshal(raw, &generic); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if _, ok := generic["skipped"]; ok {
		t.Fatalf("response must not contain 'skipped' field (always 0, dead): %s", string(raw))
	}
}

func TestPostsAutoApproveAPIEmptyBodyOK(t *testing.T) {
	// An auto-approve POST with an empty body (Content-Length 0) must NOT
	// return 400. The handler skips decoding and runs the sweep with defaults.
	_, server, token := newSafetyTestServer(t)
	req, err := http.NewRequest(http.MethodPost, server.URL+"/posts/auto-approve", nil)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("do request: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		raw, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected 200 for empty body, got %d body=%s", resp.StatusCode, string(raw))
	}
}

func TestPostsAutoApproveAPIDryRunNoMutation(t *testing.T) {
	store, server, token := newSafetyTestServer(t)
	accountID := testAccountID(t, store)
	campaign, _ := store.CreateCampaign(t.Context(), domain.Campaign{Name: "dry run api", Status: domain.CampaignStatusActive})
	created, _ := store.CreatePost(t.Context(), db.CreatePostParams{
		Post: domain.Post{
			AccountID:       accountID,
			Platform:        domain.PlatformX,
			Text:            "dry run post",
			Status:          domain.PostStatusScheduled,
			ScheduledAt:     time.Now().UTC().Add(time.Hour),
			MaxAttempts:     3,
			EditorialStatus: domain.EditorialStatusNeedsReview,
		},
		CampaignID:       campaign.ID,
		EditorialStatus:  domain.EditorialStatusNeedsReview,
		RequiresApproval: true,
	})

	raw, status := safetyDo(t, server, token, http.MethodPost, "/posts/auto-approve", map[string]any{"dry_run": true})
	if status != http.StatusOK {
		t.Fatalf("auto-approve dry-run status=%d body=%s", status, string(raw))
	}
	post, _ := store.GetPost(t.Context(), created.Post.ID)
	if post.EditorialStatus != domain.EditorialStatusNeedsReview {
		t.Fatalf("dry-run must not promote, got %s", post.EditorialStatus)
	}
	if post.AutoApprovedReason != "" {
		t.Fatalf("dry-run must not set auto_approved_reason, got %q", post.AutoApprovedReason)
	}
}
