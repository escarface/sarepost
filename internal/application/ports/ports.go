package ports

import (
	"context"

	"github.com/saredigital/sarepost/internal/domain"
	"github.com/saredigital/sarepost/internal/postflow"
)

type ProviderRegistry interface {
	Get(platform domain.Platform) (postflow.Provider, bool)
}

type CredentialsStore interface {
	LoadCredentials(ctx context.Context, accountID string) (postflow.Credentials, error)
	SaveCredentials(ctx context.Context, accountID string, credentials postflow.Credentials) error
}

type PublishFailureNotification struct {
	Post    domain.Post
	Account domain.SocialAccount
	Error   error
}

type PublishFailureNotifier interface {
	NotifyPublishFailure(ctx context.Context, notification PublishFailureNotification) error
}
