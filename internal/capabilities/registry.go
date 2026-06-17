package capabilities

type Surface string

const (
	SurfaceAPI Surface = "api"
	SurfaceMCP Surface = "mcp"
	SurfaceCLI Surface = "cli"
)

const (
	CapabilityHealthCheck           = "health.check"
	CapabilityScheduleList          = "schedule.list"
	CapabilityDraftsList            = "drafts.list"
	CapabilityPostsCreate           = "posts.create"
	CapabilityPostsSchedule         = "posts.schedule"
	CapabilityPostsPreviewSchedule  = "posts.preview_schedule"
	CapabilityPostsEdit             = "posts.edit"
	CapabilityPostsDelete           = "posts.delete"
	CapabilityPostsCancel           = "posts.cancel"
	CapabilityPostsValidate         = "posts.validate"
	CapabilityAccountsList          = "accounts.list"
	CapabilityAccountsCreateStatic  = "accounts.create_static"
	CapabilityAccountsConnect       = "accounts.connect"
	CapabilityAccountsDisconnect    = "accounts.disconnect"
	CapabilityAccountsDelete        = "accounts.delete"
	CapabilityAccountsSetXPremium   = "accounts.set_x_premium"
	CapabilityFailedList            = "failed.list"
	CapabilityDLQRequeue            = "dlq.requeue"
	CapabilityDLQDelete             = "dlq.delete"
	CapabilityMediaUpload           = "media.upload"
	CapabilityMediaList             = "media.list"
	CapabilityMediaDelete           = "media.delete"
	CapabilitySettingsTimezone      = "settings.timezone"
	CapabilityCampaignsCreate       = "campaigns.create"
	CapabilityCampaignsList         = "campaigns.list"
	CapabilityCampaignsUpdate       = "campaigns.update"
	CapabilityCampaignsArchive      = "campaigns.archive"
	CapabilityCampaignsPostsAdd     = "campaigns.posts.add"
	CapabilityCampaignsDraftsCreate = "campaigns.drafts.create"
	CapabilityCampaignsBacklog      = "campaigns.backlog"
	CapabilityPostsApprove          = "posts.approve"
)

type Capability struct {
	ID               string
	RequiredSurfaces []Surface
}

func RequiredParityCapabilities() []Capability {
	return []Capability{
		{ID: CapabilityHealthCheck, RequiredSurfaces: []Surface{SurfaceAPI, SurfaceMCP, SurfaceCLI}},
		{ID: CapabilityScheduleList, RequiredSurfaces: []Surface{SurfaceAPI, SurfaceMCP, SurfaceCLI}},
		{ID: CapabilityDraftsList, RequiredSurfaces: []Surface{SurfaceAPI, SurfaceMCP, SurfaceCLI}},
		{ID: CapabilityPostsCreate, RequiredSurfaces: []Surface{SurfaceAPI, SurfaceMCP, SurfaceCLI}},
		{ID: CapabilityPostsSchedule, RequiredSurfaces: []Surface{SurfaceAPI, SurfaceMCP, SurfaceCLI}},
		{ID: CapabilityPostsPreviewSchedule, RequiredSurfaces: []Surface{SurfaceAPI, SurfaceMCP, SurfaceCLI}},
		{ID: CapabilityPostsEdit, RequiredSurfaces: []Surface{SurfaceAPI, SurfaceMCP, SurfaceCLI}},
		{ID: CapabilityPostsDelete, RequiredSurfaces: []Surface{SurfaceAPI, SurfaceMCP, SurfaceCLI}},
		{ID: CapabilityPostsCancel, RequiredSurfaces: []Surface{SurfaceAPI, SurfaceMCP, SurfaceCLI}},
		{ID: CapabilityPostsValidate, RequiredSurfaces: []Surface{SurfaceAPI, SurfaceMCP, SurfaceCLI}},
		{ID: CapabilityAccountsList, RequiredSurfaces: []Surface{SurfaceAPI, SurfaceMCP, SurfaceCLI}},
		{ID: CapabilityAccountsCreateStatic, RequiredSurfaces: []Surface{SurfaceAPI, SurfaceMCP, SurfaceCLI}},
		{ID: CapabilityAccountsConnect, RequiredSurfaces: []Surface{SurfaceAPI, SurfaceMCP, SurfaceCLI}},
		{ID: CapabilityAccountsDisconnect, RequiredSurfaces: []Surface{SurfaceAPI, SurfaceMCP, SurfaceCLI}},
		{ID: CapabilityAccountsDelete, RequiredSurfaces: []Surface{SurfaceAPI, SurfaceMCP, SurfaceCLI}},
		{ID: CapabilityAccountsSetXPremium, RequiredSurfaces: []Surface{SurfaceAPI, SurfaceMCP, SurfaceCLI}},
		{ID: CapabilityFailedList, RequiredSurfaces: []Surface{SurfaceAPI, SurfaceMCP, SurfaceCLI}},
		{ID: CapabilityDLQRequeue, RequiredSurfaces: []Surface{SurfaceAPI, SurfaceMCP, SurfaceCLI}},
		{ID: CapabilityDLQDelete, RequiredSurfaces: []Surface{SurfaceAPI, SurfaceMCP, SurfaceCLI}},
		{ID: CapabilityMediaUpload, RequiredSurfaces: []Surface{SurfaceAPI, SurfaceMCP, SurfaceCLI}},
		{ID: CapabilityMediaList, RequiredSurfaces: []Surface{SurfaceAPI, SurfaceMCP, SurfaceCLI}},
		{ID: CapabilityMediaDelete, RequiredSurfaces: []Surface{SurfaceAPI, SurfaceMCP, SurfaceCLI}},
		{ID: CapabilitySettingsTimezone, RequiredSurfaces: []Surface{SurfaceAPI, SurfaceMCP, SurfaceCLI}},
		{ID: CapabilityCampaignsCreate, RequiredSurfaces: []Surface{SurfaceAPI, SurfaceMCP, SurfaceCLI}},
		{ID: CapabilityCampaignsList, RequiredSurfaces: []Surface{SurfaceAPI, SurfaceMCP, SurfaceCLI}},
		{ID: CapabilityCampaignsUpdate, RequiredSurfaces: []Surface{SurfaceAPI, SurfaceMCP, SurfaceCLI}},
		{ID: CapabilityCampaignsArchive, RequiredSurfaces: []Surface{SurfaceAPI, SurfaceMCP, SurfaceCLI}},
		{ID: CapabilityCampaignsPostsAdd, RequiredSurfaces: []Surface{SurfaceAPI, SurfaceMCP, SurfaceCLI}},
		{ID: CapabilityCampaignsDraftsCreate, RequiredSurfaces: []Surface{SurfaceAPI, SurfaceMCP, SurfaceCLI}},
		{ID: CapabilityCampaignsBacklog, RequiredSurfaces: []Surface{SurfaceAPI, SurfaceMCP, SurfaceCLI}},
		{ID: CapabilityPostsApprove, RequiredSurfaces: []Surface{SurfaceAPI, SurfaceMCP, SurfaceCLI}},
	}
}
