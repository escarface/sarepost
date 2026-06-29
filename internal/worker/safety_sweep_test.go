package worker

import (
	"path/filepath"
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

	// Force-expire the held lease by overwriting it to a past timestamp, then
	// run the sweep; the post is promoted exactly once.
	if err := store.SetSetting(t.Context(), "safety_sweep_lease", time.Now().UTC().Add(-time.Hour).Format(time.RFC3339Nano)); err != nil {
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
