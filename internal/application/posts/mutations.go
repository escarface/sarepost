package posts

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/escarface/sarepost/internal/application/ports"
	"github.com/escarface/sarepost/internal/db"
	"github.com/escarface/sarepost/internal/domain"
	"github.com/escarface/sarepost/internal/postflow"
)

var (
	ErrPostIDRequired         = errors.New("post id is required")
	ErrScheduledAtNeeded      = errors.New("scheduled_at is required to schedule")
	ErrPostApprovalRequired   = errors.New("post requires approval before scheduling")
	ErrScheduleConflict       = errors.New("schedule conflict for account")
	ErrDuplicateContentRecent = errors.New("duplicate content was scheduled recently")
)

const (
	scheduleConflictWindow = 5 * time.Minute
	recentDuplicateWindow  = 7 * 24 * time.Hour
)

type MutationsStore interface {
	CancelPost(ctx context.Context, id string) error
	DeletePostEditable(ctx context.Context, id string) error
	ScheduleDraftPost(ctx context.Context, id string, scheduledAt time.Time) error
	UpdatePostEditable(ctx context.Context, id, text string, scheduledAt time.Time, mediaIDs []string, replaceMedia bool) error
	UpdatePostEditorialMetadata(ctx context.Context, id string, editorialStatus domain.EditorialStatus, requiresApproval *bool, tags []string, approvedAt *time.Time) error
	UpdateThreadEditable(ctx context.Context, rootPostID string, steps []db.ThreadStepUpdate) error
	GetPost(ctx context.Context, id string) (domain.Post, error)
	GetAccount(ctx context.Context, id string) (domain.SocialAccount, error)
	GetCampaign(ctx context.Context, id string) (domain.Campaign, error)
	GetMediaByIDs(ctx context.Context, ids []string) ([]domain.Media, error)
	ListSchedule(ctx context.Context, from time.Time, to time.Time) ([]domain.Post, error)
}

type MutationsService struct {
	Store    MutationsStore
	Registry ports.ProviderRegistry
}

type EditInput struct {
	PostID           string
	PostIDs          []string
	Text             string
	Intent           string
	ScheduledAt      time.Time
	MediaIDs         []string
	ReplaceMedia     bool
	Segments         []ThreadSegmentInput
	EditorialStatus  domain.EditorialStatus
	RequiresApproval *bool
	Tags             []string
}

type SchedulePreview struct {
	PostID         string         `json:"post_id"`
	AccountID      string         `json:"account_id"`
	AccountName    string         `json:"account_name,omitempty"`
	Platform       string         `json:"platform"`
	Text           string         `json:"text"`
	Media          []domain.Media `json:"media,omitempty"`
	CampaignID     string         `json:"campaign_id,omitempty"`
	CampaignName   string         `json:"campaign_name,omitempty"`
	ScheduledAt    time.Time      `json:"scheduled_at"`
	ScheduledLocal string         `json:"scheduled_local"`
	Timezone       string         `json:"timezone"`
	Warnings       []string       `json:"warnings,omitempty"`
}

type preparedEdit struct {
	postID           string
	resultPostID     string
	text             string
	scheduledAt      time.Time
	mediaIDs         []string
	replaceMedia     bool
	steps            []db.ThreadStepUpdate
	editorialStatus  domain.EditorialStatus
	requiresApproval *bool
	tags             []string
	approvedAt       *time.Time
}

func ResolveScheduledAtForEdit(intent string, scheduledAt time.Time, currentScheduledAt time.Time, now func() time.Time) (time.Time, error) {
	intent = strings.ToLower(strings.TrimSpace(intent))
	switch intent {
	case "draft":
		return time.Time{}, nil
	case "publish_now":
		if now == nil {
			now = time.Now
		}
		return now().UTC(), nil
	case "schedule":
		if scheduledAt.IsZero() {
			return time.Time{}, ErrScheduledAtNeeded
		}
	}
	if scheduledAt.IsZero() {
		if intent == "" {
			if currentScheduledAt.IsZero() {
				return time.Time{}, nil
			}
			return currentScheduledAt.UTC(), nil
		}
		return time.Time{}, nil
	}
	return scheduledAt.UTC(), nil
}

