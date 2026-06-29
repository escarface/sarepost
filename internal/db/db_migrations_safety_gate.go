package db

import (
	"context"
	"database/sql"
	"encoding/json"
	"strings"
	"time"

	"github.com/escarface/sarepost/internal/domain"
)

// migrationAddSafetyGate is migration v14. It is non-destructive and idempotent:
//   - Creates the safety_rules table (CREATE TABLE IF NOT EXISTS).
//   - Adds auto_approved_reason and blocked_reason columns to campaign_posts
//     (where editorial metadata already lives), guarded against duplicate-column
//     errors so re-entry is safe.
//   - Seeds 10 global default rules (4 length_range, 4 hashtag_max, 1 link_max,
//     1 banned_terms disabled) guarded by existence checks so re-running the Up
//     function does not duplicate rows.
//
// Design deviation: the SDD design proposed adding the audit columns to the
// posts table, but in this codebase editorial_status, requires_approval and
// approved_at all live in campaign_posts (joined in GetPost). Placing the audit
// columns there keeps the eligibility query and approve mutation coherent with
// the existing ApprovePost pattern and avoids a schema split.
func migrationAddSafetyGate(ctx context.Context, tx *sql.Tx) error {
	queries := []string{
		`CREATE TABLE IF NOT EXISTS safety_rules (
			id TEXT PRIMARY KEY,
			name TEXT NOT NULL,
			kind TEXT NOT NULL,
			params TEXT NOT NULL DEFAULT '{}',
			scope TEXT NOT NULL DEFAULT 'global',
			platform TEXT,
			severity TEXT NOT NULL,
			enabled INTEGER NOT NULL DEFAULT 1,
			created_at TEXT NOT NULL,
			updated_at TEXT NOT NULL
		);`,
		`CREATE INDEX IF NOT EXISTS idx_safety_rules_kind_platform ON safety_rules(kind, platform);`,
	}
	for _, q := range queries {
		if _, err := tx.ExecContext(ctx, q); err != nil {
			return err
		}
	}

	alterColumns := []string{
		`ALTER TABLE campaign_posts ADD COLUMN auto_approved_reason TEXT NOT NULL DEFAULT '';`,
		`ALTER TABLE campaign_posts ADD COLUMN blocked_reason TEXT NOT NULL DEFAULT '';`,
	}
	for _, q := range alterColumns {
		if _, err := tx.ExecContext(ctx, q); err != nil {
			if strings.Contains(strings.ToLower(err.Error()), "duplicate column name") {
				continue
			}
			return err
		}
	}

	if err := seedDefaultSafetyRulesTx(ctx, tx); err != nil {
		return err
	}
	return nil
}

func seedDefaultSafetyRulesTx(ctx context.Context, tx *sql.Tx) error {
	now := time.Now().UTC().Format(time.RFC3339Nano)
	defaults := defaultSafetyRules(now)
	for _, r := range defaults {
		exists, err := ruleExistsTx(ctx, tx, r)
		if err != nil {
			return err
		}
		if exists {
			continue
		}
		paramsJSON, err := json.Marshal(r.Params)
		if err != nil {
			return err
		}
		platformArg := sql.NullString{}
		if r.Platform != nil {
			platformArg = sql.NullString{String: string(*r.Platform), Valid: true}
		}
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO safety_rules (id, name, kind, params, scope, platform, severity, enabled, created_at, updated_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		`, r.ID, r.Name, r.Kind, string(paramsJSON), r.Scope, platformArg, r.Severity, boolInt(r.Enabled), now, now); err != nil {
			return err
		}
	}
	return nil
}

func ruleExistsTx(ctx context.Context, tx *sql.Tx, r domain.SafetyRule) (bool, error) {
	platformArg := sql.NullString{}
	if r.Platform != nil {
		platformArg = sql.NullString{String: string(*r.Platform), Valid: true}
	}
	var count int
	err := tx.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM safety_rules WHERE kind = ? AND COALESCE(platform, '') = COALESCE(?, '') AND scope = ?
	`, r.Kind, platformArg, r.Scope).Scan(&count)
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

// defaultSafetyRules returns the seeded defaults keyed by (kind, platform).
// IDs are deterministic so re-seeding attempts hit the existence check.
func defaultSafetyRules(now string) []domain.SafetyRule {
	x := domain.PlatformX
	li := domain.PlatformLinkedIn
	fb := domain.PlatformFacebook
	ig := domain.PlatformInstagram
	return []domain.SafetyRule{
		{ID: "sft_len_x", Name: "X length range", Kind: domain.RuleLengthRange, Params: domain.SafetyRuleParams{MinLen: 1, MaxLen: 280}, Scope: domain.ScopeGlobal, Platform: &x, Severity: domain.SeverityBlock, Enabled: true},
		{ID: "sft_len_linkedin", Name: "LinkedIn length range", Kind: domain.RuleLengthRange, Params: domain.SafetyRuleParams{MinLen: 1, MaxLen: 3000}, Scope: domain.ScopeGlobal, Platform: &li, Severity: domain.SeverityBlock, Enabled: true},
		{ID: "sft_len_facebook", Name: "Facebook length range", Kind: domain.RuleLengthRange, Params: domain.SafetyRuleParams{MinLen: 1, MaxLen: 63206}, Scope: domain.ScopeGlobal, Platform: &fb, Severity: domain.SeverityBlock, Enabled: true},
		{ID: "sft_len_instagram", Name: "Instagram length range", Kind: domain.RuleLengthRange, Params: domain.SafetyRuleParams{MinLen: 1, MaxLen: 2200}, Scope: domain.ScopeGlobal, Platform: &ig, Severity: domain.SeverityBlock, Enabled: true},
		{ID: "sft_hashtag_x", Name: "X hashtag max", Kind: domain.RuleHashtagMax, Params: domain.SafetyRuleParams{HashtagMax: 10}, Scope: domain.ScopeGlobal, Platform: &x, Severity: domain.SeverityBlock, Enabled: true},
		{ID: "sft_hashtag_linkedin", Name: "LinkedIn hashtag max", Kind: domain.RuleHashtagMax, Params: domain.SafetyRuleParams{HashtagMax: 5}, Scope: domain.ScopeGlobal, Platform: &li, Severity: domain.SeverityBlock, Enabled: true},
		{ID: "sft_hashtag_facebook", Name: "Facebook hashtag max", Kind: domain.RuleHashtagMax, Params: domain.SafetyRuleParams{HashtagMax: 10}, Scope: domain.ScopeGlobal, Platform: &fb, Severity: domain.SeverityBlock, Enabled: true},
		{ID: "sft_hashtag_instagram", Name: "Instagram hashtag max", Kind: domain.RuleHashtagMax, Params: domain.SafetyRuleParams{HashtagMax: 30}, Scope: domain.ScopeGlobal, Platform: &ig, Severity: domain.SeverityBlock, Enabled: true},
		{ID: "sft_link_global", Name: "Link max global", Kind: domain.RuleLinkMax, Params: domain.SafetyRuleParams{LinkMax: 1}, Scope: domain.ScopeGlobal, Severity: domain.SeverityBlock, Enabled: true},
		{ID: "sft_banned_global", Name: "Banned terms global", Kind: domain.RuleBannedTerms, Params: domain.SafetyRuleParams{BannedPatterns: nil}, Scope: domain.ScopeGlobal, Severity: domain.SeverityBlock, Enabled: false},
	}
}
