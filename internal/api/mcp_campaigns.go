package api

import (
	"context"
	"errors"
	"strings"
	"time"

	campaignsapp "github.com/escarface/sarepost/internal/application/campaigns"
	"github.com/escarface/sarepost/internal/domain"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type mcpCampaignInput struct {
	Name             string   `json:"name,omitempty" jsonschema:"Campaign name."`
	Objective        string   `json:"objective,omitempty" jsonschema:"Campaign objective."`
	Status           string   `json:"status,omitempty" jsonschema:"Status: active|paused|archived."`
	StartsAt         string   `json:"starts_at,omitempty" jsonschema:"RFC3339 start date."`
	EndsAt           string   `json:"ends_at,omitempty" jsonschema:"RFC3339 end date."`
	Notes            string   `json:"notes,omitempty"`
	Tags             []string `json:"tags,omitempty"`
	Timezone         string   `json:"timezone,omitempty" jsonschema:"IANA timezone."`
	Audience         string   `json:"audience,omitempty"`
	Tone             string   `json:"tone,omitempty"`
	CTA              string   `json:"cta,omitempty"`
	Restrictions     string   `json:"restrictions,omitempty"`
	BrandProfileID   string   `json:"brand_profile_id,omitempty"`
	BrandProfileName string   `json:"brand_profile,omitempty"`
	VisualStyle      string   `json:"visual_style,omitempty"`
	ImagePrompt      string   `json:"image_prompt,omitempty"`
	ImageSize        string   `json:"image_size,omitempty"`
}

type mcpCampaignMutationInput struct {
	CampaignID string `json:"campaign_id" jsonschema:"Campaign ID."`
	mcpCampaignInput
}

type mcpListCampaignsInput struct {
	Status string `json:"status,omitempty"`
	Tag    string `json:"tag,omitempty"`
	Limit  int    `json:"limit,omitempty"`
}

type mcpAddPostToCampaignInput struct {
	CampaignID       string   `json:"campaign_id"`
	PostID           string   `json:"post_id"`
	EditorialStatus  string   `json:"editorial_status,omitempty"`
	RequiresApproval bool     `json:"requires_approval,omitempty"`
	Tags             []string `json:"tags,omitempty"`
}

type mcpEditorialBacklogInput struct {
	CampaignID      string `json:"campaign_id,omitempty"`
	Platform        string `json:"platform,omitempty"`
	EditorialStatus string `json:"editorial_status,omitempty"`
	Tag             string `json:"tag,omitempty"`
	From            string `json:"from,omitempty"`
	To              string `json:"to,omitempty"`
	Limit           int    `json:"limit,omitempty"`
}

type mcpCreateCampaignDraftsInput struct {
	CampaignID        string   `json:"campaign_id"`
	AccountID         string   `json:"account_id,omitempty"`
	AccountIDs        []string `json:"account_ids,omitempty"`
	Idea              string   `json:"idea,omitempty"`
	VariantsPerPost   int      `json:"variants_per_post,omitempty"`
	BrandProfileID    string   `json:"brand_profile_id,omitempty"`
	BrandProfileName  string   `json:"brand_profile,omitempty"`
	GenerateImages    bool     `json:"generate_images,omitempty"`
	ImagePrompt       string   `json:"image_prompt,omitempty"`
	ImageSize         string   `json:"image_size,omitempty"`
	EditorialStatus   string   `json:"editorial_status,omitempty"`
	RequiresApproval  *bool    `json:"requires_approval,omitempty"`
	Tags              []string `json:"tags,omitempty"`
	IdempotencyPrefix string   `json:"idempotency_prefix,omitempty"`
}

type mcpGenerateCampaignCalendarInput struct {
	CampaignID        string   `json:"campaign_id"`
	AccountID         string   `json:"account_id,omitempty"`
	AccountIDs        []string `json:"account_ids,omitempty"`
	Idea              string   `json:"idea,omitempty"`
	From              string   `json:"from,omitempty"`
	Days              int      `json:"days,omitempty"`
	PostsPerDay       int      `json:"posts_per_day,omitempty"`
	Slots             []string `json:"slots,omitempty"`
	BrandProfileID    string   `json:"brand_profile_id,omitempty"`
	BrandProfileName  string   `json:"brand_profile,omitempty"`
	GenerateImages    bool     `json:"generate_images,omitempty"`
	ImagePrompt       string   `json:"image_prompt,omitempty"`
	ImageSize         string   `json:"image_size,omitempty"`
	EditorialStatus   string   `json:"editorial_status,omitempty"`
	RequiresApproval  *bool    `json:"requires_approval,omitempty"`
	Tags              []string `json:"tags,omitempty"`
	IdempotencyPrefix string   `json:"idempotency_prefix,omitempty"`
}

type mcpApprovePostInput struct {
	PostID string `json:"post_id"`
}

type mcpCampaignOutput struct {
	Campaign domain.Campaign `json:"campaign"`
}

type mcpCampaignListOutput struct {
	Count int               `json:"count"`
	Items []domain.Campaign `json:"items"`
}

type mcpBacklogOutput struct {
	Count int                           `json:"count"`
	Items []domain.EditorialBacklogItem `json:"items"`
}

type mcpAddPostToCampaignOutput struct {
	Post mcpPostSummary `json:"post"`
}

type mcpCreateCampaignDraftsOutput struct {
	Campaign     domain.Campaign  `json:"campaign"`
	CreatedCount int              `json:"created_count"`
	Posts        []mcpPostSummary `json:"posts"`
}

type mcpGenerateCampaignCalendarOutput struct {
	Campaign     domain.Campaign        `json:"campaign"`
	CreatedCount int                    `json:"created_count"`
	Items        []mcpCalendarDraftItem `json:"items"`
}

type mcpCalendarDraftItem struct {
	PlannedAt string         `json:"planned_at"`
	Post      mcpPostSummary `json:"post"`
}

type mcpApprovePostOutput struct {
	Post mcpPostSummary `json:"post"`
}

func (s Server) mcpCreateCampaignTool(ctx context.Context, _ *mcp.CallToolRequest, in mcpCampaignInput) (*mcp.CallToolResult, mcpCampaignOutput, error) {
	startsAt, err := parseOptionalRFC3339(in.StartsAt, "starts_at")
	if err != nil {
		return nil, mcpCampaignOutput{}, err
	}
	endsAt, err := parseOptionalRFC3339(in.EndsAt, "ends_at")
	if err != nil {
		return nil, mcpCampaignOutput{}, err
	}
	brandProfileID, err := s.resolveBrandProfileID(ctx, in.BrandProfileID, in.BrandProfileName)
	if err != nil {
		return nil, mcpCampaignOutput{}, err
	}
	campaign, err := campaignsapp.Service{Store: s.Store}.Create(ctx, campaignsapp.CreateInput{
		Name:           in.Name,
		Objective:      in.Objective,
		StartsAt:       startsAt,
		EndsAt:         endsAt,
		Notes:          in.Notes,
		Tags:           in.Tags,
		Timezone:       in.Timezone,
		Audience:       in.Audience,
		Tone:           in.Tone,
		CTA:            in.CTA,
		Restrictions:   in.Restrictions,
		BrandProfileID: brandProfileID,
		VisualStyle:    in.VisualStyle,
		ImagePrompt:    in.ImagePrompt,
		ImageSize:      in.ImageSize,
	})
	if err != nil {
		return nil, mcpCampaignOutput{}, err
	}
	return nil, mcpCampaignOutput{Campaign: campaign}, nil
}

func (s Server) mcpListCampaignsTool(ctx context.Context, _ *mcp.CallToolRequest, in mcpListCampaignsInput) (*mcp.CallToolResult, mcpCampaignListOutput, error) {
	items, err := campaignsapp.Service{Store: s.Store}.List(ctx, campaignsapp.ListFilter{
		Status: domain.CampaignStatus(strings.TrimSpace(in.Status)),
		Tag:    strings.TrimSpace(in.Tag),
		Limit:  clampMCPListLimit(in.Limit),
	})
	if err != nil {
		return nil, mcpCampaignListOutput{}, err
	}
	return nil, mcpCampaignListOutput{Count: len(items), Items: items}, nil
}

func (s Server) mcpUpdateCampaignTool(ctx context.Context, _ *mcp.CallToolRequest, in mcpCampaignMutationInput) (*mcp.CallToolResult, mcpCampaignOutput, error) {
	startsAt, err := parseOptionalRFC3339(in.StartsAt, "starts_at")
	if err != nil {
		return nil, mcpCampaignOutput{}, err
	}
	endsAt, err := parseOptionalRFC3339(in.EndsAt, "ends_at")
	if err != nil {
		return nil, mcpCampaignOutput{}, err
	}
	brandProfileID, err := s.resolveBrandProfileID(ctx, in.BrandProfileID, in.BrandProfileName)
	if err != nil {
		return nil, mcpCampaignOutput{}, err
	}
	campaign, err := campaignsapp.Service{Store: s.Store}.Update(ctx, campaignsapp.UpdateInput{
		ID:             in.CampaignID,
		Name:           in.Name,
		Objective:      in.Objective,
		Status:         domain.CampaignStatus(strings.TrimSpace(in.Status)),
		StartsAt:       startsAt,
		EndsAt:         endsAt,
		Notes:          in.Notes,
		Tags:           in.Tags,
		Timezone:       in.Timezone,
		Audience:       in.Audience,
		Tone:           in.Tone,
		CTA:            in.CTA,
		Restrictions:   in.Restrictions,
		BrandProfileID: brandProfileID,
		VisualStyle:    in.VisualStyle,
		ImagePrompt:    in.ImagePrompt,
		ImageSize:      in.ImageSize,
	})
	if err != nil {
		return nil, mcpCampaignOutput{}, err
	}
	return nil, mcpCampaignOutput{Campaign: campaign}, nil
}

func (s Server) mcpArchiveCampaignTool(ctx context.Context, _ *mcp.CallToolRequest, in mcpCampaignMutationInput) (*mcp.CallToolResult, mcpCampaignOutput, error) {
	campaign, err := campaignsapp.Service{Store: s.Store}.Archive(ctx, in.CampaignID)
	if err != nil {
		return nil, mcpCampaignOutput{}, err
	}
	return nil, mcpCampaignOutput{Campaign: campaign}, nil
}

func (s Server) mcpAddPostToCampaignTool(ctx context.Context, _ *mcp.CallToolRequest, in mcpAddPostToCampaignInput) (*mcp.CallToolResult, mcpAddPostToCampaignOutput, error) {
	if strings.TrimSpace(in.PostID) == "" || strings.TrimSpace(in.CampaignID) == "" {
		return nil, mcpAddPostToCampaignOutput{}, errors.New("campaign_id and post_id are required")
	}
	status := domain.EditorialStatus(strings.TrimSpace(in.EditorialStatus))
	if status == "" {
		status = domain.EditorialStatusDrafting
	}
	if err := s.Store.AddPostToCampaign(ctx, in.PostID, in.CampaignID, status, in.RequiresApproval, in.Tags); err != nil {
		return nil, mcpAddPostToCampaignOutput{}, err
	}
	post, err := s.Store.GetPost(ctx, in.PostID)
	if err != nil {
		return nil, mcpAddPostToCampaignOutput{}, err
	}
	return nil, mcpAddPostToCampaignOutput{Post: toMCPPostSummary(post)}, nil
}

func (s Server) mcpListEditorialBacklogTool(ctx context.Context, _ *mcp.CallToolRequest, in mcpEditorialBacklogInput) (*mcp.CallToolResult, mcpBacklogOutput, error) {
	from, err := parseOptionalRFC3339PreserveLocation(in.From, "from")
	if err != nil {
		return nil, mcpBacklogOutput{}, err
	}
	to, err := parseOptionalRFC3339(in.To, "to")
	if err != nil {
		return nil, mcpBacklogOutput{}, err
	}
	items, err := s.Store.ListEditorialBacklog(ctx, domain.EditorialBacklogFilter{
		CampaignID:      strings.TrimSpace(in.CampaignID),
		Platform:        domain.Platform(strings.TrimSpace(in.Platform)),
		EditorialStatus: domain.EditorialStatus(strings.TrimSpace(in.EditorialStatus)),
		Tag:             strings.TrimSpace(in.Tag),
		From:            from,
		To:              to,
		Limit:           clampMCPListLimit(in.Limit),
	})
	if err != nil {
		return nil, mcpBacklogOutput{}, err
	}
	return nil, mcpBacklogOutput{Count: len(items), Items: items}, nil
}

func (s Server) mcpCreateCampaignDraftsTool(ctx context.Context, _ *mcp.CallToolRequest, in mcpCreateCampaignDraftsInput) (*mcp.CallToolResult, mcpCreateCampaignDraftsOutput, error) {
	accountIDs := append([]string(nil), in.AccountIDs...)
	if strings.TrimSpace(in.AccountID) != "" {
		accountIDs = append(accountIDs, strings.TrimSpace(in.AccountID))
	}
	brandProfileID, err := s.resolveBrandProfileID(ctx, in.BrandProfileID, in.BrandProfileName)
	if err != nil {
		return nil, mcpCreateCampaignDraftsOutput{}, err
	}
	out, err := campaignsapp.DraftService{
		Store:             s.Store,
		Generator:         s.generationService(),
		ImageGenerator:    s.generationService(),
		MediaStore:        s,
		Registry:          s.providerRegistry(),
		DefaultMaxRetries: s.DefaultMaxRetries,
	}.CreateDrafts(ctx, campaignsapp.CreateDraftsInput{
		CampaignID:        in.CampaignID,
		AccountIDs:        accountIDs,
		Idea:              in.Idea,
		VariantsPerPost:   in.VariantsPerPost,
		BrandProfileID:    brandProfileID,
		GenerateImages:    in.GenerateImages,
		ImagePrompt:       in.ImagePrompt,
		ImageSize:         in.ImageSize,
		EditorialStatus:   domain.EditorialStatus(strings.TrimSpace(in.EditorialStatus)),
		RequiresApproval:  in.RequiresApproval,
		Tags:              in.Tags,
		IdempotencyPrefix: in.IdempotencyPrefix,
	})
	if err != nil {
		return nil, mcpCreateCampaignDraftsOutput{}, err
	}
	posts := make([]mcpPostSummary, 0, len(out.Posts))
	for _, post := range out.Posts {
		posts = append(posts, toMCPPostSummary(post))
	}
	return nil, mcpCreateCampaignDraftsOutput{Campaign: out.Campaign, CreatedCount: out.CreatedCount, Posts: posts}, nil
}

func (s Server) mcpGenerateCampaignCalendarTool(ctx context.Context, _ *mcp.CallToolRequest, in mcpGenerateCampaignCalendarInput) (*mcp.CallToolResult, mcpGenerateCampaignCalendarOutput, error) {
	from, err := parseOptionalRFC3339(in.From, "from")
	if err != nil {
		return nil, mcpGenerateCampaignCalendarOutput{}, err
	}
	accountIDs := append([]string(nil), in.AccountIDs...)
	if strings.TrimSpace(in.AccountID) != "" {
		accountIDs = append(accountIDs, strings.TrimSpace(in.AccountID))
	}
	brandProfileID, err := s.resolveBrandProfileID(ctx, in.BrandProfileID, in.BrandProfileName)
	if err != nil {
		return nil, mcpGenerateCampaignCalendarOutput{}, err
	}
	out, err := campaignsapp.DraftService{
		Store:             s.Store,
		Generator:         s.generationService(),
		ImageGenerator:    s.generationService(),
		MediaStore:        s,
		Registry:          s.providerRegistry(),
		DefaultMaxRetries: s.DefaultMaxRetries,
	}.CreateCalendarDrafts(ctx, campaignsapp.CreateCalendarDraftsInput{
		CampaignID:        in.CampaignID,
		AccountIDs:        accountIDs,
		Idea:              in.Idea,
		From:              from,
		Days:              in.Days,
		PostsPerDay:       in.PostsPerDay,
		Slots:             in.Slots,
		BrandProfileID:    brandProfileID,
		GenerateImages:    in.GenerateImages,
		ImagePrompt:       in.ImagePrompt,
		ImageSize:         in.ImageSize,
		EditorialStatus:   domain.EditorialStatus(strings.TrimSpace(in.EditorialStatus)),
		RequiresApproval:  in.RequiresApproval,
		Tags:              in.Tags,
		IdempotencyPrefix: in.IdempotencyPrefix,
	})
	if err != nil {
		return nil, mcpGenerateCampaignCalendarOutput{}, err
	}
	items := make([]mcpCalendarDraftItem, 0, len(out.Items))
	for _, item := range out.Items {
		items = append(items, mcpCalendarDraftItem{
			PlannedAt: item.PlannedAt.Format(time.RFC3339),
			Post:      toMCPPostSummary(item.Post),
		})
	}
	return nil, mcpGenerateCampaignCalendarOutput{Campaign: out.Campaign, CreatedCount: out.CreatedCount, Items: items}, nil
}

func (s Server) mcpApprovePostTool(ctx context.Context, _ *mcp.CallToolRequest, in mcpApprovePostInput) (*mcp.CallToolResult, mcpApprovePostOutput, error) {
	postID := strings.TrimSpace(in.PostID)
	if postID == "" {
		return nil, mcpApprovePostOutput{}, errors.New("post_id is required")
	}
	if err := s.Store.ApprovePost(ctx, postID); err != nil {
		return nil, mcpApprovePostOutput{}, err
	}
	post, err := s.Store.GetPost(ctx, postID)
	if err != nil {
		return nil, mcpApprovePostOutput{}, err
	}
	return nil, mcpApprovePostOutput{Post: toMCPPostSummary(post)}, nil
}