func (s MutationsService) Cancel(ctx context.Context, postID string) error {
	postID = strings.TrimSpace(postID)
	if postID == "" {
		return ErrPostIDRequired
	}
	return s.Store.CancelPost(ctx, postID)
}

func (s MutationsService) DeleteEditable(ctx context.Context, postID string) error {
	postID = strings.TrimSpace(postID)
	if postID == "" {
		return ErrPostIDRequired
	}
	return s.Store.DeletePostEditable(ctx, postID)
}

func (s MutationsService) ScheduleDraft(ctx context.Context, postID string, scheduledAt time.Time) (domain.Post, error) {
	postID = strings.TrimSpace(postID)
	if postID == "" {
		return domain.Post{}, ErrPostIDRequired
	}
	if scheduledAt.IsZero() {
		return domain.Post{}, ErrScheduledAtNeeded
	}
	current, err := s.Store.GetPost(ctx, postID)
	if err != nil {
		return domain.Post{}, err
	}
	if postRequiresApproval(current) {
		return domain.Post{}, ErrPostApprovalRequired
	}
	if err := s.ensureCampaignCanSchedule(ctx, current); err != nil {
		return domain.Post{}, err
	}
	if err := s.ensureScheduleGuardrails(ctx, current, current.Text, scheduledAt.UTC()); err != nil {
		return domain.Post{}, err
	}
	if err := s.Store.ScheduleDraftPost(ctx, postID, scheduledAt.UTC()); err != nil {
		return domain.Post{}, err
	}
	return s.Store.GetPost(ctx, postID)
}

func (s MutationsService) PreviewSchedule(ctx context.Context, postID string, scheduledAt time.Time, loc *time.Location) (SchedulePreview, error) {
	postID = strings.TrimSpace(postID)
	if postID == "" {
		return SchedulePreview{}, ErrPostIDRequired
	}
	if scheduledAt.IsZero() {
		return SchedulePreview{}, ErrScheduledAtNeeded
	}
	if loc == nil {
		loc = time.UTC
	}
	current, err := s.Store.GetPost(ctx, postID)
	if err != nil {
		return SchedulePreview{}, err
	}
	account, err := s.Store.GetAccount(ctx, current.AccountID)
	if err != nil {
		return SchedulePreview{}, err
	}
	preview := SchedulePreview{
		PostID:         current.ID,
		AccountID:      current.AccountID,
		AccountName:    account.DisplayName,
		Platform:       string(current.Platform),
		Text:           current.Text,
		Media:          append([]domain.Media(nil), current.Media...),
		CampaignID:     current.CampaignID,
		ScheduledAt:    scheduledAt.UTC(),
		ScheduledLocal: scheduledAt.In(loc).Format(time.RFC3339),
		Timezone:       loc.String(),
	}
	if postRequiresApproval(current) {
		preview.Warnings = append(preview.Warnings, ErrPostApprovalRequired.Error())
	}
	if campaignID := strings.TrimSpace(current.CampaignID); campaignID != "" {
		campaign, err := s.Store.GetCampaign(ctx, campaignID)
		if err != nil {
			return SchedulePreview{}, err
		}
		preview.CampaignName = campaign.Name
		if campaign.Status == domain.CampaignStatusArchived {
			preview.Warnings = append(preview.Warnings, ErrCampaignArchived.Error())
		}
	}
	warnings, err := s.scheduleGuardrailWarnings(ctx, current, current.Text, scheduledAt.UTC())
	if err != nil {
		return SchedulePreview{}, err
	}
	preview.Warnings = append(preview.Warnings, warnings...)
	return preview, nil
}

func postRequiresApproval(post domain.Post) bool {
	if !post.RequiresApproval {
		return false
	}
	if post.EditorialStatus == domain.EditorialStatusApproved {
		return false
	}
	return post.ApprovedAt == nil
}

func (s MutationsService) UpdateEditable(ctx context.Context, in EditInput, now func() time.Time) (domain.Post, error) {
	posts, err := s.UpdateEditableMany(ctx, in, now)
	if err != nil {
		return domain.Post{}, err
	}
	if len(posts) == 0 {
		return domain.Post{}, ErrPostIDRequired
	}
	return posts[0], nil
}

