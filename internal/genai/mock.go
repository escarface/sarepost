package genai

import (
	"bytes"
	"context"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"strings"
)

// MockTextProvider returns deterministic copy without any network call.
type MockTextProvider struct {
	ModelID string
}

func (m MockTextProvider) Name() string { return ProviderMock }

func (m MockTextProvider) GenerateText(_ context.Context, req TextRequest) (TextResult, error) {
	prompt := strings.TrimSpace(req.Prompt)
	if prompt == "" {
		return TextResult{}, fmt.Errorf("prompt is required")
	}
	model := m.ModelID
	if model == "" {
		model = "mock-text"
	}
	return TextResult{
		Text:  fmt.Sprintf("[mock] %s", prompt),
		Model: model,
	}, nil
}

// MockImageProvider returns a small solid-color PNG without any network call.
type MockImageProvider struct {
	ModelID string
}

func (m MockImageProvider) Name() string { return ProviderMock }

func (m MockImageProvider) GenerateImage(_ context.Context, req ImageRequest) (ImageResult, error) {
	if strings.TrimSpace(req.Prompt) == "" {
		return ImageResult{}, fmt.Errorf("prompt is required")
	}
	img := image.NewRGBA(image.Rect(0, 0, 8, 8))
	for x := 0; x < 8; x++ {
		for y := 0; y < 8; y++ {
			img.Set(x, y, color.RGBA{R: 0x4a, G: 0x6c, B: 0xff, A: 0xff})
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return ImageResult{}, err
	}
	model := m.ModelID
	if model == "" {
		model = "mock-image"
	}
	return ImageResult{Data: buf.Bytes(), MimeType: "image/png", Model: model}, nil
}
