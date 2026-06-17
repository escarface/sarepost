package campaigns

import (
	"context"
	"errors"
	"fmt"
	"strings"

	generationapp "github.com/escarface/sarepost/internal/application/generation"
	"github.com/escarface/sarepost/internal/application/ports"
	postsapp "github.com/escarface/sarepost/internal/application/posts"
	"github.com/escarface/sarepost/internal/domain"
)

var (
	ErrDraftStoreNotConfigured     = errors.New("campaign draft store is not configured")
	ErrDraftGeneratorNotConfigured = errors.New("campaign draft generator is not configured")
	ErrDraftRegistryNotConfigured  = errors.New("campaign draft provider registry is not configured")
	ErrCampaignArchived            = postsapp.ErrCampaignArchived
	ErrAccountIDsRequired          = errors.New("account_ids are required")
)

type DraftStore interface {
	postsapp.Store
}

type TextGenerator interface {
	GenerateText(ctx context.Context, in generationapp.GenerateTextInput) (generationapp.GenerateTextOutput, error)
}

type DraftService struct {
	Store             DraftStore
	Generator         TextGenerator
	Registry          ports.ProviderRegistry
	DefaultMaxRetries int
}

type CreateDraftsInput struct {
	CampaignID        string
	AccountIDs        []string
	Idea              string
	VariantsPerPost   int
	BrandProfileID    string
	EditorialStatus   domain.EditorialStatus
	RequiresApproval  *bool
	Tags              []string
	IdempotencyPrefix string
}

type CreateDraftsOutput struct {
	Campaign     domain.Campaign `json:"campaign"`
	Posts        []domain.Post   `json:"posts"`
	CreatedCount int             `json:"created_count"`
}

func (s DraftService) CreateDrafts(ctx context.Context, in CreateDraftsInput) (CreateDraftsOutput, error) {
	if s.Store == nil {
		return CreateDraftsOutput{}, ErrDraftStoreNotConfigured
	}
	if s.Generator == nil {
		return CreateDraftsOutput{}, ErrDraftGeneratorNotConfigured
	}
	if s.Registry == nil {
		return CreateDraftsOutput{}, ErrDraftRegistryNotConfigured
	}
	campaignID := strings.TrimSpace(in.CampaignID)
	if campaignID == "" {
		return CreateDraftsOutput{}, ErrCampaignIDRequired
	}
	accountIDs := postsapp.NormalizeAccountIDs("", in.AccountIDs)
	if len(accountIDs) == 0 {
		return CreateDraftsOutput{}, ErrAccountIDsRequired
	}
	campaign, err := s.Store.GetCampaign(ctx, campaignID)
	if err != nil {
		return CreateDraftsOutput{}, mapCampaignNotFound(err)
	}
	if campaign.Status == domain.CampaignStatusArchived {
		return CreateDraftsOutput{}, ErrCampaignArchived
	}
	variants := in.VariantsPerPost
	if variants <= 0 {
		variants = 1
	}
	if variants > 10 {
		variants = 10
	}
	editorialStatus := in.EditorialStatus
	if editorialStatus == "" {
		editorialStatus = domain.EditorialStatusNeedsReview
	}
	requiresApproval := true
	if in.RequiresApproval != nil {
		requiresApproval = *in.RequiresApproval
	}
	tags := mergeTags(campaign.Tags, in.Tags)
	brandProfileID := strings.TrimSpace(in.BrandProfileID)
	if brandProfileID == "" {
		brandProfileID = strings.TrimSpace(campaign.BrandProfileID)
	}
	createSvc := postsapp.CreateService{
		Store:             s.Store,
		Registry:          s.Registry,
		DefaultMaxRetries: s.DefaultMaxRetries,
	}

	out := CreateDraftsOutput{Campaign: campaign}
	for _, accountID := range accountIDs {
		account, err := s.Store.GetAccount(ctx, accountID)
		if err != nil {
			return CreateDraftsOutput{}, err
		}
		for variant := 1; variant <= variants; variant++ {
			generated, err := s.Generator.GenerateText(ctx, generationapp.GenerateTextInput{
				Prompt:         buildCampaignDraftPrompt(campaign, account.Platform, strings.TrimSpace(in.Idea), variant, variants),
				Platform:       account.Platform,
				BrandProfileID: brandProfileID,
				MaxTokens:      500,
			})
			if err != nil {
				return CreateDraftsOutput{}, err
			}
			idempotencyKey := strings.TrimSpace(in.IdempotencyPrefix)
			if idempotencyKey != "" {
				idempotencyKey = fmt.Sprintf("%s:%s:%d", idempotencyKey, account.ID, variant)
			}
			created, err := createSvc.Create(ctx, postsapp.CreateInput{
				AccountIDs:       []string{account.ID},
				Text:             generated.Text,
				CampaignID:       campaign.ID,
				EditorialStatus:  editorialStatus,
				RequiresApproval: requiresApproval,
				Tags:             tags,
				IdempotencyKey:   idempotencyKey,
			})
			if err != nil {
				return CreateDraftsOutput{}, err
			}
			out.CreatedCount += created.CreatedCount
			for _, item := range created.Items {
				out.Posts = append(out.Posts, item.Post)
			}
		}
	}
	return out, nil
}

func buildCampaignDraftPrompt(campaign domain.Campaign, platform domain.Platform, idea string, variant, total int) string {
	var b strings.Builder
	b.WriteString("Create a social post draft from this editorial campaign brief.")
	b.WriteString("\nCampaign: ")
	b.WriteString(strings.TrimSpace(campaign.Name))
	if objective := strings.TrimSpace(campaign.Objective); objective != "" {
		b.WriteString("\nObjective: ")
		b.WriteString(objective)
	}
	if audience := strings.TrimSpace(campaign.Audience); audience != "" {
		b.WriteString("\nAudience: ")
		b.WriteString(audience)
	}
	if tone := strings.TrimSpace(campaign.Tone); tone != "" {
		b.WriteString("\nTone: ")
		b.WriteString(tone)
	}
	if cta := strings.TrimSpace(campaign.CTA); cta != "" {
		b.WriteString("\nCTA: ")
		b.WriteString(cta)
	}
	if restrictions := strings.TrimSpace(campaign.Restrictions); restrictions != "" {
		b.WriteString("\nRestrictions: ")
		b.WriteString(restrictions)
	}
	if idea != "" {
		b.WriteString("\nIdea: ")
		b.WriteString(idea)
	}
	b.WriteString("\nPlatform: ")
	b.WriteString(string(platform))
	if total > 1 {
		b.WriteString(fmt.Sprintf("\nVariant: %d of %d. Make this variant meaningfully different from the others.", variant, total))
	}
	b.WriteString("\nReturn only the final post copy.")
	return b.String()
}

func mergeTags(left, right []string) []string {
	return normalizeTags(append(append([]string(nil), left...), right...))
}