func (s MutationsService) UpdateEditableMany(ctx context.Context, in EditInput, now func() time.Time) ([]domain.Post, error) {
	postIDs := normalizeEditablePostIDs(in.PostID, in.PostIDs)
	if len(postIDs) == 0 {
		return nil, ErrPostIDRequired
	}

	prepared := make([]preparedEdit, 0, len(postIDs))
	for _, postID := range postIDs {
		item, err := s.prepareEditableUpdate(ctx, postID, in, now)
		if err != nil {
			return nil, err
		}
		prepared = append(prepared, item)
	}

	updated := make([]domain.Post, 0, len(prepared))
	for _, item := range prepared {
		if len(item.steps) > 0 {
			if err := s.Store.UpdateThreadEditable(ctx, item.resultPostID, item.steps); err != nil {
				return nil, err
			}
			post, err := s.Store.GetPost(ctx, item.resultPostID)
			if err != nil {
				return nil, err
			}
			updated = append(updated, post)
			continue
		}

		if err := s.Store.UpdatePostEditable(ctx, item.postID, item.text, item.scheduledAt, item.mediaIDs, item.replaceMedia); err != nil {
			return nil, err
		}
		if item.editorialStatus != "" || item.requiresApproval != nil || item.tags != nil || item.approvedAt != nil {
			if err := s.Store.UpdatePostEditorialMetadata(ctx, item.postID, item.editorialStatus, item.requiresApproval, item.tags, item.approvedAt); err != nil {
				return nil, err
			}
		}
		post, err := s.Store.GetPost(ctx, item.resultPostID)
		if err != nil {
			return nil, err
		}
		updated = append(updated, post)
	}

	return updated, nil
}

func (s MutationsService) prepareEditableUpdate(ctx context.Context, postID string, in EditInput, now func() time.Time) (preparedEdit, error) {
	postID = strings.TrimSpace(postID)
	if postID == "" {
		return preparedEdit{}, ErrPostIDRequired
	}
	current, err := s.Store.GetPost(ctx, postID)
	if err != nil {
		return preparedEdit{}, err
	}
	if in.EditorialStatus != "" {
		current.EditorialStatus = in.EditorialStatus
	}
	var approvedAt *time.Time
	if in.EditorialStatus == domain.EditorialStatusApproved {
		approved := false
		in.RequiresApproval = &approved
		if current.ApprovedAt == nil {
			nowValue := now().UTC()
			current.ApprovedAt = &nowValue
		}
		current.RequiresApproval = false
		approvedAt = current.ApprovedAt
	}
	if in.RequiresApproval != nil {
		current.RequiresApproval = *in.RequiresApproval
		if !*in.RequiresApproval {
			approvedAt = current.ApprovedAt
		}
	}
	scheduledAt, err := ResolveScheduledAtForEdit(in.Intent, in.ScheduledAt, current.ScheduledAt, now)
	if err != nil {
		return preparedEdit{}, err
	}
	if !scheduledAt.IsZero() && postRequiresApproval(current) {
		return preparedEdit{}, ErrPostApprovalRequired
	}
	if !scheduledAt.IsZero() {
		if err := s.ensureCampaignCanSchedule(ctx, current); err != nil {
			return preparedEdit{}, err
		}
	}
	candidateText := current.Text

	if len(in.Segments) > 0 {
		if len(in.Segments) > MaxThreadSegments {
			return preparedEdit{}, ErrThreadTooLong
		}
		if err := s.validateEditableThread(ctx, current, in.Segments); err != nil {
			return preparedEdit{}, err
		}
		steps := make([]db.ThreadStepUpdate, 0, len(in.Segments))
		for _, segment := range in.Segments {
			text := strings.TrimSpace(segment.Text)
			if text == "" {
				return preparedEdit{}, ErrTextRequired
			}
			if len(steps) == 0 {
				candidateText = text
			}
			steps = append(steps, db.ThreadStepUpdate{
				Text:        text,
				ScheduledAt: scheduledAt,
				MediaIDs:    normalizeMediaIDs(segment.MediaIDs),
			})
		}
		rootID := strings.TrimSpace(current.ID)
		if current.RootPostID != nil && strings.TrimSpace(*current.RootPostID) != "" {
			rootID = strings.TrimSpace(*current.RootPostID)
		}
		if !scheduledAt.IsZero() {
			if err := s.ensureScheduleGuardrails(ctx, current, candidateText, scheduledAt); err != nil {
				return preparedEdit{}, err
			}
		}
		return preparedEdit{
			postID:           postID,
			resultPostID:     rootID,
			scheduledAt:      scheduledAt,
			steps:            steps,
			editorialStatus:  in.EditorialStatus,
			requiresApproval: in.RequiresApproval,
			tags:             normalizeTags(in.Tags),
			approvedAt:       approvedAt,
		}, nil
	}

	text := strings.TrimSpace(in.Text)
	if text == "" {
		return preparedEdit{}, ErrTextRequired
	}

	mediaIDs := mediaIDsFromPost(current.Media)
	if in.ReplaceMedia {
		mediaIDs = normalizeMediaIDs(in.MediaIDs)
	}
	if err := s.validateEditableDraft(ctx, current, text, mediaIDs, in.ReplaceMedia); err != nil {
		return preparedEdit{}, err
	}
	if !scheduledAt.IsZero() {
		if err := s.ensureScheduleGuardrails(ctx, current, text, scheduledAt); err != nil {
			return preparedEdit{}, err
		}
	}

	return preparedEdit{
		postID:           postID,
		resultPostID:     postID,
		text:             text,
		scheduledAt:      scheduledAt,
		mediaIDs:         mediaIDs,
		replaceMedia:     in.ReplaceMedia,
		editorialStatus:  in.EditorialStatus,
		requiresApproval: in.RequiresApproval,
		tags:             normalizeTags(in.Tags),
		approvedAt:       approvedAt,
	}, nil
}

