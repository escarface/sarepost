package api

import (
	"context"
	"strings"

	contentsourcesapp "github.com/escarface/sarepost/internal/application/contentsources"
	"github.com/escarface/sarepost/internal/domain"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type mcpContentSourceInput struct {
	Title          string   `json:"title"`
	Body           string   `json:"body"`
	SourceURL      string   `json:"source_url,omitempty"`
	CampaignID     string   `json:"campaign_id,omitempty"`
	BrandProfileID string   `json:"brand_profile_id,omitempty"`
	Tags           []string `json:"tags,omitempty"`
}

type mcpContentSourceUpdateInput struct {
	ContentSourceID string `json:"content_source_id"`
	mcpContentSourceInput
	Status string `json:"status,omitempty"`
}

type mcpContentSourceIDInput struct {
	ContentSourceID string `json:"content_source_id"`
}

type mcpListContentSourcesInput struct {
	Status          string `json:"status,omitempty"`
	IncludeArchived bool   `json:"include_archived,omitempty"`
	Tag             string `json:"tag,omitempty"`
	Limit           int    `json:"limit,omitempty"`
}

type mcpGenerateContentSourceAnglesInput struct {
	ContentSourceID string `json:"content_source_id"`
	Count           int    `json:"count,omitempty"`
	Instructions    string `json:"instructions,omitempty"`
}

type mcpContentSourceOutput struct {
	Source domain.ContentSource `json:"source"`
}

type mcpContentSourceListOutput struct {
	Count int                    `json:"count"`
	Items []domain.ContentSource `json:"items"`
}

type mcpContentSourceAnglesOutput struct {
	Result contentsourcesapp.GenerateAnglesOutput `json:"result"`
}

func (s Server) mcpCreateContentSourceTool(ctx context.Context, _ *mcp.CallToolRequest, in mcpContentSourceInput) (*mcp.CallToolResult, mcpContentSourceOutput, error) {
	source, err := s.contentSourceService().Create(ctx, contentsourcesapp.CreateInput{
		Title:          in.Title,
		Body:           in.Body,
		SourceURL:      in.SourceURL,
		CampaignID:     in.CampaignID,
		BrandProfileID: in.BrandProfileID,
		Tags:           in.Tags,
	})
	return nil, mcpContentSourceOutput{Source: source}, err
}

func (s Server) mcpListContentSourcesTool(ctx context.Context, _ *mcp.CallToolRequest, in mcpListContentSourcesInput) (*mcp.CallToolResult, mcpContentSourceListOutput, error) {
	items, err := s.contentSourceService().List(ctx, domain.ContentSourceListFilter{
		Status:          domain.ContentSourceStatus(strings.TrimSpace(in.Status)),
		IncludeArchived: in.IncludeArchived,
		Tag:             strings.TrimSpace(in.Tag),
		Limit:           in.Limit,
	})
	return nil, mcpContentSourceListOutput{Count: len(items), Items: items}, err
}

func (s Server) mcpGetContentSourceTool(ctx context.Context, _ *mcp.CallToolRequest, in mcpContentSourceIDInput) (*mcp.CallToolResult, mcpContentSourceOutput, error) {
	source, err := s.contentSourceService().Get(ctx, in.ContentSourceID)
	return nil, mcpContentSourceOutput{Source: source}, err
}

func (s Server) mcpUpdateContentSourceTool(ctx context.Context, _ *mcp.CallToolRequest, in mcpContentSourceUpdateInput) (*mcp.CallToolResult, mcpContentSourceOutput, error) {
	source, err := s.contentSourceService().Update(ctx, contentsourcesapp.UpdateInput{
		ID:             in.ContentSourceID,
		Title:          in.Title,
		Body:           in.Body,
		SourceURL:      in.SourceURL,
		CampaignID:     in.CampaignID,
		BrandProfileID: in.BrandProfileID,
		Tags:           in.Tags,
		Status:         domain.ContentSourceStatus(strings.TrimSpace(in.Status)),
	})
	return nil, mcpContentSourceOutput{Source: source}, err
}

func (s Server) mcpArchiveContentSourceTool(ctx context.Context, _ *mcp.CallToolRequest, in mcpContentSourceIDInput) (*mcp.CallToolResult, mcpContentSourceOutput, error) {
	source, err := s.contentSourceService().Archive(ctx, in.ContentSourceID)
	return nil, mcpContentSourceOutput{Source: source}, err
}

func (s Server) mcpGenerateContentSourceAnglesTool(ctx context.Context, _ *mcp.CallToolRequest, in mcpGenerateContentSourceAnglesInput) (*mcp.CallToolResult, mcpContentSourceAnglesOutput, error) {
	out, err := s.contentSourceService().GenerateAngles(ctx, contentsourcesapp.GenerateAnglesInput{
		ID:           in.ContentSourceID,
		Count:        in.Count,
		Instructions: in.Instructions,
	})
	return nil, mcpContentSourceAnglesOutput{Result: out}, err
}
