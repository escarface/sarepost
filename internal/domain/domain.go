package domain

import "time"

type Platform string

const (
	PlatformX         Platform = "x"
	PlatformLinkedIn  Platform = "linkedin"
	PlatformFacebook  Platform = "facebook"
	PlatformInstagram Platform = "instagram"
)

type AuthMethod string

const (
	AuthMethodStatic AuthMethod = "static"
	AuthMethodOAuth  AuthMethod = "oauth"
)

type AccountKind string

const (
	AccountKindDefault      AccountKind = "default"
	AccountKindPersonal     AccountKind = "personal"
	AccountKindOrganization AccountKind = "organization"
)

func NormalizeAccountKind(platform Platform, raw AccountKind) AccountKind {
	switch platform {
	case PlatformLinkedIn:
		switch raw {
		case "", AccountKindDefault, AccountKindPersonal:
			return AccountKindPersonal
		case AccountKindOrganization:
			return AccountKindOrganization
		default:
			return ""
		}
	default:
		switch raw {
		case "", AccountKindDefault:
			return AccountKindDefault
		default:
			return ""
		}
	}
}

type AccountStatus string

const (
	AccountStatusConnected    AccountStatus = "connected"
	AccountStatusDisconnected AccountStatus = "disconnected"
	AccountStatusError        AccountStatus = "error"
)

type PostStatus string

const (
	PostStatusDraft      PostStatus = "draft"
	PostStatusScheduled  PostStatus = "scheduled"
	PostStatusPublishing PostStatus = "publishing"
	PostStatusPublished  PostStatus = "published"
	PostStatusFailed     PostStatus = "failed"
	PostStatusCanceled   PostStatus = "canceled"
)

type CampaignStatus string

const (
	CampaignStatusActive   CampaignStatus = "active"
	CampaignStatusPaused   CampaignStatus = "paused"
	CampaignStatusArchived CampaignStatus = "archived"
)

type EditorialStatus string

const (
	EditorialStatusIdea        EditorialStatus = "idea"
	EditorialStatusDrafting    EditorialStatus = "drafting"
	EditorialStatusNeedsReview EditorialStatus = "needs_review"
	EditorialStatusApproved    EditorialStatus = "approved"
	EditorialStatusScheduled   EditorialStatus = "scheduled"
)

type Campaign struct {
	ID             string         `json:"id"`
	Name           string         `json:"name"`
	Objective      string         `json:"objective,omitempty"`
	Status         CampaignStatus `json:"status"`
	StartsAt       time.Time      `json:"starts_at,omitempty"`
	EndsAt         time.Time      `json:"ends_at,omitempty"`
	Notes          string         `json:"notes,omitempty"`
	Tags           []string       `json:"tags,omitempty"`
	Timezone       string         `json:"timezone,omitempty"`
	Audience       string         `json:"audience,omitempty"`
	Tone           string         `json:"tone,omitempty"`
	CTA            string         `json:"cta,omitempty"`
	Restrictions   string         `json:"restrictions,omitempty"`
	BrandProfileID string         `json:"brand_profile_id,omitempty"`
	VisualStyle    string         `json:"visual_style,omitempty"`
	ImagePrompt    string         `json:"image_prompt,omitempty"`
	ImageSize      string         `json:"image_size,omitempty"`
	CreatedAt      time.Time      `json:"created_at"`
	UpdatedAt      time.Time      `json:"updated_at"`
}

type CampaignListFilter struct {
	Status CampaignStatus
	Tag    string
	Limit  int
}

type EditorialBacklogFilter struct {
	CampaignID      string
	Platform        Platform
	EditorialStatus EditorialStatus
	Tag             string
	From            time.Time
	To              time.Time
	Limit           int
}

type EditorialBacklogItem struct {
	Post     Post     `json:"post"`
	Campaign Campaign `json:"campaign,omitempty"`
}

type Media struct {
	ID           string    `json:"id"`
	Kind         string    `json:"kind"`
	OriginalName string    `json:"original_name"`
	StoragePath  string    `json:"storage_path"`
	MimeType     string    `json:"mime_type"`
	SizeBytes    int64     `json:"size_bytes"`
	CreatedAt    time.Time `json:"created_at"`
	Tags         []string  `json:"tags,omitempty"`
}

type MediaListFilter struct {
	Tag   string
	Limit int
}

type SocialAccount struct {
	ID                string        `json:"id"`
	Platform          Platform      `json:"platform"`
	AccountKind       AccountKind   `json:"account_kind"`
	DisplayName       string        `json:"display_name"`
	ExternalAccountID string        `json:"external_account_id"`
	XPremium          bool          `json:"x_premium"`
	AuthMethod        AuthMethod    `json:"auth_method"`
	Status            AccountStatus `json:"status"`
	LastError         *string       `json:"last_error,omitempty"`
	CreatedAt         time.Time     `json:"created_at"`
	UpdatedAt         time.Time     `json:"updated_at"`
}

type OauthState struct {
	ID           string      `json:"id"`
	Platform     Platform    `json:"platform"`
	AccountKind  AccountKind `json:"account_kind"`
	State        string      `json:"state"`
	CodeVerifier string      `json:"code_verifier"`
	ExpiresAt    time.Time   `json:"expires_at"`
	CreatedAt    time.Time   `json:"created_at"`
}

type Post struct {
	ID                 string          `json:"id"`
	AccountID          string          `json:"account_id"`
	Platform           Platform        `json:"platform"`
	Text               string          `json:"text"`
	Status             PostStatus      `json:"status"`
	ScheduledAt        time.Time       `json:"scheduled_at"`
	ThreadGroupID      string          `json:"thread_group_id,omitempty"`
	ThreadPosition     int             `json:"thread_position,omitempty"`
	ParentPostID       *string         `json:"parent_post_id,omitempty"`
	RootPostID         *string         `json:"root_post_id,omitempty"`
	NextRetryAt        *time.Time      `json:"next_retry_at,omitempty"`
	Attempts           int             `json:"attempts"`
	MaxAttempts        int             `json:"max_attempts"`
	IdempotencyKey     *string         `json:"idempotency_key,omitempty"`
	PublishedAt        *time.Time      `json:"published_at,omitempty"`
	ExternalID         *string         `json:"external_id,omitempty"`
	PublishedURL       *string         `json:"published_url,omitempty"`
	Error              *string         `json:"error,omitempty"`
	CampaignID         string          `json:"campaign_id,omitempty"`
	EditorialStatus    EditorialStatus `json:"editorial_status,omitempty"`
	RequiresApproval   bool            `json:"requires_approval,omitempty"`
	ApprovedAt         *time.Time      `json:"approved_at,omitempty"`
	AutoApprovedReason string          `json:"auto_approved_reason,omitempty"`
	BlockedReason      string          `json:"blocked_reason,omitempty"`
	Tags               []string        `json:"tags,omitempty"`
	CreatedAt          time.Time       `json:"created_at"`
	UpdatedAt          time.Time       `json:"updated_at"`
	Media              []Media         `json:"media,omitempty"`
}

type DeadLetter struct {
	ID          string    `json:"id"`
	PostID      string    `json:"post_id"`
	Reason      string    `json:"reason"`
	LastError   string    `json:"last_error"`
	AttemptedAt time.Time `json:"attempted_at"`
}