func normalizeTags(tags []string) []string {
	if tags == nil {
		return nil
	}
	out := make([]string, 0, len(tags))
	seen := map[string]struct{}{}
	for _, raw := range tags {
		tag := strings.TrimSpace(raw)
		if tag == "" {
			continue
		}
		if _, ok := seen[tag]; ok {
			continue
		}
		seen[tag] = struct{}{}
		out = append(out, tag)
	}
	return out
}

func (s MutationsService) ensureCampaignCanSchedule(ctx context.Context, post domain.Post) error {
	campaignID := strings.TrimSpace(post.CampaignID)
	if campaignID == "" {
		return nil
	}
	campaign, err := s.Store.GetCampaign(ctx, campaignID)
	if err != nil {
		return err
	}
	if campaign.Status == domain.CampaignStatusArchived {
		return ErrCampaignArchived
	}
	return nil
}

func (s MutationsService) ensureScheduleGuardrails(ctx context.Context, post domain.Post, candidateText string, scheduledAt time.Time) error {
	warnings, err := s.scheduleGuardrailWarnings(ctx, post, candidateText, scheduledAt)
	if err != nil {
		return err
	}
	for _, warning := range warnings {
		switch warning {
		case ErrScheduleConflict.Error():
			return ErrScheduleConflict
		case ErrDuplicateContentRecent.Error():
			return ErrDuplicateContentRecent
		}
	}
	return nil
}

// ValidateNewSchedule applies the same account conflict and duplicate-content
// guardrails used when scheduling an existing draft, without mutating a post.
func (s MutationsService) ValidateNewSchedule(ctx context.Context, accountID, text string, scheduledAt time.Time) error {
	if s.Store == nil {
		return ErrStoreNotConfigured
	}
	return s.ensureScheduleGuardrails(ctx, domain.Post{AccountID: strings.TrimSpace(accountID)}, strings.TrimSpace(text), scheduledAt)
}

