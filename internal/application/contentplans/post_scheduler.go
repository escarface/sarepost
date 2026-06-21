package contentplans

import (
	"context"
	"errors"
	"strings"

	"github.com/escarface/sarepost/internal/application/ports"
	postsapp "github.com/escarface/sarepost/internal/application/posts"
	"github.com/escarface/sarepost/internal/domain"
)

type PostStore interface {
	postsapp.Store
	postsapp.MutationsStore
}

type PostScheduler struct {
	Store             PostStore
	Registry          ports.ProviderRegistry
	DefaultMaxRetries int
}

func (s PostScheduler) ScheduleVariant(ctx context.Context, variant domain.ContentPlanVariant, block domain.ContentPlanBlock) (domain.Post, error) {
	if s.Store == nil || s.Registry == nil {
		return domain.Post{}, errors.New("content plan post scheduler is not configured")
	}
	if err := (postsapp.MutationsService{Store: s.Store, Registry: s.Registry}).ValidateNewSchedule(ctx, variant.AccountID, variant.Text, variant.PlannedAt); err != nil {
		return domain.Post{}, err
	}
	mediaIDs := []string(nil)
	if mediaID := strings.TrimSpace(variant.MediaID); mediaID != "" {
		mediaIDs = []string{mediaID}
	}
	out, err := (postsapp.CreateService{Store: s.Store, Registry: s.Registry, DefaultMaxRetries: s.DefaultMaxRetries}).Create(ctx, postsapp.CreateInput{
		AccountIDs: []string{variant.AccountID}, Text: variant.Text, ScheduledAt: variant.PlannedAt,
		MediaIDs: mediaIDs, IdempotencyKey: "content-plan:" + variant.ID,
		CampaignID: block.CampaignID, EditorialStatus: domain.EditorialStatusScheduled,
	})
	if err != nil {
		return domain.Post{}, err
	}
	if len(out.Items) == 0 {
		return domain.Post{}, errors.New("content plan post was not created")
	}
	return out.Items[0].Post, nil
}
