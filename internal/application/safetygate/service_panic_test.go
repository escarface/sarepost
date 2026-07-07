package safetygate

import (
	"context"
	"strings"
	"testing"

	"github.com/escarface/sarepost/internal/domain"
)

// TestApproveEligibleRecoversPerPostPanicAndContinuesNextSweep proves the
// per-post recover() in the sweep loop: when one post's evaluation/mutation
// panics, the panic is recovered, logged with stack, recorded as an error, and
// the sweep continues to the remaining posts. A second sweep (next tick) still
// works, proving the service did not die. (R2-W1 regression.)
func TestApproveEligibleRecoversPerPostPanicAndContinuesNextSweep(t *testing.T) {
	rules := []domain.SafetyRule{
		{ID: "sft_ban", Kind: domain.RuleBannedTerms, Params: domain.SafetyRuleParams{BannedPatterns: nil}, Severity: domain.SeverityBlock, Enabled: true},
	}
	posts := []domain.Post{
		{ID: "pst_panic", Platform: domain.PlatformX, Text: "panicking post"},
		{ID: "pst_ok", Platform: domain.PlatformX, Text: "clean post"},
	}
	fake := &fakeStore{
		rules:         rules,
		eligible:      posts,
		panicOnUpdate: map[string]bool{"pst_panic": true},
	}
	svc := Service{Store: fake, Clock: fixedClock(t), MaxBatchSize: 100}

	// First sweep: pst_panic's mutation panics; per-post recover must catch it,
	// record an error, and continue to pst_ok.
	summary, err := svc.ApproveEligible(context.Background())
	if err != nil {
		t.Fatalf("approve eligible returned top-level error: %v (per-post panic must be recovered, not propagated)", err)
	}
	if summary.Evaluated != 2 {
		t.Fatalf("expected 2 evaluated, got %d", summary.Evaluated)
	}
	if !fake.approved("pst_ok") {
		t.Fatalf("pst_ok must be approved after pst_panic panicked (sweep must continue other posts)")
	}
	if fake.approved("pst_panic") {
		t.Fatalf("pst_panic must NOT be approved (it panicked)")
	}
	if len(summary.Errors) != 1 || !strings.Contains(summary.Errors[0].Error(), "pst_panic") {
		t.Fatalf("expected one error referencing pst_panic, got %+v", summary.Errors)
	}

	// Second sweep (next tick): the service is still usable; a new post is approved.
	fake.eligible = []domain.Post{{ID: "pst_ok2", Platform: domain.PlatformX, Text: "next tick post"}}
	summary2, err := svc.ApproveEligible(context.Background())
	if err != nil {
		t.Fatalf("second approve eligible: %v", err)
	}
	if summary2.Approved != 1 {
		t.Fatalf("expected 1 approved on second sweep, got %d", summary2.Approved)
	}
	if !fake.approved("pst_ok2") {
		t.Fatalf("pst_ok2 must be approved on the next sweep (service stayed alive)")
	}
}
