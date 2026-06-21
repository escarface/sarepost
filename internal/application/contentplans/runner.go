package contentplans

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	generationapp "github.com/escarface/sarepost/internal/application/generation"
	"github.com/escarface/sarepost/internal/domain"
)

type RunnerStore interface {
	GetContentPlan(ctx context.Context, id string) (domain.ContentPlan, error)
	UpdateContentPlanItemIdea(ctx context.Context, id, idea string) error
	MarkContentPlanVariantGenerating(ctx context.Context, id string) error
	CompleteContentPlanVariant(ctx context.Context, id, text, mediaID string) error
	FailContentPlanVariant(ctx context.Context, id string, failure error) error
	FinishContentPlanJob(ctx context.Context, id string, status domain.ContentPlanJobStatus, failure string) error
	ContentPlanJobActive(ctx context.Context, id string) (bool, error)
	RenewContentPlanJob(ctx context.Context, id string, leaseUntil time.Time) error
}

type TextGenerator interface {
	GenerateText(ctx context.Context, in generationapp.GenerateTextInput) (generationapp.GenerateTextOutput, error)
}

type ImageGenerator interface {
	GenerateImage(ctx context.Context, in generationapp.GenerateImageInput) (generationapp.GenerateImageOutput, error)
}

type GeneratedMediaStore interface {
	PersistGeneratedMedia(ctx context.Context, data []byte, mimeType string, tags []string) (domain.Media, error)
}

type Runner struct {
	Store          RunnerStore
	Text           TextGenerator
	Images         ImageGenerator
	Media          GeneratedMediaStore
	MaxConcurrency int
}

func (r Runner) RunJob(ctx context.Context, job domain.ContentPlanJob) error {
	if r.Store == nil || r.Text == nil {
		return errors.New("content plan runner is not configured")
	}
	plan, err := r.Store.GetContentPlan(ctx, job.PlanID)
	if err != nil {
		_ = r.Store.FinishContentPlanJob(ctx, job.ID, domain.ContentPlanJobFailed, err.Error())
		return err
	}
	blocks := make(map[string]domain.ContentPlanBlock, len(plan.Blocks))
	for _, block := range plan.Blocks {
		blocks[block.ID] = block
	}
	limit := r.MaxConcurrency
	if limit <= 0 || limit > 2 {
		limit = 2
	}
	ready, failed := 0, 0
	for _, item := range plan.Items {
		if err := ctx.Err(); err != nil {
			return err
		}
		active, err := r.Store.ContentPlanJobActive(ctx, job.ID)
		if err != nil {
			return err
		}
		if !active {
			return nil
		}
		block := blocks[item.BlockID]
		idea := strings.TrimSpace(item.Idea)
		if idea == "" {
			generated, generationErr := r.Text.GenerateText(ctx, generationapp.GenerateTextInput{
				Prompt:         buildIdeaPrompt(plan, block, item),
				BrandProfileID: block.BrandProfileID,
				ForceWebSearch: block.ForceWebSearch,
				MaxTokens:      300,
			})
			if generationErr != nil {
				for _, variant := range item.Variants {
					if !isGeneratableVariant(variant.Status) {
						continue
					}
					_ = r.Store.FailContentPlanVariant(ctx, variant.ID, generationErr)
					failed++
				}
				continue
			}
			idea = strings.TrimSpace(generated.Text)
			if err := r.Store.UpdateContentPlanItemIdea(ctx, item.ID, idea); err != nil {
				return err
			}
		}

		sem := make(chan struct{}, limit)
		var wg sync.WaitGroup
		var countsMu sync.Mutex
		for _, candidate := range item.Variants {
			variant := candidate
			if !isGeneratableVariant(variant.Status) {
				continue
			}
			wg.Add(1)
			go func() {
				defer wg.Done()
				sem <- struct{}{}
				defer func() { <-sem }()
				active, err := r.Store.ContentPlanJobActive(ctx, job.ID)
				if err != nil || !active {
					return
				}
				if err := r.generateVariant(ctx, plan, block, item, variant, idea); err != nil {
					_ = r.Store.FailContentPlanVariant(ctx, variant.ID, err)
					countsMu.Lock()
					failed++
					countsMu.Unlock()
					return
				}
				countsMu.Lock()
				ready++
				countsMu.Unlock()
			}()
		}
		wg.Wait()
		if err := r.Store.RenewContentPlanJob(ctx, job.ID, time.Now().UTC().Add(5*time.Minute)); err != nil {
			return err
		}
	}
	status := domain.ContentPlanJobCompleted
	failure := ""
	if ready == 0 && failed > 0 {
		status = domain.ContentPlanJobFailed
		failure = "all content plan variants failed"
	}
	return r.Store.FinishContentPlanJob(ctx, job.ID, status, failure)
}

