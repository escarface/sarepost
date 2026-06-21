package db

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/escarface/sarepost/internal/domain"
)

func (s *Store) EnqueueContentPlanJob(ctx context.Context, planID string) (domain.ContentPlanJob, error) {
	id, err := NewID("plj")
	if err != nil {
		return domain.ContentPlanJob{}, err
	}
	now := time.Now().UTC()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return domain.ContentPlanJob{}, err
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, `INSERT INTO content_plan_jobs (id, plan_id, status, created_at, updated_at) VALUES (?, ?, ?, ?, ?)`, id, planID, domain.ContentPlanJobPending, now.Format(time.RFC3339Nano), now.Format(time.RFC3339Nano)); err != nil {
		return domain.ContentPlanJob{}, err
	}
	if _, err := tx.ExecContext(ctx, `UPDATE content_plans SET status = ?, updated_at = ? WHERE id = ?`, domain.ContentPlanStatusQueued, now.Format(time.RFC3339Nano), planID); err != nil {
		return domain.ContentPlanJob{}, err
	}
	if err := tx.Commit(); err != nil {
		return domain.ContentPlanJob{}, err
	}
	return domain.ContentPlanJob{ID: id, PlanID: planID, Status: domain.ContentPlanJobPending, CreatedAt: now, UpdatedAt: now}, nil
}

func (s *Store) ClaimContentPlanJob(ctx context.Context, lease time.Duration) (domain.ContentPlanJob, error) {
	now, leaseUntil := time.Now().UTC(), time.Now().UTC().Add(lease)
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return domain.ContentPlanJob{}, err
	}
	defer tx.Rollback()
	var job domain.ContentPlanJob
	var createdAt, updatedAt string
	err = tx.QueryRowContext(ctx, `
		SELECT id, plan_id, status, error, created_at, updated_at
		FROM content_plan_jobs
		WHERE (status = ? AND lease_until < ?)
		   OR (status = ? AND NOT EXISTS (
			SELECT 1 FROM content_plan_jobs active
			WHERE active.status = ? AND (active.lease_until IS NULL OR active.lease_until >= ?)
		   ))
		ORDER BY CASE WHEN status = ? THEN 0 ELSE 1 END, created_at
		LIMIT 1
	`, domain.ContentPlanJobRunning, now.Format(time.RFC3339Nano), domain.ContentPlanJobPending, domain.ContentPlanJobRunning, now.Format(time.RFC3339Nano), domain.ContentPlanJobRunning).Scan(&job.ID, &job.PlanID, &job.Status, &job.Error, &createdAt, &updatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.ContentPlanJob{}, ErrNoContentPlanJob
	}
	if err != nil {
		return domain.ContentPlanJob{}, err
	}
	result, err := tx.ExecContext(ctx, `UPDATE content_plan_jobs SET status = ?, lease_until = ?, updated_at = ? WHERE id = ?`, domain.ContentPlanJobRunning, leaseUntil.Format(time.RFC3339Nano), now.Format(time.RFC3339Nano), job.ID)
	if err != nil {
		return domain.ContentPlanJob{}, err
	}
	if count, _ := result.RowsAffected(); count != 1 {
		return domain.ContentPlanJob{}, fmt.Errorf("claim content plan job %s", job.ID)
	}
	if _, err := tx.ExecContext(ctx, `UPDATE content_plans SET status = ?, updated_at = ? WHERE id = ?`, domain.ContentPlanStatusGenerating, now.Format(time.RFC3339Nano), job.PlanID); err != nil {
		return domain.ContentPlanJob{}, err
	}
	if err := tx.Commit(); err != nil {
		return domain.ContentPlanJob{}, err
	}
	job.Status, job.LeaseUntil, job.UpdatedAt = domain.ContentPlanJobRunning, &leaseUntil, now
	job.CreatedAt = parseTime(createdAt)
	return job, nil
}

func (s *Store) UpdateContentPlanItemIdea(ctx context.Context, id, idea string) error {
	result, err := s.db.ExecContext(ctx, `UPDATE content_plan_items SET idea = ? WHERE id = ?`, strings.TrimSpace(idea), strings.TrimSpace(id))
	return requireContentPlanRow(result, err, "content plan item")
}

