package safetygate

import (
	"context"
	"fmt"
	"log/slog"
	"runtime/debug"
	"sort"
	"strings"
	"time"

	"github.com/escarface/sarepost/internal/domain"
)

// Store is the local infrastructure contract for the safety gate, mirroring the
// per-package Store pattern used by posts/create.go. *db.Store satisfies it.
type Store interface {
	ListSafetyRules(ctx context.Context) ([]domain.SafetyRule, error)
	ListEligiblePostsForAutoApprove(ctx context.Context, limit int) ([]domain.Post, error)
	UpdatePostAutoApprove(ctx context.Context, postID string, approved bool, reason, blockedReason string, now time.Time) error
}

// Service implements the auto-approve safety gate use cases.
type Service struct {
	Store        Store
	Clock        func() time.Time
	MaxBatchSize int
}

// ApproveSummary reports the outcome of an ApproveEligible sweep.
type ApproveSummary struct {
	Evaluated int     `json:"evaluated"`
	Approved  int     `json:"approved"`
	Blocked   int     `json:"blocked"`
	Errors    []error `json:"-"`
}

func (s Service) clock() func() time.Time {
	if s.Clock != nil {
		return s.Clock
	}
	return time.Now
}

func (s Service) maxBatch() int {
	if s.MaxBatchSize > 0 {
		return s.MaxBatchSize
	}
	return 100
}

// Evaluate applies all enabled, platform-applicable rules to a post and returns
// a deterministic verdict. The verdict is independent of rule iteration order.
// Disabled or platform-mismatched rules are audited as "skipped".
func Evaluate(ctx context.Context, post domain.Post, rules []domain.SafetyRule) domain.SafetyVerdict {
	_ = ctx
	entries := make([]domain.AuditEntry, 0, len(rules))
	var failedBlocks []string
	var notes []string
	var blockDetails []string

	// Iterate in a stable sorted order so audit output is deterministic
	// regardless of caller-provided ordering.
	ordered := make([]domain.SafetyRule, len(rules))
	copy(ordered, rules)
	sort.Slice(ordered, func(i, j int) bool { return ordered[i].ID < ordered[j].ID })

	for _, rule := range ordered {
		if !rule.AppliesTo(post.Platform) {
			entries = append(entries, domain.AuditEntry{RuleID: rule.ID, Kind: rule.Kind, Outcome: domain.AuditSkipped})
			continue
		}
		passed, detail := evaluateRule(post, rule)
		outcome := domain.AuditPass
		if !passed {
			outcome = domain.AuditFail
			switch rule.Severity {
			case domain.SeverityBlock:
				failedBlocks = append(failedBlocks, rule.ID)
				blockDetails = append(blockDetails, formatFailure(rule, detail))
			default:
				notes = append(notes, formatFailure(rule, detail))
			}
		}
		entries = append(entries, domain.AuditEntry{RuleID: rule.ID, Kind: rule.Kind, Outcome: outcome})
	}

	verdict := domain.SafetyVerdict{
		AuditedAs:    domain.BuildAuditedAs(entries),
		FailedBlocks: failedBlocks,
		Notes:        notes,
	}
	if len(failedBlocks) > 0 {
		verdict.Status = domain.VerdictNeedsManual
		verdict.BlockedReason = strings.Join(blockDetails, "; ")
	} else {
		verdict.Status = domain.VerdictApproved
	}
	return verdict
}

// ApproveEligible runs one sweep: loads rules, lists eligible posts, evaluates
// each, and persists the verdict. Per-post mutations are independent so a
// mid-batch store error does not roll back already-committed posts
// (REQ-PROMOTE partial-success scenario).
func (s Service) ApproveEligible(ctx context.Context) (ApproveSummary, error) {
	return s.runSweep(ctx, false)
}

// PreviewEligible evaluates eligible posts without persisting any mutation
// (dry-run). The summary reflects what would happen.
func (s Service) PreviewEligible(ctx context.Context, limit int) (ApproveSummary, error) {
	if limit <= 0 {
		limit = s.maxBatch()
	}
	return s.runSweepWithLimit(ctx, true, limit)
}

func (s Service) runSweep(ctx context.Context, dryRun bool) (ApproveSummary, error) {
	return s.runSweepWithLimit(ctx, dryRun, s.maxBatch())
}

func (s Service) runSweepWithLimit(ctx context.Context, dryRun bool, limit int) (ApproveSummary, error) {
	rules, err := s.Store.ListSafetyRules(ctx)
	if err != nil {
		return ApproveSummary{}, err
	}
	posts, err := s.Store.ListEligiblePostsForAutoApprove(ctx, limit)
	if err != nil {
		return ApproveSummary{}, err
	}

	summary := ApproveSummary{Errors: []error{}}
	now := s.clock()()

	for _, post := range posts {
		summary.Evaluated++
		status, err := s.evaluatePost(ctx, post, rules, dryRun, now)
		if err != nil {
			summary.Errors = append(summary.Errors, err)
			continue
		}
		switch status {
		case domain.VerdictApproved:
			summary.Approved++
		case domain.VerdictNeedsManual:
			summary.Blocked++
		}
	}
	return summary, nil
}

// evaluatePost runs Evaluate plus the (conditional) per-post mutation for one
// post. A deferred recover isolates panics to this post: a panicking rule or
// store mutation is recovered, logged with stack, and returned as an error so
// the sweep continues with the remaining posts instead of killing the worker
// goroutine (R2-W1).
func (s Service) evaluatePost(ctx context.Context, post domain.Post, rules []domain.SafetyRule, dryRun bool, now time.Time) (status domain.SafetyVerdictStatus, err error) {
	defer func() {
		if r := recover(); r != nil {
			slog.Default().Error("safety sweep per-post panic recovered, skipping post",
				"post_id", post.ID, "panic", r, "stack", string(debug.Stack()))
			err = fmt.Errorf("post %s: panic recovered: %v", post.ID, r)
		}
	}()
	verdict := Evaluate(ctx, post, rules)
	switch verdict.Status {
	case domain.VerdictApproved:
		if dryRun {
			return verdict.Status, nil
		}
		if err := s.Store.UpdatePostAutoApprove(ctx, post.ID, true, verdict.AuditedAs, "", now); err != nil {
			return verdict.Status, fmt.Errorf("post %s: %w", post.ID, err)
		}
		return verdict.Status, nil
	case domain.VerdictNeedsManual:
		if dryRun {
			return verdict.Status, nil
		}
		if err := s.Store.UpdatePostAutoApprove(ctx, post.ID, false, "", verdict.BlockedReason, now); err != nil {
			return verdict.Status, fmt.Errorf("post %s: %w", post.ID, err)
		}
		return verdict.Status, nil
	default:
		return verdict.Status, nil
	}
}

func formatFailure(rule domain.SafetyRule, detail string) string {
	detail = strings.TrimSpace(detail)
	if detail == "" {
		return string(rule.Kind) + ":" + rule.ID
	}
	return string(rule.Kind) + ":" + rule.ID + " " + detail
}
