package db

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/escarface/sarepost/internal/domain"
)

func (s *Store) CreateCampaign(ctx context.Context, campaign domain.Campaign) (domain.Campaign, error) {
	if campaign.ID == "" {
		id, err := NewID("cmp")
		if err != nil {
			return domain.Campaign{}, err
		}
		campaign.ID = id
	}
	if campaign.Status == "" {
		campaign.Status = domain.CampaignStatusActive
	}
	now := time.Now().UTC()
	campaign.CreatedAt = now
	campaign.UpdatedAt = now
	tags, err := json.Marshal(campaign.Tags)
	if err != nil {
		return domain.Campaign{}, err
	}
	_, err = s.db.ExecContext(ctx, `
		INSERT INTO campaigns (id, name, objective, status, starts_at, ends_at, notes, tags, timezone, audience, tone, cta, restrictions, brand_profile_id, visual_style, image_prompt, image_size, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, campaign.ID, strings.TrimSpace(campaign.Name), strings.TrimSpace(campaign.Objective), campaign.Status, formatOptionalTime(campaign.StartsAt), formatOptionalTime(campaign.EndsAt), strings.TrimSpace(campaign.Notes), string(tags), strings.TrimSpace(campaign.Timezone), strings.TrimSpace(campaign.Audience), strings.TrimSpace(campaign.Tone), strings.TrimSpace(campaign.CTA), strings.TrimSpace(campaign.Restrictions), strings.TrimSpace(campaign.BrandProfileID), strings.TrimSpace(campaign.VisualStyle), strings.TrimSpace(campaign.ImagePrompt), strings.TrimSpace(campaign.ImageSize), now.Format(time.RFC3339Nano), now.Format(time.RFC3339Nano))
	if err != nil {
		return domain.Campaign{}, err
	}
	return s.GetCampaign(ctx, campaign.ID)
}

func (s *Store) GetCampaign(ctx context.Context, id string) (domain.Campaign, error) {
	var c domain.Campaign
	var startsAt, endsAt, tags, createdAt, updatedAt string
	err := s.db.QueryRowContext(ctx, `
		SELECT id, name, objective, status, starts_at, ends_at, notes, tags, timezone, audience, tone, cta, restrictions, brand_profile_id, visual_style, image_prompt, image_size, created_at, updated_at
		FROM campaigns
		WHERE id = ?
	`, strings.TrimSpace(id)).Scan(&c.ID, &c.Name, &c.Objective, &c.Status, &startsAt, &endsAt, &c.Notes, &tags, &c.Timezone, &c.Audience, &c.Tone, &c.CTA, &c.Restrictions, &c.BrandProfileID, &c.VisualStyle, &c.ImagePrompt, &c.ImageSize, &createdAt, &updatedAt)
	if err != nil {
		return domain.Campaign{}, err
	}
	c.StartsAt = parseOptionalTime(startsAt)
	c.EndsAt = parseOptionalTime(endsAt)
	c.Tags = parseStringList(tags)
	c.CreatedAt = parseTime(createdAt)
	c.UpdatedAt = parseTime(updatedAt)
	return c, nil
}

func (s *Store) ListCampaigns(ctx context.Context, filter domain.CampaignListFilter) ([]domain.Campaign, error) {
	query := `
		SELECT id
		FROM campaigns
		WHERE 1=1
	`
	args := []any{}
	if filter.Status != "" {
		query += ` AND status = ?`
		args = append(args, filter.Status)
	}
	if tag := strings.TrimSpace(filter.Tag); tag != "" {
		query += ` AND tags LIKE ?`
		args = append(args, "%"+tag+"%")
	}
	query += ` ORDER BY updated_at DESC, name ASC`
	if filter.Limit > 0 {
		query += ` LIMIT ?`
		args = append(args, filter.Limit)
	}
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	ids := []string{}
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	out := make([]domain.Campaign, 0, len(ids))
	for _, id := range ids {
		campaign, err := s.GetCampaign(ctx, id)
		if err != nil {
			return nil, err
		}
		out = append(out, campaign)
	}
	return out, nil
}

func (s *Store) UpdateCampaign(ctx context.Context, campaign domain.Campaign) (domain.Campaign, error) {
	tags, err := json.Marshal(campaign.Tags)
	if err != nil {
		return domain.Campaign{}, err
	}
	now := time.Now().UTC()
	result, err := s.db.ExecContext(ctx, `
		UPDATE campaigns
		SET name = ?, objective = ?, status = ?, starts_at = ?, ends_at = ?, notes = ?, tags = ?, timezone = ?, audience = ?, tone = ?, cta = ?, restrictions = ?, brand_profile_id = ?, visual_style = ?, image_prompt = ?, image_size = ?, updated_at = ?
		WHERE id = ?
	`, strings.TrimSpace(campaign.Name), strings.TrimSpace(campaign.Objective), campaign.Status, formatOptionalTime(campaign.StartsAt), formatOptionalTime(campaign.EndsAt), strings.TrimSpace(campaign.Notes), string(tags), strings.TrimSpace(campaign.Timezone), strings.TrimSpace(campaign.Audience), strings.TrimSpace(campaign.Tone), strings.TrimSpace(campaign.CTA), strings.TrimSpace(campaign.Restrictions), strings.TrimSpace(campaign.BrandProfileID), strings.TrimSpace(campaign.VisualStyle), strings.TrimSpace(campaign.ImagePrompt), strings.TrimSpace(campaign.ImageSize), now.Format(time.RFC3339Nano), strings.TrimSpace(campaign.ID))
	if err != nil {
		return domain.Campaign{}, err
	}
	if rows, _ := result.RowsAffected(); rows == 0 {
		return domain.Campaign{}, sql.ErrNoRows
	}
	return s.GetCampaign(ctx, campaign.ID)
}

func (s *Store) AddPostToCampaign(ctx context.Context, postID, campaignID string, editorialStatus domain.EditorialStatus, requiresApproval bool, tags []string) error {
	postID = strings.TrimSpace(postID)
	campaignID = strings.TrimSpace(campaignID)
	if postID == "" || campaignID == "" {
		return fmt.Errorf("post_id and campaign_id are required")
	}
	if editorialStatus == "" {
		editorialStatus = domain.EditorialStatusDrafting
	}
	tagJSON, err := json.Marshal(normalizeStringList(tags))
	if err != nil {
		return err
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	_, err = s.db.ExecContext(ctx, `
		INSERT INTO campaign_posts (post_id, campaign_id, editorial_status, requires_approval, tags, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(post_id) DO UPDATE SET
			campaign_id = excluded.campaign_id,
			editorial_status = excluded.editorial_status,
			requires_approval = excluded.requires_approval,
			tags = excluded.tags,
			updated_at = excluded.updated_at
	`, postID, campaignID, editorialStatus, boolInt(requiresApproval), string(tagJSON), now, now)
	return err
}

func (s *Store) ApprovePost(ctx context.Context, postID string) error {
	now := time.Now().UTC()
	result, err := s.db.ExecContext(ctx, `
		UPDATE campaign_posts
		SET editorial_status = ?, requires_approval = 0, approved_at = ?, updated_at = ?
		WHERE post_id = ?
	`, domain.EditorialStatusApproved, now.Format(time.RFC3339Nano), now.Format(time.RFC3339Nano), strings.TrimSpace(postID))
	if err != nil {
		return err
	}
	if rows, _ := result.RowsAffected(); rows == 0 {
		return fmt.Errorf("post editorial metadata not found")
	}
	return nil
}

func (s *Store) ListEditorialBacklog(ctx context.Context, filter domain.EditorialBacklogFilter) ([]domain.EditorialBacklogItem, error) {
	query := `
		SELECT p.id
		FROM posts p
		JOIN campaign_posts cp ON cp.post_id = p.id
		JOIN accounts a ON a.id = p.account_id
		WHERE p.status IN (?, ?, ?)
	`
	args := []any{domain.PostStatusDraft, domain.PostStatusScheduled, domain.PostStatusFailed}
	if campaignID := strings.TrimSpace(filter.CampaignID); campaignID != "" {
		query += ` AND cp.campaign_id = ?`
		args = append(args, campaignID)
	}
	if filter.Platform != "" {
		query += ` AND a.platform = ?`
		args = append(args, filter.Platform)
	}
	if filter.EditorialStatus != "" {
		query += ` AND cp.editorial_status = ?`
		args = append(args, filter.EditorialStatus)
	}
	if tag := strings.TrimSpace(filter.Tag); tag != "" {
		query += ` AND cp.tags LIKE ?`
		args = append(args, "%"+tag+"%")
	}
	if !filter.From.IsZero() {
		query += ` AND p.scheduled_at >= ?`
		args = append(args, filter.From.UTC().Format(time.RFC3339Nano))
	}
	if !filter.To.IsZero() {
		query += ` AND p.scheduled_at <= ?`
		args = append(args, filter.To.UTC().Format(time.RFC3339Nano))
	}
	query += ` ORDER BY p.updated_at DESC`
	if filter.Limit > 0 {
		query += ` LIMIT ?`
		args = append(args, filter.Limit)
	}
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	ids := []string{}
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	out := make([]domain.EditorialBacklogItem, 0, len(ids))
	for _, id := range ids {
		post, err := s.GetPost(ctx, id)
		if err != nil {
			return nil, err
		}
		item := domain.EditorialBacklogItem{Post: post}
		if post.CampaignID != "" {
			campaign, err := s.GetCampaign(ctx, post.CampaignID)
			if err != nil {
				return nil, err
			}
			item.Campaign = campaign
		}
		out = append(out, item)
	}
	return out, nil
}

func formatOptionalTime(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.UTC().Format(time.RFC3339Nano)
}

func parseOptionalTime(raw string) time.Time {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return time.Time{}
	}
	return parseTime(raw)
}

func parseTime(raw string) time.Time {
	t, _ := time.Parse(time.RFC3339Nano, strings.TrimSpace(raw))
	return t
}

func parseStringList(raw string) []string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	var out []string
	if err := json.Unmarshal([]byte(raw), &out); err != nil {
		return nil
	}
	return normalizeStringList(out)
}

func normalizeStringList(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	out := make([]string, 0, len(values))
	for _, raw := range values {
		value := strings.TrimSpace(raw)
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	return out
}

func boolInt(v bool) int {
	if v {
		return 1
	}
	return 0
}
