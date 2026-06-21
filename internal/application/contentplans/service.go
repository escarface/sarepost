package contentplans

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	generationapp "github.com/escarface/sarepost/internal/application/generation"
	"github.com/escarface/sarepost/internal/domain"
)

var (
	ErrAccountNotConnected = errors.New("content plan account is not connected")
	ErrBrandProfileMissing = errors.New("content plan brand profile not found")
	ErrPlanNotDraft        = errors.New("content plan is not a draft")
	ErrPlanNotReviewable   = errors.New("content plan is not ready for regeneration")
)

type Store interface {
	GetAccount(ctx context.Context, id string) (domain.SocialAccount, error)
	CreateContentPlan(ctx context.Context, plan domain.ContentPlan) (domain.ContentPlan, error)
	GetContentPlan(ctx context.Context, id string) (domain.ContentPlan, error)
	EnqueueContentPlanJob(ctx context.Context, planID string) (domain.ContentPlanJob, error)
}

type ProfileCatalog interface {
	ListBrandProfiles(ctx context.Context) ([]generationapp.BrandProfile, error)
}

type Service struct {
	Store     Store
	Profiles  ProfileCatalog
	Scheduler VariantScheduler
}

type VariantScheduler interface {
	ScheduleVariant(ctx context.Context, variant domain.ContentPlanVariant, block domain.ContentPlanBlock) (domain.Post, error)
}

type editorialStore interface {
	CancelContentPlan(ctx context.Context, planID string) error
	UpdateContentPlanVariant(ctx context.Context, planID, variantID, text string, plannedAt time.Time, mediaID string) error
	ResetContentPlanVariants(ctx context.Context, planID string, variantIDs []string) error
	ResetContentPlanItems(ctx context.Context, planID string, itemIDs []string) error
	MarkContentPlanVariantScheduled(ctx context.Context, planID, variantID, postID string) error
	RefreshContentPlanScheduleStatus(ctx context.Context, planID string) error
}

type ScheduleItem struct {
	VariantID string      `json:"variant_id"`
	Post      domain.Post `json:"post"`
}

type ScheduleConflict struct {
	VariantID string `json:"variant_id"`
	Error     string `json:"error"`
}

type ScheduleResult struct {
	Scheduled []ScheduleItem     `json:"scheduled"`
	Conflicts []ScheduleConflict `json:"conflicts"`
}

type CreateInput struct {
	Name      string
	Objective string
	From      time.Time
	To        time.Time
	Timezone  string
	Blocks    []BlockInput
}

func (s Service) Preview(ctx context.Context, in CreateInput) (Preview, error) {
	if s.Store == nil {
		return Preview{}, errors.New("content plan store is not configured")
	}
	if strings.TrimSpace(in.Name) == "" {
		return Preview{}, errors.New("content plan name is required")
	}
	if len(in.Blocks) == 0 {
		return Preview{}, errors.New("at least one content plan block is required")
	}
	profiles := make(map[string]bool)
	if s.Profiles != nil {
		list, err := s.Profiles.ListBrandProfiles(ctx)
		if err != nil {
			return Preview{}, err
		}
		for _, profile := range list {
			profiles[strings.TrimSpace(profile.ID)] = true
		}
	}
	for index, block := range in.Blocks {
		brandProfileID := strings.TrimSpace(block.BrandProfileID)
		if brandProfileID == "" || !profiles[brandProfileID] {
			return Preview{}, fmt.Errorf("block %d: %w: %s", index+1, ErrBrandProfileMissing, brandProfileID)
		}
		for _, accountID := range uniqueNonEmpty(block.AccountIDs) {
			account, err := s.Store.GetAccount(ctx, accountID)
			if err != nil {
				return Preview{}, fmt.Errorf("block %d account %s: %w", index+1, accountID, err)
			}
			if account.Status != domain.AccountStatusConnected {
				return Preview{}, fmt.Errorf("block %d account %s: %w", index+1, accountID, ErrAccountNotConnected)
			}
		}
	}
	return BuildPlanSchedule(PreviewInput{From: in.From, To: in.To, Timezone: in.Timezone, Blocks: in.Blocks})
}

func (s Service) Create(ctx context.Context, in CreateInput) (domain.ContentPlan, Preview, error) {
	preview, err := s.Preview(ctx, in)
	if err != nil {
		return domain.ContentPlan{}, Preview{}, err
	}
	plan, err := s.buildPlanGraph(ctx, in, preview)
	if err != nil {
		return domain.ContentPlan{}, Preview{}, err
	}
	created, err := s.Store.CreateContentPlan(ctx, plan)
	if err != nil {
		return domain.ContentPlan{}, Preview{}, err
	}
	return created, preview, nil
}

