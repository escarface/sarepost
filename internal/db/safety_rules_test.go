package db

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/escarface/sarepost/internal/domain"
)

func TestSafetyRulesListReturnsSeededDefaults(t *testing.T) {
	store := openTestStore(t)

	rules, err := store.ListSafetyRules(t.Context())
	if err != nil {
		t.Fatalf("list rules: %v", err)
	}
	if len(rules) != 10 {
		t.Fatalf("expected 10 seeded rules, got %d", len(rules))
	}
	for _, r := range rules {
		if r.ID == "" || r.Kind == "" || r.Severity == "" {
			t.Fatalf("seeded rule missing core fields: %+v", r)
		}
	}
}

func TestSafetyRuleGet(t *testing.T) {
	store := openTestStore(t)
	rules, _ := store.ListSafetyRules(t.Context())
	seeded := rules[0]

	got, err := store.GetSafetyRule(t.Context(), seeded.ID)
	if err != nil {
		t.Fatalf("get rule: %v", err)
	}
	if got.ID != seeded.ID || got.Kind != seeded.Kind {
		t.Fatalf("get returned wrong rule: %+v want %+v", got, seeded)
	}
}

func TestSafetyRuleGetNotFound(t *testing.T) {
	store := openTestStore(t)
	_, err := store.GetSafetyRule(t.Context(), "sft_does_not_exist")
	if !errors.Is(err, ErrSafetyRuleNotFound) {
		t.Fatalf("expected ErrSafetyRuleNotFound, got %v", err)
	}
}

func TestSafetyRuleUpsertCreatesWithGeneratedID(t *testing.T) {
	store := openTestStore(t)
	x := domain.PlatformX
	rule := domain.SafetyRule{
		Name:     "custom banned",
		Kind:     domain.RuleBannedTerms,
		Params:   domain.SafetyRuleParams{BannedPatterns: []string{"scam\\b"}},
		Scope:    domain.ScopeGlobal,
		Platform: &x,
		Severity: domain.SeverityBlock,
		Enabled:  true,
	}
	created, err := store.UpsertSafetyRule(t.Context(), rule)
	if err != nil {
		t.Fatalf("upsert create: %v", err)
	}
	if !strings.HasPrefix(created.ID, "sft_") {
		t.Fatalf("created rule id = %q, want sft_ prefix", created.ID)
	}
	if created.CreatedAt.IsZero() || created.UpdatedAt.IsZero() {
		t.Fatalf("created timestamps must be set: %+v", created)
	}
	if created.CreatedAt != created.UpdatedAt {
		t.Fatalf("on create CreatedAt should equal UpdatedAt")
	}

	fetched, _ := store.GetSafetyRule(t.Context(), created.ID)
	if len(fetched.Params.BannedPatterns) != 1 || fetched.Params.BannedPatterns[0] != "scam\\b" {
		t.Fatalf("persisted params wrong: %+v", fetched.Params)
	}
	if fetched.Platform == nil || *fetched.Platform != domain.PlatformX {
		t.Fatalf("persisted platform wrong: %+v", fetched.Platform)
	}
}

