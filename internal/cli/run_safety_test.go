package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/escarface/sarepost/internal/api"
	"github.com/escarface/sarepost/internal/db"
	"github.com/escarface/sarepost/internal/domain"
)

func newSafetyCLIEnv(t *testing.T) (*db.Store, *httptest.Server, string) {
	t.Helper()
	tempDir := t.TempDir()
	store, err := db.Open(filepath.Join(tempDir, "safety_cli.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })
	token := "tok_safety_cli"
	srv := api.Server{
		Store:             store,
		DataDir:           tempDir,
		DefaultMaxRetries: 3,
		APIToken:          token,
	}
	server := httptest.NewServer(srv.Handler())
	t.Cleanup(server.Close)
	return store, server, token
}

func safetyCLIRun(t *testing.T, server *httptest.Server, token string, args ...string) (int, string, string) {
	t.Helper()
	full := append([]string{"--base-url", server.URL, "--api-token", token, "--json"}, args...)
	var out bytes.Buffer
	var errOut bytes.Buffer
	code := Run(context.Background(), full, &out, &errOut)
	return code, out.String(), errOut.String()
}

func TestCLISafetyRulesList(t *testing.T) {
	_, server, token := newSafetyCLIEnv(t)
	code, out, errOut := safetyCLIRun(t, server, token, "safety-rules", "list")
	if code != 0 {
		t.Fatalf("cli list code=%d err=%s out=%s", code, errOut, out)
	}
	var parsed struct {
		Count int `json:"count"`
		Items []struct {
			ID string `json:"id"`
		} `json:"items"`
	}
	if err := json.Unmarshal([]byte(out), &parsed); err != nil {
		t.Fatalf("decode list: %v out=%s", err, out)
	}
	if parsed.Count != 10 {
		t.Fatalf("expected 10 rules, got %d", parsed.Count)
	}
}

func TestCLISafetyRulesUpsertGetDelete(t *testing.T) {
	_, server, token := newSafetyCLIEnv(t)

	code, out, errOut := safetyCLIRun(t, server, token, "safety-rules", "upsert",
		"--name", "cli banned",
		"--kind", "banned_terms",
		"--params-json", `{"banned_patterns":["scam\\b"]}`,
		"--severity", "block",
		"--enabled",
	)
	if code != 0 {
		t.Fatalf("upsert code=%d err=%s out=%s", code, errOut, out)
	}
	var created struct {
		ID string `json:"id"`
	}
	json.Unmarshal([]byte(out), &created)
	if !strings.HasPrefix(created.ID, "sft_") {
		t.Fatalf("expected sft_ id, got %q out=%s", created.ID, out)
	}

	code, out, errOut = safetyCLIRun(t, server, token, "safety-rules", "get", "--id", created.ID)
	if code != 0 {
		t.Fatalf("get code=%d err=%s", code, errOut)
	}
	var got struct {
		ID string `json:"id"`
	}
	json.Unmarshal([]byte(out), &got)
	if got.ID != created.ID {
		t.Fatalf("get returned wrong id: %s", got.ID)
	}

	code, _, errOut = safetyCLIRun(t, server, token, "safety-rules", "delete", "--id", created.ID)
	if code != 0 {
		t.Fatalf("delete code=%d err=%s", code, errOut)
	}

	code, _, _ = safetyCLIRun(t, server, token, "safety-rules", "get", "--id", created.ID)
	if code != 1 {
		t.Fatalf("expected exit 1 for missing rule after delete, got %d", code)
	}
}

func TestCLISafetyRulesUpsertRejectsUnknownKindAndSeverity(t *testing.T) {
	// R1-W1: parity with HTTP/MCP — a typo'd Kind/Severity must surface a
	// non-zero exit with a message, not silently persist a never-blocking rule.
	// CLI goes through the HTTP server, so this also proves the server-side
	// guard reaches the CLI surface.
	_, server, token := newSafetyCLIEnv(t)
	cases := []struct {
		name    string
		args    []string
		wantErr string
	}{
		{
			name:    "unknown kind typo banned_term",
			args:    []string{"safety-rules", "upsert", "--name", "bad", "--kind", "banned_term", "--severity", "block", "--enabled"},
			wantErr: "kind",
		},
		{
			name:    "unknown severity typo blok",
			args:    []string{"safety-rules", "upsert", "--name", "bad", "--kind", "banned_terms", "--severity", "blok", "--enabled"},
			wantErr: "severity",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			code, _, errOut := safetyCLIRun(t, server, token, tc.args...)
			if code == 0 {
				t.Fatalf("expected non-zero exit, got 0")
			}
			if !strings.Contains(errOut, tc.wantErr) {
				t.Fatalf("stderr %q must mention %q", errOut, tc.wantErr)
			}
		})
	}
}

func TestCLIPostsAutoApprove(t *testing.T) {
	store, server, token := newSafetyCLIEnv(t)
	accountID := createSafetyCLIAccount(t, store)
	campaign, _ := store.CreateCampaign(t.Context(), domain.Campaign{Name: "cli auto approve", Status: domain.CampaignStatusActive})
	created, _ := store.CreatePost(t.Context(), db.CreatePostParams{
		Post: domain.Post{
			AccountID:       accountID,
			Platform:        domain.PlatformX,
			Text:            "clean cli post",
			Status:          domain.PostStatusScheduled,
			ScheduledAt:     time.Now().UTC().Add(time.Hour),
			MaxAttempts:     3,
			EditorialStatus: domain.EditorialStatusNeedsReview,
		},
		CampaignID:       campaign.ID,
		EditorialStatus:  domain.EditorialStatusNeedsReview,
		RequiresApproval: true,
	})

	code, out, errOut := safetyCLIRun(t, server, token, "posts", "auto-approve")
	if code != 0 {
		t.Fatalf("auto-approve code=%d err=%s out=%s", code, errOut, out)
	}
	var summary struct {
		Approved int `json:"approved"`
	}
	json.Unmarshal([]byte(out), &summary)
	if summary.Approved < 1 {
		t.Fatalf("expected >=1 approved, got %+v out=%s", summary, out)
	}
	post, _ := store.GetPost(t.Context(), created.Post.ID)
	if post.EditorialStatus != domain.EditorialStatusApproved {
		t.Fatalf("expected post approved, got %s", post.EditorialStatus)
	}
}

func TestCLIPostsAutoApproveDryRun(t *testing.T) {
	store, server, token := newSafetyCLIEnv(t)
	accountID := createSafetyCLIAccount(t, store)
	campaign, _ := store.CreateCampaign(t.Context(), domain.Campaign{Name: "cli dry run", Status: domain.CampaignStatusActive})
	created, _ := store.CreatePost(t.Context(), db.CreatePostParams{
		Post: domain.Post{
			AccountID:       accountID,
			Platform:        domain.PlatformX,
			Text:            "dry run cli post",
			Status:          domain.PostStatusScheduled,
			ScheduledAt:     time.Now().UTC().Add(time.Hour),
			MaxAttempts:     3,
			EditorialStatus: domain.EditorialStatusNeedsReview,
		},
		CampaignID:       campaign.ID,
		EditorialStatus:  domain.EditorialStatusNeedsReview,
		RequiresApproval: true,
	})

	code, out, errOut := safetyCLIRun(t, server, token, "posts", "auto-approve", "--dry-run")
	if code != 0 {
		t.Fatalf("dry-run code=%d err=%s out=%s", code, errOut, out)
	}
	post, _ := store.GetPost(t.Context(), created.Post.ID)
	if post.EditorialStatus != domain.EditorialStatusNeedsReview {
		t.Fatalf("dry-run must not promote, got %s", post.EditorialStatus)
	}
}

func createSafetyCLIAccount(t *testing.T, store *db.Store) string {
	t.Helper()
	account, err := store.UpsertAccount(t.Context(), db.UpsertAccountParams{
		Platform:          domain.PlatformX,
		DisplayName:       "Safety CLI X",
		ExternalAccountID: "safety_cli_x",
		AuthMethod:        domain.AuthMethodStatic,
		Status:            domain.AccountStatusConnected,
	})
	if err != nil {
		t.Fatalf("create account: %v", err)
	}
	return account.ID
}