func (s Service) buildPlanGraph(ctx context.Context, in CreateInput, preview Preview) (domain.ContentPlan, error) {
	plan := domain.ContentPlan{
		Name:      strings.TrimSpace(in.Name),
		Objective: strings.TrimSpace(in.Objective),
		Timezone:  strings.TrimSpace(in.Timezone),
		StartsAt:  in.From,
		EndsAt:    in.To,
		Status:    domain.ContentPlanStatusDraft,
		Blocks:    make([]domain.ContentPlanBlock, len(in.Blocks)),
	}
	for i, block := range in.Blocks {
		blockID, err := contentPlanID("plb")
		if err != nil {
			return domain.ContentPlan{}, err
		}
		weekdays := make([]int, len(block.Weekdays))
		for j, weekday := range block.Weekdays {
			weekdays[j] = int(weekday)
		}
		plan.Blocks[i] = domain.ContentPlanBlock{
			ID:             blockID,
			BrandProfileID: strings.TrimSpace(block.BrandProfileID),
			CampaignID:     strings.TrimSpace(block.CampaignID),
			Instructions:   strings.TrimSpace(block.Instructions),
			AccountIDs:     uniqueNonEmpty(block.AccountIDs),
			Weekdays:       weekdays,
			Slots:          append([]string(nil), block.Slots...),
			GenerateImages: block.GenerateImages,
			ImagePrompt:    strings.TrimSpace(block.ImagePrompt),
			ForceWebSearch: block.ForceWebSearch,
		}
	}
	for position, planned := range preview.Items {
		item := domain.ContentPlanItem{BlockID: plan.Blocks[planned.BlockIndex].ID, PlannedAt: planned.PlannedAt, Position: position + 1}
		for _, accountID := range planned.AccountIDs {
			account, err := s.Store.GetAccount(ctx, accountID)
			if err != nil {
				return domain.ContentPlan{}, err
			}
			item.Variants = append(item.Variants, domain.ContentPlanVariant{AccountID: account.ID, Platform: account.Platform, Status: domain.ContentPlanVariantPending, PlannedAt: planned.PlannedAt})
		}
		plan.Items = append(plan.Items, item)
	}
	return plan, nil
}

type draftUpdateStore interface {
	UpdateContentPlan(ctx context.Context, plan domain.ContentPlan) (domain.ContentPlan, error)
}

func (s Service) Update(ctx context.Context, planID string, in CreateInput) (domain.ContentPlan, Preview, error) {
	store, ok := s.Store.(draftUpdateStore)
	if !ok {
		return domain.ContentPlan{}, Preview{}, errors.New("content plan update store is not configured")
	}
	current, err := s.Store.GetContentPlan(ctx, strings.TrimSpace(planID))
	if err != nil {
		return domain.ContentPlan{}, Preview{}, err
	}
	if current.Status != domain.ContentPlanStatusDraft {
		return domain.ContentPlan{}, Preview{}, ErrPlanNotDraft
	}
	preview, err := s.Preview(ctx, in)
	if err != nil {
		return domain.ContentPlan{}, Preview{}, err
	}
	plan, err := s.buildPlanGraph(ctx, in, preview)
	if err != nil {
		return domain.ContentPlan{}, Preview{}, err
	}
	plan.ID, plan.CreatedAt = current.ID, current.CreatedAt
	updated, err := store.UpdateContentPlan(ctx, plan)
	if err != nil {
		return domain.ContentPlan{}, Preview{}, err
	}
	return updated, preview, nil
}

func (s Service) Generate(ctx context.Context, planID string) (domain.ContentPlanJob, error) {
	if s.Store == nil {
		return domain.ContentPlanJob{}, errors.New("content plan store is not configured")
	}
	plan, err := s.Store.GetContentPlan(ctx, strings.TrimSpace(planID))
	if err != nil {
		return domain.ContentPlanJob{}, err
	}
	if plan.Status != domain.ContentPlanStatusDraft {
		return domain.ContentPlanJob{}, ErrPlanNotDraft
	}
	return s.Store.EnqueueContentPlanJob(ctx, plan.ID)
}

func (s Service) Cancel(ctx context.Context, planID string) error {
	store, ok := s.Store.(editorialStore)
	if !ok {
		return errors.New("content plan editorial store is not configured")
	}
	return store.CancelContentPlan(ctx, strings.TrimSpace(planID))
}

