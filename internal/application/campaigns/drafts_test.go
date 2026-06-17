package campaigns

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"testing"

	generationapp "github.com/escarface/sarepost/internal/application/generation"
	"github.com/escarface/sarepost/internal/db"
	"github.com/escarface/sarepost/internal/domain"
	"github.com/escarface/sarepost/internal/postflow"
)

type draftStore struct {
	campaigns   map[string]domain.Campaign
	accounts    map[string]domain.SocialAccount
	createCalls []db.CreatePostParams
	createSeq   int
}

func (s *draftStore) GetCampaign(_ context.Context, id string) (domain.Campaign, error) {
	campaign, ok := s.campaigns[strings.TrimSpace(id)]
	if !ok {
		return domain.Campaign{}, sql.ErrNoRows
	}
	return campaign, nil
}

func (s *draftStore) GetAccount(_ context.Context, id string) (domain.SocialAccount, error) {
	account, ok := s.accounts[strings.TrimSpace(id)]
	if !ok {
		return domain.SocialAccount{}, sql.ErrNoRows
	}
	return account, nil
}

func (s *draftStore) GetMediaByIDs(context.Context, []string) ([]domain.Media, error) {
	return nil, nil
}
func (s *draftStore) GetPostByIdempotencyKey(context.Context, string) (domain.Post, error) {
	return domain.Post{}, sql.ErrNoRows
}
func (s *draftStore) ListThreadPosts(context.Context, string) ([]domain.Post, error) { return nil, nil }
func (s *draftStore) DeletePostEditable(context.Context, string) error               { return nil }

func (s *draftStore) CreatePost(_ context.Context, params db.CreatePostParams) (db.CreatePostResult, error) {
	s.createSeq++
	s.createCalls = append(s.createCalls, params)
	post := params.Post
	post.ID = fmt.Sprintf("pst_%d", s.createSeq)
	post.CampaignID = params.CampaignID
	post.EditorialStatus = params.EditorialStatus
	post.RequiresApproval = params.RequiresApproval
	post.Tags = append([]string(nil), params.PostTags...)
	return db.CreatePostResult{Post: post, Created: true}, nil
}

type draftGenerator struct {
	prompts []generationapp.GenerateTextInput
	err     error
}

func (g *draftGenerator) GenerateText(_ context.Context, in generationapp.GenerateTextInput) (generationapp.GenerateTextOutput, error) {
	g.prompts = append(g.prompts, in)
	if g.err != nil {
		return generationapp.GenerateTextOutput{}, g.err
	}
	return generationapp.GenerateTextOutput{
		Text:     "generated for " + string(in.Platform),
		Model:    "mock-text",
		Provider: "mock",
	}, nil
}

type draftProviderRegistry struct {
	providers map[domain.Platform]postflow.Provider
}

func (r draftProviderRegistry) Get(platform domain.Platform) (postflow.Provider, bool) {
	provider, ok := r.providers[platform]
	return provider, ok
}

type draftProvider struct {
	platform domain.Platform
}

func (p draftProvider) Platform() domain.Platform { return p.platform }
func (p draftProvider) ValidateDraft(context.Context, domain.SocialAccount, postflow.Draft) ([]string, error) {
	return nil, nil
}
func (p draftProvider) Publish(context.Context, domain.SocialAccount, postflow.Credentials, domain.Post, postflow.PublishOptions) (postflow.PublishResult, error) {
	return postflow.PublishResult{}, nil
}
func (p draftProvider) RefreshIfNeeded(context.Context, domain.SocialAccount, postflow.Credentials) (postflow.Credentials, bool, error) {
	return postflow.Credentials{}, false, nil
}

