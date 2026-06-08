package api

import (
	"context"
	"strings"

	"github.com/saredigital/sarepost/internal/domain"
)

func (s Server) resolveTargetAccount(ctx context.Context, accountIDRaw string) (domain.SocialAccount, error) {
	return s.Store.GetAccount(ctx, strings.TrimSpace(accountIDRaw))
}
