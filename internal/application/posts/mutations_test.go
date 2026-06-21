package posts

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/escarface/sarepost/internal/db"
	"github.com/escarface/sarepost/internal/domain"
	"github.com/escarface/sarepost/internal/postflow"
)

type fakeMutationsStore struct {
	cancelID               string
	deleteID               string
	scheduleID             string
	scheduleAt             time.Time
	updateID               string
	updateText             string
	updateScheduledAt      time.Time
	updateMediaIDs         []string
	updateReplaceMedia     bool
	updateEditorialStatus  domain.EditorialStatus
	updateRequiresApproval *bool
	updateTags             []string
	updateApprovedAt       *time.Time
	updateThreadRoot       string
	updateThreadSteps      []db.ThreadStepUpdate
	post                   domain.Post
	account                domain.SocialAccount
	campaign               domain.Campaign
	mediaByID              map[string]domain.Media
	schedule               []domain.Post
	err                    error
}

func (f *fakeMutationsStore) CancelPost(_ context.Context, id string) error {
	f.cancelID = id
	return f.err
}

func (f *fakeMutationsStore) DeletePostEditable(_ context.Context, id string) error {
	f.deleteID = id
	return f.err
}

func (f *fakeMutationsStore) ScheduleDraftPost(_ context.Context, id string, scheduledAt time.Time) error {
	f.scheduleID = id
	f.scheduleAt = scheduledAt
	return f.err
}

func (f *fakeMutationsStore) UpdatePostEditable(_ context.Context, id, text string, scheduledAt time.Time, mediaIDs []string, replaceMedia bool) error {
	f.updateID = id
	f.updateText = text
	f.updateScheduledAt = scheduledAt
	f.updateReplaceMedia = replaceMedia
	f.updateMediaIDs = append([]string(nil), mediaIDs...)
	return f.err
}

func (f *fakeMutationsStore) UpdatePostEditorialMetadata(_ context.Context, _ string, editorialStatus domain.EditorialStatus, requiresApproval *bool, tags []string, approvedAt *time.Time) error {
	f.updateEditorialStatus = editorialStatus
	if requiresApproval != nil {
		v := *requiresApproval
		f.updateRequiresApproval = &v
	} else {
		f.updateRequiresApproval = nil
	}
	f.updateTags = append([]string(nil), tags...)
	if approvedAt != nil {
		v := *approvedAt
		f.updateApprovedAt = &v
	} else {
		f.updateApprovedAt = nil
	}
	return f.err
}

func (f *fakeMutationsStore) UpdateThreadEditable(_ context.Context, rootPostID string, steps []db.ThreadStepUpdate) error {
	f.updateThreadRoot = rootPostID
	f.updateThreadSteps = append([]db.ThreadStepUpdate(nil), steps...)
	return f.err
}

func (f *fakeMutationsStore) GetPost(_ context.Context, id string) (domain.Post, error) {
	if f.err != nil {
		return domain.Post{}, f.err
	}
	post := f.post
	post.ID = id
	return post, nil
}

func (f *fakeMutationsStore) GetAccount(_ context.Context, _ string) (domain.SocialAccount, error) {
	if f.err != nil {
		return domain.SocialAccount{}, f.err
	}
	return f.account, nil
}

func (f *fakeMutationsStore) GetCampaign(_ context.Context, _ string) (domain.Campaign, error) {
	if f.err != nil {
		return domain.Campaign{}, f.err
	}
	if strings.TrimSpace(f.campaign.ID) == "" {
		return domain.Campaign{Status: domain.CampaignStatusActive}, nil
	}
	return f.campaign, nil
}

func (f *fakeMutationsStore) GetMediaByIDs(_ context.Context, ids []string) ([]domain.Media, error) {
	if f.err != nil {
		return nil, f.err
	}
	out := make([]domain.Media, 0, len(ids))
	for _, id := range ids {
		item, ok := f.mediaByID[strings.TrimSpace(id)]
		if !ok {
			return nil, errors.New("media not found: " + strings.TrimSpace(id))
		}
		out = append(out, item)
	}
	return out, nil
}

