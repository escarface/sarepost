package domain

import "time"

type ContentSourceStatus string

const (
	ContentSourceStatusNew       ContentSourceStatus = "new"
	ContentSourceStatusProcessed ContentSourceStatus = "processed"
	ContentSourceStatusArchived  ContentSourceStatus = "archived"
)

type ContentSource struct {
	ID             string              `json:"id"`
	Title          string              `json:"title"`
	Body           string              `json:"body"`
	SourceURL      string              `json:"source_url,omitempty"`
	CampaignID     string              `json:"campaign_id,omitempty"`
	BrandProfileID string              `json:"brand_profile_id,omitempty"`
	Tags           []string            `json:"tags,omitempty"`
	Status         ContentSourceStatus `json:"status"`
	CreatedAt      time.Time           `json:"created_at"`
	UpdatedAt      time.Time           `json:"updated_at"`
}

type ContentSourceListFilter struct {
	Status          ContentSourceStatus
	IncludeArchived bool
	Tag             string
	Limit           int
}
