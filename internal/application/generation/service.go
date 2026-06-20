package generation

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/escarface/sarepost/internal/domain"
	"github.com/escarface/sarepost/internal/genai"
	"github.com/escarface/sarepost/internal/secure"
)

// ErrProviderNotConfigured is returned when no provider has been configured for
// the requested generation kind.
var ErrProviderNotConfigured = errors.New("generation provider is not configured")

// TextProviderFactory builds a text provider from driver + config.
type TextProviderFactory func(genai.Driver, genai.ProviderConfig) (genai.TextProvider, error)

// ImageProviderFactory builds an image provider from driver + config.
type ImageProviderFactory func(genai.Driver, genai.ProviderConfig) (genai.ImageProvider, error)

// MediaReader loads stored media bytes by id. Satisfied by the adapter so the
// application layer stays free of storage/filesystem concerns.
type MediaReader interface {
	ReadMedia(ctx context.Context, mediaID string) (data []byte, mimeType string, err error)
}

// Service orchestrates in-app text and image generation. Business logic only —
// it persists nothing beyond settings; the adapter persists generated media.
type Service struct {
	Store        SettingsStore
	Cipher       *secure.Cipher
	Driver       genai.Driver
	TextFactory  TextProviderFactory
	ImageFactory ImageProviderFactory
	MediaReader  MediaReader
}

// GenerateTextInput is a text-generation request.
type GenerateTextInput struct {
	Prompt         string
	Platform       domain.Platform
	BrandProfileID string
	MaxTokens      int
}

// GenerateTextOutput is the generated copy.
type GenerateTextOutput struct {
	Text     string
	Model    string
	Provider string
}

// GenerateImageInput is an image-generation request.
type GenerateImageInput struct {
	Prompt         string
	BrandProfileID string
	Size           string
}

// GenerateImageOutput carries the raw generated image.
type GenerateImageOutput struct {
	Data     []byte
	MimeType string
	Model    string
	Provider string
}

func (s Service) textFactory() TextProviderFactory {
	if s.TextFactory != nil {
		return s.TextFactory
	}
	return genai.NewTextProvider
}

func (s Service) imageFactory() ImageProviderFactory {
	if s.ImageFactory != nil {
		return s.ImageFactory
	}
	return genai.NewImageProvider
}

func (s Service) driver() genai.Driver {
	if s.Driver == "" {
		return genai.DriverMock
	}
	return s.Driver
}

// GenerateText builds a brand/platform-aware prompt and delegates to the provider.
func (s Service) GenerateText(ctx context.Context, in GenerateTextInput) (GenerateTextOutput, error) {
	prompt := strings.TrimSpace(in.Prompt)
	if prompt == "" {
		return GenerateTextOutput{}, errors.New("prompt is required")
	}
	cfg, ok, err := s.getProviderConfig(ctx, kindText)
	if err != nil {
		return GenerateTextOutput{}, err
	}
	if !ok {
		return GenerateTextOutput{}, ErrProviderNotConfigured
	}
	profile, err := s.resolveBrandProfile(ctx, in.BrandProfileID)
	if err != nil {
		return GenerateTextOutput{}, err
	}
	provider, err := s.textFactory()(s.driver(), genai.ProviderConfig{
		Provider: cfg.Provider, Model: cfg.Model, APIKey: cfg.APIKey,
	})
	if err != nil {
		return GenerateTextOutput{}, err
	}
	result, err := provider.GenerateText(ctx, genai.TextRequest{
		System:    buildTextSystemPrompt(profile, in.Platform),
		Prompt:    prompt,
		MaxTokens: in.MaxTokens,
	})
	if err != nil {
		return GenerateTextOutput{}, err
	}
	return GenerateTextOutput{Text: result.Text, Model: result.Model, Provider: provider.Name()}, nil
}

