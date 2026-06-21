package db

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/escarface/sarepost/internal/domain"
)

var ErrNoContentPlanJob = errors.New("no content plan job available")

func (s *Store) CreateContentPlan(ctx context.Context, plan domain.ContentPlan) (domain.ContentPlan, error) {
	if strings.TrimSpace(plan.Name) == "" {
		return domain.ContentPlan{}, errors.New("content plan name is required")
	}
	if plan.ID == "" {
		id, err := NewID("plan")
		if err != nil {
			return domain.ContentPlan{}, err
		}
		plan.ID = id
	}
	if plan.Status == "" {
		plan.Status = domain.ContentPlanStatusDraft
	}
	if strings.TrimSpace(plan.Timezone) == "" {
		plan.Timezone = "UTC"
	}
	now := time.Now().UTC()
	plan.CreatedAt, plan.UpdatedAt = now, now
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return domain.ContentPlan{}, err
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, `INSERT INTO content_plans (id, name, objective, timezone, starts_at, ends_at, status, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		plan.ID, strings.TrimSpace(plan.Name), strings.TrimSpace(plan.Objective), plan.Timezone, plan.StartsAt.Format(time.RFC3339Nano), plan.EndsAt.Format(time.RFC3339Nano), plan.Status, now.Format(time.RFC3339Nano), now.Format(time.RFC3339Nano)); err != nil {
		return domain.ContentPlan{}, err
	}
	for i := range plan.Blocks {
		block := &plan.Blocks[i]
		if block.ID == "" {
			block.ID, err = NewID("plb")
			if err != nil {
				return domain.ContentPlan{}, err
			}
		}
		block.PlanID = plan.ID
		accounts, _ := json.Marshal(block.AccountIDs)
		weekdays, _ := json.Marshal(block.Weekdays)
		slots, _ := json.Marshal(block.Slots)
		if _, err := tx.ExecContext(ctx, `INSERT INTO content_plan_blocks (id, plan_id, brand_profile_id, campaign_id, instructions, account_ids, weekdays, slots, generate_images, image_prompt, force_web_search, position) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			block.ID, plan.ID, block.BrandProfileID, block.CampaignID, block.Instructions, string(accounts), string(weekdays), string(slots), boolInt(block.GenerateImages), block.ImagePrompt, boolInt(block.ForceWebSearch), i+1); err != nil {
			return domain.ContentPlan{}, err
		}
	}
	for i := range plan.Items {
		item := &plan.Items[i]
		if item.ID == "" {
			item.ID, err = NewID("pli")
			if err != nil {
				return domain.ContentPlan{}, err
			}
		}
		item.PlanID = plan.ID
		item.Position = i + 1
		if item.BlockID == "" && len(plan.Blocks) > 0 {
			item.BlockID = plan.Blocks[0].ID
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO content_plan_items (id, plan_id, block_id, planned_at, idea, position) VALUES (?, ?, ?, ?, ?, ?)`, item.ID, plan.ID, item.BlockID, item.PlannedAt.Format(time.RFC3339Nano), item.Idea, item.Position); err != nil {
			return domain.ContentPlan{}, err
		}
		for j := range item.Variants {
			variant := &item.Variants[j]
			if variant.ID == "" {
				variant.ID, err = NewID("plv")
				if err != nil {
					return domain.ContentPlan{}, err
				}
			}
			variant.PlanID, variant.ItemID, variant.PlannedAt = plan.ID, item.ID, item.PlannedAt
			if variant.Status == "" {
				variant.Status = domain.ContentPlanVariantPending
			}
			if variant.Platform == "" {
				var platform string
				if err := tx.QueryRowContext(ctx, `SELECT platform FROM accounts WHERE id = ?`, variant.AccountID).Scan(&platform); err != nil {
					return domain.ContentPlan{}, err
				}
				variant.Platform = domain.Platform(platform)
			}
			if _, err := tx.ExecContext(ctx, `INSERT INTO content_plan_variants (id, plan_id, item_id, account_id, platform, text, media_id, status, error, post_id, planned_at, generation_runs) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
				variant.ID, plan.ID, item.ID, variant.AccountID, variant.Platform, variant.Text, nullableText(variant.MediaID), variant.Status, variant.Error, nullableText(variant.PostID), variant.PlannedAt.Format(time.RFC3339Nano), variant.GenerationRuns); err != nil {
				return domain.ContentPlan{}, err
			}
		}
	}
	if err := tx.Commit(); err != nil {
		return domain.ContentPlan{}, err
	}
	return s.GetContentPlan(ctx, plan.ID)
}

