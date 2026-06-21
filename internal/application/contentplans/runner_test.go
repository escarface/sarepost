package contentplans

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	generationapp "github.com/escarface/sarepost/internal/application/generation"
	"github.com/escarface/sarepost/internal/domain"
)

type runnerStore struct {
	mu       sync.Mutex
	plan     domain.ContentPlan
	ideas    map[string]string
	variants map[string]domain.ContentPlanVariant
	jobState domain.ContentPlanJobStatus
}

func (s *runnerStore) GetContentPlan(context.Context, string) (domain.ContentPlan, error) {
	return s.plan, nil
}
func (s *runnerStore) UpdateContentPlanItemIdea(_ context.Context, id, idea string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.ideas[id] = idea
	return nil
}
func (s *runnerStore) MarkContentPlanVariantGenerating(context.Context, string) error { return nil }
func (s *runnerStore) CompleteContentPlanVariant(_ context.Context, id, text, mediaID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	v := s.variants[id]
	v.Text, v.MediaID, v.Status = text, mediaID, domain.ContentPlanVariantReady
	s.variants[id] = v
	return nil
}
func (s *runnerStore) FailContentPlanVariant(_ context.Context, id string, failure error) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	v := s.variants[id]
	v.Status, v.Error = domain.ContentPlanVariantFailed, failure.Error()
	s.variants[id] = v
	return nil
}
func (s *runnerStore) FinishContentPlanJob(_ context.Context, _ string, status domain.ContentPlanJobStatus, _ string) error {
	s.jobState = status
	return nil
}
func (s *runnerStore) ContentPlanJobActive(context.Context, string) (bool, error)   { return true, nil }
func (s *runnerStore) RenewContentPlanJob(context.Context, string, time.Time) error { return nil }

type sequenceGenerator struct {
	mu        sync.Mutex
	responses []string
	errAt     int
	calls     int
}

type planImageGenerator struct {
	inputs []generationapp.GenerateImageInput
}

func (g *planImageGenerator) GenerateImage(_ context.Context, in generationapp.GenerateImageInput) (generationapp.GenerateImageOutput, error) {
	g.inputs = append(g.inputs, in)
	return generationapp.GenerateImageOutput{Data: []byte("image"), MimeType: "image/png"}, nil
}

type planMediaStore struct{}

func (planMediaStore) PersistGeneratedMedia(context.Context, []byte, string, []string) (domain.Media, error) {
	return domain.Media{ID: "media_plan"}, nil
}

func (g *sequenceGenerator) GenerateText(_ context.Context, _ generationapp.GenerateTextInput) (generationapp.GenerateTextOutput, error) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.calls++
	if g.errAt == g.calls {
		return generationapp.GenerateTextOutput{}, errors.New("provider unavailable")
	}
	return generationapp.GenerateTextOutput{Text: g.responses[g.calls-1]}, nil
}

func TestRunnerGeneratesSharedIdeaAndPlatformVariants(t *testing.T) {
	item := domain.ContentPlanItem{ID: "item_1", BlockID: "block_1", PlannedAt: time.Now(), Variants: []domain.ContentPlanVariant{
		{ID: "variant_li", AccountID: "acc_li", Platform: domain.PlatformLinkedIn, Status: domain.ContentPlanVariantPending},
		{ID: "variant_ig", AccountID: "acc_ig", Platform: domain.PlatformInstagram, Status: domain.ContentPlanVariantPending},
	}}
	store := &runnerStore{
		plan:     domain.ContentPlan{ID: "plan_1", Objective: "Teach", Blocks: []domain.ContentPlanBlock{{ID: "block_1", BrandProfileID: "brand_sare", Instructions: "Focus on workflows"}}, Items: []domain.ContentPlanItem{item}},
		ideas:    map[string]string{},
		variants: map[string]domain.ContentPlanVariant{"variant_li": item.Variants[0], "variant_ig": item.Variants[1]},
	}
	generator := &sequenceGenerator{responses: []string{"Shared editorial angle", "LinkedIn copy", "Instagram copy"}}

	err := (Runner{Store: store, Text: generator, MaxConcurrency: 2}).RunJob(t.Context(), domain.ContentPlanJob{ID: "job_1", PlanID: "plan_1"})
	if err != nil {
		t.Fatalf("run job: %v", err)
	}
	if store.ideas["item_1"] != "Shared editorial angle" {
		t.Fatalf("expected persisted shared idea, got %#v", store.ideas)
	}
	if store.variants["variant_li"].Status != domain.ContentPlanVariantReady || store.variants["variant_ig"].Status != domain.ContentPlanVariantReady {
		t.Fatalf("expected ready variants, got %#v", store.variants)
	}
	if store.jobState != domain.ContentPlanJobCompleted {
		t.Fatalf("expected completed job, got %s", store.jobState)
	}
}

