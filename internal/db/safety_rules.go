package db

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/escarface/sarepost/internal/domain"
)

// ErrSafetyRuleNotFound is returned when a rule lookup misses.
var ErrSafetyRuleNotFound = errors.New("safety rule not found")

const safetyRulesColumns = `id, name, kind, params, scope, platform, severity, enabled, created_at, updated_at`

// ListSafetyRules returns all rules (enabled and disabled). Filtering by
// enabled happens in the use case.
func (s *Store) ListSafetyRules(ctx context.Context) ([]domain.SafetyRule, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT `+safetyRulesColumns+` FROM safety_rules ORDER BY kind ASC, COALESCE(platform, '') ASC, id ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]domain.SafetyRule, 0)
	for rows.Next() {
		r, err := scanSafetyRuleRow(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

// GetSafetyRule returns a single rule by id.
func (s *Store) GetSafetyRule(ctx context.Context, id string) (domain.SafetyRule, error) {
	row := s.db.QueryRowContext(ctx, `SELECT `+safetyRulesColumns+` FROM safety_rules WHERE id = ?`, strings.TrimSpace(id))
	r, err := scanSafetyRuleRow(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return domain.SafetyRule{}, ErrSafetyRuleNotFound
		}
		return domain.SafetyRule{}, err
	}
	return r, nil
}

// UpsertSafetyRule creates a rule when ID is empty (generating a fresh id) or
// updates an existing rule by ID, preserving CreatedAt and advancing UpdatedAt.
func (s *Store) UpsertSafetyRule(ctx context.Context, rule domain.SafetyRule) (domain.SafetyRule, error) {
	rule.Name = strings.TrimSpace(rule.Name)
	if rule.Kind == "" {
		return domain.SafetyRule{}, fmt.Errorf("safety rule kind is required")
	}
	if rule.Scope == "" {
		rule.Scope = domain.ScopeGlobal
	}
	if rule.Severity == "" {
		rule.Severity = domain.SeverityBlock
	}
	paramsJSON, err := json.Marshal(rule.Params)
	if err != nil {
		return domain.SafetyRule{}, err
	}
	platformArg := sql.NullString{}
	if rule.Platform != nil {
		platformArg = sql.NullString{String: string(*rule.Platform), Valid: true}
	}
	now := time.Now().UTC()
	nowFmt := now.Format(time.RFC3339Nano)

	id := strings.TrimSpace(rule.ID)
	if id == "" {
		newID, err := NewID("sft")
		if err != nil {
			return domain.SafetyRule{}, err
		}
		id = newID
		_, err = s.db.ExecContext(ctx, `
			INSERT INTO safety_rules (id, name, kind, params, scope, platform, severity, enabled, created_at, updated_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		`, id, rule.Name, rule.Kind, string(paramsJSON), rule.Scope, platformArg, rule.Severity, boolInt(rule.Enabled), nowFmt, nowFmt)
		if err != nil {
			return domain.SafetyRule{}, err
		}
	} else {
		result, err := s.db.ExecContext(ctx, `
			UPDATE safety_rules
			SET name = ?, kind = ?, params = ?, scope = ?, platform = ?, severity = ?, enabled = ?, updated_at = ?
			WHERE id = ?
		`, rule.Name, rule.Kind, string(paramsJSON), rule.Scope, platformArg, rule.Severity, boolInt(rule.Enabled), nowFmt, id)
		if err != nil {
			return domain.SafetyRule{}, err
		}
		if rows, _ := result.RowsAffected(); rows == 0 {
			// No existing row -> insert with the provided id (treat as create).
			_, err = s.db.ExecContext(ctx, `
				INSERT INTO safety_rules (id, name, kind, params, scope, platform, severity, enabled, created_at, updated_at)
				VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
			`, id, rule.Name, rule.Kind, string(paramsJSON), rule.Scope, platformArg, rule.Severity, boolInt(rule.Enabled), nowFmt, nowFmt)
			if err != nil {
				return domain.SafetyRule{}, err
			}
		}
	}
	return s.GetSafetyRule(ctx, id)
}

// DeleteSafetyRule removes a rule by id.
func (s *Store) DeleteSafetyRule(ctx context.Context, id string) error {
	id = strings.TrimSpace(id)
	result, err := s.db.ExecContext(ctx, `DELETE FROM safety_rules WHERE id = ?`, id)
	if err != nil {
		return err
	}
	if rows, _ := result.RowsAffected(); rows == 0 {
		return ErrSafetyRuleNotFound
	}
	return nil
}

// ListEligiblePostsForAutoApprove returns posts that are safe-gate eligible:
// campaign-linked, editorial_status=needs_review, requires_approval=true, and
// no prior auto_approved_reason. Ordered by created_at for deterministic sweep.
func (s *Store) ListEligiblePostsForAutoApprove(ctx context.Context, limit int) ([]domain.Post, error) {
	if limit <= 0 {
		limit = 100
	}
	rows, err := s.db.QueryContext(ctx, `
		SELECT p.id
		FROM posts p
		JOIN campaign_posts cp ON cp.post_id = p.id
		WHERE cp.editorial_status = ?
		  AND cp.requires_approval = 1
		  AND TRIM(COALESCE(cp.auto_approved_reason, '')) = ''
		ORDER BY p.created_at ASC, p.id ASC
		LIMIT ?
	`, domain.EditorialStatusNeedsReview, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	ids := make([]string, 0, limit)
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	out := make([]domain.Post, 0, len(ids))
	for _, id := range ids {
		p, err := s.GetPost(ctx, id)
		if err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, nil
}

// UpdatePostAutoApprove records the safety gate verdict for a post. When
// approved is true the post is promoted: editorial_status=approved,
// requires_approval=0, approved_at=now, auto_approved_reason=reason,
// blocked_reason cleared. When approved is false the post stays needs_review
// and blocked_reason is set (auto_approved_reason cleared).
func (s *Store) UpdatePostAutoApprove(ctx context.Context, postID string, approved bool, reason, blockedReason string, now time.Time) error {
	postID = strings.TrimSpace(postID)
	if postID == "" {
		return fmt.Errorf("post id is required")
	}
	nowFmt := now.UTC().Format(time.RFC3339Nano)
	if approved {
		_, err := s.db.ExecContext(ctx, `
			UPDATE campaign_posts
			SET editorial_status = ?, requires_approval = 0, approved_at = ?, auto_approved_reason = ?, blocked_reason = '', updated_at = ?
			WHERE post_id = ?
		`, domain.EditorialStatusApproved, nowFmt, strings.TrimSpace(reason), nowFmt, postID)
		return err
	}
	_, err := s.db.ExecContext(ctx, `
		UPDATE campaign_posts
		SET auto_approved_reason = '', blocked_reason = ?, updated_at = ?
		WHERE post_id = ?
	`, strings.TrimSpace(blockedReason), nowFmt, postID)
	return err
}

type safetyRuleScanner interface {
	Scan(dest ...any) error
}

func scanSafetyRuleRow(scanner safetyRuleScanner) (domain.SafetyRule, error) {
	var r domain.SafetyRule
	var params, scope, severity, createdAt, updatedAt string
	var platform sql.NullString
	var enabled int
	err := scanner.Scan(&r.ID, &r.Name, &r.Kind, &params, &scope, &platform, &severity, &enabled, &createdAt, &updatedAt)
	if err != nil {
		return domain.SafetyRule{}, err
	}
	if err := json.Unmarshal([]byte(params), &r.Params); err != nil {
		return domain.SafetyRule{}, fmt.Errorf("decode safety rule params: %w", err)
	}
	r.Scope = domain.SafetyRuleScope(scope)
	if platform.Valid && strings.TrimSpace(platform.String) != "" {
		p := domain.Platform(strings.TrimSpace(platform.String))
		r.Platform = &p
	}
	r.Severity = domain.SafetyRuleSeverity(severity)
	r.Enabled = enabled == 1
	r.CreatedAt = parseTime(createdAt)
	r.UpdatedAt = parseTime(updatedAt)
	return r, nil
}
