package parity_test

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/escarface/sarepost/internal/db"
	"github.com/escarface/sarepost/internal/domain"
)

// normalizedSafetyRule is the transport-neutral shape used to compare rules
// across HTTP, MCP, and CLI (all three return the same JSON fields).
type normalizedSafetyRule struct {
	ID       string         `json:"id"`
	Kind     string         `json:"kind"`
	Severity string         `json:"severity"`
	Enabled  bool           `json:"enabled"`
	Platform string         `json:"platform"`
	Params   map[string]any `json:"params"`
}

type safetyRulesListPayload struct {
	Count int                    `json:"count"`
	Items []normalizedSafetyRule `json:"items"`
}

func (e *parityEnv) apiSafetyRulesList() safetyRulesListPayload {
	e.t.Helper()
	raw, status := e.apiJSON(http.MethodGet, "/safety-rules", nil, "")
	if status != http.StatusOK {
		e.t.Fatalf("api safety-rules list status=%d body=%s", status, string(raw))
	}
	var out safetyRulesListPayload
	mustJSON(e.t, raw, &out)
	return out
}

func (e *parityEnv) cliSafetyRulesList() safetyRulesListPayload {
	e.t.Helper()
	raw := e.runCLI("safety-rules", "list")
	var out safetyRulesListPayload
	mustJSON(e.t, raw, &out)
	return out
}

func (e *parityEnv) mcpSafetyRulesList() safetyRulesListPayload {
	e.t.Helper()
	out := e.mcpCallTool("postflow_list_safety_rules", map[string]any{})
	raw, _ := json.Marshal(out)
	var payload safetyRulesListPayload
	mustJSON(e.t, raw, &payload)
	if payload.Items == nil {
		// MCP wraps items in structuredContent; re-decode from items slice.
		if items, ok := out["items"].([]any); ok {
			payload.Items = make([]normalizedSafetyRule, 0, len(items))
			for _, item := range items {
				obj, _ := item.(map[string]any)
				rawItem, _ := json.Marshal(obj)
				var r normalizedSafetyRule
				_ = json.Unmarshal(rawItem, &r)
				payload.Items = append(payload.Items, r)
			}
			payload.Count = len(payload.Items)
		}
	}
	return payload
}

func (e *parityEnv) apiSafetyRulesUpsert(body map[string]any) normalizedSafetyRule {
	e.t.Helper()
	raw, status := e.apiJSON(http.MethodPost, "/safety-rules", body, "application/json")
	if status != http.StatusOK && status != http.StatusCreated {
		e.t.Fatalf("api safety-rules upsert status=%d body=%s", status, string(raw))
	}
	var out normalizedSafetyRule
	mustJSON(e.t, raw, &out)
	return out
}

func (e *parityEnv) mcpSafetyRulesUpsert(args map[string]any) normalizedSafetyRule {
	e.t.Helper()
	out := e.mcpCallTool("postflow_upsert_safety_rule", args)
	raw, _ := json.Marshal(out)
	var r normalizedSafetyRule
	mustJSON(e.t, raw, &r)
	return r
}

func (e *parityEnv) cliSafetyRulesUpsert(args []string) normalizedSafetyRule {
	e.t.Helper()
	full := append([]string{"safety-rules", "upsert"}, args...)
	raw := e.runCLI(full...)
	var out normalizedSafetyRule
	mustJSON(e.t, raw, &out)
	return out
}

func (e *parityEnv) apiAutoApprove(dryRun bool) map[string]any {
	e.t.Helper()
	body := map[string]any{}
	if dryRun {
		body["dry_run"] = true
	}
	raw, status := e.apiJSON(http.MethodPost, "/posts/auto-approve", body, "application/json")
	if status != http.StatusOK {
		e.t.Fatalf("api auto-approve status=%d body=%s", status, string(raw))
	}
	var out map[string]any
	mustJSON(e.t, raw, &out)
	return out
}

func (e *parityEnv) mcpAutoApprove(dryRun bool) map[string]any {
	e.t.Helper()
	args := map[string]any{}
	if dryRun {
		args["dry_run"] = true
	}
	return e.mcpCallTool("postflow_auto_approve_posts", args)
}

func (e *parityEnv) cliAutoApprove(dryRun bool) map[string]any {
	e.t.Helper()
	args := []string{"posts", "auto-approve"}
	if dryRun {
		args = append(args, "--dry-run")
	}
	raw := e.runCLI(args...)
	var out map[string]any
	mustJSON(e.t, raw, &out)
	return out
}

func seedSafetyEligiblePost(t *testing.T, env *parityEnv, text string) string {
	t.Helper()
	campaign, err := env.store.CreateCampaign(t.Context(), domain.Campaign{Name: "parity safety " + text, Status: domain.CampaignStatusActive})
	if err != nil {
		t.Fatalf("create parity campaign: %v", err)
	}
	created, err := env.store.CreatePost(t.Context(), db.CreatePostParams{
		Post: domain.Post{
			AccountID:       env.account.ID,
			Platform:        env.account.Platform,
			Text:            text,
			Status:          domain.PostStatusScheduled,
			ScheduledAt:     time.Now().UTC().Add(2 * time.Hour),
			MaxAttempts:     3,
			EditorialStatus: domain.EditorialStatusNeedsReview,
		},
		CampaignID:       campaign.ID,
		EditorialStatus:  domain.EditorialStatusNeedsReview,
		RequiresApproval: true,
	})
	if err != nil {
		t.Fatalf("seed parity eligible post: %v", err)
	}
	return created.Post.ID
}

