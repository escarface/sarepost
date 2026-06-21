package contentplans

import (
	"context"
	"errors"
	"testing"
	"time"

	generationapp "github.com/escarface/sarepost/internal/application/generation"
	"github.com/escarface/sarepost/internal/domain"
)

type serviceStore struct {
	accounts   map[string]domain.SocialAccount
	created    domain.ContentPlan
	job        domain.ContentPlanJob
	scheduled  map[string]string
	resetItems []string
}

func (s *serviceStore) GetAccount(_ context.Context, id string) (domain.SocialAccount, error) {
	account, ok := s.accounts[id]
	if !ok {
		return domain.SocialAccount{}, errors.New("not found")
	}
	return account, nil
}

func (s *serviceStore) CreateContentPlan(_ context.Context, plan domain.ContentPlan) (domain.ContentPlan, error) {
	s.created = plan
	plan.ID = "plan_1"
	return plan, nil
}

func (s *serviceStore) GetContentPlan(_ context.Context, _ string) (domain.ContentPlan, error) {
	return s.created, nil
}

func (s *serviceStore) EnqueueContentPlanJob(_ context.Context, planID string) (domain.ContentPlanJob, error) {
	s.job = domain.ContentPlanJob{ID: "job_1", PlanID: planID, Status: domain.ContentPlanJobPending}
	return s.job, nil
}

func (s *serviceStore) CancelContentPlan(context.Context, string) error { return nil }
func (s *serviceStore) UpdateContentPlanVariant(_ context.Context, _, id, text string, plannedAt time.Time, mediaID string) error {
	for itemIndex := range s.created.Items {
		for variantIndex := range s.created.Items[itemIndex].Variants {
			variant := &s.created.Items[itemIndex].Variants[variantIndex]
			if variant.ID == id {
				variant.Text, variant.PlannedAt, variant.MediaID = text, plannedAt, mediaID
			}
		}
	}
	return nil
}
func (s *serviceStore) ResetContentPlanVariants(context.Context, string, []string) error { return nil }
func (s *serviceStore) ResetContentPlanItems(_ context.Context, _ string, itemIDs []string) error {
	s.resetItems = append([]string(nil), itemIDs...)
	return nil
}
func (s *serviceStore) MarkContentPlanVariantScheduled(_ context.Context, _, variantID, postID string) error {
	if s.scheduled == nil {
		s.scheduled = map[string]string{}
	}
	s.scheduled[variantID] = postID
	return nil
}
func (s *serviceStore) RefreshContentPlanScheduleStatus(context.Context, string) error { return nil }

type profileCatalog struct{ profiles []generationapp.BrandProfile }

func (p profileCatalog) ListBrandProfiles(context.Context) ([]generationapp.BrandProfile, error) {
	return p.profiles, nil
}

func TestServiceCreatesMultiBrandPlanWithSharedItems(t *testing.T) {
	from := time.Date(2026, time.July, 6, 0, 0, 0, 0, time.UTC)
	store := &serviceStore{accounts: map[string]domain.SocialAccount{
		"acc_li": {ID: "acc_li", Platform: domain.PlatformLinkedIn, Status: domain.AccountStatusConnected},
		"acc_ig": {ID: "acc_ig", Platform: domain.PlatformInstagram, Status: domain.AccountStatusConnected},
	}}
	service := Service{Store: store, Profiles: profileCatalog{profiles: []generationapp.BrandProfile{{ID: "brand_sare"}, {ID: "brand_client"}}}}

	plan, preview, err := service.Create(t.Context(), CreateInput{
		Name:      "July plan",
		Objective: "Build trust",
		From:      from,
		To:        from.AddDate(0, 0, 6).Add(23*time.Hour + 59*time.Minute),
		Timezone:  "UTC",
		Blocks: []BlockInput{
			{BrandProfileID: "brand_sare", AccountIDs: []string{"acc_li", "acc_ig"}, Weekdays: []time.Weekday{time.Monday}, Slots: []string{"09:00"}},
			{BrandProfileID: "brand_client", AccountIDs: []string{"acc_ig"}, Weekdays: []time.Weekday{time.Wednesday}, Slots: []string{"17:00"}},
		},
	})
	if err != nil {
		t.Fatalf("create plan: %v", err)
	}
	if plan.ID != "plan_1" || preview.ItemCount != 2 || preview.VariantCount != 3 {
		t.Fatalf("unexpected result plan=%#v preview=%#v", plan, preview)
	}
	if len(store.created.Blocks) != 2 || len(store.created.Items) != 2 {
		t.Fatalf("expected persisted plan graph, got %#v", store.created)
	}
	if got := len(store.created.Items[0].Variants); got != 2 {
		t.Fatalf("expected shared item with two platform variants, got %d", got)
	}
}

