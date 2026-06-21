package domain

import "time"

type ContentPlanStatus string

const (
	ContentPlanStatusDraft              ContentPlanStatus = "draft"
	ContentPlanStatusQueued             ContentPlanStatus = "queued"
	ContentPlanStatusGenerating         ContentPlanStatus = "generating"
	ContentPlanStatusReview             ContentPlanStatus = "review"
	ContentPlanStatusPartiallyScheduled ContentPlanStatus = "partially_scheduled"
	ContentPlanStatusScheduled          ContentPlanStatus = "scheduled"
	ContentPlanStatusCanceled           ContentPlanStatus = "canceled"
	ContentPlanStatusFailed             ContentPlanStatus = "failed"
)

type ContentPlanVariantStatus string

const (
	ContentPlanVariantPending    ContentPlanVariantStatus = "pending"
	ContentPlanVariantGenerating ContentPlanVariantStatus = "generating"
	ContentPlanVariantReady      ContentPlanVariantStatus = "ready"
	ContentPlanVariantFailed     ContentPlanVariantStatus = "failed"
	ContentPlanVariantApproved   ContentPlanVariantStatus = "approved"
	ContentPlanVariantScheduled  ContentPlanVariantStatus = "scheduled"
)

type ContentPlanJobStatus string

const (
	ContentPlanJobPending   ContentPlanJobStatus = "pending"
	ContentPlanJobRunning   ContentPlanJobStatus = "running"
	ContentPlanJobCompleted ContentPlanJobStatus = "completed"
	ContentPlanJobFailed    ContentPlanJobStatus = "failed"
	ContentPlanJobCanceled  ContentPlanJobStatus = "canceled"
)

type ContentPlan struct {
	ID        string             `json:"id"`
	Name      string             `json:"name"`
	Objective string             `json:"objective,omitempty"`
	Timezone  string             `json:"timezone"`
	StartsAt  time.Time          `json:"starts_at"`
	EndsAt    time.Time          `json:"ends_at"`
	Status    ContentPlanStatus  `json:"status"`
	Blocks    []ContentPlanBlock `json:"blocks,omitempty"`
	Items     []ContentPlanItem  `json:"items,omitempty"`
	CreatedAt time.Time          `json:"created_at"`
	UpdatedAt time.Time          `json:"updated_at"`
}

type ContentPlanBlock struct {
	ID             string   `json:"id"`
	PlanID         string   `json:"plan_id"`
	BrandProfileID string   `json:"brand_profile_id"`
	CampaignID     string   `json:"campaign_id,omitempty"`
	Instructions   string   `json:"instructions,omitempty"`
	AccountIDs     []string `json:"account_ids"`
	Weekdays       []int    `json:"weekdays"`
	Slots          []string `json:"slots"`
	GenerateImages bool     `json:"generate_images"`
	ImagePrompt    string   `json:"image_prompt,omitempty"`
	ForceWebSearch bool     `json:"force_web_search"`
}

type ContentPlanItem struct {
	ID        string               `json:"id"`
	PlanID    string               `json:"plan_id"`
	BlockID   string               `json:"block_id"`
	PlannedAt time.Time            `json:"planned_at"`
	Idea      string               `json:"idea,omitempty"`
	Position  int                  `json:"position"`
	Variants  []ContentPlanVariant `json:"variants,omitempty"`
}

type ContentPlanVariant struct {
	ID             string                   `json:"id"`
	PlanID         string                   `json:"plan_id"`
	ItemID         string                   `json:"item_id"`
	AccountID      string                   `json:"account_id"`
	Platform       Platform                 `json:"platform"`
	Text           string                   `json:"text,omitempty"`
	MediaID        string                   `json:"media_id,omitempty"`
	Status         ContentPlanVariantStatus `json:"status"`
	Error          string                   `json:"error,omitempty"`
	PostID         string                   `json:"post_id,omitempty"`
	PlannedAt      time.Time                `json:"planned_at"`
	GenerationRuns int                      `json:"generation_runs"`
}

type ContentPlanJob struct {
	ID         string               `json:"id"`
	PlanID     string               `json:"plan_id"`
	Status     ContentPlanJobStatus `json:"status"`
	LeaseUntil *time.Time           `json:"lease_until,omitempty"`
	Error      string               `json:"error,omitempty"`
	CreatedAt  time.Time            `json:"created_at"`
	UpdatedAt  time.Time            `json:"updated_at"`
}
