package publishcycle

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/escarface/sarepost/internal/db"
	"github.com/escarface/sarepost/internal/domain"
	"github.com/escarface/sarepost/internal/postflow"
)

// openEditorialGuardStore opens a fresh SQLite store for end-to-end editorial
// guard integration tests through the publish cycle runner.
func openEditorialGuardStore(t *testing.T) *db.Store {
	t.Helper()
	store, err := db.Open(filepath.Join(t.TempDir(), "editorial_guard.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })
	return store
}

// seedEditorialGuardPost seeds a scheduled, due campaign post with the given
// editorial state and returns the post id and account.
func seedEditorialGuardPost(t *testing.T, store *db.Store, editorialStatus domain.EditorialStatus, requiresApproval bool) (string, domain.SocialAccount) {
	t.Helper()
	account, err := store.UpsertAccount(context.Background(), db.UpsertAccountParams{
		Platform:          domain.PlatformX,
		DisplayName:       "Editorial Guard",
		ExternalAccountID: "edguard_" + t.Name(),
		AuthMethod:        domain.AuthMethodStatic,
		Status:            domain.AccountStatusConnected,
	})
	if err != nil {
		t.Fatalf("upsert account: %v", err)
	}
	campaign, err := store.CreateCampaign(context.Background(), domain.Campaign{Name: "edguard campaign", Status: domain.CampaignStatusActive})
	if err != nil {
		t.Fatalf("create campaign: %v", err)
	}
	created, err := store.CreatePost(context.Background(), db.CreatePostParams{
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
	return created.Post.ID, account
}

// TestRunnerDoesNotPublishBlockedPost proves the publish cycle never calls the
// provider for a post blocked by the safety gate (needs_review +
// requires_approval + blocked_reason). The ClaimDuePosts editorial guard must
// exclude it before the runner iterates. (R4-C1 end-to-end regression.)
func TestRunnerDoesNotPublishBlockedPost(t *testing.T) {
	store := openEditorialGuardStore(t)
	ctx := context.Background()
	postID, _ := seedEditorialGuardPost(t, store, domain.EditorialStatusNeedsReview, true)
	if err := store.UpdatePostAutoApprove(ctx, postID, false, "", "blocked: banned_terms", time.Now().UTC()); err != nil {
		t.Fatalf("block post: %v", err)
	}

	provider := &fakeProvider{platform: domain.PlatformX, publishExternalID: "ext_blocked"}
	runner := Runner{
		Store:        store,
		Registry:     fakeRegistry{providers: map[domain.Platform]postflow.Provider{domain.PlatformX: provider}},
		Credentials:  &fakeCredentialsStore{},
		RetryBackoff: time.Second,
		Interval:     time.Second,
	}
	runner.RunOnce(ctx)

	if provider.publishCalls != 0 {
		t.Fatalf("blocked post must NOT be published, got %d provider.Publish calls", provider.publishCalls)
	}
	post, err := store.GetPost(ctx, postID)
	if err != nil {
		t.Fatalf("get post: %v", err)
	}
	if post.Status != domain.PostStatusScheduled {
		t.Fatalf("blocked post must remain scheduled, got status %s", post.Status)
	}
}

// TestRunnerPublishesAutoApprovedPost proves a post promoted by the safety gate
// (editorial_status=approved, requires_approval=0) flows through to the provider
// and is marked published. (Positive control for the editorial guard.)
func TestRunnerPublishesAutoApprovedPost(t *testing.T) {
	store := openEditorialGuardStore(t)
	ctx := context.Background()
	postID, _ := seedEditorialGuardPost(t, store, domain.EditorialStatusNeedsReview, true)
	if err := store.UpdatePostAutoApprove(ctx, postID, true, "auto:all_rules_passed", "", time.Now().UTC()); err != nil {
		t.Fatalf("approve post: %v", err)
	}

	provider := &fakeProvider{platform: domain.PlatformX, publishExternalID: "ext_ok"}
	runner := Runner{
		Store:        store,
		Registry:     fakeRegistry{providers: map[domain.Platform]postflow.Provider{domain.PlatformX: provider}},
		Credentials:  &fakeCredentialsStore{},
		RetryBackoff: time.Second,
		Interval:     time.Second,
	}
	runner.RunOnce(ctx)

	if provider.publishCalls != 1 {
		t.Fatalf("auto-approved post must be published exactly once, got %d", provider.publishCalls)
	}
	post, err := store.GetPost(ctx, postID)
	if err != nil {
		t.Fatalf("get post: %v", err)
	}
	if post.Status != domain.PostStatusPublished {
		t.Fatalf("expected published status, got %s", post.Status)
	}
}
