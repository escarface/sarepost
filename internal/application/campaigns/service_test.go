package campaigns

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/escarface/sarepost/internal/domain"
)

func TestServiceCreatesListsUpdatesAndArchivesCampaign(t *testing.T) {
	store := newMemoryStore()
	svc := Service{Store: store}

	start := time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(2026, 7, 31, 23, 59, 0, 0, time.UTC)
	created, err := svc.Create(t.Context(), CreateInput{
		Name:         "Launch July",
		Objective:    "Promote the summer offer",
		StartsAt:     start,
		EndsAt:       end,
		Notes:        "Focus on LinkedIn and Instagram",
		Tags:         []string{"launch", "summer"},
		Timezone:     "Europe/Madrid",
		Audience:     "Spanish founders",
		Tone:         "direct",
		CTA:          "Book a call",
		Restrictions: "No discounts language",
	})
	if err != nil {
		t.Fatalf("create campaign: %v", err)
	}
	if created.ID == "" {
		t.Fatalf("expected generated campaign id")
	}
	if created.Status != domain.CampaignStatusActive {
		t.Fatalf("expected active status, got %s", created.Status)
	}
	if len(created.Tags) != 2 || created.Tags[0] != "launch" || created.Tags[1] != "summer" {
		t.Fatalf("expected normalized tags, got %#v", created.Tags)
	}

	listed, err := svc.List(t.Context(), ListFilter{Status: domain.CampaignStatusActive})
	if err != nil {
		t.Fatalf("list campaigns: %v", err)
	}
	if len(listed) != 1 || listed[0].ID != created.ID {
		t.Fatalf("expected created campaign in active list, got %#v", listed)
	}

	updated, err := svc.Update(t.Context(), UpdateInput{
		ID:       created.ID,
		Name:     "Launch July Updated",
		Status:   domain.CampaignStatusPaused,
		Timezone: "Europe/Madrid",
		Tags:     []string{"summer"},
	})
	if err != nil {
		t.Fatalf("update campaign: %v", err)
	}
	if updated.Name != "Launch July Updated" || updated.Status != domain.CampaignStatusPaused {
		t.Fatalf("unexpected updated campaign: %#v", updated)
	}

	archived, err := svc.Archive(t.Context(), created.ID)
	if err != nil {
		t.Fatalf("archive campaign: %v", err)
	}
	if archived.Status != domain.CampaignStatusArchived {
		t.Fatalf("expected archived status, got %s", archived.Status)
	}
}

func TestServiceRejectsInvalidCampaignInput(t *testing.T) {
	svc := Service{Store: newMemoryStore()}
	if _, err := svc.Create(t.Context(), CreateInput{Name: "   "}); !errors.Is(err, ErrNameRequired) {
		t.Fatalf("expected ErrNameRequired, got %v", err)
	}
	if _, err := svc.Create(t.Context(), CreateInput{Name: "Bad zone", Timezone: "Nope/Nowhere"}); !errors.Is(err, ErrInvalidTimezone) {
		t.Fatalf("expected ErrInvalidTimezone, got %v", err)
	}
}

type memoryStore struct {
	items map[string]domain.Campaign
	next  int
}

func newMemoryStore() *memoryStore {
	return &memoryStore{items: map[string]domain.Campaign{}}
}

func (s *memoryStore) CreateCampaign(_ context.Context, campaign domain.Campaign) (domain.Campaign, error) {
	s.next++
	campaign.ID = "cmp_test"
	if s.next > 1 {
		campaign.ID = "cmp_other"
	}
	now := time.Date(2026, 6, 17, 10, 0, 0, 0, time.UTC)
	campaign.CreatedAt = now
	campaign.UpdatedAt = now
	s.items[campaign.ID] = campaign
	return campaign, nil
}

func (s *memoryStore) ListCampaigns(_ context.Context, filter ListFilter) ([]domain.Campaign, error) {
	out := make([]domain.Campaign, 0, len(s.items))
	for _, item := range s.items {
		if filter.Status != "" && item.Status != filter.Status {
			continue
		}
		out = append(out, item)
	}
	return out, nil
}

func (s *memoryStore) GetCampaign(_ context.Context, id string) (domain.Campaign, error) {
	item, ok := s.items[id]
	if !ok {
		return domain.Campaign{}, ErrCampaignNotFound
	}
	return item, nil
}

func (s *memoryStore) UpdateCampaign(_ context.Context, campaign domain.Campaign) (domain.Campaign, error) {
	if _, ok := s.items[campaign.ID]; !ok {
		return domain.Campaign{}, ErrCampaignNotFound
	}
	campaign.UpdatedAt = time.Date(2026, 6, 17, 11, 0, 0, 0, time.UTC)
	s.items[campaign.ID] = campaign
	return campaign, nil
}