func TestRunnerKeepsSuccessfulVariantsWhenAnotherFails(t *testing.T) {
	item := domain.ContentPlanItem{ID: "item_1", BlockID: "block_1", Variants: []domain.ContentPlanVariant{
		{ID: "variant_1", Platform: domain.PlatformLinkedIn, Status: domain.ContentPlanVariantPending},
		{ID: "variant_2", Platform: domain.PlatformInstagram, Status: domain.ContentPlanVariantPending},
	}}
	store := &runnerStore{plan: domain.ContentPlan{ID: "plan_1", Blocks: []domain.ContentPlanBlock{{ID: "block_1"}}, Items: []domain.ContentPlanItem{item}}, ideas: map[string]string{}, variants: map[string]domain.ContentPlanVariant{"variant_1": item.Variants[0], "variant_2": item.Variants[1]}}
	generator := &sequenceGenerator{responses: []string{"Idea", "LinkedIn", "Instagram"}, errAt: 3}

	err := (Runner{Store: store, Text: generator, MaxConcurrency: 1}).RunJob(t.Context(), domain.ContentPlanJob{ID: "job_1", PlanID: "plan_1"})
	if err != nil {
		t.Fatalf("partial failures must be persisted rather than aborting the job: %v", err)
	}
	statuses := map[domain.ContentPlanVariantStatus]int{}
	statuses[store.variants["variant_1"].Status]++
	statuses[store.variants["variant_2"].Status]++
	if statuses[domain.ContentPlanVariantReady] != 1 || statuses[domain.ContentPlanVariantFailed] != 1 {
		t.Fatalf("unexpected partial results: %#v", store.variants)
	}
}

func TestRunnerResumesVariantLeftGeneratingByExpiredWorker(t *testing.T) {
	item := domain.ContentPlanItem{ID: "item_resume", BlockID: "block_resume", Idea: "Persisted idea", Variants: []domain.ContentPlanVariant{{ID: "variant_resume", Platform: domain.PlatformX, Status: domain.ContentPlanVariantGenerating}}}
	store := &runnerStore{plan: domain.ContentPlan{ID: "plan_resume", Blocks: []domain.ContentPlanBlock{{ID: "block_resume"}}, Items: []domain.ContentPlanItem{item}}, ideas: map[string]string{}, variants: map[string]domain.ContentPlanVariant{"variant_resume": item.Variants[0]}}
	generator := &sequenceGenerator{responses: []string{"Resumed copy"}}
	if err := (Runner{Store: store, Text: generator}).RunJob(t.Context(), domain.ContentPlanJob{ID: "job_resume", PlanID: "plan_resume"}); err != nil {
		t.Fatalf("resume job: %v", err)
	}
	if got := store.variants["variant_resume"]; got.Status != domain.ContentPlanVariantReady || got.Text != "Resumed copy" {
		t.Fatalf("expected resumed ready variant, got %#v", got)
	}
}

func TestRunnerGeneratesPlatformSizedImageWhenBlockRequestsIt(t *testing.T) {
	item := domain.ContentPlanItem{ID: "item_image", BlockID: "block_image", Idea: "Visual idea", Variants: []domain.ContentPlanVariant{{ID: "variant_image", Platform: domain.PlatformLinkedIn, Status: domain.ContentPlanVariantPending}}}
	store := &runnerStore{plan: domain.ContentPlan{ID: "plan_image", Blocks: []domain.ContentPlanBlock{{ID: "block_image", GenerateImages: true}}, Items: []domain.ContentPlanItem{item}}, ideas: map[string]string{}, variants: map[string]domain.ContentPlanVariant{"variant_image": item.Variants[0]}}
	images := &planImageGenerator{}
	err := (Runner{Store: store, Text: &sequenceGenerator{responses: []string{"LinkedIn visual copy"}}, Images: images, Media: planMediaStore{}}).RunJob(t.Context(), domain.ContentPlanJob{ID: "job_image", PlanID: "plan_image"})
	if err != nil {
		t.Fatalf("run image plan: %v", err)
	}
	if len(images.inputs) != 1 || images.inputs[0].Size != "1536x1024" {
		t.Fatalf("unexpected image inputs: %#v", images.inputs)
	}
	if got := store.variants["variant_image"].MediaID; got != "media_plan" {
		t.Fatalf("expected attached media, got %q", got)
	}
}
