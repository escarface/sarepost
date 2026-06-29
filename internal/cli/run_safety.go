package cli

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"strings"
)

type safetyRulesListResponse struct {
	Count int             `json:"count"`
	Items []safetyRuleDTO `json:"items"`
}

type safetyRuleDTO struct {
	ID        string         `json:"id"`
	Name      string         `json:"name"`
	Kind      string         `json:"kind"`
	Params    map[string]any `json:"params"`
	Scope     string         `json:"scope"`
	Platform  string         `json:"platform,omitempty"`
	Severity  string         `json:"severity"`
	Enabled   bool           `json:"enabled"`
	CreatedAt string         `json:"created_at"`
	UpdatedAt string         `json:"updated_at"`
}

type autoApproveSummaryDTO struct {
	Evaluated int      `json:"evaluated"`
	Approved  int      `json:"approved"`
	Blocked   int      `json:"blocked"`
	Errors    []string `json:"errors,omitempty"`
}

func runSafetyRules(ctx context.Context, client *APIClient, cfg config, args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprintln(stderr, "usage: postflow safety-rules <list|get|upsert|delete> [flags]")
		return 2
	}
	switch args[0] {
	case "list":
		return runSafetyRulesList(ctx, client, cfg, args[1:], stdout, stderr)
	case "get":
		return runSafetyRulesGet(ctx, client, cfg, args[1:], stdout, stderr)
	case "upsert":
		return runSafetyRulesUpsert(ctx, client, cfg, args[1:], stdout, stderr)
	case "delete":
		return runSafetyRulesDelete(ctx, client, cfg, args[1:], stdout, stderr)
	default:
		fmt.Fprintf(stderr, "unknown safety-rules subcommand: %s\n", args[0])
		return 2
	}
}

func runSafetyRulesList(ctx context.Context, client *APIClient, cfg config, args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("safety-rules list", flag.ContinueOnError)
	fs.SetOutput(stderr)
	limit := fs.Int("limit", 200, "Max rules")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	path := "/safety-rules"
	if *limit > 0 {
		path = fmt.Sprintf("%s?limit=%d", path, *limit)
	}
	var out safetyRulesListResponse
	if err := client.Get(ctx, path, nil, &out); err != nil {
		fmt.Fprintln(stderr, err.Error())
		return 1
	}
	printOutput(stdout, cfg.asJSON, out, func() {
		fmt.Fprintf(stdout, "count: %d\n", out.Count)
		for _, item := range out.Items {
			fmt.Fprintf(stdout, "- [%s] %s %s enabled=%t\n", item.Severity, item.ID, item.Kind, item.Enabled)
		}
	})
	return 0
}

func runSafetyRulesGet(ctx context.Context, client *APIClient, cfg config, args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("safety-rules get", flag.ContinueOnError)
	fs.SetOutput(stderr)
	id := fs.String("id", "", "Safety rule ID")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if strings.TrimSpace(*id) == "" {
		fmt.Fprintln(stderr, "--id is required")
		return 2
	}
	var out safetyRuleDTO
	if err := client.Get(ctx, "/safety-rules/"+strings.TrimSpace(*id), nil, &out); err != nil {
		fmt.Fprintln(stderr, err.Error())
		return 1
	}
	printOutput(stdout, cfg.asJSON, out, func() {
		fmt.Fprintf(stdout, "rule: %s %s enabled=%t\n", out.ID, out.Kind, out.Enabled)
	})
	return 0
}

func runSafetyRulesUpsert(ctx context.Context, client *APIClient, cfg config, args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("safety-rules upsert", flag.ContinueOnError)
	fs.SetOutput(stderr)
	id := fs.String("id", "", "Optional existing rule ID to update")
	name := fs.String("name", "", "Rule display name")
	kind := fs.String("kind", "", "Rule kind: banned_terms|length_range|hashtag_max|link_max|required_contains")
	paramsJSON := fs.String("params-json", "", "Typed params JSON, e.g. {\"banned_patterns\":[\"spam\"]}")
	platform := fs.String("platform", "", "Optional platform: x|linkedin|facebook|instagram")
	severity := fs.String("severity", "block", "Severity: block|review")
	enabled := fs.Bool("enabled", false, "Whether the rule is active")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if strings.TrimSpace(*kind) == "" {
		fmt.Fprintln(stderr, "--kind is required")
		return 2
	}
	payload := map[string]any{
		"name":     strings.TrimSpace(*name),
		"kind":     strings.TrimSpace(*kind),
		"scope":    "global",
		"severity": strings.TrimSpace(*severity),
		"enabled":  *enabled,
	}
	if strings.TrimSpace(*id) != "" {
		payload["id"] = strings.TrimSpace(*id)
	}
	if strings.TrimSpace(*platform) != "" {
		payload["platform"] = strings.TrimSpace(*platform)
	}
	if strings.TrimSpace(*paramsJSON) != "" {
		var params map[string]any
		if err := json.Unmarshal([]byte(strings.TrimSpace(*paramsJSON)), &params); err != nil {
			fmt.Fprintf(stderr, "--params-json must be valid JSON: %v\n", err)
			return 2
		}
		payload["params"] = params
	}
	var out safetyRuleDTO
	if err := client.Post(ctx, "/safety-rules", payload, &out); err != nil {
		fmt.Fprintln(stderr, err.Error())
		return 1
	}
	printOutput(stdout, cfg.asJSON, out, func() {
		fmt.Fprintf(stdout, "safety rule saved: %s %s\n", out.ID, out.Kind)
	})
	return 0
}

func runSafetyRulesDelete(ctx context.Context, client *APIClient, cfg config, args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("safety-rules delete", flag.ContinueOnError)
	fs.SetOutput(stderr)
	id := fs.String("id", "", "Safety rule ID")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if strings.TrimSpace(*id) == "" {
		fmt.Fprintln(stderr, "--id is required")
		return 2
	}
	if err := client.Delete(ctx, "/safety-rules/"+strings.TrimSpace(*id), nil); err != nil {
		fmt.Fprintln(stderr, err.Error())
		return 1
	}
	printOutput(stdout, cfg.asJSON, map[string]any{"id": strings.TrimSpace(*id), "deleted": true}, func() {
		fmt.Fprintf(stdout, "safety rule deleted: %s\n", strings.TrimSpace(*id))
	})
	return 0
}

func runPostsAutoApprove(ctx context.Context, client *APIClient, cfg config, args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("posts auto-approve", flag.ContinueOnError)
	fs.SetOutput(stderr)
	limit := fs.Int("limit", 0, "Max posts to evaluate (0 = server default)")
	dryRun := fs.Bool("dry-run", false, "Evaluate without mutating any post")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	payload := map[string]any{}
	if *limit > 0 {
		payload["limit"] = *limit
	}
	if *dryRun {
		payload["dry_run"] = true
	}
	var out autoApproveSummaryDTO
	if err := client.Post(ctx, "/posts/auto-approve", payload, &out); err != nil {
		fmt.Fprintln(stderr, err.Error())
		return 1
	}
	printOutput(stdout, cfg.asJSON, out, func() {
		fmt.Fprintf(stdout, "evaluated: %d approved: %d blocked: %d\n", out.Evaluated, out.Approved, out.Blocked)
		for _, e := range out.Errors {
			fmt.Fprintf(stdout, "error: %s\n", e)
		}
	})
	return 0
}