func isGeneratableVariant(status domain.ContentPlanVariantStatus) bool {
	return status == domain.ContentPlanVariantPending || status == domain.ContentPlanVariantFailed || status == domain.ContentPlanVariantGenerating
}

func (r Runner) generateVariant(ctx context.Context, plan domain.ContentPlan, block domain.ContentPlanBlock, item domain.ContentPlanItem, variant domain.ContentPlanVariant, idea string) error {
	if err := r.Store.MarkContentPlanVariantGenerating(ctx, variant.ID); err != nil {
		return err
	}
	generated, err := r.Text.GenerateText(ctx, generationapp.GenerateTextInput{
		Prompt:         buildVariantPrompt(plan, block, item, variant, idea),
		Platform:       variant.Platform,
		BrandProfileID: block.BrandProfileID,
		ForceWebSearch: block.ForceWebSearch,
		MaxTokens:      500,
	})
	if err != nil {
		return err
	}
	mediaID := ""
	if block.GenerateImages {
		if r.Images == nil || r.Media == nil {
			return errors.New("content plan image generation is not configured")
		}
		image, err := r.Images.GenerateImage(ctx, generationapp.GenerateImageInput{
			Prompt:         buildVariantImagePrompt(block, variant, idea, generated.Text),
			BrandProfileID: block.BrandProfileID,
			Size:           contentPlanImageSize(variant.Platform),
		})
		if err != nil {
			return err
		}
		media, err := r.Media.PersistGeneratedMedia(ctx, image.Data, image.MimeType, []string{"content-plan", plan.ID})
		if err != nil {
			return err
		}
		mediaID = media.ID
	}
	return r.Store.CompleteContentPlanVariant(ctx, variant.ID, strings.TrimSpace(generated.Text), mediaID)
}

func contentPlanImageSize(platform domain.Platform) string {
	switch platform {
	case domain.PlatformLinkedIn, domain.PlatformFacebook:
		return "1536x1024"
	default:
		return "1024x1024"
	}
}

func buildIdeaPrompt(plan domain.ContentPlan, block domain.ContentPlanBlock, item domain.ContentPlanItem) string {
	return fmt.Sprintf("Create one concise shared editorial idea for a multi-platform content plan.\nPlan objective: %s\nBlock instructions: %s\nPlanned local time: %s\nReturn only the editorial angle, not the final post.", plan.Objective, block.Instructions, item.PlannedAt.Format("2006-01-02 15:04"))
}

func buildVariantPrompt(plan domain.ContentPlan, block domain.ContentPlanBlock, item domain.ContentPlanItem, variant domain.ContentPlanVariant, idea string) string {
	return fmt.Sprintf("Write the final social post from this shared editorial idea.\nIdea: %s\nPlan objective: %s\nBlock instructions: %s\nPlatform: %s\nPlanned local time: %s\nAdapt structure, length, tone and CTA to the platform. Return only the final post copy.", idea, plan.Objective, block.Instructions, variant.Platform, item.PlannedAt.Format("2006-01-02 15:04"))
}

func buildVariantImagePrompt(block domain.ContentPlanBlock, variant domain.ContentPlanVariant, idea, text string) string {
	prompt := strings.TrimSpace(block.ImagePrompt)
	if prompt == "" {
		prompt = "Create a brand-consistent social image related to the editorial idea."
	}
	return fmt.Sprintf("%s\nPlatform: %s\nEditorial idea: %s\nPost copy context: %s\nDo not render long body copy inside the image.", prompt, variant.Platform, idea, text)
}
