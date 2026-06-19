package campaigns

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	generationapp "github.com/escarface/sarepost/internal/application/generation"
	"github.com/escarface/sarepost/internal/db"
	"github.com/escarface/sarepost/internal/domain"
	"github.com/escarface/sarepost/internal/postflow"
)

type draftStore struct {
	campaigns   map[string]domain.Campaign
	accounts    map[string]domain.SocialAccount
	mediaByID   map[string]domain.Media
	createCalls []db.CreatePostParams
	createSeq   int
	mediaSeq    int
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

func (s *draftStore) GetMediaByIDs(_ context.Context, ids []string) ([]domain.Media, error) {
	out := make([]domain.Media, 0)
	for _, id := range ids {
		media, ok := s.mediaByID[strings.TrimSpace(id)]
		if !ok {
			continue
		}
		out = append(out, media)
	}
	return out, nil
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
	images  []generationapp.GenerateImageInput
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

func (g *draftGenerator) GenerateImage(_ context.Context, in generationapp.GenerateImageInput) (generationapp.GenerateImageOutput, error) {
	g.images = append(g.images, in)
	if g.err != nil {
		return generationapp.GenerateImageOutput{}, g.err
	}
	return generationapp.GenerateImageOutput{
		Data:     []byte("fake-image"),
		MimeType: "image/png",
		Model:    "mock-image",
		Provider: "mock",
	}, nil
}

func (s *draftStore) PersistGeneratedMedia(_ context.Context, data []byte, mimeType string, tags []string) (domain.Media, error) {
	s.mediaSeq++
	if s.mediaByID == nil {
		s.mediaByID = make(map[string]domain.Media)
	}
	item := domain.Media{
		ID:           fmt.Sprintf("med_%d", s.mediaSeq),
		Kind:         "image",
		OriginalName: fmt.Sprintf("generated-%d.png", s.mediaSeq),
		MimeType:     mimeType,
		SizeBytes:    int64(len(data)),
		Tags:         append([]string(nil), tags...),
	}
	s.mediaByID[item.ID] = item
	return item, nil
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

func TestDraftServiceUsesCampaignDefaultBrandProfile(t *testing.T) {
	store := &draftStore{
		campaigns: map[string]domain.Campaign{
			"cmp_1": {
				ID:             "cmp_1",
				Name:           "Launch",
				Status:         domain.CampaignStatusActive,
				BrandProfileID: "brand_campaign",
			},
		},
		accounts: map[string]domain.SocialAccount{
			"acc_x": {ID: "acc_x", Platform: domain.PlatformX, Status: domain.AccountStatusConnected},
		},
	}
	generator := &draftGenerator{}
	svc := DraftService{
		Store:     store,
		Generator: generator,
		Registry: draftProviderRegistry{providers: map[domain.Platform]postflow.Provider{
			domain.PlatformX: draftProvider{platform: domain.PlatformX},
		}},
	}

	if _, err := svc.CreateDrafts(t.Context(), CreateDraftsInput{CampaignID: "cmp_1", AccountIDs: []string{"acc_x"}}); err != nil {
		t.Fatalf("create campaign drafts: %v", err)
	}
	if len(generator.prompts) != 1 {
		t.Fatalf("expected one prompt, got %d", len(generator.prompts))
	}
	if generator.prompts[0].BrandProfileID != "brand_campaign" {
		t.Fatalf("expected campaign default brand profile, got %q", generator.prompts[0].BrandProfileID)
	}
}

func TestDraftServiceCreatesWeeklyCalendarDrafts(t *testing.T) {
	store := &draftStore{
		campaigns: map[string]domain.Campaign{
			"cmp_1": {
				ID:             "cmp_1",
				Name:           "Weekly campaign",
				Status:         domain.CampaignStatusActive,
				BrandProfileID: "brand_campaign",
				VisualStyle:    "technical-minimal",
				ImagePrompt:    "Stored campaign visual prompt",
				ImageSize:      "1080x1350",
				Tags:           []string{"weekly"},
			},
		},
		accounts: map[string]domain.SocialAccount{
			"acc_x": {ID: "acc_x", Platform: domain.PlatformX, Status: domain.AccountStatusConnected},
		},
	}
	generator := &draftGenerator{}
	svc := DraftService{
		Store:     store,
		Generator: generator,
		Registry: draftProviderRegistry{providers: map[domain.Platform]postflow.Provider{
			domain.PlatformX: draftProvider{platform: domain.PlatformX},
		}},
	}
	from := time.Date(2026, 7, 6, 9, 0, 0, 0, time.FixedZone("CEST", 2*60*60))

	out, err := svc.CreateCalendarDrafts(t.Context(), CreateCalendarDraftsInput{
		CampaignID:        "cmp_1",
		AccountIDs:        []string{"acc_x"},
		From:              from,
		Days:              2,
		Slots:             []string{"09:00", "17:00"},
		Idea:              "educate the market",
		IdempotencyPrefix: "week-1",
	})
	if err != nil {
		t.Fatalf("create calendar drafts: %v", err)
	}
	if out.CreatedCount != 4 || len(out.Items) != 4 || len(generator.prompts) != 4 {
		t.Fatalf("expected four planned drafts, got created=%d items=%d prompts=%d", out.CreatedCount, len(out.Items), len(generator.prompts))
	}
	if got := out.Items[0].PlannedAt.Format(time.RFC3339); got != "2026-07-06T09:00:00+02:00" {
		t.Fatalf("unexpected first planned time: %s", got)
	}
	if got := out.Items[3].PlannedAt.Format(time.RFC3339); got != "2026-07-07T17:00:00+02:00" {
		t.Fatalf("unexpected last planned time: %s", got)
	}
	if generator.prompts[0].BrandProfileID != "brand_campaign" {
		t.Fatalf("expected campaign default brand profile, got %q", generator.prompts[0].BrandProfileID)
	}
	if !strings.Contains(generator.prompts[0].Prompt, "Planned local time: 2026-07-06 09:00") {
		t.Fatalf("prompt missing planned time: %s", generator.prompts[0].Prompt)
	}
	for _, call := range store.createCalls {
		if !call.Post.ScheduledAt.IsZero() || call.Post.Status != domain.PostStatusDraft {
			t.Fatalf("calendar generation must create unscheduled drafts, got status=%s scheduled=%s", call.Post.Status, call.Post.ScheduledAt)
		}
		if call.EditorialStatus != domain.EditorialStatusNeedsReview || !call.RequiresApproval {
			t.Fatalf("expected review-gated draft, got status=%s requires=%v", call.EditorialStatus, call.RequiresApproval)
		}
	}
}

func TestDraftServiceCreatesCalendarDraftsWithGeneratedImages(t *testing.T) {
	store := &draftStore{
		campaigns: map[string]domain.Campaign{
			"cmp_1": {
				ID:             "cmp_1",
				Name:           "Weekly campaign",
				Status:         domain.CampaignStatusActive,
				BrandProfileID: "brand_campaign",
				VisualStyle:    "technical-minimal",
				ImagePrompt:    "Stored campaign visual prompt",
				ImageSize:      "1080x1350",
				Tags:           []string{"weekly"},
			},
		},
		accounts: map[string]domain.SocialAccount{
			"acc_ig": {ID: "acc_ig", Platform: domain.PlatformInstagram, Status: domain.AccountStatusConnected},
		},
	}
	generator := &draftGenerator{}
	svc := DraftService{
		Store:          store,
		Generator:      generator,
		ImageGenerator: generator,
		MediaStore:     store,
		Registry: draftProviderRegistry{providers: map[domain.Platform]postflow.Provider{
			domain.PlatformInstagram: draftProvider{platform: domain.PlatformInstagram},
		}},
	}
	from := time.Date(2026, 7, 6, 9, 0, 0, 0, time.FixedZone("CEST", 2*60*60))

	out, err := svc.CreateCalendarDrafts(t.Context(), CreateCalendarDraftsInput{
		CampaignID:     "cmp_1",
		AccountIDs:     []string{"acc_ig"},
		From:           from,
		Days:           1,
		Slots:          []string{"09:00"},
		Idea:           "educate the market",
		GenerateImages: true,
	})
	if err != nil {
		t.Fatalf("create calendar drafts with images: %v", err)
	}
	if out.CreatedCount != 1 || len(out.Items) != 1 {
		t.Fatalf("expected one planned draft, got created=%d items=%d", out.CreatedCount, len(out.Items))
	}
	if len(generator.images) != 1 {
		t.Fatalf("expected one generated image prompt, got %d", len(generator.images))
	}
	if generator.images[0].BrandProfileID != "brand_campaign" {
		t.Fatalf("expected campaign brand profile for image generation, got %q", generator.images[0].BrandProfileID)
	}
	if generator.images[0].Size != "1080x1350" {
		t.Fatalf("expected campaign image size override, got %q", generator.images[0].Size)
	}
	if !strings.Contains(generator.images[0].Prompt, "Instagram") || !strings.Contains(generator.images[0].Prompt, "Stored campaign visual prompt") || !strings.Contains(generator.images[0].Prompt, "technical-minimal") {
		t.Fatalf("expected platform-specific image prompt, got %q", generator.images[0].Prompt)
	}
	if len(store.createCalls) != 1 || len(store.createCalls[0].MediaIDs) != 1 {
		t.Fatalf("expected one draft with one media attachment, got %#v", store.createCalls)
	}
	if got := store.createCalls[0].MediaIDs[0]; got != "med_1" {
		t.Fatalf("expected persisted generated media med_1, got %q", got)
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