func (s *Store) GetContentPlan(ctx context.Context, id string) (domain.ContentPlan, error) {
	var plan domain.ContentPlan
	var startsAt, endsAt, createdAt, updatedAt string
	if err := s.db.QueryRowContext(ctx, `SELECT id, name, objective, timezone, starts_at, ends_at, status, created_at, updated_at FROM content_plans WHERE id = ?`, strings.TrimSpace(id)).Scan(
		&plan.ID, &plan.Name, &plan.Objective, &plan.Timezone, &startsAt, &endsAt, &plan.Status, &createdAt, &updatedAt); err != nil {
		return domain.ContentPlan{}, err
	}
	plan.StartsAt, _ = time.Parse(time.RFC3339Nano, startsAt)
	plan.EndsAt, _ = time.Parse(time.RFC3339Nano, endsAt)
	plan.CreatedAt, _ = time.Parse(time.RFC3339Nano, createdAt)
	plan.UpdatedAt, _ = time.Parse(time.RFC3339Nano, updatedAt)

	rows, err := s.db.QueryContext(ctx, `SELECT id, brand_profile_id, campaign_id, instructions, account_ids, weekdays, slots, generate_images, image_prompt, force_web_search FROM content_plan_blocks WHERE plan_id = ? ORDER BY position`, plan.ID)
	if err != nil {
		return domain.ContentPlan{}, err
	}
	for rows.Next() {
		var block domain.ContentPlanBlock
		var accounts, weekdays, slots string
		var generateImages, forceWebSearch int
		if err := rows.Scan(&block.ID, &block.BrandProfileID, &block.CampaignID, &block.Instructions, &accounts, &weekdays, &slots, &generateImages, &block.ImagePrompt, &forceWebSearch); err != nil {
			rows.Close()
			return domain.ContentPlan{}, err
		}
		block.PlanID, block.GenerateImages, block.ForceWebSearch = plan.ID, generateImages != 0, forceWebSearch != 0
		_ = json.Unmarshal([]byte(accounts), &block.AccountIDs)
		_ = json.Unmarshal([]byte(weekdays), &block.Weekdays)
		_ = json.Unmarshal([]byte(slots), &block.Slots)
		plan.Blocks = append(plan.Blocks, block)
	}
	if err := rows.Close(); err != nil {
		return domain.ContentPlan{}, err
	}

	itemRows, err := s.db.QueryContext(ctx, `SELECT id, block_id, planned_at, idea, position FROM content_plan_items WHERE plan_id = ? ORDER BY position`, plan.ID)
	if err != nil {
		return domain.ContentPlan{}, err
	}
	for itemRows.Next() {
		var item domain.ContentPlanItem
		var plannedAt string
		if err := itemRows.Scan(&item.ID, &item.BlockID, &plannedAt, &item.Idea, &item.Position); err != nil {
			itemRows.Close()
			return domain.ContentPlan{}, err
		}
		item.PlanID, item.PlannedAt = plan.ID, parseTime(plannedAt)
		plan.Items = append(plan.Items, item)
	}
	if err := itemRows.Close(); err != nil {
		return domain.ContentPlan{}, err
	}
	variantsByItem, err := s.listContentPlanVariantsByPlan(ctx, plan.ID)
	if err != nil {
		return domain.ContentPlan{}, err
	}
	for i := range plan.Items {
		plan.Items[i].Variants = variantsByItem[plan.Items[i].ID]
	}
	return plan, nil
}