// GenerateImage builds a brand-aware image prompt and delegates to the provider.
func (s Service) GenerateImage(ctx context.Context, in GenerateImageInput) (GenerateImageOutput, error) {
	prompt := strings.TrimSpace(in.Prompt)
	if prompt == "" {
		return GenerateImageOutput{}, errors.New("prompt is required")
	}
	cfg, ok, err := s.getProviderConfig(ctx, kindImage)
	if err != nil {
		return GenerateImageOutput{}, err
	}
	if !ok {
		return GenerateImageOutput{}, ErrProviderNotConfigured
	}
	profile, err := s.resolveBrandProfile(ctx, in.BrandProfileID)
	if err != nil {
		return GenerateImageOutput{}, err
	}
	provider, err := s.imageFactory()(s.driver(), genai.ProviderConfig{
		Provider: cfg.Provider, Model: cfg.Model, APIKey: cfg.APIKey,
	})
	if err != nil {
		return GenerateImageOutput{}, err
	}
	refImage, refMime, err := s.loadReferenceImage(ctx, profile)
	if err != nil {
		return GenerateImageOutput{}, err
	}
	result, err := provider.GenerateImage(ctx, genai.ImageRequest{
		Prompt:       buildImagePrompt(prompt, profile, len(refImage) > 0),
		Size:         in.Size,
		RefImage:     refImage,
		RefImageMime: refMime,
	})
	if err != nil {
		return GenerateImageOutput{}, err
	}
	return GenerateImageOutput{
		Data: result.Data, MimeType: result.MimeType, Model: result.Model, Provider: provider.Name(),
	}, nil
}

func (s Service) resolveBrandProfile(ctx context.Context, id string) (BrandProfile, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return BrandProfile{}, nil
	}
	profile, ok, err := s.GetBrandProfile(ctx, id)
	if err != nil {
		return BrandProfile{}, err
	}
	if !ok {
		return BrandProfile{}, fmt.Errorf("brand profile %s not found", id)
	}
	return profile, nil
}

func buildTextSystemPrompt(profile BrandProfile, platform domain.Platform) string {
	var b strings.Builder
	b.WriteString("You are a social media copywriter generating a single post.")
	if sp := strings.TrimSpace(profile.SystemPrompt); sp != "" {
		b.WriteString("\n\nBrand context:\n")
		b.WriteString(sp)
	}
	if tone := strings.TrimSpace(profile.Tone); tone != "" {
		b.WriteString("\n\nTone: ")
		b.WriteString(tone)
	}
	if rules := platformGuidance(platform); rules != "" {
		b.WriteString("\n\n")
		b.WriteString(rules)
	}
	b.WriteString("\n\nReturn only the post copy, with no preamble or surrounding quotes.")
	return b.String()
}

func (s Service) loadReferenceImage(ctx context.Context, profile BrandProfile) ([]byte, string, error) {
	mediaID := strings.TrimSpace(profile.ImageRefMediaID)
	if mediaID == "" || s.MediaReader == nil {
		return nil, "", nil
	}
	data, mime, err := s.MediaReader.ReadMedia(ctx, mediaID)
	if err != nil {
		return nil, "", fmt.Errorf("load brand reference image: %w", err)
	}
	return data, mime, nil
}

func buildImagePrompt(prompt string, profile BrandProfile, hasReference bool) string {
	var b strings.Builder
	b.WriteString(prompt)
	if sp := strings.TrimSpace(profile.SystemPrompt); sp != "" {
		b.WriteString("\n\nBrand context:\n")
		b.WriteString(sp)
	}
	if tone := strings.TrimSpace(profile.Tone); tone != "" {
		b.WriteString("\n\nDesired tone/look: ")
		b.WriteString(tone)
	}
	if hasReference {
		b.WriteString("\n\nMatch the visual style, color palette, and overall look of the provided reference image.")
	}
	return b.String()
}

func platformGuidance(platform domain.Platform) string {
	switch platform {
	case domain.PlatformX:
		return "Target platform: X (Twitter). Keep it under 280 characters and punchy."
	case domain.PlatformLinkedIn:
		return "Target platform: LinkedIn. Professional, value-driven tone; short paragraphs are fine."
	case domain.PlatformFacebook:
		return "Target platform: Facebook. Conversational and engaging."
	case domain.PlatformInstagram:
		return "Target platform: Instagram. Visual-first caption with a few relevant hashtags."
	default:
		return ""
	}
}
