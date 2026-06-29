package safetygate

import (
	"context"
	"errors"
	"math/rand"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/escarface/sarepost/internal/domain"
)

func xPlatform() *domain.Platform { p := domain.PlatformX; return &p }

func TestEvaluateAllRulesPassApproved(t *testing.T) {
	rules := []domain.SafetyRule{
		{ID: "sft_ban", Kind: domain.RuleBannedTerms, Params: domain.SafetyRuleParams{BannedPatterns: []string{"spam\\b"}}, Severity: domain.SeverityBlock, Enabled: true, Platform: xPlatform()},
		{ID: "sft_len", Kind: domain.RuleLengthRange, Params: domain.SafetyRuleParams{MinLen: 1, MaxLen: 280}, Severity: domain.SeverityBlock, Enabled: true, Platform: xPlatform()},
	}
	post := domain.Post{ID: "pst_1", Platform: domain.PlatformX, Text: "Hello world"}

	verdict := Evaluate(context.Background(), post, rules)
	if verdict.Status != domain.VerdictApproved {
		t.Fatalf("expected approved, got %s", verdict.Status)
	}
	if len(verdict.FailedBlocks) != 0 {
		t.Fatalf("expected no failed blocks, got %v", verdict.FailedBlocks)
	}
	if !strings.Contains(verdict.AuditedAs, "banned_terms:pass") || !strings.Contains(verdict.AuditedAs, "length_range:pass") {
		t.Fatalf("audited-as missing pass entries: %q", verdict.AuditedAs)
	}
}

func TestEvaluateBlockRuleFailsNeedsManualReview(t *testing.T) {
	rules := []domain.SafetyRule{
		{ID: "sft_ban", Kind: domain.RuleBannedTerms, Params: domain.SafetyRuleParams{BannedPatterns: []string{"spam\\b"}}, Severity: domain.SeverityBlock, Enabled: true, Platform: xPlatform()},
		{ID: "sft_len", Kind: domain.RuleLengthRange, Params: domain.SafetyRuleParams{MinLen: 1, MaxLen: 280}, Severity: domain.SeverityBlock, Enabled: true, Platform: xPlatform()},
	}
	post := domain.Post{ID: "pst_2", Platform: domain.PlatformX, Text: "buy spam now click spam spam"}

	verdict := Evaluate(context.Background(), post, rules)
	if verdict.Status != domain.VerdictNeedsManual {
		t.Fatalf("expected needs_manual_review, got %s", verdict.Status)
	}
	if !contains(verdict.FailedBlocks, "sft_ban") {
		t.Fatalf("expected sft_ban in failed blocks, got %v", verdict.FailedBlocks)
	}
	if contains(verdict.FailedBlocks, "sft_len") {
		t.Fatalf("sft_len should not fail (len within range), got %v", verdict.FailedBlocks)
	}
}

func TestEvaluateVerdictStableAcrossIterationOrder(t *testing.T) {
	rules := []domain.SafetyRule{
		{ID: "sft_ban", Kind: domain.RuleBannedTerms, Params: domain.SafetyRuleParams{BannedPatterns: []string{"spam"}}, Severity: domain.SeverityBlock, Enabled: true},
		{ID: "sft_len", Kind: domain.RuleLengthRange, Params: domain.SafetyRuleParams{MinLen: 1, MaxLen: 280}, Severity: domain.SeverityBlock, Enabled: true},
		{ID: "sft_link", Kind: domain.RuleLinkMax, Params: domain.SafetyRuleParams{LinkMax: 1}, Severity: domain.SeverityBlock, Enabled: true},
		{ID: "sft_hash", Kind: domain.RuleHashtagMax, Params: domain.SafetyRuleParams{HashtagMax: 10}, Severity: domain.SeverityBlock, Enabled: true},
	}
	post := domain.Post{ID: "pst_3", Platform: domain.PlatformX, Text: "buy spam now"}

	base := Evaluate(context.Background(), post, rules)
	// Run many shuffled orderings; the verdict must be identical.
	rng := rand.New(rand.NewSource(time.Now().UnixNano()))
	for i := 0; i < 50; i++ {
		shuffled := make([]domain.SafetyRule, len(rules))
		copy(shuffled, rules)
		rng.Shuffle(len(shuffled), func(a, b int) { shuffled[a], shuffled[b] = shuffled[b], shuffled[a] })
		got := Evaluate(context.Background(), post, shuffled)
		if got.Status != base.Status || !sameSlice(got.FailedBlocks, base.FailedBlocks) || got.AuditedAs != base.AuditedAs {
			t.Fatalf("verdict unstable across shuffle:\n base=%+v\n got =%+v", base, got)
		}
	}
}