func TestDraftServiceCreatesGeneratedCampaignDrafts(t *testing.T) {
	store := &draftStore{
		campaigns: map[string]domain.Campaign{
			"cmp_1": {
				ID:           "cmp_1",
				Name:         "Launch",
				Objective:    "Drive demos",
				Status:       domain.CampaignStatusActive,
				Audience:     "SaaS founders",
				Tone:         "direct",
				CTA:          "Book a demo",
				Restrictions: "No hype",
				Tags:         []string{"launch"},
			},
		},
		accounts: map[string]domain.SocialAccount{
			"acc_x":  {ID: "acc_x", Platform: domain.PlatformX, Status: domain.AccountStatusConnected},
			"acc_li": {ID: "acc_li", Platform: domain.PlatformLinkedIn, Status: domain.AccountStatusConnected},
		},
	}
	generator := &draftGenerator{}
	svc := DraftService{
		Store:     store,
		Generator: generator,
		Registry: draftProviderRegistry{providers: map[domain.Platform]postflow.Provider{
			domain.PlatformX:        draftProvider{platform: domain.PlatformX},
			domain.PlatformLinkedIn: draftProvider{platform: domain.PlatformLinkedIn},
		}},
	}

	out, err := svc.CreateDrafts(t.Context(), CreateDraftsInput{
		CampaignID:      "cmp_1",
		AccountIDs:      []string{"acc_x", "acc_li"},
		Idea:            "announce the waitlist",
		VariantsPerPost: 2,
		Tags:            []string{"campaign", "variant"},
		BrandProfileID:  "brand_1",
	})
	if err != nil {
		t.Fatalf("create campaign drafts: %v", err)
	}
	if out.CreatedCount != 4 || len(out.Posts) != 4 {
		t.Fatalf("expected four drafts, got created=%d posts=%d", out.CreatedCount, len(out.Posts))
	}
	if len(generator.prompts) != 4 {
		t.Fatalf("expected four generation prompts, got %d", len(generator.prompts))
	}
	if generator.prompts[0].Platform != domain.PlatformX || generator.prompts[0].BrandProfileID != "brand_1" {
		t.Fatalf("expected platform and brand profile to be passed to generator: %#v", generator.prompts[0])
	}
	if !strings.Contains(generator.prompts[0].Prompt, "Drive demos") || !strings.Contains(generator.prompts[0].Prompt, "announce the waitlist") {
		t.Fatalf("prompt missing campaign brief or idea: %s", generator.prompts[0].Prompt)
	}
	for _, call := range store.createCalls {
		if call.CampaignID != "cmp_1" {
			t.Fatalf("expected campaign linkage, got %q", call.CampaignID)
		}
		if call.EditorialStatus != domain.EditorialStatusNeedsReview || !call.RequiresApproval {
			t.Fatalf("expected review-gated draft, got status=%s requires=%v", call.EditorialStatus, call.RequiresApproval)
		}
		if len(call.PostTags) != 3 || call.PostTags[0] != "launch" {
			t.Fatalf("expected campaign tags merged with input tags, got %#v", call.PostTags)
		}
		if !call.Post.ScheduledAt.IsZero() || call.Post.Status != domain.PostStatusDraft {
			t.Fatalf("expected unscheduled draft, got status=%s scheduled=%s", call.Post.Status, call.Post.ScheduledAt)
		}
	}
}

func TestDraftServiceRejectsArchivedCampaign(t *testing.T) {
	store := &draftStore{
		campaigns: map[string]domain.Campaign{
			"cmp_archived": {ID: "cmp_archived", Name: "Old", Status: domain.CampaignStatusArchived},
		},
		accounts: map[string]domain.SocialAccount{
			"acc_x": {ID: "acc_x", Platform: domain.PlatformX, Status: domain.AccountStatusConnected},
		},
	}
	svc := DraftService{
		Store:     store,
		Generator: &draftGenerator{},
		Registry: draftProviderRegistry{providers: map[domain.Platform]postflow.Provider{
			domain.PlatformX: draftProvider{platform: domain.PlatformX},
		}},
	}
	_, err := svc.CreateDrafts(t.Context(), CreateDraftsInput{CampaignID: "cmp_archived", AccountIDs: []string{"acc_x"}, Idea: "idea"})
	if !errors.Is(err, ErrCampaignArchived) {
		t.Fatalf("expected ErrCampaignArchived, got %v", err)
	}
}
