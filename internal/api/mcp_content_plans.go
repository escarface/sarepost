package api

import (
	"context"
	"errors"
	"strings"
	"time"

	contentplansapp "github.com/escarface/sarepost/internal/application/contentplans"
	"github.com/escarface/sarepost/internal/domain"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type mcpContentPlanBlockInput struct {
	BrandProfileID string   `json:"brand_profile_id"`
	CampaignID     string   `json:"campaign_id,omitempty"`
	AccountIDs     []string `json:"account_ids"`
	Instructions   string   `json:"instructions,omitempty"`
	Weekdays       []string `json:"weekdays"`
	Slots          []string `json:"slots"`
	GenerateImages bool     `json:"generate_images,omitempty"`
	ImagePrompt    string   `json:"image_prompt,omitempty"`
	ForceWebSearch bool     `json:"force_web_search,omitempty"`
}

type mcpContentPlanInput struct {
	Name      string                     `json:"name"`
	Objective string                     `json:"objective,omitempty"`
	From      string                     `json:"from"`
	To        string                     `json:"to"`
	Timezone  string                     `json:"timezone"`
	Blocks    []mcpContentPlanBlockInput `json:"blocks"`
}

type mcpContentPlanIDInput struct {
	PlanID string `json:"plan_id"`
}
type mcpContentPlanUpdateInput struct {
	PlanID string `json:"plan_id"`
	mcpContentPlanInput
}
type mcpContentPlanVariantsInput struct {
	PlanID     string   `json:"plan_id"`
	VariantIDs []string `json:"variant_ids"`
}
type mcpContentPlanRegenerateInput struct {
	PlanID     string   `json:"plan_id"`
	VariantIDs []string `json:"variant_ids,omitempty"`
	ItemIDs    []string `json:"item_ids,omitempty"`
}
type mcpContentPlanVariantUpdateInput struct {
	PlanID    string `json:"plan_id"`
	VariantID string `json:"variant_id"`
	Text      string `json:"text"`
	PlannedAt string `json:"planned_at"`
	MediaID   string `json:"media_id,omitempty"`
}
type mcpListContentPlansInput struct {
	Limit int `json:"limit,omitempty"`
}
type mcpContentPlanOutput struct {
	Plan domain.ContentPlan `json:"plan"`
}
type mcpContentPlanListOutput struct {
	Count int                  `json:"count"`
	Items []domain.ContentPlan `json:"items"`
}
type mcpContentPlanPreviewOutput struct {
	Preview contentplansapp.Preview `json:"preview"`
}
type mcpContentPlanJobOutput struct {
	Job domain.ContentPlanJob `json:"job"`
}
type mcpContentPlanScheduleOutput struct {
	Result contentplansapp.ScheduleResult `json:"result"`
}

func (s Server) mcpPreviewContentPlanTool(ctx context.Context, _ *mcp.CallToolRequest, in mcpContentPlanInput) (*mcp.CallToolResult, mcpContentPlanPreviewOutput, error) {
	input, err := mcpContentPlanCreateInput(in)
	if err != nil {
		return nil, mcpContentPlanPreviewOutput{}, err
	}
	preview, err := s.contentPlanService().Preview(ctx, input)
	return nil, mcpContentPlanPreviewOutput{Preview: preview}, err
}

func (s Server) mcpCreateContentPlanTool(ctx context.Context, _ *mcp.CallToolRequest, in mcpContentPlanInput) (*mcp.CallToolResult, mcpContentPlanOutput, error) {
	input, err := mcpContentPlanCreateInput(in)
	if err != nil {
		return nil, mcpContentPlanOutput{}, err
	}
	plan, _, err := s.contentPlanService().Create(ctx, input)
	return nil, mcpContentPlanOutput{Plan: plan}, err
}

func (s Server) mcpUpdateContentPlanTool(ctx context.Context, _ *mcp.CallToolRequest, in mcpContentPlanUpdateInput) (*mcp.CallToolResult, mcpContentPlanOutput, error) {
	input, err := mcpContentPlanCreateInput(in.mcpContentPlanInput)
	if err != nil {
		return nil, mcpContentPlanOutput{}, err
	}
	plan, _, err := s.contentPlanService().Update(ctx, in.PlanID, input)
	return nil, mcpContentPlanOutput{Plan: plan}, err
}

func (s Server) mcpListContentPlansTool(ctx context.Context, _ *mcp.CallToolRequest, in mcpListContentPlansInput) (*mcp.CallToolResult, mcpContentPlanListOutput, error) {
	items, err := s.Store.ListContentPlans(ctx, in.Limit)
	return nil, mcpContentPlanListOutput{Count: len(items), Items: items}, err
}

func (s Server) mcpGetContentPlanTool(ctx context.Context, _ *mcp.CallToolRequest, in mcpContentPlanIDInput) (*mcp.CallToolResult, mcpContentPlanOutput, error) {
	plan, err := s.Store.GetContentPlan(ctx, strings.TrimSpace(in.PlanID))
	return nil, mcpContentPlanOutput{Plan: plan}, err
}

func (s Server) mcpGenerateContentPlanTool(ctx context.Context, _ *mcp.CallToolRequest, in mcpContentPlanIDInput) (*mcp.CallToolResult, mcpContentPlanJobOutput, error) {
	job, err := s.contentPlanService().Generate(ctx, in.PlanID)
	return nil, mcpContentPlanJobOutput{Job: job}, err
}

func (s Server) mcpCancelContentPlanTool(ctx context.Context, _ *mcp.CallToolRequest, in mcpContentPlanIDInput) (*mcp.CallToolResult, mcpContentPlanOutput, error) {
	if err := s.contentPlanService().Cancel(ctx, in.PlanID); err != nil {
		return nil, mcpContentPlanOutput{}, err
	}
	plan, err := s.Store.GetContentPlan(ctx, in.PlanID)
	return nil, mcpContentPlanOutput{Plan: plan}, err
}

func (s Server) mcpRetryContentPlanTool(ctx context.Context, _ *mcp.CallToolRequest, in mcpContentPlanVariantsInput) (*mcp.CallToolResult, mcpContentPlanJobOutput, error) {
	job, err := s.contentPlanService().Retry(ctx, in.PlanID, in.VariantIDs)
	return nil, mcpContentPlanJobOutput{Job: job}, err
}

func (s Server) mcpRegenerateContentPlanTool(ctx context.Context, _ *mcp.CallToolRequest, in mcpContentPlanRegenerateInput) (*mcp.CallToolResult, mcpContentPlanJobOutput, error) {
	job, err := s.contentPlanService().Regenerate(ctx, in.PlanID, in.VariantIDs, in.ItemIDs)
	return nil, mcpContentPlanJobOutput{Job: job}, err
}

func (s Server) mcpUpdateContentPlanVariantTool(ctx context.Context, _ *mcp.CallToolRequest, in mcpContentPlanVariantUpdateInput) (*mcp.CallToolResult, mcpContentPlanOutput, error) {
	plannedAt, err := time.Parse(time.RFC3339, strings.TrimSpace(in.PlannedAt))
	if err != nil {
		return nil, mcpContentPlanOutput{}, errors.New("planned_at must be RFC3339")
	}
	if err := s.contentPlanService().UpdateVariant(ctx, in.PlanID, in.VariantID, in.Text, plannedAt, in.MediaID); err != nil {
		return nil, mcpContentPlanOutput{}, err
	}
	plan, err := s.Store.GetContentPlan(ctx, in.PlanID)
	return nil, mcpContentPlanOutput{Plan: plan}, err
}

func (s Server) mcpScheduleContentPlanTool(ctx context.Context, _ *mcp.CallToolRequest, in mcpContentPlanVariantsInput) (*mcp.CallToolResult, mcpContentPlanScheduleOutput, error) {
	result, err := s.contentPlanService().Schedule(ctx, in.PlanID, in.VariantIDs)
	return nil, mcpContentPlanScheduleOutput{Result: result}, err
}

func mcpContentPlanCreateInput(in mcpContentPlanInput) (contentplansapp.CreateInput, error) {
	from, err := time.Parse(time.RFC3339, strings.TrimSpace(in.From))
	if err != nil {
		return contentplansapp.CreateInput{}, errors.New("from must be RFC3339")
	}
	to, err := time.Parse(time.RFC3339, strings.TrimSpace(in.To))
	if err != nil {
		return contentplansapp.CreateInput{}, errors.New("to must be RFC3339")
	}
	out := contentplansapp.CreateInput{Name: in.Name, Objective: in.Objective, From: from, To: to, Timezone: in.Timezone}
	for _, block := range in.Blocks {
		weekdays := make([]time.Weekday, 0, len(block.Weekdays))
		for _, raw := range block.Weekdays {
			weekday, ok := parseContentPlanWeekday(raw)
			if !ok {
				return contentplansapp.CreateInput{}, errors.New("invalid weekday: " + raw)
			}
			weekdays = append(weekdays, weekday)
		}
		out.Blocks = append(out.Blocks, contentplansapp.BlockInput{BrandProfileID: block.BrandProfileID, CampaignID: block.CampaignID, AccountIDs: block.AccountIDs, Instructions: block.Instructions, Weekdays: weekdays, Slots: block.Slots, GenerateImages: block.GenerateImages, ImagePrompt: block.ImagePrompt, ForceWebSearch: block.ForceWebSearch})
	}
	return out, nil
}
