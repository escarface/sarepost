package db

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/escarface/sarepost/internal/domain"
)

func TestCampaignPersistenceAndBacklog(t *testing.T) {
	store, err := Open(filepath.Join(t.TempDir(), "campaigns.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer store.Close()

	account := createTestAccount(t, store, domain.PlatformX)
	campaign, err := store.CreateCampaign(t.Context(), domain.Campaign{
		Name:           "Editorial Sprint",
		Objective:      "Build product awareness",
		Status:         domain.CampaignStatusActive,
		StartsAt:       time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC),
		EndsAt:         time.Date(2026, 7, 31, 23, 59, 0, 0, time.UTC),
		Notes:          "Weekly cadence",
		Tags:           []string{"editorial", "product"},
		Timezone:       "Europe/Madrid",
		Audience:       "Founders",
		Tone:           "clear",
		CTA:            "Start trial",
		Restrictions:   "Avoid hype",
		BrandProfileID: "brand_sare",
		VisualStyle:    "technical-minimal",
		ImagePrompt:    "Warm white process blueprint",
		ImageSize:      "1024x1024",
	})
	if err != nil {
		t.Fatalf("create campaign: %v", err)
	}
	if campaign.BrandProfileID != "brand_sare" {
		t.Fatalf("expected brand profile id to persist, got %q", campaign.BrandProfileID)
	}
	if campaign.VisualStyle != "technical-minimal" || campaign.ImagePrompt != "Warm white process blueprint" || campaign.ImageSize != "1024x1024" {
		t.Fatalf("expected visual campaign fields to persist, got %#v", campaign)
	}

	postResult, err := store.CreatePost(t.Context(), CreatePostParams{
		Post: domain.Post{
			AccountID: account.ID,
			Text:      "Draft needing review",
		},
		CampaignID:       campaign.ID,
		EditorialStatus:  domain.EditorialStatusNeedsReview,
		RequiresApproval: true,
		PostTags:         []string{"product"},
	})
	if err != nil {
		t.Fatalf("create campaign draft: %v", err)
	}

	post, err := store.GetPost(t.Context(), postResult.Post.ID)
	if err != nil {
		t.Fatalf("get post: %v", err)
	}
	if post.CampaignID != campaign.ID {
		t.Fatalf("expected campaign id %q, got %q", campaign.ID, post.CampaignID)
	}
	if post.EditorialStatus != domain.EditorialStatusNeedsReview || !post.RequiresApproval || post.ApprovedAt != nil {
		t.Fatalf("unexpected editorial metadata: %#v", post)
	}

	backlog, err := store.ListEditorialBacklog(t.Context(), domain.EditorialBacklogFilter{
		CampaignID:      campaign.ID,
		EditorialStatus: domain.EditorialStatusNeedsReview,
	})
	if err != nil {
		t.Fatalf("list backlog: %v", err)
	}
	if len(backlog) != 1 || backlog[0].Post.ID != post.ID || backlog[0].Campaign.ID != campaign.ID {
		t.Fatalf("expected one campaign backlog item, got %#v", backlog)
	}

	if err := store.ApprovePost(t.Context(), post.ID); err != nil {
		t.Fatalf("approve post: %v", err)
	}
	approved, err := store.GetPost(t.Context(), post.ID)
	if err != nil {
		t.Fatalf("get approved post: %v", err)
	}
	if approved.EditorialStatus != domain.EditorialStatusApproved || approved.ApprovedAt == nil {
		t.Fatalf("expected approved metadata, got %#v", approved)
	}
}