func (s *Store) ListContentPlans(ctx context.Context, limit int) ([]domain.ContentPlan, error) {
	if limit <= 0 {
		limit = 50
	}
	if limit > 200 {
		limit = 200
	}
	rows, err := s.db.QueryContext(ctx, `SELECT id FROM content_plans ORDER BY created_at DESC LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return nil, err
		}
		ids = append(ids, id)
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}
	out := make([]domain.ContentPlan, 0, len(ids))
	for _, id := range ids {
		plan, err := s.GetContentPlan(ctx, id)
		if err != nil {
			return nil, err
		}
		out = append(out, plan)
	}
	return out, nil
}

func (s *Store) UpdateContentPlan(ctx context.Context, plan domain.ContentPlan) (domain.ContentPlan, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return domain.ContentPlan{}, err
	}
	defer tx.Rollback()
	var status domain.ContentPlanStatus
	if err := tx.QueryRowContext(ctx, `SELECT status FROM content_plans WHERE id = ?`, strings.TrimSpace(plan.ID)).Scan(&status); err != nil {
		return domain.ContentPlan{}, err
	}
	if status != domain.ContentPlanStatusDraft {
		return domain.ContentPlan{}, errors.New("content plan is not a draft")
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	if _, err := tx.ExecContext(ctx, `UPDATE content_plans SET name = ?, objective = ?, timezone = ?, starts_at = ?, ends_at = ?, updated_at = ? WHERE id = ?`, strings.TrimSpace(plan.Name), strings.TrimSpace(plan.Objective), strings.TrimSpace(plan.Timezone), plan.StartsAt.Format(time.RFC3339Nano), plan.EndsAt.Format(time.RFC3339Nano), now, plan.ID); err != nil {
		return domain.ContentPlan{}, err
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM content_plan_blocks WHERE plan_id = ?`, plan.ID); err != nil {
		return domain.ContentPlan{}, err
	}
	for i := range plan.Blocks {
		block := &plan.Blocks[i]
		block.PlanID = plan.ID
		accounts, _ := json.Marshal(block.AccountIDs)
		weekdays, _ := json.Marshal(block.Weekdays)
		slots, _ := json.Marshal(block.Slots)
		if _, err := tx.ExecContext(ctx, `INSERT INTO content_plan_blocks (id, plan_id, brand_profile_id, campaign_id, instructions, account_ids, weekdays, slots, generate_images, image_prompt, force_web_search, position) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`, block.ID, plan.ID, block.BrandProfileID, block.CampaignID, block.Instructions, string(accounts), string(weekdays), string(slots), boolInt(block.GenerateImages), block.ImagePrompt, boolInt(block.ForceWebSearch), i+1); err != nil {
			return domain.ContentPlan{}, err
		}
	}
	for i := range plan.Items {
		item := &plan.Items[i]
		if item.ID == "" {
			item.ID, err = NewID("pli")
			if err != nil {
				return domain.ContentPlan{}, err
			}
		}
		item.PlanID, item.Position = plan.ID, i+1
		if _, err := tx.ExecContext(ctx, `INSERT INTO content_plan_items (id, plan_id, block_id, planned_at, idea, position) VALUES (?, ?, ?, ?, ?, ?)`, item.ID, plan.ID, item.BlockID, item.PlannedAt.Format(time.RFC3339Nano), item.Idea, item.Position); err != nil {
			return domain.ContentPlan{}, err
		}
		for j := range item.Variants {
			variant := &item.Variants[j]
			if variant.ID == "" {
				variant.ID, err = NewID("plv")
				if err != nil {
					return domain.ContentPlan{}, err
				}
			}
			variant.PlanID, variant.ItemID, variant.PlannedAt = plan.ID, item.ID, item.PlannedAt
			if variant.Status == "" {
				variant.Status = domain.ContentPlanVariantPending
			}
			if _, err := tx.ExecContext(ctx, `INSERT INTO content_plan_variants (id, plan_id, item_id, account_id, platform, text, media_id, status, error, post_id, planned_at, generation_runs) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`, variant.ID, plan.ID, item.ID, variant.AccountID, variant.Platform, variant.Text, nullableText(variant.MediaID), variant.Status, variant.Error, nullableText(variant.PostID), variant.PlannedAt.Format(time.RFC3339Nano), variant.GenerationRuns); err != nil {
				return domain.ContentPlan{}, err
			}
		}
	}
	if err := tx.Commit(); err != nil {
		return domain.ContentPlan{}, err
	}
	return s.GetContentPlan(ctx, plan.ID)
}

func (s *Store) UpdateContentPlanVariant(ctx context.Context, planID, variantID, text string, plannedAt time.Time, mediaID string) error {
	result, err := s.db.ExecContext(ctx, `UPDATE content_plan_variants SET text = ?, planned_at = ?, media_id = ?, status = ?, error = '' WHERE id = ? AND plan_id = ? AND status NOT IN (?, ?)`,
		strings.TrimSpace(text), plannedAt.Format(time.RFC3339Nano), nullableText(mediaID), domain.ContentPlanVariantReady, strings.TrimSpace(variantID), strings.TrimSpace(planID), domain.ContentPlanVariantScheduled, domain.ContentPlanVariantGenerating)
	return requireContentPlanRow(result, err, "editable content plan variant")
}

func (s *Store) ResetContentPlanVariants(ctx context.Context, planID string, variantIDs []string) error {
	ids := normalizeStringList(variantIDs)
	if len(ids) == 0 {
		return errors.New("variant_ids are required")
	}
	placeholders := strings.TrimRight(strings.Repeat("?,", len(ids)), ",")
	args := []any{domain.ContentPlanVariantPending, strings.TrimSpace(planID)}
	for _, id := range ids {
		args = append(args, id)
	}
	query := `UPDATE content_plan_variants SET text = '', media_id = NULL, status = ?, error = '' WHERE plan_id = ? AND id IN (` + placeholders + `) AND status != 'scheduled'`
	result, err := s.db.ExecContext(ctx, query, args...)
	return requireContentPlanRow(result, err, "content plan variants")
}