func (s MutationsService) scheduleGuardrailWarnings(ctx context.Context, post domain.Post, candidateText string, scheduledAt time.Time) ([]string, error) {
	if scheduledAt.IsZero() {
		return nil, nil
	}
	items, err := s.Store.ListSchedule(ctx, scheduledAt.Add(-recentDuplicateWindow), scheduledAt.Add(recentDuplicateWindow))
	if err != nil {
		return nil, err
	}
	warningSet := make(map[string]struct{}, 2)
	candidateAccount := strings.TrimSpace(post.AccountID)
	candidateRoot := rootPostIDForGuardrail(post)
	candidateNormalized := normalizeDuplicateText(candidateText)
	for _, item := range items {
		if strings.TrimSpace(item.ID) == strings.TrimSpace(post.ID) || rootPostIDForGuardrail(item) == candidateRoot {
			continue
		}
		if strings.TrimSpace(item.AccountID) != candidateAccount {
			continue
		}
		delta := item.ScheduledAt.Sub(scheduledAt)
		if delta < 0 {
			delta = -delta
		}
		if delta <= scheduleConflictWindow {
			warningSet[ErrScheduleConflict.Error()] = struct{}{}
		}
		if candidateNormalized != "" && normalizeDuplicateText(item.Text) == candidateNormalized {
			warningSet[ErrDuplicateContentRecent.Error()] = struct{}{}
		}
	}
	warnings := make([]string, 0, len(warningSet))
	for _, warning := range []string{ErrScheduleConflict.Error(), ErrDuplicateContentRecent.Error()} {
		if _, ok := warningSet[warning]; ok {
			warnings = append(warnings, warning)
		}
	}
	return warnings, nil
}

func rootPostIDForGuardrail(post domain.Post) string {
	if post.RootPostID != nil && strings.TrimSpace(*post.RootPostID) != "" {
		return strings.TrimSpace(*post.RootPostID)
	}
	return strings.TrimSpace(post.ID)
}

func normalizeDuplicateText(text string) string {
	return strings.ToLower(strings.Join(strings.Fields(strings.TrimSpace(text)), " "))
}

func normalizeEditablePostIDs(primary string, many []string) []string {
	seen := make(map[string]struct{}, len(many)+1)
	out := make([]string, 0, len(many)+1)
	add := func(raw string) {
		id := strings.TrimSpace(raw)
		if id == "" {
			return
		}
		if _, exists := seen[id]; exists {
			return
		}
		seen[id] = struct{}{}
		out = append(out, id)
	}
	for _, id := range many {
		add(id)
	}
	add(primary)
	return out
}

func (s MutationsService) validateEditableThread(ctx context.Context, post domain.Post, segments []ThreadSegmentInput) error {
	if s.Registry == nil {
		return nil
	}

	account, err := s.Store.GetAccount(ctx, strings.TrimSpace(post.AccountID))
	if err != nil {
		return ValidationError{Err: ErrAccountNotFound}
	}
	provider, ok := s.Registry.Get(account.Platform)
	if !ok {
		return ValidationError{Err: ErrProviderNotConfigured}
	}
	if err := validateThreadSegmentsForAccount(ctx, s.Store, provider, account, segments); err != nil {
		return ValidationError{Err: err}
	}
	return nil
}

func (s MutationsService) validateEditableDraft(ctx context.Context, post domain.Post, text string, mediaIDs []string, replaceMedia bool) error {
	if s.Registry == nil {
		return nil
	}

	account, err := s.Store.GetAccount(ctx, strings.TrimSpace(post.AccountID))
	if err != nil {
		return ValidationError{Err: ErrAccountNotFound}
	}
	provider, ok := s.Registry.Get(account.Platform)
	if !ok {
		return ValidationError{Err: ErrProviderNotConfigured}
	}

	media := post.Media
	if replaceMedia {
		media, err = s.Store.GetMediaByIDs(ctx, mediaIDs)
		if err != nil {
			return ValidationError{Err: err}
		}
	}
	if _, err := provider.ValidateDraft(ctx, account, postflow.Draft{
		Text:  text,
		Media: media,
	}); err != nil {
		return ValidationError{Err: err}
	}
	return nil
}

func mediaIDsFromPost(media []domain.Media) []string {
	if len(media) == 0 {
		return nil
	}
	out := make([]string, 0, len(media))
	for _, item := range media {
		id := strings.TrimSpace(item.ID)
		if id == "" {
			continue
		}
		out = append(out, id)
	}
	return out
}