func TestEvaluateReviewSeverityFailStillApproved(t *testing.T) {
	rules := []domain.SafetyRule{
		{ID: "sft_ban", Kind: domain.RuleBannedTerms, Params: domain.SafetyRuleParams{BannedPatterns: []string{"spam"}}, Severity: domain.SeverityBlock, Enabled: true},
		{ID: "sft_cta", Kind: domain.RuleRequiredContains, Params: domain.SafetyRuleParams{Needles: []string{"cta:"}}, Severity: domain.SeverityReview, Enabled: true},
	}
	post := domain.Post{ID: "pst_4", Platform: domain.PlatformX, Text: "hello world"}

	verdict := Evaluate(context.Background(), post, rules)
	if verdict.Status != domain.VerdictApproved {
		t.Fatalf("review-only failure should still approve, got %s", verdict.Status)
	}
	if len(verdict.Notes) == 0 {
		t.Fatalf("expected non-blocking note for review failure")
	}
	if !contains(verdict.FailedBlocks, "sft_cta") {
		// review failures are notes, not blocks
	}
	if !strings.Contains(verdict.AuditedAs, "required_contains:fail") {
		t.Fatalf("audited-as should record review failure: %q", verdict.AuditedAs)
	}
}

func TestEvaluateDisabledRuleSkipped(t *testing.T) {
	rules := []domain.SafetyRule{
		{ID: "sft_ban", Kind: domain.RuleBannedTerms, Params: domain.SafetyRuleParams{BannedPatterns: []string{"spam"}}, Severity: domain.SeverityBlock, Enabled: false},
	}
	post := domain.Post{ID: "pst_5", Platform: domain.PlatformX, Text: "buy spam now"}

	verdict := Evaluate(context.Background(), post, rules)
	if verdict.Status != domain.VerdictApproved {
		t.Fatalf("disabled rule should be skipped -> approved, got %s", verdict.Status)
	}
	if !strings.Contains(verdict.AuditedAs, "banned_terms:skipped") {
		t.Fatalf("disabled rule should be audited as skipped: %q", verdict.AuditedAs)
	}
}

func TestEvaluatePlatformScopedRuleSkippedForOtherPlatform(t *testing.T) {
	li := domain.PlatformLinkedIn
	rules := []domain.SafetyRule{
		{ID: "sft_len_li", Kind: domain.RuleLengthRange, Params: domain.SafetyRuleParams{MaxLen: 5}, Severity: domain.SeverityBlock, Enabled: true, Platform: &li},
	}
	post := domain.Post{ID: "pst_6", Platform: domain.PlatformX, Text: "way too long for the linkedin rule but on x"}

	verdict := Evaluate(context.Background(), post, rules)
	if verdict.Status != domain.VerdictApproved {
		t.Fatalf("platform-mismatched rule should be skipped -> approved, got %s", verdict.Status)
	}
}

func TestEvaluateBannedTermsRecordsSnippet(t *testing.T) {
	rules := []domain.SafetyRule{
		{ID: "sft_ban", Kind: domain.RuleBannedTerms, Params: domain.SafetyRuleParams{BannedPatterns: []string{"spam"}}, Severity: domain.SeverityBlock, Enabled: true},
	}
	post := domain.Post{ID: "pst_7", Platform: domain.PlatformX, Text: "buy spam now"}

	verdict := Evaluate(context.Background(), post, rules)
	if verdict.Status != domain.VerdictNeedsManual {
		t.Fatalf("expected needs_manual_review, got %s", verdict.Status)
	}
	reason := blockedReason(verdict)
	if !strings.Contains(reason, "banned_terms") || !strings.Contains(reason, "spam") {
		t.Fatalf("blocked reason should include kind + snippet: %q", reason)
	}
}

