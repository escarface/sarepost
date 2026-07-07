package worker

import (
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/escarface/sarepost/internal/db"
	"github.com/escarface/sarepost/internal/domain"
)

func newSafetySweepStore(t *testing.T) *db.Store {
	t.Helper()
	store, err := db.Open(filepath.Join(t.TempDir(), "safety_sweep.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })
	return store
}

func seedSweepEligiblePost(t *testing.T, store *db.Store, text string) string {
	t.Helper()
	account, err := store.UpsertAccount(t.Context(), db.UpsertAccountParams{
		Platform:          domain.PlatformX,
		DisplayName:       "Sweep X",
		ExternalAccountID: "sweep_" + strings.ReplaceAll(text, " ", "_"),
		AuthMethod:        domain.AuthMethodStatic,
		Status:            domain.AccountStatusConnected,
	})
	if err != nil {
		t.Fatalf("upsert account: %v", err)
	}
	campaign, err := store.CreateCampaign(t.Context(), domain.Campaign{Name: "sweep campaign " + text, Status: domain.CampaignStatusActive})
	if err != nil {
		t.Fatalf("create campaign: %v", err)
	}
	created, err := store.CreatePost(t.Context(), db.CreatePostParams{
		Post: domain.Post{
			AccountID:       account.ID,
			Platform:        domain.PlatformX,
			Text:            text,
			Status:          domain.PostStatusScheduled,
			ScheduledAt:     time.Now().UTC().Add(time.Hour),
			MaxAttempts:     3,
			EditorialStatus: domain.EditorialStatusNeedsReview,
		},
		CampaignID:       campaign.ID,
		EditorialStatus:  domain.EditorialStatusNeedsReview,
		RequiresApproval: true,
	})
	if err != nil {
		t.Fatalf("seed post: %v", err)
	}
	return created.Post.ID
}

func TestClaimSafetySweepLease(t *testing.T) {
	store := newSafetySweepStore(t)
	ctx := t.Context()

	// First claim with a short lease succeeds.
	got, err := store.ClaimSafetySweep(ctx, 50*time.Millisecond)
	if err != nil {
		t.Fatalf("first claim: %v", err)
	}
	if !got {
		t.Fatalf("expected first claim to succeed")
	}

	// Second claim while the lease is active must fail (no overlap).
	got2, err := store.ClaimSafetySweep(ctx, 30*time.Second)
	if err != nil {
		t.Fatalf("second claim: %v", err)
	}
	if got2 {
		t.Fatalf("expected second claim to be rejected while lease active")
	}

	// After the short lease expires, a new claim succeeds.
	time.Sleep(80 * time.Millisecond)
	got3, err := store.ClaimSafetySweep(ctx, 10*time.Millisecond)
	if err != nil {
		t.Fatalf("third claim: %v", err)
	}
	if !got3 {
		t.Fatalf("expected claim to succeed after lease expired")
	}
}

func TestRunSafetySweepPromotesEligiblePost(t *testing.T) {
	store := newSafetySweepStore(t)
	postID := seedSweepEligiblePost(t, store, "clean sweep post")

	w := Worker{Store: store, SafetySweepInterval: 30 * time.Second, SafetyBatchSize: 100}
	w.runSafetySweep(t.Context())

	post, err := store.GetPost(t.Context(), postID)
	if err != nil {
		t.Fatalf("get post: %v", err)
	}
	if post.EditorialStatus != domain.EditorialStatusApproved {
		t.Fatalf("expected sweep to promote post, got %s", post.EditorialStatus)
	}
	if post.AutoApprovedReason == "" {
		t.Fatalf("expected auto_approved_reason set by sweep")
	}
	if !strings.Contains(post.AutoApprovedReason, "sft_") {
		t.Fatalf("auto_approved_reason should include rule id: %q", post.AutoApprovedReason)
	}
}

func TestRunSafetySweepLeasePreventsDoublePromote(t *testing.T) {
	store := newSafetySweepStore(t)
	postID := seedSweepEligiblePost(t, store, "double sweep post")

	w := Worker{Store: store, SafetySweepInterval: 30 * time.Second, SafetyBatchSize: 100}

	// Hold the lease manually with a short lease, then run the sweep; the
	// sweep must skip because the lease is already held by another owner.
	if held, err := store.ClaimSafetySweep(t.Context(), 30*time.Second); err != nil || !held {
		t.Fatalf("manual claim setup failed: held=%v err=%v", held, err)
	}
	w.runSafetySweep(t.Context())

	// Post must NOT have been promoted because the sweep could not acquire the lease.
	post, err := store.GetPost(t.Context(), postID)
	if err != nil {
		t.Fatalf("get post: %v", err)
	}
	if post.EditorialStatus != domain.EditorialStatusNeedsReview {
		t.Fatalf("leased-out sweep must not promote, got %s", post.EditorialStatus)
	}

	// Force-expire the held lease by overwriting it to a past epoch-nanosecond
	// value, then run the sweep; the post is promoted exactly once.
	if err := store.SetSetting(t.Context(), "safety_sweep_lease", strconv.FormatInt(time.Now().UTC().Add(-time.Hour).UnixNano(), 10)); err != nil {
		t.Fatalf("expire lease: %v", err)
	}
	w.runSafetySweep(t.Context())
	post2, _ := store.GetPost(t.Context(), postID)
	if post2.EditorialStatus != domain.EditorialStatusApproved {
		t.Fatalf("expected post promoted after lease freed, got %s", post2.EditorialStatus)
	}
	approvedAt := post2.ApprovedAt
	// A second sweep after promotion finds no eligible post; ApprovedAt stays.
	w.runSafetySweep(t.Context())
	post3, _ := store.GetPost(t.Context(), postID)
	if post3.ApprovedAt == nil || !post3.ApprovedAt.Equal(*approvedAt) {
		t.Fatalf("ApprovedAt must not change on re-sweep of already-approved post")
	}
}

// TestReleaseSafetySweepAllowsImmediateClaim proves the lease release primitive
// frees the lease so the next claim succeeds without waiting for the lease
// duration to elapse. (R2-C2 component.)
func TestReleaseSafetySweepAllowsImmediateClaim(t *testing.T) {
	store := newSafetySweepStore(t)
	ctx := t.Context()

	held, err := store.ClaimSafetySweep(ctx, 2*time.Minute)
	if err != nil || !held {
		t.Fatalf("first claim failed: held=%v err=%v", held, err)
	}
	// Second claim while lease active must be denied.
	held2, err := store.ClaimSafetySweep(ctx, 2*time.Minute)
	if err != nil {
		t.Fatalf("second claim: %v", err)
	}
	if held2 {
		t.Fatalf("expected second claim denied while lease active")
	}
	// Release the lease.
	if err := store.ReleaseSafetySweep(ctx); err != nil {
		t.Fatalf("release lease: %v", err)
	}
	// Immediate re-claim (no sleep) must succeed.
	held3, err := store.ClaimSafetySweep(ctx, 2*time.Minute)
	if err != nil {
		t.Fatalf("re-claim: %v", err)
	}
	if !held3 {
		t.Fatalf("expected claim to succeed immediately after release")
	}
}

// TestClaimSafetySweepNumericLeaseComparison proves the lease is stored and
// compared as integer epoch-nanoseconds, not RFC3339Nano strings. RFC3339Nano
// drops trailing zeros, so lexicographic string ordering is not chronological.
// (R1-W2 regression.)
func TestClaimSafetySweepNumericLeaseComparison(t *testing.T) {
	store := newSafetySweepStore(t)
	ctx := t.Context()
	nowNs := time.Now().UTC().UnixNano()

	// Set the lease to a future epoch-nanosecond value. Numeric comparison must
	// deny the claim. (With the old RFC3339Nano string comparison the epoch
	// string '1...' would sort before the RFC3339Nano '2...' now-string and the
	// claim would wrongly succeed.)
	if err := store.SetSetting(ctx, "safety_sweep_lease", strconv.FormatInt(nowNs+int64(time.Hour), 10)); err != nil {
		t.Fatalf("set future lease: %v", err)
	}
	held, err := store.ClaimSafetySweep(ctx, 30*time.Second)
	if err != nil {
		t.Fatalf("claim against future lease: %v", err)
	}
	if held {
		t.Fatalf("claim must be denied while lease expiry is in the future")
	}

	// Set the lease to a past epoch-nanosecond value; claim must succeed.
	if err := store.SetSetting(ctx, "safety_sweep_lease", strconv.FormatInt(nowNs-int64(time.Hour), 10)); err != nil {
		t.Fatalf("set past lease: %v", err)
	}
	held2, err := store.ClaimSafetySweep(ctx, 30*time.Second)
	if err != nil {
		t.Fatalf("claim against past lease: %v", err)
	}
	if !held2 {
		t.Fatalf("claim must succeed when lease expiry is in the past")
	}
}

// TestRunSafetySweepReleasesLeaseSoNextTickProceedsImmediately proves the
// worker releases the safety-sweep lease on completion, so the next tick can
// claim immediately instead of waiting for the (default 2m) lease to expire.
// Without release-on-completion the effective cadence is the lease duration,
// not the configured interval. (R2-C2 end-to-end regression.)
func TestRunSafetySweepReleasesLeaseSoNextTickProceedsImmediately(t *testing.T) {
	store := newSafetySweepStore(t)
	ctx := t.Context()
	post1 := seedSweepEligiblePost(t, store, "release lease first")
	// Use the default 2m lease to make the regression visible: if the lease is
	// NOT released, the second sweep would be denied for ~2m.
	w := Worker{Store: store, SafetySweepInterval: 30 * time.Second, SafetyBatchSize: 100, SafetySweepLease: 2 * time.Minute}

	// First sweep acquires the lease, promotes post1, and must release the lease.
	w.runSafetySweep(ctx)
	p1, err := store.GetPost(ctx, post1)
	if err != nil {
		t.Fatalf("get post1: %v", err)
	}
	if p1.EditorialStatus != domain.EditorialStatusApproved {
		t.Fatalf("first sweep must promote post1, got %s", p1.EditorialStatus)
	}

	// Seed a second eligible post and run again IMMEDIATELY (no sleep). If the
	// lease were still held for 2m, this sweep would be denied and post2 would
	// not be promoted.
	post2 := seedSweepEligiblePost(t, store, "release lease second")
	w.runSafetySweep(ctx)
	p2, err := store.GetPost(ctx, post2)
	if err != nil {
		t.Fatalf("get post2: %v", err)
	}
	if p2.EditorialStatus != domain.EditorialStatusApproved {
		t.Fatalf("second sweep must proceed immediately after lease release (no 2m wait), got %s", p2.EditorialStatus)
	}
}