func (f *fakeMutationsStore) ListSchedule(context.Context, time.Time, time.Time) ([]domain.Post, error) {
	if f.err != nil {
		return nil, f.err
	}
	return append([]domain.Post(nil), f.schedule...), nil
}

func TestValidateNewScheduleAppliesExistingConflictGuardrails(t *testing.T) {
	when := time.Date(2026, time.July, 6, 9, 0, 0, 0, time.UTC)
	store := &fakeMutationsStore{schedule: []domain.Post{{ID: "existing", AccountID: "acc_1", Text: "Existing", ScheduledAt: when.Add(2 * time.Minute)}}}
	err := (MutationsService{Store: store}).ValidateNewSchedule(t.Context(), "acc_1", "New post", when)
	if !errors.Is(err, ErrScheduleConflict) {
		t.Fatalf("expected schedule conflict, got %v", err)
	}
}

func TestResolveScheduledAtForEdit(t *testing.T) {
	now := time.Date(2026, 3, 1, 10, 0, 0, 0, time.UTC)

	draftAt, err := ResolveScheduledAtForEdit("draft", now.Add(10*time.Minute), time.Time{}, nil)
	if err != nil {
		t.Fatalf("draft resolve failed: %v", err)
	}
	if !draftAt.IsZero() {
		t.Fatalf("expected zero scheduled_at for draft")
	}

	publishNowAt, err := ResolveScheduledAtForEdit("publish_now", time.Time{}, time.Time{}, func() time.Time { return now })
	if err != nil {
		t.Fatalf("publish_now resolve failed: %v", err)
	}
	if !publishNowAt.Equal(now) {
		t.Fatalf("expected publish_now to default to now, got %s", publishNowAt)
	}
	publishNowWithScheduled, err := ResolveScheduledAtForEdit("publish_now", now.Add(6*time.Hour), time.Time{}, func() time.Time { return now })
	if err != nil {
		t.Fatalf("publish_now with explicit schedule resolve failed: %v", err)
	}
	if !publishNowWithScheduled.Equal(now) {
		t.Fatalf("expected publish_now to ignore explicit scheduled_at and use now, got %s", publishNowWithScheduled)
	}

	_, err = ResolveScheduledAtForEdit("schedule", time.Time{}, time.Time{}, nil)
	if !errors.Is(err, ErrScheduledAtNeeded) {
		t.Fatalf("expected ErrScheduledAtNeeded, got %v", err)
	}

	explicit := now.Add(2 * time.Hour)
	scheduledAt, err := ResolveScheduledAtForEdit("schedule", explicit, time.Time{}, nil)
	if err != nil {
		t.Fatalf("schedule resolve failed: %v", err)
	}
	if !scheduledAt.Equal(explicit.UTC()) {
		t.Fatalf("expected explicit scheduled_at, got %s", scheduledAt)
	}

	preserved, err := ResolveScheduledAtForEdit("", time.Time{}, explicit, nil)
	if err != nil {
		t.Fatalf("preserve resolve failed: %v", err)
	}
	if !preserved.Equal(explicit.UTC()) {
		t.Fatalf("expected preserved scheduled_at, got %s", preserved)
	}
}

func TestScheduleDraftRequiresApprovalWhenEditorialMetadataDemandsIt(t *testing.T) {
	store := &scheduleApprovalStore{
		post: domain.Post{
			ID:               "pst_review",
			Status:           domain.PostStatusDraft,
			RequiresApproval: true,
			EditorialStatus:  domain.EditorialStatusNeedsReview,
		},
	}
	svc := MutationsService{Store: store}

	_, err := svc.ScheduleDraft(t.Context(), "pst_review", time.Date(2026, 7, 1, 10, 0, 0, 0, time.UTC))
	if !errors.Is(err, ErrPostApprovalRequired) {
		t.Fatalf("expected ErrPostApprovalRequired, got %v", err)
	}
	if store.scheduled {
		t.Fatalf("post should not be scheduled before approval")
	}

	approvedAt := time.Date(2026, 6, 17, 10, 0, 0, 0, time.UTC)
	store.post.EditorialStatus = domain.EditorialStatusApproved
	store.post.ApprovedAt = &approvedAt
	if _, err := svc.ScheduleDraft(t.Context(), "pst_review", time.Date(2026, 7, 1, 10, 0, 0, 0, time.UTC)); err != nil {
		t.Fatalf("schedule approved post: %v", err)
	}
	if !store.scheduled {
		t.Fatalf("expected approved post to be scheduled")
	}
}

