// Package genai provides provider-agnostic text and image generation clients.
//
// It mirrors the structure of internal/postflow: small provider interfaces,
// concrete clients per provider (Anthropic, OpenAI), and mock implementations
// used by the default driver and tests. Provider selection is dynamic — the
// API key is supplied per call from encrypted settings — so the package exposes
// factory functions (NewTextProvider/NewImageProvider) instead of a registry.
package genai

import (
	"context"
	"errors"
	"fmt"
	"strings"
)

// Driver selects the concrete provider implementations. "mock" never performs
// network calls and is the default for local development and tests.
type Driver string

const (
	DriverMock Driver = "mock"
	DriverLive Driver = "live"
)

// Provider identifiers persisted in settings and selected in the UI.
const (
	ProviderAnthropic = "anthropic"
	ProviderOpenAI    = "openai"
	ProviderMock      = "mock"
)

// ErrProviderNotSupported is returned when a provider id has no implementation.
var ErrProviderNotSupported = errors.New("generation provider not supported")

// TextRequest is a single text-generation call.
type TextRequest struct {
	System    string
	Prompt    string
	MaxTokens int
}

// TextResult is the generated text.
type TextResult struct {
	Text  string
	Model string
}

// ImageRequest is a single image-generation call.
type ImageRequest struct {
	Prompt string
	// Size is a provider-agnostic hint like "1024x1024". Empty means provider default.
	Size string
}

// ImageResult carries the raw generated image bytes and its MIME type.
type ImageResult struct {
	Data     []byte
	MimeType string
	Model    string
}

// TextProvider generates post copy from a prompt.
type TextProvider interface {
	Name() string
	GenerateText(ctx context.Context, req TextRequest) (TextResult, error)
}

// ImageProvider generates an image from a prompt.
type ImageProvider interface {
	Name() string
	GenerateImage(ctx context.Context, req ImageRequest) (ImageResult, error)
}

// ProviderConfig describes which provider/model/credentials to use for a call.
type ProviderConfig struct {
	Provider string
	Model    string
	APIKey   string
	// BaseURL overrides the provider endpoint (used by tests). Empty means default.
	BaseURL string
}

// SupportedTextProviders lists the provider ids that can generate text.
func SupportedTextProviders() []string {
	return []string{ProviderAnthropic, ProviderOpenAI}
}

// SupportedImageProviders lists the provider ids that can generate images.
func SupportedImageProviders() []string {
	return []string{ProviderOpenAI}
}

// NewTextProvider builds a text provider from config for the given driver.
func NewTextProvider(driver Driver, cfg ProviderConfig) (TextProvider, error) {
	if driver == DriverMock {
		return MockTextProvider{ModelID: cfg.Model}, nil
	}
	if strings.TrimSpace(cfg.APIKey) == "" {
		return nil, errors.New("api key is required for live text generation")
	}
	switch cfg.Provider {
	case ProviderAnthropic:
		return newAnthropicTextProvider(cfg), nil
	case ProviderOpenAI:
		return newOpenAITextProvider(cfg), nil
	default:
		return nil, fmt.Errorf("%w: %q", ErrProviderNotSupported, cfg.Provider)
	}
}

// NewImageProvider builds an image provider from config for the given driver.
func NewImageProvider(driver Driver, cfg ProviderConfig) (ImageProvider, error) {
	if driver == DriverMock {
		return MockImageProvider{ModelID: cfg.Model}, nil
	}
	if strings.TrimSpace(cfg.APIKey) == "" {
		return nil, errors.New("api key is required for live image generation")
	}
	switch cfg.Provider {
	case ProviderOpenAI:
		return newOpenAIImageProvider(cfg), nil
	default:
		return nil, fmt.Errorf("%w: %q", ErrProviderNotSupported, cfg.Provider)
	}
}