func TestSafetyRuleUpsertUpdatesPreservingCreatedAt(t *testing.T) {
	store := openTestStore(t)
	created, err := store.UpsertSafetyRule(t.Context(), domain.SafetyRule{
		Name:     "orig",
		Kind:     domain.RuleLinkMax,
		Params:   domain.SafetyRuleParams{LinkMax: 1},
		Scope:    domain.ScopeGlobal,
		Severity: domain.SeverityBlock,
		Enabled:  true,
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	origCreatedAt := created.CreatedAt

	// Force a measurable UpdatedAt delta.
	time.Sleep(2 * time.Millisecond)

	updated, err := store.UpsertSafetyRule(t.Context(), domain.SafetyRule{
		ID:       created.ID,
		Name:     "orig updated",
		Kind:     domain.RuleLinkMax,
		Params:   domain.SafetyRuleParams{LinkMax: 2},
		Scope:    domain.ScopeGlobal,
		Severity: domain.SeverityReview,
		Enabled:  false,
	})
	if err != nil {
		t.Fatalf("update: %v", err)
	}
	if updated.ID != created.ID {
		t.Fatalf("update changed id: %q -> %q", created.ID, updated.ID)
	}
	if !updated.CreatedAt.Equal(origCreatedAt) {
		t.Fatalf("update must preserve CreatedAt: got %v want %v", updated.CreatedAt, origCreatedAt)
	}
	if !updated.UpdatedAt.After(origCreatedAt) {
		t.Fatalf("update must advance UpdatedAt: got %v want after %v", updated.UpdatedAt, origCreatedAt)
	}
	if updated.Params.LinkMax != 2 || updated.Severity != domain.SeverityReview || updated.Enabled {
		t.Fatalf("updated fields wrong: %+v", updated)
	}
}

func TestSafetyRuleDelete(t *testing.T) {
	store := openTestStore(t)
	created, _ := store.UpsertSafetyRule(t.Context(), domain.SafetyRule{Name: "tmp", Kind: domain.RuleLinkMax, Severity: domain.SeverityBlock, Enabled: true})

	if err := store.DeleteSafetyRule(t.Context(), created.ID); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if _, err := store.GetSafetyRule(t.Context(), created.ID); !errors.Is(err, ErrSafetyRuleNotFound) {
		t.Fatalf("expected not found after delete, got %v", err)
	}
}

func TestSafetyRuleDeleteNotFound(t *testing.T) {
	store := openTestStore(t)
	if err := store.DeleteSafetyRule(t.Context(), "sft_missing"); !errors.Is(err, ErrSafetyRuleNotFound) {
		t.Fatalf("expected ErrSafetyRuleNotFound, got %v", err)
	}
}

func TestListEligiblePostsForAutoApproveFilters(t *testing.T) {
	store := openTestStore(t)
	accountID := mustSeedSafetyAccount(t, store, domain.PlatformX, "elig-acct")
	campaign, err := store.CreateCampaign(t.Context(), domain.Campaign{Name: "elig campaign", Status: domain.CampaignStatusActive})
	if err != nil {
		t.Fatalf("create campaign: %v", err)
	}

	eligible := seedCampaignPost(t, store, accountID, campaign.ID, "eligible", domain.EditorialStatusNeedsReview, true, "")
	alreadyApproved := seedCampaignPost(t, store, accountID, campaign.ID, "already approved", domain.EditorialStatusApproved, false, "")
	noApproval := seedCampaignPost(t, store, accountID, campaign.ID, "no approval needed", domain.EditorialStatusNeedsReview, false, "")
	alreadyAutoApproved := seedCampaignPost(t, store, accountID, campaign.ID, "already auto approved", domain.EditorialStatusNeedsReview, true, "length_range:pass")

	got, err := store.ListEligiblePostsForAutoApprove(t.Context(), 100)
	if err != nil {
		t.Fatalf("list eligible: %v", err)
	}
	ids := postIDs(got)
	if !containsID(ids, eligible) {
		t.Fatalf("eligible post missing from list: %v", ids)
	}
	for _, excluded := range []string{alreadyApproved, noApproval, alreadyAutoApproved} {
		if containsID(ids, excluded) {
			t.Fatalf("ineligible post %s should be excluded: %v", excluded, ids)
		}
	}
}

func TestListEligiblePostsForAutoApproveRespectsLimit(t *testing.T) {
	store := openTestStore(t)
	accountID := mustSeedSafetyAccount(t, store, domain.PlatformX, "limit-acct")
	campaign, _ := store.CreateCampaign(t.Context(), domain.Campaign{Name: "limit campaign", Status: domain.CampaignStatusActive})

	for i := 0; i < 5; i++ {
		seedCampaignPost(t, store, accountID, campaign.ID, "limited post", domain.EditorialStatusNeedsReview, true, "")
	}
	got, err := store.ListEligiblePostsForAutoApprove(t.Context(), 2)
	if err != nil {
		t.Fatalf("list eligible: %v", err)
	}
	if len(got) > 2 {
		t.Fatalf("limit not respected: got %d, want <= 2", len(got))
	}
}

func TestUpdatePostAutoApprovePromotesApproved(t *testing.T) {
	store := openTestStore(t)
	accountID := mustSeedSafetyAccount(t, store, domain.PlatformX, "promote-acct")
	campaign, _ := store.CreateCampaign(t.Context(), domain.Campaign{Name: "promote campaign", Status: domain.CampaignStatusActive})
	postID := seedCampaignPost(t, store, accountID, campaign.ID, "promote me", domain.EditorialStatusNeedsReview, true, "")

	now := time.Now().UTC().Truncate(time.Second)
	if err := store.UpdatePostAutoApprove(t.Context(), postID, true, "length_range:pass;link_max:pass", "", now); err != nil {
		t.Fatalf("update auto approve: %v", err)
	}

	post, err := store.GetPost(t.Context(), postID)
	if err != nil {
		t.Fatalf("get post: %v", err)
	}
	if post.EditorialStatus != domain.EditorialStatusApproved {
		t.Fatalf("expected approved, got %s", post.EditorialStatus)
	}
	if post.RequiresApproval {
		t.Fatalf("requires_approval should be cleared")
	}
	if post.ApprovedAt == nil || !post.ApprovedAt.Equal(now) {
		t.Fatalf("approved_at wrong: %+v want %v", post.ApprovedAt, now)
	}
	if post.AutoApprovedReason != "length_range:pass;link_max:pass" {
		t.Fatalf("auto_approved_reason wrong: %q", post.AutoApprovedReason)
	}
	if post.BlockedReason != "" {
		t.Fatalf("blocked_reason should be empty on approve, got %q", post.BlockedReason)
	}
}

func TestUpdatePostAutoApproveBlockedStaysNeedsReview(t *testing.T) {
	store := openTestStore(t)
	accountID := mustSeedSafetyAccount(t, store, domain.PlatformX, "block-acct")
	campaign, _ := store.CreateCampaign(t.Context(), domain.Campaign{Name: "block campaign", Status: domain.CampaignStatusActive})
	postID := seedCampaignPost(t, store, accountID, campaign.ID, "block me", domain.EditorialStatusNeedsReview, true, "")

	now := time.Now().UTC().Truncate(time.Second)
	if err := store.UpdatePostAutoApprove(t.Context(), postID, false, "", "banned_terms:sft_banned matched 'spam'", now); err != nil {
		t.Fatalf("update blocked: %v", err)
	}

	post, err := store.GetPost(t.Context(), postID)
	if err != nil {
		t.Fatalf("get post: %v", err)
	}
	if post.EditorialStatus != domain.EditorialStatusNeedsReview {
		t.Fatalf("blocked post must stay needs_review, got %s", post.EditorialStatus)
	}
	if !post.RequiresApproval {
		t.Fatalf("blocked post must keep requires_approval=true")
	}
	if post.BlockedReason != "banned_terms:sft_banned matched 'spam'" {
		t.Fatalf("blocked_reason wrong: %q", post.BlockedReason)
	}
	if post.AutoApprovedReason != "" {
		t.Fatalf("auto_approved_reason should be empty on block, got %q", post.AutoApprovedReason)
	}
	if post.ApprovedAt != nil {
		t.Fatalf("approved_at should stay nil on block")
	}
}

func seedCampaignPost(t *testing.T, store *Store, accountID, campaignID, text string, status domain.EditorialStatus, requiresApproval bool, autoApprovedReason string) string {
	t.Helper()
	created, err := store.CreatePost(t.Context(), CreatePostParams{
		Post: domain.Post{
			AccountID:       accountID,
			Platform:        domain.PlatformX,
			Text:            text,
			Status:          domain.PostStatusScheduled,
			ScheduledAt:     testNow(),
			MaxAttempts:     3,
			EditorialStatus: status,
		},
		CampaignID:       campaignID,
		EditorialStatus:  status,
		RequiresApproval: requiresApproval,
	})
	if err != nil {
		t.Fatalf("seed campaign post: %v", err)
	}
	if strings.TrimSpace(autoApprovedReason) != "" {
		if _, err := store.db.ExecContext(t.Context(), `UPDATE campaign_posts SET auto_approved_reason = ? WHERE post_id = ?`, autoApprovedReason, created.Post.ID); err != nil {
			t.Fatalf("set auto_approved_reason: %v", err)
		}
	}
	return created.Post.ID
}

func postIDs(posts []domain.Post) []string {
	out := make([]string, 0, len(posts))
	for _, p := range posts {
		out = append(out, p.ID)
	}
	return out
}

func containsID(ids []string, want string) bool {
	for _, id := range ids {
		if strings.TrimSpace(id) == strings.TrimSpace(want) {
			return true
		}
	}
	return false
}
