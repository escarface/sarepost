package api

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"

	safetyapp "github.com/escarface/sarepost/internal/application/safetygate"
	"github.com/escarface/sarepost/internal/db"
	"github.com/escarface/sarepost/internal/domain"
)

// safetyRuleResponse is the JSON shape for a rule across HTTP and MCP.
type safetyRuleResponse struct {
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

func toSafetyRuleResponse(r domain.SafetyRule) safetyRuleResponse {
	out := safetyRuleResponse{
		ID:        r.ID,
		Name:      r.Name,
		Kind:      string(r.Kind),
		Scope:     string(r.Scope),
		Severity:  string(r.Severity),
		Enabled:   r.Enabled,
		CreatedAt: formatMCPTime(r.CreatedAt),
		UpdatedAt: formatMCPTime(r.UpdatedAt),
	}
	if r.Platform != nil {
		out.Platform = string(*r.Platform)
	}
	if raw, err := json.Marshal(r.Params); err == nil {
		var params map[string]any
		_ = json.Unmarshal(raw, &params)
		out.Params = params
	}
	if out.Params == nil {
		out.Params = map[string]any{}
	}
	return out
}

type safetyRuleUpsertRequest struct {
	ID       string         `json:"id,omitempty"`
	Name     string         `json:"name"`
	Kind     string         `json:"kind"`
	Params   map[string]any `json:"params,omitempty"`
	Scope    string         `json:"scope,omitempty"`
	Platform string         `json:"platform,omitempty"`
	Severity string         `json:"severity,omitempty"`
	Enabled  bool           `json:"enabled"`
}

func (s Server) handleListSafetyRules(w http.ResponseWriter, r *http.Request) {
	rules, err := s.Store.ListSafetyRules(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	items := make([]safetyRuleResponse, 0, len(rules))
	for _, rule := range rules {
		items = append(items, toSafetyRuleResponse(rule))
	}
	writeJSON(w, http.StatusOK, map[string]any{"count": len(items), "items": items})
}

func (s Server) handleGetSafetyRule(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimSpace(r.PathValue("id"))
	if id == "" {
		writeError(w, http.StatusBadRequest, errors.New("rule id is required"))
		return
	}
	rule, err := s.Store.GetSafetyRule(r.Context(), id)
	if err != nil {
		if errors.Is(err, db.ErrSafetyRuleNotFound) {
			writeError(w, http.StatusNotFound, errors.New("safety rule not found"))
			return
		}
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, toSafetyRuleResponse(rule))
}

func (s Server) handleUpsertSafetyRule(w http.ResponseWriter, r *http.Request) {
	var req safetyRuleUpsertRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, errors.New("invalid json body"))
		return
	}
	rule, err := upsertRequestToRule(req)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	saved, err := s.Store.UpsertSafetyRule(r.Context(), rule)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, toSafetyRuleResponse(saved))
}

func (s Server) handleDeleteSafetyRule(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimSpace(r.PathValue("id"))
	if id == "" {
		writeError(w, http.StatusBadRequest, errors.New("rule id is required"))
		return
	}
	if err := s.Store.DeleteSafetyRule(r.Context(), id); err != nil {
		if errors.Is(err, db.ErrSafetyRuleNotFound) {
			writeError(w, http.StatusNotFound, errors.New("safety rule not found"))
			return
		}
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

type autoApproveRequest struct {
	Limit  int  `json:"limit,omitempty"`
	DryRun bool `json:"dry_run,omitempty"`
}

func (s Server) handleAutoApprovePosts(w http.ResponseWriter, r *http.Request) {
	var req autoApproveRequest
	if r.ContentLength != 0 {
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil && !errors.Is(err, io.EOF) {
			writeError(w, http.StatusBadRequest, errors.New("invalid json body"))
			return
		}
	}
	svc := safetyapp.Service{Store: s.Store}
	if req.DryRun {
		summary, err := svc.PreviewEligible(r.Context(), req.Limit)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err)
			return
		}
		writeJSON(w, http.StatusOK, approveSummaryResponse(summary))
		return
	}
	if req.Limit > 0 {
		svc.MaxBatchSize = req.Limit
	}
	summary, err := svc.ApproveEligible(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, approveSummaryResponse(summary))
}

func approveSummaryResponse(summary safetyapp.ApproveSummary) map[string]any {
	errs := make([]string, 0, len(summary.Errors))
	for _, e := range summary.Errors {
		errs = append(errs, e.Error())
	}
	return map[string]any{
		"evaluated": summary.Evaluated,
		"approved":  summary.Approved,
		"blocked":   summary.Blocked,
		"errors":    errs,
	}
}

func upsertRequestToRule(req safetyRuleUpsertRequest) (domain.SafetyRule, error) {
	kind := strings.TrimSpace(req.Kind)
	if kind == "" {
		return domain.SafetyRule{}, errors.New("kind is required")
	}
	params := domain.SafetyRuleParams{}
	if req.Params != nil {
		params = decodeSafetyRuleParams(req.Params)
	}
	rule := domain.SafetyRule{
		ID:       strings.TrimSpace(req.ID),
		Name:     strings.TrimSpace(req.Name),
		Kind:     domain.SafetyRuleKind(kind),
		Params:   params,
		Scope:    domain.SafetyRuleScope(strings.TrimSpace(req.Scope)),
		Severity: domain.SafetyRuleSeverity(strings.TrimSpace(req.Severity)),
		Enabled:  req.Enabled,
	}
	if p := strings.TrimSpace(req.Platform); p != "" {
		platform := domain.Platform(p)
		rule.Platform = &platform
	}
	// Preserve the documented "block (default)" behavior: an omitted severity
	// must never persist as "" (which the evaluator would treat as non-blocking
	// notes, silently never blocking — the R1-W1 bug). Default to block first.
	if rule.Severity == "" {
		rule.Severity = domain.SeverityBlock
	}
	// R1-W1: validate Kind/Severity at the application/use-case boundary so a
	// typo'd rule cannot be persisted. HTTP, MCP, and CLI all funnel through
	// this function, so rejection is consistent across surfaces.
	if err := safetyapp.ValidateRule(rule); err != nil {
		return domain.SafetyRule{}, err
	}
	return rule, nil
}

func decodeSafetyRuleParams(raw map[string]any) domain.SafetyRuleParams {
	var params domain.SafetyRuleParams
	if v, ok := raw["banned_patterns"].([]any); ok {
		for _, item := range v {
			if s, ok := item.(string); ok {
				params.BannedPatterns = append(params.BannedPatterns, s)
			}
		}
	}
	if v, ok := raw["needles"].([]any); ok {
		for _, item := range v {
			if s, ok := item.(string); ok {
				params.Needles = append(params.Needles, s)
			}
		}
	}
	if v, ok := raw["min_len"].(float64); ok {
		params.MinLen = int(v)
	}
	if v, ok := raw["max_len"].(float64); ok {
		params.MaxLen = int(v)
	}
	if v, ok := raw["hashtag_max"].(float64); ok {
		params.HashtagMax = int(v)
	}
	if v, ok := raw["link_max"].(float64); ok {
		params.LinkMax = int(v)
	}
	return params
}
