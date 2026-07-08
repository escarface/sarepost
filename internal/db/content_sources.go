package db

import (
	"context"
	"database/sql"
	"encoding/json"
	"strings"
	"time"

	"github.com/escarface/sarepost/internal/domain"
)

func (s *Store) CreateContentSource(ctx context.Context, source domain.ContentSource) (domain.ContentSource, error) {
	if source.ID == "" {
		id, err := NewID("src")
		if err != nil {
			return domain.ContentSource{}, err
		}
		source.ID = id
	}
	if source.Status == "" {
		source.Status = domain.ContentSourceStatusNew
	}
	now := time.Now().UTC()
	tags, err := json.Marshal(normalizeStringList(source.Tags))
	if err != nil {
		return domain.ContentSource{}, err
	}
	_, err = s.db.ExecContext(ctx, `
		INSERT INTO content_sources (id, title, body, source_url, campaign_id, brand_profile_id, tags, status, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, source.ID, strings.TrimSpace(source.Title), strings.TrimSpace(source.Body), strings.TrimSpace(source.SourceURL), strings.TrimSpace(source.CampaignID), strings.TrimSpace(source.BrandProfileID), string(tags), source.Status, now.Format(time.RFC3339Nano), now.Format(time.RFC3339Nano))
	if err != nil {
		return domain.ContentSource{}, err
	}
	return s.GetContentSource(ctx, source.ID)
}

func (s *Store) GetContentSource(ctx context.Context, id string) (domain.ContentSource, error) {
	var source domain.ContentSource
	var tags, createdAt, updatedAt string
	err := s.db.QueryRowContext(ctx, `
		SELECT id, title, body, source_url, campaign_id, brand_profile_id, tags, status, created_at, updated_at
		FROM content_sources
		WHERE id = ?
	`, strings.TrimSpace(id)).Scan(&source.ID, &source.Title, &source.Body, &source.SourceURL, &source.CampaignID, &source.BrandProfileID, &tags, &source.Status, &createdAt, &updatedAt)
	if err != nil {
		return domain.ContentSource{}, err
	}
	source.Tags = parseStringList(tags)
	source.CreatedAt = parseTime(createdAt)
	source.UpdatedAt = parseTime(updatedAt)
	return source, nil
}

func (s *Store) ListContentSources(ctx context.Context, filter domain.ContentSourceListFilter) ([]domain.ContentSource, error) {
	query := `
		SELECT id
		FROM content_sources
		WHERE 1=1
	`
	args := []any{}
	if filter.Status != "" {
		query += ` AND status = ?`
		args = append(args, filter.Status)
	} else if !filter.IncludeArchived {
		query += ` AND status != ?`
		args = append(args, domain.ContentSourceStatusArchived)
	}
	if tag := strings.TrimSpace(filter.Tag); tag != "" {
		query += ` AND tags LIKE ?`
		args = append(args, "%"+tag+"%")
	}
	query += ` ORDER BY updated_at DESC, title ASC`
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
	out := make([]domain.ContentSource, 0, len(ids))
	for _, id := range ids {
		source, err := s.GetContentSource(ctx, id)
		if err != nil {
			return nil, err
		}
		out = append(out, source)
	}
	return out, nil
}

func (s *Store) UpdateContentSource(ctx context.Context, source domain.ContentSource) (domain.ContentSource, error) {
	tags, err := json.Marshal(normalizeStringList(source.Tags))
	if err != nil {
		return domain.ContentSource{}, err
	}
	now := time.Now().UTC()
	result, err := s.db.ExecContext(ctx, `
		UPDATE content_sources
		SET title = ?, body = ?, source_url = ?, campaign_id = ?, brand_profile_id = ?, tags = ?, status = ?, updated_at = ?
		WHERE id = ?
	`, strings.TrimSpace(source.Title), strings.TrimSpace(source.Body), strings.TrimSpace(source.SourceURL), strings.TrimSpace(source.CampaignID), strings.TrimSpace(source.BrandProfileID), string(tags), source.Status, now.Format(time.RFC3339Nano), strings.TrimSpace(source.ID))
	if err != nil {
		return domain.ContentSource{}, err
	}
	if rows, _ := result.RowsAffected(); rows == 0 {
		return domain.ContentSource{}, sql.ErrNoRows
	}
	return s.GetContentSource(ctx, source.ID)
}