type scheduleApprovalStore struct {
	post      domain.Post
	account   domain.SocialAccount
	campaign  domain.Campaign
	schedule  []domain.Post
	scheduled bool
}

func (s *scheduleApprovalStore) CancelPost(context.Context, string) error         { return nil }
func (s *scheduleApprovalStore) DeletePostEditable(context.Context, string) error { return nil }
func (s *scheduleApprovalStore) ScheduleDraftPost(_ context.Context, _ string, scheduledAt time.Time) error {
	s.scheduled = true
	s.post.ScheduledAt = scheduledAt
	s.post.Status = domain.PostStatusScheduled
	return nil
}
func (s *scheduleApprovalStore) UpdatePostEditable(context.Context, string, string, time.Time, []string, bool) error {
	return nil
}
func (s *scheduleApprovalStore) UpdatePostEditorialMetadata(context.Context, string, domain.EditorialStatus, *bool, []string, *time.Time) error {
	return nil
}
func (s *scheduleApprovalStore) UpdateThreadEditable(context.Context, string, []db.ThreadStepUpdate) error {
	return nil
}
func (s *scheduleApprovalStore) GetPost(_ context.Context, _ string) (domain.Post, error) {
	return s.post, nil
}
func (s *scheduleApprovalStore) GetAccount(context.Context, string) (domain.SocialAccount, error) {
	return s.account, nil
}
func (s *scheduleApprovalStore) GetCampaign(context.Context, string) (domain.Campaign, error) {
	if strings.TrimSpace(s.campaign.ID) == "" {
		return domain.Campaign{Status: domain.CampaignStatusActive}, nil
	}
	return s.campaign, nil
}
func (s *scheduleApprovalStore) GetMediaByIDs(context.Context, []string) ([]domain.Media, error) {
	return nil, nil
}
func (s *scheduleApprovalStore) ListSchedule(context.Context, time.Time, time.Time) ([]domain.Post, error) {
	return append([]domain.Post(nil), s.schedule...), nil
}

func TestScheduleDraftRejectsArchivedCampaign(t *testing.T) {
	store := &scheduleApprovalStore{
		post: domain.Post{
			ID:               "pst_archived_campaign",
			Status:           domain.PostStatusDraft,
			CampaignID:       "cam_archived",
			EditorialStatus:  domain.EditorialStatusApproved,
			RequiresApproval: false,
		},
		campaign: domain.Campaign{ID: "cam_archived", Status: domain.CampaignStatusArchived},
	}
	svc := MutationsService{Store: store}

	_, err := svc.ScheduleDraft(t.Context(), "pst_archived_campaign", time.Date(2026, 7, 1, 10, 0, 0, 0, time.UTC))
	if !errors.Is(err, ErrCampaignArchived) {
		t.Fatalf("expected ErrCampaignArchived, got %v", err)
	}
	if store.scheduled {
		t.Fatalf("post should not be scheduled for archived campaign")
	}
}

func TestScheduleDraftRejectsCalendarConflict(t *testing.T) {
	target := time.Date(2026, 7, 1, 10, 0, 0, 0, time.UTC)
	store := &scheduleApprovalStore{
		post: domain.Post{
			ID:        "pst_candidate",
			AccountID: "acc_1",
			Platform:  domain.PlatformX,
			Text:      "new launch post",
			Status:    domain.PostStatusDraft,
		},
		schedule: []domain.Post{{
			ID:          "pst_existing",
			AccountID:   "acc_1",
			Platform:    domain.PlatformX,
			Text:        "other post",
			Status:      domain.PostStatusScheduled,
			ScheduledAt: target.Add(3 * time.Minute),
		}},
	}
	svc := MutationsService{Store: store}

	_, err := svc.ScheduleDraft(t.Context(), "pst_candidate", target)
	if !errors.Is(err, ErrScheduleConflict) {
		t.Fatalf("expected ErrScheduleConflict, got %v", err)
	}
	if store.scheduled {
		t.Fatalf("post should not be scheduled with a nearby account conflict")
	}
}