func (s *Store) ResetContentPlanItems(ctx context.Context, planID string, itemIDs []string) error {
	ids := normalizeStringList(itemIDs)
	if len(ids) == 0 {
		return errors.New("item_ids are required")
	}
	placeholders := strings.TrimRight(strings.Repeat("?,", len(ids)), ",")
	itemArgs := []any{strings.TrimSpace(planID)}
	for _, id := range ids {
		itemArgs = append(itemArgs, id)
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	itemsResult, err := tx.ExecContext(ctx, `UPDATE content_plan_items SET idea = '' WHERE plan_id = ? AND id IN (`+placeholders+`)`, itemArgs...)
	if err := requireContentPlanRow(itemsResult, err, "content plan items"); err != nil {
		return err
	}
	variantArgs := []any{domain.ContentPlanVariantPending, strings.TrimSpace(planID)}
	for _, id := range ids {
		variantArgs = append(variantArgs, id)
	}
	variantsResult, err := tx.ExecContext(ctx, `UPDATE content_plan_variants SET text = '', media_id = NULL, status = ?, error = '' WHERE plan_id = ? AND item_id IN (`+placeholders+`) AND status != 'scheduled'`, variantArgs...)
	if err := requireContentPlanRow(variantsResult, err, "regeneratable content plan item variants"); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Store) CancelContentPlan(ctx context.Context, planID string) error {
	now := time.Now().UTC().Format(time.RFC3339Nano)
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	result, err := tx.ExecContext(ctx, `UPDATE content_plans SET status = ?, updated_at = ? WHERE id = ? AND status NOT IN (?, ?)`, domain.ContentPlanStatusCanceled, now, strings.TrimSpace(planID), domain.ContentPlanStatusScheduled, domain.ContentPlanStatusCanceled)
	if err := requireContentPlanRow(result, err, "cancelable content plan"); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `UPDATE content_plan_jobs SET status = ?, lease_until = NULL, updated_at = ? WHERE plan_id = ? AND status IN (?, ?)`, domain.ContentPlanJobCanceled, now, strings.TrimSpace(planID), domain.ContentPlanJobPending, domain.ContentPlanJobRunning); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Store) MarkContentPlanVariantScheduled(ctx context.Context, planID, variantID, postID string) error {
	result, err := s.db.ExecContext(ctx, `UPDATE content_plan_variants SET status = ?, post_id = ?, error = '' WHERE plan_id = ? AND id = ? AND status IN (?, ?)`,
		domain.ContentPlanVariantScheduled, strings.TrimSpace(postID), strings.TrimSpace(planID), strings.TrimSpace(variantID), domain.ContentPlanVariantReady, domain.ContentPlanVariantApproved)
	return requireContentPlanRow(result, err, "schedulable content plan variant")
}

func (s *Store) RefreshContentPlanScheduleStatus(ctx context.Context, planID string) error {
	var total, scheduled int
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*), COALESCE(SUM(CASE WHEN status = ? THEN 1 ELSE 0 END), 0) FROM content_plan_variants WHERE plan_id = ?`, domain.ContentPlanVariantScheduled, strings.TrimSpace(planID)).Scan(&total, &scheduled); err != nil {
		return err
	}
	if total == 0 || scheduled == 0 {
		return nil
	}
	status := domain.ContentPlanStatusPartiallyScheduled
	if scheduled == total {
		status = domain.ContentPlanStatusScheduled
	}
	result, err := s.db.ExecContext(ctx, `UPDATE content_plans SET status = ?, updated_at = ? WHERE id = ?`, status, time.Now().UTC().Format(time.RFC3339Nano), strings.TrimSpace(planID))
	return requireContentPlanRow(result, err, "content plan")
}

func (s *Store) listContentPlanVariantsByPlan(ctx context.Context, planID string) (map[string][]domain.ContentPlanVariant, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id, plan_id, item_id, account_id, platform, text, media_id, status, error, post_id, planned_at, generation_runs FROM content_plan_variants WHERE plan_id = ? ORDER BY rowid`, planID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make(map[string][]domain.ContentPlanVariant)
	for rows.Next() {
		var variant domain.ContentPlanVariant
		var mediaID, postID sql.NullString
		var plannedAt string
		if err := rows.Scan(&variant.ID, &variant.PlanID, &variant.ItemID, &variant.AccountID, &variant.Platform, &variant.Text, &mediaID, &variant.Status, &variant.Error, &postID, &plannedAt, &variant.GenerationRuns); err != nil {
			return nil, err
		}
		variant.MediaID, variant.PostID, variant.PlannedAt = mediaID.String, postID.String, parseTime(plannedAt)
		out[variant.ItemID] = append(out[variant.ItemID], variant)
	}
	return out, rows.Err()
}

func requireContentPlanRow(result sql.Result, err error, name string) error {
	if err != nil {
		return err
	}
	count, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if count == 0 {
		return fmt.Errorf("%s not found", name)
	}
	return nil
}

func nullableText(value string) any {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	return value
}