func TestEvaluateLengthRangeFail(t *testing.T) {
	rules := []domain.SafetyRule{
		{ID: "sft_len", Kind: domain.RuleLengthRange, Params: domain.SafetyRuleParams{MinLen: 1, MaxLen: 10}, Severity: domain.SeverityBlock, Enabled: true},
	}
	post := domain.Post{ID: "pst_8", Platform: domain.PlatformX, Text: "this text is way too long to pass"}

	verdict := Evaluate(context.Background(), post, rules)
	if verdict.Status != domain.VerdictNeedsManual {
		t.Fatalf("expected needs_manual_review for length, got %s", verdict.Status)
	}
	if !contains(verdict.FailedBlocks, "sft_len") {
		t.Fatalf("expected sft_len in failed blocks, got %v", verdict.FailedBlocks)
	}
}

func TestEvaluateHashtagMaxFail(t *testing.T) {
	rules := []domain.SafetyRule{
		{ID: "sft_hash", Kind: domain.RuleHashtagMax, Params: domain.SafetyRuleParams{HashtagMax: 2}, Severity: domain.SeverityBlock, Enabled: true},
	}
	post := domain.Post{ID: "pst_9", Platform: domain.PlatformInstagram, Text: "a #b #c #d #e"}

	verdict := Evaluate(context.Background(), post, rules)
	if verdict.Status != domain.VerdictNeedsManual {
		t.Fatalf("expected needs_manual_review for hashtags, got %s", verdict.Status)
	}
}

func TestEvaluateLinkMaxFail(t *testing.T) {
	rules := []domain.SafetyRule{
		{ID: "sft_link", Kind: domain.RuleLinkMax, Params: domain.SafetyRuleParams{LinkMax: 0}, Severity: domain.SeverityBlock, Enabled: true},
	}
	post := domain.Post{ID: "pst_10", Platform: domain.PlatformX, Text: "visit https://example.com now"}

	verdict := Evaluate(context.Background(), post, rules)
	if verdict.Status != domain.VerdictNeedsManual {
		t.Fatalf("expected needs_manual_review for links, got %s", verdict.Status)
	}
}

func TestApproveEligibleBatchWithOneFailure(t *testing.T) {
	rules := []domain.SafetyRule{
		{ID: "sft_ban", Kind: domain.RuleBannedTerms, Params: domain.SafetyRuleParams{BannedPatterns: []string{"spam"}}, Severity: domain.SeverityBlock, Enabled: true},
	}
	posts := []domain.Post{
		{ID: "pst_a", Platform: domain.PlatformX, Text: "clean post a"},
		{ID: "pst_b", Platform: domain.PlatformX, Text: "buy spam now"},
		{ID: "pst_c", Platform: domain.PlatformX, Text: "clean post c"},
	}
	fake := &fakeStore{rules: rules, eligible: posts}
	svc := Service{Store: fake, Clock: fixedClock(t), MaxBatchSize: 100}

	summary, err := svc.ApproveEligible(context.Background())
	if err != nil {
		t.Fatalf("approve eligible: %v", err)
	}
	if summary.Evaluated != 3 || summary.Approved != 2 || summary.Blocked != 1 {
		t.Fatalf("summary wrong: %+v", summary)
	}
	if !fake.approved("pst_a") || !fake.approved("pst_c") {
		t.Fatalf("expected pst_a and pst_c approved, got %+v", fake.updates)
	}
	if _, ok := fake.blocked("pst_b"); !ok {
		t.Fatalf("expected pst_b blocked, got %+v", fake.updates)
	}
}