func TestScheduleDraftRejectsRecentDuplicateContent(t *testing.T) {
	target := time.Date(2026, 7, 8, 10, 0, 0, 0, time.UTC)
	store := &scheduleApprovalStore{
		post: domain.Post{
			ID:        "pst_candidate",
			AccountID: "acc_1",
			Platform:  domain.PlatformX,
			Text:      "Same message",
			Status:    domain.PostStatusDraft,
		},
		schedule: []domain.Post{{
			ID:          "pst_existing",
			AccountID:   "acc_1",
			Platform:    domain.PlatformX,
			Text:        " same   message ",
			Status:      domain.PostStatusScheduled,
			ScheduledAt: target.AddDate(0, 0, -3),
		}},
	}
	svc := MutationsService{Store: store}

	_, err := svc.ScheduleDraft(t.Context(), "pst_candidate", target)
	if !errors.Is(err, ErrDuplicateContentRecent) {
		t.Fatalf("expected ErrDuplicateContentRecent, got %v", err)
	}
	if store.scheduled {
		t.Fatalf("post should not be scheduled with recent duplicate content")
	}
}

func TestPreviewScheduleReturnsNormalizedPreviewAndWarnings(t *testing.T) {
	target := time.Date(2026, 7, 8, 10, 0, 0, 0, time.UTC)
	approvedAt := time.Date(2026, 6, 17, 10, 0, 0, 0, time.UTC)
	madrid := time.FixedZone("Europe/Madrid", 2*60*60)
	store := &scheduleApprovalStore{
		post: domain.Post{
			ID:               "pst_preview",
			AccountID:        "acc_1",
			Platform:         domain.PlatformX,
			Text:             "Same message",
			Status:           domain.PostStatusDraft,
			CampaignID:       "cam_archived",
			RequiresApproval: true,
			EditorialStatus:  domain.EditorialStatusNeedsReview,
			Media: []domain.Media{{
				ID:           "med_1",
				Kind:         "image",
				OriginalName: "launch.png",
			}},
			ApprovedAt: nil,
		},
		account:  domain.SocialAccount{ID: "acc_1", DisplayName: "Main X", Platform: domain.PlatformX},
		campaign: domain.Campaign{ID: "cam_archived", Name: "Archived launch", Status: domain.CampaignStatusArchived},
		schedule: []domain.Post{{
			ID:          "pst_existing",
			AccountID:   "acc_1",
			Platform:    domain.PlatformX,
			Text:        " same   message ",
			Status:      domain.PostStatusScheduled,
			ScheduledAt: target.Add(3 * time.Minute),
			ApprovedAt:  &approvedAt,
		}},
	}
	svc := MutationsService{Store: store}

	preview, err := svc.PreviewSchedule(t.Context(), "pst_preview", target, madrid)
	if err != nil {
		t.Fatalf("preview schedule: %v", err)
	}
	if preview.PostID != "pst_preview" || preview.AccountName != "Main X" || preview.Platform != string(domain.PlatformX) {
		t.Fatalf("unexpected preview identity: %+v", preview)
	}
	if !preview.ScheduledAt.Equal(target) || preview.ScheduledLocal != "2026-07-08T12:00:00+02:00" || preview.Timezone != "Europe/Madrid" {
		t.Fatalf("unexpected schedule fields: %+v", preview)
	}
	for _, expected := range []error{ErrPostApprovalRequired, ErrCampaignArchived, ErrScheduleConflict, ErrDuplicateContentRecent} {
		if !stringSliceContains(preview.Warnings, expected.Error()) {
			t.Fatalf("expected warning %q in %+v", expected.Error(), preview.Warnings)
		}
	}
	if len(preview.Media) != 1 || preview.Media[0].ID != "med_1" {
		t.Fatalf("expected media in preview, got %+v", preview.Media)
	}
	if store.scheduled {
		t.Fatalf("preview must not schedule the post")
	}
}

