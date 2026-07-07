package db

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/escarface/sarepost/internal/domain"
)

// openClaimDueStore opens a fresh SQLite store for ClaimDuePosts editorial
// guard regression tests.
func openClaimDueStore(t *testing.T) *Store {
	t.Helper()
	store, err := Open(filepath.Join(t.TempDir(), "claim_due_editorial.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })
	return store
}

// seedClaimDueCampaignPost seeds a scheduled, due post linked to a campaign
// with the given editorial state. Returns the post id.
func seedClaimDueCampaignPost(t *testing.T, store *Store, editorialStatus domain.EditorialStatus, requiresApproval bool) string {
	t.Helper()
	account := createTestAccount(t, store, domain.PlatformX)
	campaign, err := store.CreateCampaign(context.Background(), domain.Campaign{Name: "claim due editorial campaign", Status: domain.CampaignStatusActive})
	if err != nil {
		t.Fatalf("create campaign: %v", err)
	}
	created, err := store.CreatePost(context.Background(), CreatePostParams{
		Post: domain.Post{
			AccountID:   account.ID,
			Platform:    domain.PlatformX,
			Text:        "editorial guard due post",
			Status:      domain.PostStatusScheduled,
			ScheduledAt: time.Now().UTC().Add(-1 * time.Minute),
			MaxAttempts: 3,
		},
		CampaignID:       campaign.ID,
		EditorialStatus:  editorialStatus,
		RequiresApproval: requiresApproval,
	})
	if err != nil {
		t.Fatalf("create post: %v", err)
	}
	return created.Post.ID
}

func claimContains(claimed []domain.Post, id string) bool {
	for _, p := range claimed {
		if p.ID == id {
			return true
		}
	}
	return false
}

// TestClaimDuePostsExcludesBlockedNeedsReviewPost asserts that a post pending
// editorial sign-off that has been blocked by the safety gate (editorial_status
// = needs_review, requires_approval = 1, blocked_reason set) is NOT claimed for
// publishing even when its scheduled_at is in the past. (R4-C1 regression.)
func TestClaimDuePostsExcludesBlockedNeedsReviewPost(t *testing.T) {
	store := openClaimDueStore(t)
	ctx := context.Background()
	postID := seedClaimDueCampaignPost(t, store, domain.EditorialStatusNeedsReview, true)
	if err := store.UpdatePostAutoApprove(ctx, postID, false, "", "blocked: banned_terms:sft_banned_global spam", time.Now().UTC()); err != nil {
		t.Fatalf("block post: %v", err)
	}

	claimed, err := store.ClaimDuePosts(ctx, 10)
	if err != nil {
		t.Fatalf("claim due posts: %v", err)
	}
	if claimContains(claimed, postID) {
		t.Fatalf("blocked needs_review+requires_approval post must NOT be claimed for publishing")
	}
}

// TestClaimDuePostsExcludesNotYetEvaluatedNeedsReviewPost asserts that a post
// pending editorial sign-off that the safety gate has NOT yet evaluated
// (editorial_status = needs_review, requires_approval = 1, auto_approved_reason
// empty) is NOT claimed. It must wait for the sweep to promote or block it.
func TestClaimDuePostsExcludesNotYetEvaluatedNeedsReviewPost(t *testing.T) {
	store := openClaimDueStore(t)
	ctx := context.Background()
	postID := seedClaimDueCampaignPost(t, store, domain.EditorialStatusNeedsReview, true)

	claimed, err := store.ClaimDuePosts(ctx, 10)
	if err != nil {
		t.Fatalf("claim due posts: %v", err)
	}
	if claimContains(claimed, postID) {
		t.Fatalf("not-yet-evaluated needs_review+requires_approval post must NOT be claimed before the safety sweep signs it off")
	}
}

// TestClaimDuePostsReturnsAutoApprovedPost asserts that a post the safety gate
// promoted to approved (editorial_status = approved, requires_approval = 0,
// auto_approved_reason set) IS claimable when due.
func TestClaimDuePostsReturnsAutoApprovedPost(t *testing.T) {
	store := openClaimDueStore(t)
	ctx := context.Background()
	postID := seedClaimDueCampaignPost(t, store, domain.EditorialStatusNeedsReview, true)
	if err := store.UpdatePostAutoApprove(ctx, postID, true, "auto:all_rules_passed", "", time.Now().UTC()); err != nil {
		t.Fatalf("approve post: %v", err)
	}

	claimed, err := store.ClaimDuePosts(ctx, 10)
	if err != nil {
		t.Fatalf("claim due posts: %v", err)
	}
	if !claimContains(claimed, postID) {
		t.Fatalf("auto-approved post must be claimable for publishing")
	}
}

// TestClaimDuePostsReturnsRequiresApprovalFalsePost asserts that a campaign post
// which does not require approval (requires_approval = 0) IS claimable even when
// editorial_status = needs_review — it never enters the safety gate.
func TestClaimDuePostsReturnsRequiresApprovalFalsePost(t *testing.T) {
	store := openClaimDueStore(t)
	ctx := context.Background()
	postID := seedClaimDueCampaignPost(t, store, domain.EditorialStatusNeedsReview, false)

	claimed, err := store.ClaimDuePosts(ctx, 10)
	if err != nil {
		t.Fatalf("claim due posts: %v", err)
	}
	if !claimContains(claimed, postID) {
		t.Fatalf("requires_approval=0 post must be claimable for publishing")
	}
}

// TestClaimDuePostsReturnsPostWithoutCampaign asserts that a post with no
// campaign_posts row (created without a CampaignID) remains claimable, preserving
// backwards compatibility for non-campaign publishing flows.
func TestClaimDuePostsReturnsPostWithoutCampaign(t *testing.T) {
	store := openClaimDueStore(t)
	ctx := context.Background()
	account := createTestAccount(t, store, domain.PlatformX)
	created, err := store.CreatePost(ctx, CreatePostParams{
		Post: domain.Post{
			AccountID:   account.ID,
			Platform:    domain.PlatformX,
			Text:        "no campaign due post",
			Status:      domain.PostStatusScheduled,
			ScheduledAt: time.Now().UTC().Add(-1 * time.Minute),
			MaxAttempts: 3,
		},
	})
	if err != nil {
		t.Fatalf("create post: %v", err)
	}

	claimed, err := store.ClaimDuePosts(ctx, 10)
	if err != nil {
		t.Fatalf("claim due posts: %v", err)
	}
	if !claimContains(claimed, created.Post.ID) {
		t.Fatalf("post without campaign must remain claimable (backwards compatibility)")
	}
}
