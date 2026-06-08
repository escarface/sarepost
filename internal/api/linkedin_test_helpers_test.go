package api

import (
	"github.com/escarface/sarepost/internal/domain"
	"github.com/escarface/sarepost/internal/postflow"
)

func testRegistryWithRealLinkedIn() *postflow.ProviderRegistry {
	return postflow.NewProviderRegistry(
		postflow.NewMockProvider(domain.PlatformX),
		postflow.NewLinkedInProvider(postflow.LinkedInProviderConfig{}),
		postflow.NewMockProvider(domain.PlatformFacebook),
		postflow.NewMockProvider(domain.PlatformInstagram),
	)
}