func (s Service) UpdateVariant(ctx context.Context, planID, variantID, text string, plannedAt time.Time, mediaID string) error {
	store, ok := s.Store.(editorialStore)
	if !ok {
		return errors.New("content plan editorial store is not configured")
	}
	if strings.TrimSpace(text) == "" {
		return errors.New("variant text is required")
	}
	if plannedAt.IsZero() {
		return errors.New("variant planned_at is required")
	}
	return store.UpdateContentPlanVariant(ctx, strings.TrimSpace(planID), strings.TrimSpace(variantID), text, plannedAt, mediaID)
}

func (s Service) Retry(ctx context.Context, planID string, variantIDs []string) (domain.ContentPlanJob, error) {
	store, ok := s.Store.(editorialStore)
	if !ok {
		return domain.ContentPlanJob{}, errors.New("content plan editorial store is not configured")
	}
	plan, err := s.Store.GetContentPlan(ctx, strings.TrimSpace(planID))
	if err != nil {
		return domain.ContentPlanJob{}, err
	}
	if plan.Status != domain.ContentPlanStatusReview && plan.Status != domain.ContentPlanStatusFailed && plan.Status != domain.ContentPlanStatusPartiallyScheduled {
		return domain.ContentPlanJob{}, ErrPlanNotReviewable
	}
	if err := store.ResetContentPlanVariants(ctx, plan.ID, variantIDs); err != nil {
		return domain.ContentPlanJob{}, err
	}
	return s.Store.EnqueueContentPlanJob(ctx, plan.ID)
}

func (s Service) Regenerate(ctx context.Context, planID string, variantIDs, itemIDs []string) (domain.ContentPlanJob, error) {
	store, ok := s.Store.(editorialStore)
	if !ok {
		return domain.ContentPlanJob{}, errors.New("content plan editorial store is not configured")
	}
	plan, err := s.Store.GetContentPlan(ctx, strings.TrimSpace(planID))
	if err != nil {
		return domain.ContentPlanJob{}, err
	}
	if plan.Status != domain.ContentPlanStatusReview && plan.Status != domain.ContentPlanStatusFailed && plan.Status != domain.ContentPlanStatusPartiallyScheduled {
		return domain.ContentPlanJob{}, ErrPlanNotReviewable
	}
	if len(variantIDs) == 0 && len(itemIDs) == 0 {
		return domain.ContentPlanJob{}, errors.New("variant_ids or item_ids are required")
	}
	if len(itemIDs) > 0 {
		if err := store.ResetContentPlanItems(ctx, plan.ID, itemIDs); err != nil {
			return domain.ContentPlanJob{}, err
		}
	}
	if len(variantIDs) > 0 {
		if err := store.ResetContentPlanVariants(ctx, plan.ID, variantIDs); err != nil {
			return domain.ContentPlanJob{}, err
		}
	}
	return s.Store.EnqueueContentPlanJob(ctx, plan.ID)
}

func (s Service) Schedule(ctx context.Context, planID string, variantIDs []string) (ScheduleResult, error) {
	store, ok := s.Store.(editorialStore)
	if !ok || s.Scheduler == nil {
		return ScheduleResult{}, errors.New("content plan scheduler is not configured")
	}
	plan, err := s.Store.GetContentPlan(ctx, strings.TrimSpace(planID))
	if err != nil {
		return ScheduleResult{}, err
	}
	selected := make(map[string]bool, len(variantIDs))
	for _, id := range variantIDs {
		if id = strings.TrimSpace(id); id != "" {
			selected[id] = true
		}
	}
	blocks := make(map[string]domain.ContentPlanBlock, len(plan.Blocks))
	for _, block := range plan.Blocks {
		blocks[block.ID] = block
	}
	var result ScheduleResult
	for _, item := range plan.Items {
		for _, variant := range item.Variants {
			if len(selected) > 0 && !selected[variant.ID] {
				continue
			}
			if variant.Status != domain.ContentPlanVariantReady && variant.Status != domain.ContentPlanVariantApproved {
				continue
			}
			post, scheduleErr := s.Scheduler.ScheduleVariant(ctx, variant, blocks[item.BlockID])
			if scheduleErr != nil {
				result.Conflicts = append(result.Conflicts, ScheduleConflict{VariantID: variant.ID, Error: scheduleErr.Error()})
				continue
			}
			if err := store.MarkContentPlanVariantScheduled(ctx, plan.ID, variant.ID, post.ID); err != nil {
				return ScheduleResult{}, err
			}
			result.Scheduled = append(result.Scheduled, ScheduleItem{VariantID: variant.ID, Post: post})
		}
	}
	if err := store.RefreshContentPlanScheduleStatus(ctx, plan.ID); err != nil {
		return ScheduleResult{}, err
	}
	return result, nil
}

func contentPlanID(prefix string) (string, error) {
	var raw [8]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return "", err
	}
	return prefix + "_" + hex.EncodeToString(raw[:]), nil
}