func TestApproveEligibleMidBatchErrorPartialSuccess(t *testing.T) {
	rules := []domain.SafetyRule{
		{ID: "sft_ban", Kind: domain.RuleBannedTerms, Params: domain.SafetyRuleParams{BannedPatterns: nil}, Severity: domain.SeverityBlock, Enabled: true},
	}
	posts := []domain.Post{
		{ID: "pst_ok", Platform: domain.PlatformX, Text: "first ok"},
		{ID: "pst_err", Platform: domain.PlatformX, Text: "second fails to persist"},
	}
	fake := &fakeStore{
		rules:    rules,
		eligible: posts,
		updateErr: map[string]error{
			"pst_err": errors.New("simulated db failure"),
		},
	}
	svc := Service{Store: fake, Clock: fixedClock(t), MaxBatchSize: 100}

	summary, err := svc.ApproveEligible(context.Background())
	if err != nil {
		t.Fatalf("approve eligible should not return top-level error on partial failure: %v", err)
	}
	if summary.Approved != 1 {
		t.Fatalf("expected 1 approved before error, got %d", summary.Approved)
	}
	if summary.Blocked != 0 {
		t.Fatalf("expected 0 blocked, got %d", summary.Blocked)
	}
	if len(summary.Errors) != 1 || !strings.Contains(summary.Errors[0].Error(), "pst_err") {
		t.Fatalf("expected one error referencing pst_err, got %+v", summary.Errors)
	}
	if !fake.approved("pst_ok") {
		t.Fatalf("prior approved post must remain committed, got %+v", fake.updates)
	}
}

func TestApproveEligibleDryRunDoesNotMutate(t *testing.T) {
	rules := []domain.SafetyRule{
		{ID: "sft_ban", Kind: domain.RuleBannedTerms, Params: domain.SafetyRuleParams{BannedPatterns: []string{"spam"}}, Severity: domain.SeverityBlock, Enabled: true},
	}
	posts := []domain.Post{
		{ID: "pst_dry_a", Platform: domain.PlatformX, Text: "clean"},
		{ID: "pst_dry_b", Platform: domain.PlatformX, Text: "spam"},
	}
	fake := &fakeStore{rules: rules, eligible: posts}
	svc := Service{Store: fake, Clock: fixedClock(t), MaxBatchSize: 100}

	summary, err := svc.PreviewEligible(context.Background(), 100)
	if err != nil {
		t.Fatalf("preview: %v", err)
	}
	if summary.Evaluated != 2 || summary.Approved != 1 || summary.Blocked != 1 {
		t.Fatalf("preview summary wrong: %+v", summary)
	}
	if len(fake.updates) != 0 {
		t.Fatalf("dry-run must not mutate, got %+v", fake.updates)
	}
}

// --- fake store + helpers ---

type updateRecord struct {
	postID        string
	approved      bool
	reason        string
	blockedReason string
}

type fakeStore struct {
	mu            sync.Mutex
	rules         []domain.SafetyRule
	eligible      []domain.Post
	updates       []updateRecord
	updateErr     map[string]error
	panicOnUpdate map[string]bool
}

func (f *fakeStore) ListSafetyRules(ctx context.Context) ([]domain.SafetyRule, error) {
	return f.rules, nil
}

func (f *fakeStore) ListEligiblePostsForAutoApprove(ctx context.Context, limit int) ([]domain.Post, error) {
	if limit <= 0 || limit > len(f.eligible) {
		return f.eligible, nil
	}
	return f.eligible[:limit], nil
}

func (f *fakeStore) UpdatePostAutoApprove(ctx context.Context, postID string, approved bool, reason, blockedReason string, now time.Time) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.panicOnUpdate[postID] {
		panic("simulated per-post panic for " + postID)
	}
	if err, ok := f.updateErr[postID]; ok {
		return err
	}
	f.updates = append(f.updates, updateRecord{postID: postID, approved: approved, reason: reason, blockedReason: blockedReason})
	return nil
}

func (f *fakeStore) approved(postID string) bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, u := range f.updates {
		if u.postID == postID && u.approved {
			return true
		}
	}
	return false
}

func (f *fakeStore) blocked(postID string) (string, bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, u := range f.updates {
		if u.postID == postID && !u.approved {
			return u.blockedReason, true
		}
	}
	return "", false
}

func fixedClock(t *testing.T) func() time.Time {
	t.Helper()
	fixed := time.Date(2026, 6, 29, 12, 0, 0, 0, time.UTC)
	return func() time.Time { return fixed }
}

func contains(slice []string, want string) bool {
	for _, s := range slice {
		if s == want {
			return true
		}
	}
	return false
}

func sameSlice(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	sortedA := append([]string(nil), a...)
	sortedB := append([]string(nil), b...)
	sort.Strings(sortedA)
	sort.Strings(sortedB)
	for i := range sortedA {
		if sortedA[i] != sortedB[i] {
			return false
		}
	}
	return true
}

func blockedReason(v domain.SafetyVerdict) string {
	return v.BlockedReason
}