func TestServiceRejectsDisconnectedAccountsBeforePersisting(t *testing.T) {
	from := time.Date(2026, time.July, 6, 0, 0, 0, 0, time.UTC)
	store := &serviceStore{accounts: map[string]domain.SocialAccount{"acc_x": {ID: "acc_x", Status: domain.AccountStatusDisconnected}}}
	service := Service{Store: store, Profiles: profileCatalog{profiles: []generationapp.BrandProfile{{ID: "brand_sare"}}}}

	_, _, err := service.Create(t.Context(), CreateInput{Name: "Invalid", From: from, To: from, Blocks: []BlockInput{{BrandProfileID: "brand_sare", AccountIDs: []string{"acc_x"}, Weekdays: []time.Weekday{time.Monday}, Slots: []string{"09:00"}}}})
	if !errors.Is(err, ErrAccountNotConnected) {
		t.Fatalf("expected ErrAccountNotConnected, got %v", err)
	}
	if store.created.ID != "" || store.created.Name != "" {
		t.Fatalf("invalid plan must not be persisted: %#v", store.created)
	}
}

func TestServiceEnqueuesDraftPlanOnlyOnce(t *testing.T) {
	store := &serviceStore{created: domain.ContentPlan{ID: "plan_1", Status: domain.ContentPlanStatusDraft}}
	service := Service{Store: store}
	job, err := service.Generate(t.Context(), "plan_1")
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	if job.ID != "job_1" || store.job.PlanID != "plan_1" {
		t.Fatalf("unexpected job: %#v", job)
	}
}

type variantScheduler struct{ failures map[string]error }

func (s variantScheduler) ScheduleVariant(_ context.Context, variant domain.ContentPlanVariant, _ domain.ContentPlanBlock) (domain.Post, error) {
	if err := s.failures[variant.ID]; err != nil {
		return domain.Post{}, err
	}
	return domain.Post{ID: "post_" + variant.ID}, nil
}

func TestServiceSchedulesValidVariantsAndReportsConflicts(t *testing.T) {
	store := &serviceStore{created: domain.ContentPlan{
		ID: "plan_1", Status: domain.ContentPlanStatusReview,
		Blocks: []domain.ContentPlanBlock{{ID: "block_1"}},
		Items: []domain.ContentPlanItem{{BlockID: "block_1", Variants: []domain.ContentPlanVariant{
			{ID: "variant_ok", Status: domain.ContentPlanVariantReady},
			{ID: "variant_conflict", Status: domain.ContentPlanVariantReady},
		}}},
	}}
	service := Service{Store: store, Scheduler: variantScheduler{failures: map[string]error{"variant_conflict": errors.New("schedule conflict")}}}

	result, err := service.Schedule(t.Context(), "plan_1", []string{"variant_ok", "variant_conflict"})
	if err != nil {
		t.Fatalf("schedule variants: %v", err)
	}
	if len(result.Scheduled) != 1 || result.Scheduled[0].VariantID != "variant_ok" {
		t.Fatalf("unexpected scheduled variants: %#v", result)
	}
	if len(result.Conflicts) != 1 || result.Conflicts[0].VariantID != "variant_conflict" {
		t.Fatalf("expected one reported conflict: %#v", result)
	}
	if store.scheduled["variant_ok"] != "post_variant_ok" {
		t.Fatalf("expected persisted post link, got %#v", store.scheduled)
	}
}

func TestServiceRejectsRetryWhileGenerationIsAlreadyQueued(t *testing.T) {
	store := &serviceStore{created: domain.ContentPlan{ID: "plan_queued", Status: domain.ContentPlanStatusQueued}}
	_, err := (Service{Store: store}).Retry(t.Context(), "plan_queued", []string{"variant_1"})
	if !errors.Is(err, ErrPlanNotReviewable) {
		t.Fatalf("expected ErrPlanNotReviewable, got %v", err)
	}
}

func TestServiceRegeneratesWholeEditorialItem(t *testing.T) {
	store := &serviceStore{created: domain.ContentPlan{ID: "plan_review", Status: domain.ContentPlanStatusReview}}
	job, err := (Service{Store: store}).Regenerate(t.Context(), "plan_review", nil, []string{"item_1"})
	if err != nil {
		t.Fatalf("regenerate: %v", err)
	}
	if job.ID != "job_1" || len(store.resetItems) != 1 || store.resetItems[0] != "item_1" {
		t.Fatalf("unexpected regeneration job=%#v items=%#v", job, store.resetItems)
	}
}
