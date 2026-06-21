package db

import (
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/escarface/sarepost/internal/domain"
)

func TestContentPlanPersistencePreservesBlocksItemsAndVariants(t *testing.T) {
	store, err := Open(filepath.Join(t.TempDir(), "content-plans.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer store.Close()
	linkedin := createTestAccount(t, store, domain.PlatformLinkedIn)
	instagram := createTestAccount(t, store, domain.PlatformInstagram)

	from := time.Date(2026, time.July, 6, 0, 0, 0, 0, time.UTC)
	plan, err := store.CreateContentPlan(t.Context(), domain.ContentPlan{
		Name:      "Q3 multi-brand",
		Objective: "Educate and convert",
		Timezone:  "Europe/Madrid",
		StartsAt:  from,
		EndsAt:    from.AddDate(0, 0, 30),
		Status:    domain.ContentPlanStatusDraft,
		Blocks: []domain.ContentPlanBlock{{
			BrandProfileID: "brand_sare",
			CampaignID:     "",
			Instructions:   "Explain one workflow improvement",
			AccountIDs:     []string{linkedin.ID, instagram.ID},
			Weekdays:       []int{1, 3},
			Slots:          []string{"09:00"},
			GenerateImages: true,
			ForceWebSearch: true,
		}},
		Items: []domain.ContentPlanItem{{
			PlannedAt: from.Add(9 * time.Hour),
			Variants:  []domain.ContentPlanVariant{{AccountID: linkedin.ID, Status: domain.ContentPlanVariantPending}, {AccountID: instagram.ID, Status: domain.ContentPlanVariantPending}},
		}},
	})
	if err != nil {
		t.Fatalf("create content plan: %v", err)
	}
	if plan.ID == "" || len(plan.Blocks) != 1 || len(plan.Items) != 1 || len(plan.Items[0].Variants) != 2 {
		t.Fatalf("unexpected created plan: %#v", plan)
	}

	loaded, err := store.GetContentPlan(t.Context(), plan.ID)
	if err != nil {
		t.Fatalf("get content plan: %v", err)
	}
	if loaded.Name != plan.Name || loaded.Blocks[0].BrandProfileID != "brand_sare" || !loaded.Blocks[0].GenerateImages || !loaded.Blocks[0].ForceWebSearch {
		t.Fatalf("unexpected loaded plan: %#v", loaded)
	}
	if got := loaded.Items[0].Variants[1].AccountID; got != instagram.ID {
		t.Fatalf("expected persisted account mapping, got %q", got)
	}
}

func TestContentPlanJobsAreClaimedAndRecoverExpiredLease(t *testing.T) {
	store, err := Open(filepath.Join(t.TempDir(), "content-plan-jobs.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer store.Close()

	plan, err := store.CreateContentPlan(t.Context(), domain.ContentPlan{Name: "Recoverable", StartsAt: time.Now(), EndsAt: time.Now(), Status: domain.ContentPlanStatusDraft})
	if err != nil {
		t.Fatalf("create plan: %v", err)
	}
	job, err := store.EnqueueContentPlanJob(t.Context(), plan.ID)
	if err != nil {
		t.Fatalf("enqueue job: %v", err)
	}
	secondPlan, err := store.CreateContentPlan(t.Context(), domain.ContentPlan{Name: "Waiting", StartsAt: time.Now(), EndsAt: time.Now(), Status: domain.ContentPlanStatusDraft})
	if err != nil {
		t.Fatalf("create second plan: %v", err)
	}
	if _, err := store.EnqueueContentPlanJob(t.Context(), secondPlan.ID); err != nil {
		t.Fatalf("enqueue second job: %v", err)
	}
	claimed, err := store.ClaimContentPlanJob(t.Context(), time.Minute)
	if err != nil {
		t.Fatalf("claim job: %v", err)
	}
	if claimed.ID != job.ID || claimed.Status != domain.ContentPlanJobRunning {
		t.Fatalf("unexpected claimed job: %#v", claimed)
	}
	if _, err := store.ClaimContentPlanJob(t.Context(), time.Minute); !errors.Is(err, ErrNoContentPlanJob) {
		t.Fatalf("expected no second job, got %v", err)
	}

	if _, err := store.db.ExecContext(t.Context(), `UPDATE content_plan_jobs SET lease_until = ? WHERE id = ?`, time.Now().UTC().Add(-time.Minute).Format(time.RFC3339Nano), job.ID); err != nil {
		t.Fatalf("expire lease: %v", err)
	}
	reclaimed, err := store.ClaimContentPlanJob(t.Context(), time.Minute)
	if err != nil {
		t.Fatalf("reclaim expired job: %v", err)
	}
	if reclaimed.ID != job.ID {
		t.Fatalf("expected reclaimed job %q, got %#v", job.ID, reclaimed)
	}
}

func TestCanceledContentPlanJobCannotBeCompletedByInFlightWorker(t *testing.T) {
	store, err := Open(filepath.Join(t.TempDir(), "content-plan-cancel.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer store.Close()
	now := time.Now().UTC()
	plan, err := store.CreateContentPlan(t.Context(), domain.ContentPlan{Name: "Cancel", StartsAt: now, EndsAt: now, Status: domain.ContentPlanStatusDraft})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	job, err := store.EnqueueContentPlanJob(t.Context(), plan.ID)
	if err != nil {
		t.Fatalf("enqueue: %v", err)
	}
	if _, err := store.ClaimContentPlanJob(t.Context(), time.Minute); err != nil {
		t.Fatalf("claim: %v", err)
	}
	if err := store.CancelContentPlan(t.Context(), plan.ID); err != nil {
		t.Fatalf("cancel: %v", err)
	}
	if err := store.FinishContentPlanJob(t.Context(), job.ID, domain.ContentPlanJobCompleted, ""); err != nil {
		t.Fatalf("finish canceled: %v", err)
	}
	loaded, _ := store.GetContentPlan(t.Context(), plan.ID)
	if loaded.Status != domain.ContentPlanStatusCanceled {
		t.Fatalf("worker overwrote canceled plan with %s", loaded.Status)
	}
}

func TestContentPlanGenerationTransitionsPersistProgress(t *testing.T) {
	store, err := Open(filepath.Join(t.TempDir(), "content-plan-progress.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer store.Close()
	account := createTestAccount(t, store, domain.PlatformLinkedIn)
	now := time.Now().UTC()
	plan, err := store.CreateContentPlan(t.Context(), domain.ContentPlan{
		Name: "Progress", StartsAt: now, EndsAt: now, Status: domain.ContentPlanStatusDraft,
		Blocks: []domain.ContentPlanBlock{{ID: "block_1", AccountIDs: []string{account.ID}}},
		Items:  []domain.ContentPlanItem{{ID: "item_1", BlockID: "block_1", PlannedAt: now, Variants: []domain.ContentPlanVariant{{ID: "variant_1", AccountID: account.ID}}}},
	})
	if err != nil {
		t.Fatalf("create plan: %v", err)
	}
	job, err := store.EnqueueContentPlanJob(t.Context(), plan.ID)
	if err != nil {
		t.Fatalf("enqueue: %v", err)
	}
	if err := store.UpdateContentPlanItemIdea(t.Context(), "item_1", "Shared angle"); err != nil {
		t.Fatalf("save idea: %v", err)
	}
	if err := store.MarkContentPlanVariantGenerating(t.Context(), "variant_1"); err != nil {
		t.Fatalf("mark generating: %v", err)
	}
	if err := store.CompleteContentPlanVariant(t.Context(), "variant_1", "Final LinkedIn post", ""); err != nil {
		t.Fatalf("complete variant: %v", err)
	}
	if err := store.FinishContentPlanJob(t.Context(), job.ID, domain.ContentPlanJobCompleted, ""); err != nil {
		t.Fatalf("finish job: %v", err)
	}

	loaded, err := store.GetContentPlan(t.Context(), plan.ID)
	if err != nil {
		t.Fatalf("get plan: %v", err)
	}
	if loaded.Status != domain.ContentPlanStatusReview || loaded.Items[0].Idea != "Shared angle" {
		t.Fatalf("unexpected generated plan: %#v", loaded)
	}
	variant := loaded.Items[0].Variants[0]
	if variant.Status != domain.ContentPlanVariantReady || variant.Text != "Final LinkedIn post" || variant.GenerationRuns != 1 {
		t.Fatalf("unexpected generated variant: %#v", variant)
	}
}

func TestContentPlanEditorialMutationsAreScopedAndIdempotent(t *testing.T) {
	store, err := Open(filepath.Join(t.TempDir(), "content-plan-editorial.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer store.Close()
	account := createTestAccount(t, store, domain.PlatformX)
	now := time.Now().UTC().Add(time.Hour)
	plan, err := store.CreateContentPlan(t.Context(), domain.ContentPlan{
		Name: "Editorial", StartsAt: now, EndsAt: now, Status: domain.ContentPlanStatusReview,
		Blocks: []domain.ContentPlanBlock{{ID: "block_editorial", AccountIDs: []string{account.ID}}},
		Items:  []domain.ContentPlanItem{{ID: "item_editorial", BlockID: "block_editorial", PlannedAt: now, Idea: "Original angle", Variants: []domain.ContentPlanVariant{{ID: "variant_editorial", AccountID: account.ID, Text: "Original", Status: domain.ContentPlanVariantReady}}}},
	})
	if err != nil {
		t.Fatalf("create plan: %v", err)
	}
	updatedAt := now.Add(24 * time.Hour)
	if err := store.UpdateContentPlanVariant(t.Context(), plan.ID, "variant_editorial", "Edited", updatedAt, ""); err != nil {
		t.Fatalf("update variant: %v", err)
	}
	if err := store.ResetContentPlanVariants(t.Context(), plan.ID, []string{"variant_editorial"}); err != nil {
		t.Fatalf("reset variant: %v", err)
	}
	loaded, err := store.GetContentPlan(t.Context(), plan.ID)
	if err != nil {
		t.Fatalf("get plan: %v", err)
	}
	v := loaded.Items[0].Variants[0]
	if v.Status != domain.ContentPlanVariantPending || v.Text != "" || !v.PlannedAt.Equal(updatedAt) {
		t.Fatalf("unexpected reset variant: %#v", v)
	}
	if err := store.ResetContentPlanItems(t.Context(), plan.ID, []string{"item_editorial"}); err != nil {
		t.Fatalf("reset item: %v", err)
	}
	regenerating, _ := store.GetContentPlan(t.Context(), plan.ID)
	if regenerating.Items[0].Idea != "" {
		t.Fatalf("expected shared idea to be reset, got %q", regenerating.Items[0].Idea)
	}
	plans, err := store.ListContentPlans(t.Context(), 20)
	if err != nil || len(plans) != 1 || plans[0].ID != plan.ID {
		t.Fatalf("unexpected plans: %#v err=%v", plans, err)
	}
	if err := store.CancelContentPlan(t.Context(), plan.ID); err != nil {
		t.Fatalf("cancel plan: %v", err)
	}
	canceled, _ := store.GetContentPlan(t.Context(), plan.ID)
	if canceled.Status != domain.ContentPlanStatusCanceled {
		t.Fatalf("expected canceled plan, got %s", canceled.Status)
	}
}

func TestContentPlanScheduleStatusTracksMaterializedVariants(t *testing.T) {
	store, err := Open(filepath.Join(t.TempDir(), "content-plan-schedule.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer store.Close()
	account := createTestAccount(t, store, domain.PlatformX)
	now := time.Now().UTC().Add(time.Hour)
	plan, err := store.CreateContentPlan(t.Context(), domain.ContentPlan{
		Name: "Schedule", StartsAt: now, EndsAt: now, Status: domain.ContentPlanStatusReview,
		Blocks: []domain.ContentPlanBlock{{ID: "block_schedule", AccountIDs: []string{account.ID}}},
		Items: []domain.ContentPlanItem{{ID: "item_schedule", BlockID: "block_schedule", PlannedAt: now, Variants: []domain.ContentPlanVariant{
			{ID: "variant_a", AccountID: account.ID, Text: "A", Status: domain.ContentPlanVariantReady},
			{ID: "variant_b", AccountID: account.ID, Text: "B", Status: domain.ContentPlanVariantReady},
		}}},
	})
	if err != nil {
		t.Fatalf("create plan: %v", err)
	}
	postA, err := store.CreatePost(t.Context(), CreatePostParams{Post: domain.Post{AccountID: account.ID, Text: "A", ScheduledAt: now}, IdempotencyKey: "plan-a"})
	if err != nil {
		t.Fatalf("create post A: %v", err)
	}
	if err := store.MarkContentPlanVariantScheduled(t.Context(), plan.ID, "variant_a", postA.Post.ID); err != nil {
		t.Fatalf("mark A: %v", err)
	}
	if err := store.RefreshContentPlanScheduleStatus(t.Context(), plan.ID); err != nil {
		t.Fatalf("refresh partial: %v", err)
	}
	partial, _ := store.GetContentPlan(t.Context(), plan.ID)
	if partial.Status != domain.ContentPlanStatusPartiallyScheduled {
		t.Fatalf("expected partial status, got %s", partial.Status)
	}
	postB, err := store.CreatePost(t.Context(), CreatePostParams{Post: domain.Post{AccountID: account.ID, Text: "B", ScheduledAt: now.Add(time.Hour)}, IdempotencyKey: "plan-b"})
	if err != nil {
		t.Fatalf("create post B: %v", err)
	}
	if err := store.MarkContentPlanVariantScheduled(t.Context(), plan.ID, "variant_b", postB.Post.ID); err != nil {
		t.Fatalf("mark B: %v", err)
	}
	if err := store.RefreshContentPlanScheduleStatus(t.Context(), plan.ID); err != nil {
		t.Fatalf("refresh full: %v", err)
	}
	complete, _ := store.GetContentPlan(t.Context(), plan.ID)
	if complete.Status != domain.ContentPlanStatusScheduled {
		t.Fatalf("expected scheduled status, got %s", complete.Status)
	}
}

func TestGeneratedPlanMediaCountsAsInUseBeforeScheduling(t *testing.T) {
	store, err := Open(filepath.Join(t.TempDir(), "content-plan-media.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer store.Close()
	account := createTestAccount(t, store, domain.PlatformInstagram)
	media, err := store.CreateMedia(t.Context(), domain.Media{Kind: "image", OriginalName: "plan.png", StoragePath: filepath.Join(t.TempDir(), "plan.png"), MimeType: "image/png", SizeBytes: 10})
	if err != nil {
		t.Fatalf("create media: %v", err)
	}
	now := time.Now().UTC()
	_, err = store.CreateContentPlan(t.Context(), domain.ContentPlan{Name: "Media", StartsAt: now, EndsAt: now, Status: domain.ContentPlanStatusReview, Blocks: []domain.ContentPlanBlock{{ID: "block_media", AccountIDs: []string{account.ID}}}, Items: []domain.ContentPlanItem{{ID: "item_media", BlockID: "block_media", PlannedAt: now, Variants: []domain.ContentPlanVariant{{ID: "variant_media", AccountID: account.ID, MediaID: media.ID, Status: domain.ContentPlanVariantReady}}}}})
	if err != nil {
		t.Fatalf("create plan: %v", err)
	}
	items, err := store.ListMedia(t.Context(), 10)
	if err != nil {
		t.Fatalf("list media: %v", err)
	}
	if len(items) != 1 || items[0].UsageCount != 1 {
		t.Fatalf("expected plan usage, got %#v", items)
	}
	if _, err := store.DeleteMediaIfUnused(t.Context(), media.ID); !errors.Is(err, ErrMediaInUse) {
		t.Fatalf("expected ErrMediaInUse, got %v", err)
	}
}
