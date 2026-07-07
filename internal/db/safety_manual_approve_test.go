package db

import (
	"testing"

	"github.com/escarface/sarepost/internal/domain"
)

func TestApprovePostSetsManualOverrideAndClearsBlockedReason(t *testing.T) {
	store := openTestStore(t)
	accountID := mustSeedSafetyAccount(t, store, domain.PlatformX, "manual-approve-acct")
	campaign, err := store.CreateCampaign(t.Context(), domain.Campaign{Name: "manual approve campaign", Status: domain.CampaignStatusActive})
	if err != nil {
		t.Fatalf("create campaign: %v", err)
	}

	// Seed a blocked post: needs_review, requires_approval, blocked_reason set.
	postID := seedCampaignPost(t, store, accountID, campaign.ID, "blocked post", domain.EditorialStatusNeedsReview, true, "")
	if _, err := store.db.ExecContext(t.Context(), `UPDATE campaign_posts SET blocked_reason = ? WHERE post_id = ?`, "banned_terms:sft_ban matched 'spam'", postID); err != nil {
		t.Fatalf("set blocked_reason: %v", err)
	}

	if err := store.ApprovePost(t.Context(), postID); err != nil {
		t.Fatalf("manual approve: %v", err)
	}

	post, err := store.GetPost(t.Context(), postID)
	if err != nil {
		t.Fatalf("get post: %v", err)
	}
	if post.EditorialStatus != domain.EditorialStatusApproved {
		t.Fatalf("expected approved, got %s", post.EditorialStatus)
	}
	if post.AutoApprovedReason != "manual_override" {
		t.Fatalf("expected auto_approved_reason=manual_override, got %q", post.AutoApprovedReason)
	}
	if post.BlockedReason != "" {
		t.Fatalf("manual approve must clear blocked_reason, got %q", post.BlockedReason)
	}
	if post.ApprovedAt == nil {
		t.Fatalf("approved_at must be set on manual approve")
	}
}
