package api

import (
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

func newSafetyMCPServer(t *testing.T) (*db.Store, *httptest.Server, string, string) {
	t.Helper()
	tempDir := t.TempDir()
	store, err := db.Open(filepath.Join(tempDir, "safety_mcp.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })
	token := "tok_safety_mcp"
	srv := Server{
		Store:             store,
		DataDir:           tempDir,
		DefaultMaxRetries: 3,
		APIToken:          token,
	}
	httpServer := httptest.NewServer(srv.Handler())
	t.Cleanup(httpServer.Close)
	mcpURL := httpServer.URL + "/mcp"

	initResp, _ := postMCPRequestWithAuth(t, mcpURL, "", `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-06-18","capabilities":{},"clientInfo":{"name":"safety-test","version":"1.0"}}}`, token, "")
	if initResp.StatusCode != http.StatusOK {
		t.Fatalf("initialize status=%d", initResp.StatusCode)
	}
	session := strings.TrimSpace(initResp.Header.Get("Mcp-Session-Id"))
	if session == "" {
		t.Fatalf("missing mcp session id")
	}
	return store, httpServer, mcpURL, session
}

func postMCPRequestWithAuth(t *testing.T, endpoint, sessionID, payload, bearer, accept string) (*http.Response, []byte) {
	t.Helper()
	if accept == "" {
		accept = "application/json, text/event-stream"
	}
	req, err := http.NewRequest(http.MethodPost, endpoint, strings.NewReader(payload))
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", accept)
	if bearer != "" {
		req.Header.Set("Authorization", "Bearer "+bearer)
	}
	if strings.TrimSpace(sessionID) != "" {
		req.Header.Set("Mcp-Session-Id", sessionID)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("post mcp: %v", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	return resp, normalizeMCPResponseBody(body)
}

func mcpSafetyCall(t *testing.T, mcpURL, session, token, tool string, args map[string]any) map[string]any {
	t.Helper()
	payload := map[string]any{
		"jsonrpc": "2.0",
		"id":      100,
		"method":  "tools/call",
		"params":  map[string]any{"name": tool, "arguments": args},
	}
	raw, _ := json.Marshal(payload)
	resp, body := postMCPRequestWithAuth(t, mcpURL, session, string(raw), token, "")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("mcp call %s status=%d body=%s", tool, resp.StatusCode, string(body))
	}
	var out struct {
		Error *struct {
			Message string `json:"message"`
		} `json:"error"`
		Result struct {
			IsError           bool           `json:"isError"`
			StructuredContent map[string]any `json:"structuredContent"`
		} `json:"result"`
	}
	if err := json.Unmarshal(body, &out); err != nil {
		t.Fatalf("decode mcp %s: %v body=%s", tool, err, string(body))
	}
	if out.Error != nil {
		t.Fatalf("mcp %s error: %s", tool, out.Error.Message)
	}
	if out.Result.IsError {
		t.Fatalf("mcp %s isError=true: %s", tool, string(body))
	}
	if out.Result.StructuredContent == nil {
		t.Fatalf("mcp %s missing structuredContent", tool)
	}
	return out.Result.StructuredContent
}

func mcpSafetyCallError(t *testing.T, mcpURL, session, token, tool string, args map[string]any) (bool, string) {
	t.Helper()
	payload := map[string]any{
		"jsonrpc": "2.0",
		"id":      100,
		"method":  "tools/call",
		"params":  map[string]any{"name": tool, "arguments": args},
	}
	raw, _ := json.Marshal(payload)
	resp, body := postMCPRequestWithAuth(t, mcpURL, session, string(raw), token, "")
	if resp.StatusCode != http.StatusOK {
		return true, strings.TrimSpace(string(body))
	}
	var out struct {
		Error *struct {
			Message string `json:"message"`
		} `json:"error"`
		Result struct {
			IsError bool `json:"isError"`
			Content []struct {
				Text string `json:"text"`
			} `json:"content"`
			StructuredContent map[string]any `json:"structuredContent"`
		} `json:"result"`
	}
	if err := json.Unmarshal(body, &out); err != nil {
		t.Fatalf("decode mcp %s error resp: %v body=%s", tool, err, string(body))
	}
	if out.Error != nil {
		return true, strings.TrimSpace(out.Error.Message)
	}
	if out.Result.IsError {
		if len(out.Result.Content) > 0 && strings.TrimSpace(out.Result.Content[0].Text) != "" {
			return true, strings.TrimSpace(out.Result.Content[0].Text)
		}
		if msg, _ := out.Result.StructuredContent["error"].(string); strings.TrimSpace(msg) != "" {
			return true, strings.TrimSpace(msg)
		}
		return true, "tool returned isError=true"
	}
	return false, ""
}

func TestMCPSafetyRulesUpsertRejectsUnknownKindAndSeverity(t *testing.T) {
	// R1-W1: parity with HTTP — MCP must surface isError for a typo'd
	// Kind/Severity instead of silently persisting a never-blocking rule.
	_, _, mcpURL, session := newSafetyMCPServer(t)
	token := "tok_safety_mcp"
	cases := []struct {
		name    string
		args    map[string]any
		wantErr string
	}{
		{
			name:    "unknown kind typo banned_term",
			args:    map[string]any{"name": "bad kind", "kind": "banned_term", "severity": "block", "enabled": true},
			wantErr: "kind",
		},
		{
			name:    "unknown severity typo blok",
			args:    map[string]any{"name": "bad sev", "kind": "banned_terms", "severity": "blok", "enabled": true},
			wantErr: "severity",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			isErr, msg := mcpSafetyCallError(t, mcpURL, session, token, "postflow_upsert_safety_rule", tc.args)
			if !isErr {
				t.Fatalf("expected isError for %s, got success", tc.name)
			}
			if !strings.Contains(msg, tc.wantErr) {
				t.Fatalf("error %q must mention %q", msg, tc.wantErr)
			}
		})
	}
}

func TestMCPSafetyRulesListAndCRUD(t *testing.T) {
	_, _, mcpURL, session := newSafetyMCPServer(t)
	token := "tok_safety_mcp"

	out := mcpSafetyCall(t, mcpURL, session, token, "postflow_list_safety_rules", map[string]any{})
	items, _ := out["items"].([]any)
	if len(items) != 10 {
		t.Fatalf("expected 10 rules via mcp, got %d", len(items))
	}

	created := mcpSafetyCall(t, mcpURL, session, token, "postflow_upsert_safety_rule", map[string]any{
		"name":     "mcp banned",
		"kind":     "banned_terms",
		"params":   map[string]any{"banned_patterns": []string{"scam\\b"}},
		"scope":    "global",
		"severity": "block",
		"enabled":  true,
	})
	id, _ := created["id"].(string)
	if !strings.HasPrefix(id, "sft_") {
		t.Fatalf("expected sft_ id, got %q", id)
	}

	got := mcpSafetyCall(t, mcpURL, session, token, "postflow_get_safety_rule", map[string]any{"id": id})
	if got["id"] != id {
		t.Fatalf("get returned wrong id: %v", got["id"])
	}

	deleted := mcpSafetyCall(t, mcpURL, session, token, "postflow_delete_safety_rule", map[string]any{"id": id})
	if d, _ := deleted["deleted"].(bool); !d {
		t.Fatalf("expected deleted=true, got %v", deleted["deleted"])
	}
}

func TestMCPAutoApprovePosts(t *testing.T) {
	store, _, mcpURL, session := newSafetyMCPServer(t)
	token := "tok_safety_mcp"
	accountID := testAccountID(t, store)
	campaign, _ := store.CreateCampaign(t.Context(), domain.Campaign{Name: "mcp auto approve", Status: domain.CampaignStatusActive})
	created, _ := store.CreatePost(t.Context(), db.CreatePostParams{
		Post: domain.Post{
			AccountID:       accountID,
			Platform:        domain.PlatformX,
			Text:            "clean mcp post",
			Status:          domain.PostStatusScheduled,
			ScheduledAt:     time.Now().UTC().Add(time.Hour),
			MaxAttempts:     3,
			EditorialStatus: domain.EditorialStatusNeedsReview,
		},
		CampaignID:       campaign.ID,
		EditorialStatus:  domain.EditorialStatusNeedsReview,
		RequiresApproval: true,
	})

	out := mcpSafetyCall(t, mcpURL, session, token, "postflow_auto_approve_posts", map[string]any{})
	approved, _ := out["approved"].(float64)
	if approved < 1 {
		t.Fatalf("expected >=1 approved via mcp, got %+v", out)
	}
	post, _ := store.GetPost(t.Context(), created.Post.ID)
	if post.EditorialStatus != domain.EditorialStatusApproved {
		t.Fatalf("expected post approved, got %s", post.EditorialStatus)
	}
}

func TestMCPSafetyRulesListRegisteredInToolsList(t *testing.T) {
	_, _, mcpURL, session := newSafetyMCPServer(t)
	token := "tok_safety_mcp"
	resp, body := postMCPRequestWithAuth(t, mcpURL, session, `{"jsonrpc":"2.0","id":2,"method":"tools/list","params":{}}`, token, "")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("tools/list status=%d", resp.StatusCode)
	}
	for _, name := range []string{
		"postflow_list_safety_rules",
		"postflow_get_safety_rule",
		"postflow_upsert_safety_rule",
		"postflow_delete_safety_rule",
		"postflow_auto_approve_posts",
	} {
		if !strings.Contains(string(body), name) {
			t.Fatalf("tools/list missing %q: %s", name, string(body))
		}
	}
}