func stringSliceContains(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func TestMutationsServiceValidatePostID(t *testing.T) {
	svc := MutationsService{Store: &fakeMutationsStore{}}

	if err := svc.Cancel(t.Context(), " "); !errors.Is(err, ErrPostIDRequired) {
		t.Fatalf("expected ErrPostIDRequired in cancel, got %v", err)
	}
	if err := svc.DeleteEditable(t.Context(), " "); !errors.Is(err, ErrPostIDRequired) {
		t.Fatalf("expected ErrPostIDRequired in delete, got %v", err)
	}
	if _, err := svc.ScheduleDraft(t.Context(), " ", time.Now()); !errors.Is(err, ErrPostIDRequired) {
		t.Fatalf("expected ErrPostIDRequired in schedule, got %v", err)
	}
	if _, err := svc.UpdateEditable(t.Context(), EditInput{PostID: " ", Text: "hola"}, nil); !errors.Is(err, ErrPostIDRequired) {
		t.Fatalf("expected ErrPostIDRequired in update, got %v", err)
	}
}

func TestMutationsServiceScheduleAndUpdate(t *testing.T) {
	store := &fakeMutationsStore{
		post: domain.Post{
			ID:          "pst_1",
			AccountID:   "acc_1",
			Platform:    domain.PlatformX,
			Text:        "updated",
			Status:      domain.PostStatusScheduled,
			ScheduledAt: time.Date(2026, 3, 1, 12, 0, 0, 0, time.UTC),
		},
		account: domain.SocialAccount{
			ID:       "acc_1",
			Platform: domain.PlatformX,
			Status:   domain.AccountStatusConnected,
		},
	}
	svc := MutationsService{Store: store}

	scheduledAt := time.Date(2026, 3, 1, 11, 0, 0, 0, time.UTC)
	post, err := svc.ScheduleDraft(t.Context(), "pst_1", scheduledAt)
	if err != nil {
		t.Fatalf("schedule failed: %v", err)
	}
	if store.scheduleID != "pst_1" {
		t.Fatalf("expected schedule id pst_1, got %q", store.scheduleID)
	}
	if !store.scheduleAt.Equal(scheduledAt) {
		t.Fatalf("expected schedule time %s, got %s", scheduledAt, store.scheduleAt)
	}
	if post.ID != "pst_1" {
		t.Fatalf("expected post id pst_1, got %q", post.ID)
	}

	now := time.Date(2026, 3, 1, 10, 0, 0, 0, time.UTC)
	post, err = svc.UpdateEditable(t.Context(), EditInput{
		PostID: "pst_1",
		Text:   "updated text",
		Intent: "publish_now",
	}, func() time.Time { return now })
	if err != nil {
		t.Fatalf("update failed: %v", err)
	}
	if store.updateID != "pst_1" {
		t.Fatalf("expected update id pst_1, got %q", store.updateID)
	}
	if !strings.Contains(store.updateText, "updated text") {
		t.Fatalf("expected update text to be propagated")
	}
	if !store.updateScheduledAt.Equal(now) {
		t.Fatalf("expected publish_now scheduled_at=%s, got %s", now, store.updateScheduledAt)
	}
	if post.ID != "pst_1" {
		t.Fatalf("expected updated post id pst_1, got %q", post.ID)
	}
}

func TestMutationsServiceUpdateEditablePreservesScheduledAtByDefault(t *testing.T) {
	originalScheduledAt := time.Date(2026, 3, 1, 12, 0, 0, 0, time.UTC)
	store := &fakeMutationsStore{
		post: domain.Post{
			ID:          "pst_1",
			AccountID:   "acc_1",
			Platform:    domain.PlatformX,
			Text:        "scheduled",
			Status:      domain.PostStatusScheduled,
			ScheduledAt: originalScheduledAt,
		},
		account: domain.SocialAccount{
			ID:       "acc_1",
			Platform: domain.PlatformX,
			Status:   domain.AccountStatusConnected,
		},
	}
	svc := MutationsService{Store: store}

	_, err := svc.UpdateEditable(t.Context(), EditInput{
		PostID: "pst_1",
		Text:   "updated text",
	}, nil)
	if err != nil {
		t.Fatalf("update failed: %v", err)
	}
	if !store.updateScheduledAt.Equal(originalScheduledAt) {
		t.Fatalf("expected scheduled_at to be preserved (%s), got %s", originalScheduledAt, store.updateScheduledAt)
	}
}

func TestMutationsServiceUpdateEditableReplacesMediaWhenRequested(t *testing.T) {
	store := &fakeMutationsStore{
		post: domain.Post{
			ID:        "pst_1",
			AccountID: "acc_li",
			Platform:  domain.PlatformLinkedIn,
			Status:    domain.PostStatusDraft,
			Text:      "old text",
		},
		account: domain.SocialAccount{
			ID:       "acc_li",
			Platform: domain.PlatformLinkedIn,
			Status:   domain.AccountStatusConnected,
		},
		mediaByID: map[string]domain.Media{
			"med_new": {ID: "med_new", MimeType: "image/png"},
		},
	}
	svc := MutationsService{
		Store: store,
		Registry: fakeRegistry{
			providers: map[domain.Platform]postflow.Provider{
				domain.PlatformLinkedIn: postflow.NewLinkedInProvider(postflow.LinkedInProviderConfig{}),
			},
		},
	}

	_, err := svc.UpdateEditable(t.Context(), EditInput{
		PostID:       "pst_1",
		Text:         "new text",
		MediaIDs:     []string{"med_new"},
		ReplaceMedia: true,
	}, nil)
	if err != nil {
		t.Fatalf("update failed: %v", err)
	}
	if !store.updateReplaceMedia {
		t.Fatalf("expected update to replace media")
	}
	if len(store.updateMediaIDs) != 1 || store.updateMediaIDs[0] != "med_new" {
		t.Fatalf("expected media replacement with med_new, got %#v", store.updateMediaIDs)
	}
}

func TestMutationsServiceUpdateEditableAllowsClearingMediaWhenPlatformAllowsIt(t *testing.T) {
	store := &fakeMutationsStore{
		post: domain.Post{
			ID:        "pst_1",
			AccountID: "acc_li",
			Platform:  domain.PlatformLinkedIn,
			Status:    domain.PostStatusScheduled,
			Text:      "old text",
			Media: []domain.Media{
				{ID: "med_old", MimeType: "image/png"},
			},
		},
		account: domain.SocialAccount{
			ID:       "acc_li",
			Platform: domain.PlatformLinkedIn,
			Status:   domain.AccountStatusConnected,
		},
	}
	svc := MutationsService{
		Store: store,
		Registry: fakeRegistry{
			providers: map[domain.Platform]postflow.Provider{
				domain.PlatformLinkedIn: postflow.NewLinkedInProvider(postflow.LinkedInProviderConfig{}),
			},
		},
	}

	_, err := svc.UpdateEditable(t.Context(), EditInput{
		PostID:       "pst_1",
		Text:         "new text",
		MediaIDs:     []string{},
		ReplaceMedia: true,
	}, nil)
	if err != nil {
		t.Fatalf("update failed: %v", err)
	}
	if !store.updateReplaceMedia {
		t.Fatalf("expected update to replace media")
	}
	if len(store.updateMediaIDs) != 0 {
		t.Fatalf("expected media to be cleared, got %#v", store.updateMediaIDs)
	}
}

func TestMutationsServiceUpdateEditableRejectsInstagramWithoutMedia(t *testing.T) {
	store := &fakeMutationsStore{
		post: domain.Post{
			ID:        "pst_ig",
			AccountID: "acc_ig",
			Platform:  domain.PlatformInstagram,
			Status:    domain.PostStatusScheduled,
			Text:      "caption",
			Media: []domain.Media{
				{ID: "med_ig", MimeType: "image/png"},
			},
		},
		account: domain.SocialAccount{
			ID:       "acc_ig",
			Platform: domain.PlatformInstagram,
			Status:   domain.AccountStatusConnected,
		},
	}
	svc := MutationsService{
		Store: store,
		Registry: fakeRegistry{
			providers: map[domain.Platform]postflow.Provider{
				domain.PlatformInstagram: postflow.NewInstagramProvider(postflow.MetaProviderConfig{}),
			},
		},
	}

	_, err := svc.UpdateEditable(t.Context(), EditInput{
		PostID:       "pst_ig",
		Text:         "caption updated",
		MediaIDs:     []string{},
		ReplaceMedia: true,
	}, nil)
	if err == nil {
		t.Fatalf("expected validation error")
	}
	if !IsValidationError(err) {
		t.Fatalf("expected validation error wrapper, got %v", err)
	}
	if !strings.Contains(strings.ToLower(err.Error()), "instagram") || !strings.Contains(strings.ToLower(err.Error()), "media") {
		t.Fatalf("expected instagram media validation error, got %v", err)
	}
	if store.updateID != "" {
		t.Fatalf("expected no update call when validation fails")
	}
}

func TestMutationsServiceUpdateEditableRejectsInvalidThreadFollowUpForPlatform(t *testing.T) {
	store := &fakeMutationsStore{
		post: domain.Post{
			ID:        "pst_li",
			AccountID: "acc_li",
			Platform:  domain.PlatformLinkedIn,
			Status:    domain.PostStatusScheduled,
			Text:      "root",
		},
		account: domain.SocialAccount{
			ID:       "acc_li",
			Platform: domain.PlatformLinkedIn,
			Status:   domain.AccountStatusConnected,
		},
		mediaByID: map[string]domain.Media{
			"med_1": {ID: "med_1", MimeType: "image/png"},
		},
	}
	svc := MutationsService{
		Store: store,
		Registry: fakeRegistry{
			providers: map[domain.Platform]postflow.Provider{
				domain.PlatformLinkedIn: postflow.NewLinkedInProvider(postflow.LinkedInProviderConfig{}),
			},
		},
	}

	_, err := svc.UpdateEditable(t.Context(), EditInput{
		PostID: "pst_li",
		Segments: []ThreadSegmentInput{
			{Text: "root ok"},
			{Text: "child with media", MediaIDs: []string{"med_1"}},
		},
	}, nil)
	if err == nil {
		t.Fatalf("expected validation error")
	}
	if !IsValidationError(err) {
		t.Fatalf("expected validation error wrapper, got %v", err)
	}
	if !strings.Contains(strings.ToLower(err.Error()), "linkedin") || !strings.Contains(strings.ToLower(err.Error()), "do not support media") {
		t.Fatalf("expected linkedin follow-up media validation error, got %v", err)
	}
	if store.updateThreadRoot != "" {
		t.Fatalf("expected no thread update call when validation fails")
	}
}

func TestMutationsServiceUpdateValidatesTextAndSchedule(t *testing.T) {
	svc := MutationsService{Store: &fakeMutationsStore{}}

	_, err := svc.UpdateEditable(t.Context(), EditInput{
		PostID: "pst_1",
		Text:   " ",
	}, nil)
	if !errors.Is(err, ErrTextRequired) {
		t.Fatalf("expected ErrTextRequired, got %v", err)
	}

	_, err = svc.UpdateEditable(t.Context(), EditInput{
		PostID: "pst_1",
		Text:   "hola",
		Intent: "schedule",
	}, nil)
	if !errors.Is(err, ErrScheduledAtNeeded) {
		t.Fatalf("expected ErrScheduledAtNeeded, got %v", err)
	}
}

func TestMutationsServiceUpdateRejectsTooManySegments(t *testing.T) {
	store := &fakeMutationsStore{}
	svc := MutationsService{Store: store}

	segments := make([]ThreadSegmentInput, 0, MaxThreadSegments+1)
	for i := 0; i < MaxThreadSegments+1; i++ {
		segments = append(segments, ThreadSegmentInput{Text: "segment"})
	}

	_, err := svc.UpdateEditable(t.Context(), EditInput{
		PostID:   "pst_1",
		Segments: segments,
	}, nil)
	if !errors.Is(err, ErrThreadTooLong) {
		t.Fatalf("expected ErrThreadTooLong, got %v", err)
	}
}