func TestSafetyGateSurfaceParity(t *testing.T) {
	env := newParityEnv(t)

	t.Run("safety-rules.list returns same count across surfaces", func(t *testing.T) {
		api := env.apiSafetyRulesList()
		cli := env.cliSafetyRulesList()
		mcp := env.mcpSafetyRulesList()
		if api.Count != cli.Count || api.Count != mcp.Count {
			t.Fatalf("rule count mismatch: api=%d cli=%d mcp=%d", api.Count, cli.Count, mcp.Count)
		}
		if api.Count < 10 {
			t.Fatalf("expected at least 10 seeded rules, got %d", api.Count)
		}
		// Same set of ids across surfaces.
		apiIDs := ruleIDs(api.Items)
		cliIDs := ruleIDs(cli.Items)
		mcpIDs := ruleIDs(mcp.Items)
		if !sameIDSet(apiIDs, cliIDs) || !sameIDSet(apiIDs, mcpIDs) {
			t.Fatalf("rule id sets differ:\n api=%v\n cli=%v\n mcp=%v", apiIDs, cliIDs, mcpIDs)
		}
	})

	t.Run("safety-rules.upsert returns same shape across surfaces", func(t *testing.T) {
		params := map[string]any{"banned_patterns": []string{"parity\\b"}}
		upsertBody := map[string]any{
			"name":     "parity banned",
			"kind":     "banned_terms",
			"params":   params,
			"scope":    "global",
			"severity": "block",
			"enabled":  true,
		}
		apiRule := env.apiSafetyRulesUpsert(upsertBody)
		mcpRule := env.mcpSafetyRulesUpsert(upsertBody)
		cliRule := env.cliSafetyRulesUpsert([]string{
			"--name", "parity banned",
			"--kind", "banned_terms",
			"--params-json", `{"banned_patterns":["parity\\b"]}`,
			"--severity", "block",
			"--enabled",
		})

		for label, rule := range map[string]normalizedSafetyRule{"api": apiRule, "mcp": mcpRule, "cli": cliRule} {
			if rule.Kind != "banned_terms" {
				t.Fatalf("%s upsert kind mismatch: %q", label, rule.Kind)
			}
			if rule.Severity != "block" || !rule.Enabled {
				t.Fatalf("%s upsert severity/enabled mismatch: %+v", label, rule)
			}
			if !strings.HasPrefix(rule.ID, "sft_") {
				t.Fatalf("%s upsert id prefix mismatch: %q", label, rule.ID)
			}
		}
	})

	t.Run("posts.auto-approve promotes eligible post across surfaces", func(t *testing.T) {
		// Interleave seed+approve per surface: auto-approve operates on the
		// whole eligible set, so each surface must see only its own fresh post.
		// Post text avoids "parity" so the upsert subtest's banned_terms rule
		// does not block these posts.
		cases := []struct {
			label   string
			text    string
			approve func() map[string]any
		}{
			{"api", "api surface clean post", func() map[string]any { return env.apiAutoApprove(false) }},
			{"cli", "cli surface clean post", func() map[string]any { return env.cliAutoApprove(false) }},
			{"mcp", "mcp surface clean post", func() map[string]any { return env.mcpAutoApprove(false) }},
		}
		for _, c := range cases {
			postID := seedSafetyEligiblePost(t, env, c.text)
			summary := c.approve()
			approved, _ := summary["approved"].(float64)
			if approved < 1 {
				t.Fatalf("%s auto-approve expected >=1 approved, got %+v", c.label, summary)
			}
			post, err := env.store.GetPost(t.Context(), postID)
			if err != nil {
				t.Fatalf("%s get post: %v", c.label, err)
			}
			if post.EditorialStatus != domain.EditorialStatusApproved {
				t.Fatalf("%s expected post approved, got %s", c.label, post.EditorialStatus)
			}
			if post.AutoApprovedReason == "" {
				t.Fatalf("%s expected auto_approved_reason set", c.label)
			}
		}
	})

	t.Run("posts.auto-approve dry-run does not mutate across surfaces", func(t *testing.T) {
		// Dry-run never mutates, so all three surfaces can run over the same
		// eligible set without interfering. Text avoids "parity" (banned by
		// the upsert subtest's rule).
		apiPost := seedSafetyEligiblePost(t, env, "api dry clean post")
		cliPost := seedSafetyEligiblePost(t, env, "cli dry clean post")
		mcpPost := seedSafetyEligiblePost(t, env, "mcp dry clean post")

		env.apiAutoApprove(true)
		env.cliAutoApprove(true)
		env.mcpAutoApprove(true)

		for label, postID := range map[string]string{"api": apiPost, "cli": cliPost, "mcp": mcpPost} {
			post, err := env.store.GetPost(t.Context(), postID)
			if err != nil {
				t.Fatalf("%s get post: %v", label, err)
			}
			if post.EditorialStatus != domain.EditorialStatusNeedsReview {
				t.Fatalf("%s dry-run must not promote, got %s", label, post.EditorialStatus)
			}
			if post.AutoApprovedReason != "" {
				t.Fatalf("%s dry-run must not set auto_approved_reason, got %q", label, post.AutoApprovedReason)
			}
		}
	})
}

func ruleIDs(rules []normalizedSafetyRule) []string {
	out := make([]string, 0, len(rules))
	for _, r := range rules {
		out = append(out, strings.TrimSpace(r.ID))
	}
	return out
}

func sameIDSet(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	set := make(map[string]struct{}, len(a))
	for _, id := range a {
		set[id] = struct{}{}
	}
	for _, id := range b {
		if _, ok := set[id]; !ok {
			return false
		}
	}
	return true
}
