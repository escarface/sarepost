package db

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/escarface/sarepost/internal/domain"
)

// TestListEligiblePostsForAutoApproveDoesNotStarveNewPostsBehindBlocked
// proves that blocked posts (needs_review + requires_approval=1 + blocked_reason
// set) do NOT re-enter the eligible set every sweep. Without the blocked_reason
// filter, once >=100 blocked posts exist, ORDER BY created_at ASC LIMIT 100
// re-evaluates the same 100 blocked posts forever and new clean posts starve.
// (R2-C1 regression.)
func TestListEligiblePostsForAutoApproveDoesNotStarveNewPostsBehindBlocked(t *testing.T) {
	store := openClaimDueStore(t)
	ctx := context.Background()
	campaign, err := store.CreateCampaign(ctx, domain.Campaign{Name: "starve campaign", Status: domain.CampaignStatusActive})
	if err != nil {
		t.Fatalf("create campaign: %v", err)
	}
	account := createTestAccount(t, store, domain.PlatformX)
	baseTime := time.Now().UTC().Add(-24 * time.Hour)

	// Seed 105 blocked posts (requires_approval=1, needs_review, blocked_reason set).
	for i := 0; i < 105; i++ {
		created, err := store.CreatePost(ctx, CreatePostParams{
			Post: domain.Post{
				AccountID:   account.ID,
				Platform:    domain.PlatformX,
				Text:        fmt.Sprintf("blocked post %d", i),
				Status:      domain.PostStatusScheduled,
				ScheduledAt: time.Now().UTC().Add(time.Hour),
				MaxAttempts: 3,
			},
			CampaignID:       campaign.ID,
			EditorialStatus:  domain.EditorialStatusNeedsReview,
			RequiresApproval: true,
		})
		if err != nil {
			t.Fatalf("create blocked post %d: %v", i, err)
		}
		if err := store.UpdatePostAutoApprove(ctx, created.Post.ID, false, "", "blocked: banned_terms", time.Now().UTC()); err != nil {
			t.Fatalf("block post %d: %v", i, err)
		}
		// Force an earlier created_at so blocked posts sort before the new post.
		if _, err := store.db.ExecContext(ctx, `UPDATE posts SET created_at = ? WHERE id = ?`, baseTime.Add(time.Duration(i)*time.Second).Format(time.RFC3339Nano), created.Post.ID); err != nil {
			t.Fatalf("set blocked created_at: %v", err)
		}
	}

	// Seed 1 clean new needs_review post with a clearly later created_at.
	clean, err := store.CreatePost(ctx, CreatePostParams{
		Post: domain.Post{
			AccountID:   account.ID,
			Platform:    domain.PlatformX,
			Text:        "clean new post that must not starve",
			Status:      domain.PostStatusScheduled,
			ScheduledAt: time.Now().UTC().Add(time.Hour),
			MaxAttempts: 3,
		},
		CampaignID:       campaign.ID,
		EditorialStatus:  domain.EditorialStatusNeedsReview,
		RequiresApproval: true,
	})
	if err != nil {
		t.Fatalf("create clean post: %v", err)
	}
	if _, err := store.db.ExecContext(ctx, `UPDATE posts SET created_at = ? WHERE id = ?`, baseTime.Add(2*time.Hour).Format(time.RFC3339Nano), clean.Post.ID); err != nil {
		t.Fatalf("set clean created_at: %v", err)
	}

	eligible, err := store.ListEligiblePostsForAutoApprove(ctx, 100)
	if err != nil {
		t.Fatalf("list eligible: %v", err)
	}
	if !claimContains(eligible, clean.Post.ID) {
		t.Fatalf("clean new post must be eligible and not starved behind 105 blocked posts (got %d eligible)", len(eligible))
	}
	for _, p := range eligible {
		if p.BlockedReason != "" {
			t.Fatalf("blocked post %s must not appear in eligible set, got blocked_reason=%q", p.ID, p.BlockedReason)
		}
	}
}
