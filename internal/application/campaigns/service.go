package campaigns

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"time"

	"github.com/escarface/sarepost/internal/domain"
)

var (
	ErrNameRequired       = errors.New("campaign name is required")
	ErrInvalidStatus      = errors.New("campaign status is invalid")
	ErrInvalidTimezone    = errors.New("campaign timezone is invalid")
	ErrCampaignIDRequired = errors.New("campaign id is required")
	ErrCampaignNotFound   = errors.New("campaign not found")
)

type Store interface {
	CreateCampaign(ctx context.Context, campaign domain.Campaign) (domain.Campaign, error)
	ListCampaigns(ctx context.Context, filter domain.CampaignListFilter) ([]domain.Campaign, error)
	GetCampaign(ctx context.Context, id string) (domain.Campaign, error)
	UpdateCampaign(ctx context.Context, campaign domain.Campaign) (domain.Campaign, error)
}

type Service struct {
	Store Store
}

type CreateInput struct {
	Name           string
	Objective      string
	StartsAt       time.Time
	EndsAt         time.Time
	Notes          string
	Tags           []string
	Timezone       string
	Audience       string
	Tone           string
	CTA            string
	Restrictions   string
	BrandProfileID string
}

type UpdateInput struct {
	ID             string
	Name           string
	Objective      string
	Status         domain.CampaignStatus
	StartsAt       time.Time
	EndsAt         time.Time
	Notes          string
	Tags           []string
	Timezone       string
	Audience       string
	Tone           string
	CTA            string
	Restrictions   string
	BrandProfileID string
}

type ListFilter = domain.CampaignListFilter

func (s Service) Create(ctx context.Context, in CreateInput) (domain.Campaign, error) {
	campaign := domain.Campaign{
		Name:           strings.TrimSpace(in.Name),
		Objective:      strings.TrimSpace(in.Objective),
		Status:         domain.CampaignStatusActive,
		StartsAt:       in.StartsAt.UTC(),
		EndsAt:         in.EndsAt.UTC(),
		Notes:          strings.TrimSpace(in.Notes),
		Tags:           normalizeTags(in.Tags),
		Timezone:       strings.TrimSpace(in.Timezone),
		Audience:       strings.TrimSpace(in.Audience),
		Tone:           strings.TrimSpace(in.Tone),
		CTA:            strings.TrimSpace(in.CTA),
		Restrictions:   strings.TrimSpace(in.Restrictions),
		BrandProfileID: strings.TrimSpace(in.BrandProfileID),
	}
	if campaign.Name == "" {
		return domain.Campaign{}, ErrNameRequired
	}
	if campaign.Timezone != "" {
		if _, err := time.LoadLocation(campaign.Timezone); err != nil {
			return domain.Campaign{}, ErrInvalidTimezone
		}
	}
	return s.Store.CreateCampaign(ctx, campaign)
}

func (s Service) List(ctx context.Context, filter ListFilter) ([]domain.Campaign, error) {
	filter.Tag = strings.TrimSpace(filter.Tag)
	if filter.Status != "" && !validCampaignStatus(filter.Status) {
		return nil, ErrInvalidStatus
	}
	return s.Store.ListCampaigns(ctx, filter)
}

func (s Service) Update(ctx context.Context, in UpdateInput) (domain.Campaign, error) {
	id := strings.TrimSpace(in.ID)
	if id == "" {
		return domain.Campaign{}, ErrCampaignIDRequired
	}
	current, err := s.Store.GetCampaign(ctx, id)
	if err != nil {
		return domain.Campaign{}, mapCampaignNotFound(err)
	}
	if strings.TrimSpace(in.Name) != "" {
		current.Name = strings.TrimSpace(in.Name)
	}
	current.Objective = strings.TrimSpace(in.Objective)
	if in.Status != "" {
		if !validCampaignStatus(in.Status) {
			return domain.Campaign{}, ErrInvalidStatus
		}
		current.Status = in.Status
	}
	current.StartsAt = in.StartsAt.UTC()
	current.EndsAt = in.EndsAt.UTC()
	current.Notes = strings.TrimSpace(in.Notes)
	current.Tags = normalizeTags(in.Tags)
	current.Timezone = strings.TrimSpace(in.Timezone)
	if current.Timezone != "" {
		if _, err := time.LoadLocation(current.Timezone); err != nil {
			return domain.Campaign{}, ErrInvalidTimezone
		}
	}
	current.Audience = strings.TrimSpace(in.Audience)
	current.Tone = strings.TrimSpace(in.Tone)
	current.CTA = strings.TrimSpace(in.CTA)
	current.Restrictions = strings.TrimSpace(in.Restrictions)
	if strings.TrimSpace(in.BrandProfileID) != "" {
		current.BrandProfileID = strings.TrimSpace(in.BrandProfileID)
	}
	if current.Name == "" {
		return domain.Campaign{}, ErrNameRequired
	}
	return s.Store.UpdateCampaign(ctx, current)
}

func (s Service) Archive(ctx context.Context, id string) (domain.Campaign, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return domain.Campaign{}, ErrCampaignIDRequired
	}
	current, err := s.Store.GetCampaign(ctx, id)
	if err != nil {
		return domain.Campaign{}, mapCampaignNotFound(err)
	}
	current.Status = domain.CampaignStatusArchived
	return s.Store.UpdateCampaign(ctx, current)
}

func mapCampaignNotFound(err error) error {
	if errors.Is(err, sql.ErrNoRows) {
		return ErrCampaignNotFound
	}
	return err
}

func normalizeTags(tags []string) []string {
	if len(tags) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(tags))
	out := make([]string, 0, len(tags))
	for _, raw := range tags {
		tag := strings.TrimSpace(raw)
		if tag == "" {
			continue
		}
		if _, ok := seen[tag]; ok {
			continue
		}
		seen[tag] = struct{}{}
		out = append(out, tag)
	}
	return out
}

func validCampaignStatus(status domain.CampaignStatus) bool {
	switch status {
	case domain.CampaignStatusActive, domain.CampaignStatusPaused, domain.CampaignStatusArchived:
		return true
	default:
		return false
	}
}