func (s *Store) ContentPlanJobActive(ctx context.Context, id string) (bool, error) {
	var status domain.ContentPlanJobStatus
	if err := s.db.QueryRowContext(ctx, `SELECT status FROM content_plan_jobs WHERE id = ?`, strings.TrimSpace(id)).Scan(&status); err != nil {
		return false, err
	}
	return status == domain.ContentPlanJobRunning, nil
}

func (s *Store) RenewContentPlanJob(ctx context.Context, id string, leaseUntil time.Time) error {
	result, err := s.db.ExecContext(ctx, `UPDATE content_plan_jobs SET lease_until = ?, updated_at = ? WHERE id = ? AND status = ?`, leaseUntil.UTC().Format(time.RFC3339Nano), time.Now().UTC().Format(time.RFC3339Nano), strings.TrimSpace(id), domain.ContentPlanJobRunning)
	return requireContentPlanRow(result, err, "running content plan job")
}

func (s *Store) MarkContentPlanVariantGenerating(ctx context.Context, id string) error {
	result, err := s.db.ExecContext(ctx, `UPDATE content_plan_variants SET status = ?, error = '', generation_runs = generation_runs + 1 WHERE id = ?`, domain.ContentPlanVariantGenerating, strings.TrimSpace(id))
	return requireContentPlanRow(result, err, "content plan variant")
}

func (s *Store) CompleteContentPlanVariant(ctx context.Context, id, text, mediaID string) error {
	result, err := s.db.ExecContext(ctx, `UPDATE content_plan_variants SET text = ?, media_id = ?, status = ?, error = '' WHERE id = ?`, strings.TrimSpace(text), nullableText(mediaID), domain.ContentPlanVariantReady, strings.TrimSpace(id))
	return requireContentPlanRow(result, err, "content plan variant")
}

func (s *Store) FailContentPlanVariant(ctx context.Context, id string, failure error) error {
	message := "generation failed"
	if failure != nil && strings.TrimSpace(failure.Error()) != "" {
		message = strings.TrimSpace(failure.Error())
	}
	result, err := s.db.ExecContext(ctx, `UPDATE content_plan_variants SET status = ?, error = ? WHERE id = ?`, domain.ContentPlanVariantFailed, message, strings.TrimSpace(id))
	return requireContentPlanRow(result, err, "content plan variant")
}

func (s *Store) FinishContentPlanJob(ctx context.Context, id string, status domain.ContentPlanJobStatus, failure string) error {
	now := time.Now().UTC().Format(time.RFC3339Nano)
	planStatus := domain.ContentPlanStatusReview
	switch status {
	case domain.ContentPlanJobFailed:
		planStatus = domain.ContentPlanStatusFailed
	case domain.ContentPlanJobCanceled:
		planStatus = domain.ContentPlanStatusCanceled
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var current domain.ContentPlanJobStatus
	if err := tx.QueryRowContext(ctx, `SELECT status FROM content_plan_jobs WHERE id = ?`, strings.TrimSpace(id)).Scan(&current); err != nil {
		return err
	}
	if current == domain.ContentPlanJobCanceled {
		return nil
	}
	result, err := tx.ExecContext(ctx, `UPDATE content_plan_jobs SET status = ?, lease_until = NULL, error = ?, updated_at = ? WHERE id = ?`, status, strings.TrimSpace(failure), now, strings.TrimSpace(id))
	if err := requireContentPlanRow(result, err, "content plan job"); err != nil {
		return err
	}
	if status == domain.ContentPlanJobCompleted {
		var total, scheduled int
		if err := tx.QueryRowContext(ctx, `SELECT COUNT(*), COALESCE(SUM(CASE WHEN status = ? THEN 1 ELSE 0 END), 0) FROM content_plan_variants WHERE plan_id = (SELECT plan_id FROM content_plan_jobs WHERE id = ?)`, domain.ContentPlanVariantScheduled, strings.TrimSpace(id)).Scan(&total, &scheduled); err != nil {
			return err
		}
		if scheduled > 0 {
			planStatus = domain.ContentPlanStatusPartiallyScheduled
			if scheduled == total {
				planStatus = domain.ContentPlanStatusScheduled
			}
		}
	}
	if _, err := tx.ExecContext(ctx, `UPDATE content_plans SET status = ?, updated_at = ? WHERE id = (SELECT plan_id FROM content_plan_jobs WHERE id = ?)`, planStatus, now, strings.TrimSpace(id)); err != nil {
		return err
	}
	return tx.Commit()
}
