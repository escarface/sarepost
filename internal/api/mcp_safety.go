package api

import (
	"context"
	"errors"
	"strings"

	safetyapp "github.com/escarface/sarepost/internal/application/safetygate"
	"github.com/escarface/sarepost/internal/db"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type mcpListSafetyRulesInput struct {
	Limit int `json:"limit,omitempty" jsonschema:"Max rules to return (1-500). Default: 200."`
}

type mcpListSafetyRulesOutput struct {
	Count int                  `json:"count"`
	Items []safetyRuleResponse `json:"items"`
}

func (s Server) mcpListSafetyRulesTool(ctx context.Context, _ *mcp.CallToolRequest, in mcpListSafetyRulesInput) (*mcp.CallToolResult, mcpListSafetyRulesOutput, error) {
	rules, err := s.Store.ListSafetyRules(ctx)
	if err != nil {
		return nil, mcpListSafetyRulesOutput{}, err
	}
	limit := clampMCPListLimit(in.Limit)
	if len(rules) > limit {
		rules = rules[:limit]
	}
	items := make([]safetyRuleResponse, 0, len(rules))
	for _, r := range rules {
		items = append(items, toSafetyRuleResponse(r))
	}
	return nil, mcpListSafetyRulesOutput{Count: len(items), Items: items}, nil
}

type mcpGetSafetyRuleInput struct {
	ID string `json:"id" jsonschema:"Safety rule ID."`
}

func (s Server) mcpGetSafetyRuleTool(ctx context.Context, _ *mcp.CallToolRequest, in mcpGetSafetyRuleInput) (*mcp.CallToolResult, safetyRuleResponse, error) {
	id := strings.TrimSpace(in.ID)
	if id == "" {
		return nil, safetyRuleResponse{}, errors.New("id is required")
	}
	rule, err := s.Store.GetSafetyRule(ctx, id)
	if err != nil {
		if errors.Is(err, db.ErrSafetyRuleNotFound) {
			return nil, safetyRuleResponse{}, errors.New("safety rule not found")
		}
		return nil, safetyRuleResponse{}, err
	}
	return nil, toSafetyRuleResponse(rule), nil
}

type mcpUpsertSafetyRuleInput struct {
	ID       string         `json:"id,omitempty" jsonschema:"Optional existing rule ID to update. Empty creates a new rule."`
	Name     string         `json:"name" jsonschema:"Rule display name."`
	Kind     string         `json:"kind" jsonschema:"Rule kind: banned_terms|length_range|hashtag_max|link_max|required_contains."`
	Params   map[string]any `json:"params,omitempty" jsonschema:"Typed params for the rule kind (banned_patterns, min_len, max_len, hashtag_max, link_max, needles)."`
	Scope    string         `json:"scope,omitempty" jsonschema:"Rule scope: global (default)."`
	Platform string         `json:"platform,omitempty" jsonschema:"Optional platform: x|linkedin|facebook|instagram. Empty applies to all platforms."`
	Severity string         `json:"severity,omitempty" jsonschema:"Severity: block (default) or review."`
	Enabled  bool           `json:"enabled" jsonschema:"Whether the rule is active."`
}

func (s Server) mcpUpsertSafetyRuleTool(ctx context.Context, _ *mcp.CallToolRequest, in mcpUpsertSafetyRuleInput) (*mcp.CallToolResult, safetyRuleResponse, error) {
	rule, err := upsertRequestToRule(safetyRuleUpsertRequest{
		ID:       in.ID,
		Name:     in.Name,
		Kind:     in.Kind,
		Params:   in.Params,
		Scope:    in.Scope,
		Platform: in.Platform,
		Severity: in.Severity,
		Enabled:  in.Enabled,
	})
	if err != nil {
		return nil, safetyRuleResponse{}, err
	}
	saved, err := s.Store.UpsertSafetyRule(ctx, rule)
	if err != nil {
		return nil, safetyRuleResponse{}, err
	}
	return nil, toSafetyRuleResponse(saved), nil
}

type mcpDeleteSafetyRuleInput struct {
	ID string `json:"id" jsonschema:"Safety rule ID to delete."`
}

type mcpDeleteSafetyRuleOutput struct {
	ID      string `json:"id"`
	Deleted bool   `json:"deleted"`
}

func (s Server) mcpDeleteSafetyRuleTool(ctx context.Context, _ *mcp.CallToolRequest, in mcpDeleteSafetyRuleInput) (*mcp.CallToolResult, mcpDeleteSafetyRuleOutput, error) {
	id := strings.TrimSpace(in.ID)
	if id == "" {
		return nil, mcpDeleteSafetyRuleOutput{}, errors.New("id is required")
	}
	if err := s.Store.DeleteSafetyRule(ctx, id); err != nil {
		if errors.Is(err, db.ErrSafetyRuleNotFound) {
			return nil, mcpDeleteSafetyRuleOutput{}, errors.New("safety rule not found")
		}
		return nil, mcpDeleteSafetyRuleOutput{}, err
	}
	return nil, mcpDeleteSafetyRuleOutput{ID: id, Deleted: true}, nil
}

type mcpAutoApproveInput struct {
	Limit  int  `json:"limit,omitempty" jsonschema:"Max posts to evaluate in this sweep. Default: 100."`
	DryRun bool `json:"dry_run,omitempty" jsonschema:"When true, evaluate without mutating any post."`
}

type mcpAutoApproveOutput struct {
	Evaluated int      `json:"evaluated"`
	Approved  int      `json:"approved"`
	Blocked   int      `json:"blocked"`
	Errors    []string `json:"errors,omitempty"`
}

func (s Server) mcpAutoApprovePostsTool(ctx context.Context, _ *mcp.CallToolRequest, in mcpAutoApproveInput) (*mcp.CallToolResult, mcpAutoApproveOutput, error) {
	svc := safetyapp.Service{Store: s.Store}
	if in.DryRun {
		summary, err := svc.PreviewEligible(ctx, in.Limit)
		if err != nil {
			return nil, mcpAutoApproveOutput{}, err
		}
		return nil, autoApproveOutput(summary), nil
	}
	if in.Limit > 0 {
		svc.MaxBatchSize = in.Limit
	}
	summary, err := svc.ApproveEligible(ctx)
	if err != nil {
		return nil, mcpAutoApproveOutput{}, err
	}
	return nil, autoApproveOutput(summary), nil
}

func autoApproveOutput(summary safetyapp.ApproveSummary) mcpAutoApproveOutput {
	errs := make([]string, 0, len(summary.Errors))
	for _, e := range summary.Errors {
		errs = append(errs, e.Error())
	}
	return mcpAutoApproveOutput{
		Evaluated: summary.Evaluated,
		Approved:  summary.Approved,
		Blocked:   summary.Blocked,
		Errors:    errs,
	}
}
