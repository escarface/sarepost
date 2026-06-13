package api

import (
	"strings"
	"sync"

	generationapp "github.com/escarface/sarepost/internal/application/generation"
	"github.com/escarface/sarepost/internal/domain"
	"github.com/escarface/sarepost/internal/genai"
	"github.com/escarface/sarepost/internal/postflow"
	"github.com/escarface/sarepost/internal/secure"
)

var (
	fallbackCipherOnce sync.Once
	fallbackCipher     *secure.Cipher
)

func (s Server) providerRegistry() *postflow.ProviderRegistry {
	if s.Registry != nil {
		return s.Registry
	}
	return postflow.NewProviderRegistry(
		postflow.NewMockProvider(domain.PlatformX),
		postflow.NewMockProvider(domain.PlatformLinkedIn),
		postflow.NewMockProvider(domain.PlatformFacebook),
		postflow.NewMockProvider(domain.PlatformInstagram),
	)
}

func (s Server) credentialsCipher() *secure.Cipher {
	if s.Cipher != nil {
		return s.Cipher
	}
	fallbackCipherOnce.Do(func() {
		key := make([]byte, 32)
		for i := range key {
			key[i] = byte(i + 1)
		}
		cipher, _ := secure.NewCipher(key, 1)
		fallbackCipher = cipher
	})
	return fallbackCipher
}

func (s Server) generationService() generationapp.Service {
	driver := genai.DriverMock
	if strings.EqualFold(strings.TrimSpace(s.GenerationDriver), "live") {
		driver = genai.DriverLive
	}
	return generationapp.Service{
		Store:  s.Store,
		Cipher: s.credentialsCipher(),
		Driver: driver,
	}
}
