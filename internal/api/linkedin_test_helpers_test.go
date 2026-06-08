package api

import (
	"github.com/saredigital/sarepost/internal/domain"
	"github.com/saredigital/sarepost/internal/postflow"
)

func testRegistryWithRealLinkedIn() *postflow.ProviderRegistry {
	return postflow.NewProviderRegistry(
		postflow.NewMockProvider(domain.PlatformX),
		postflow.NewLinkedInProvider(postflow.LinkedInProviderConfig{}),
		postflow.NewMockProvider(domain.PlatformFacebook),
		postflow.NewMockProvider(domain.PlatformInstagram),
	)
}
