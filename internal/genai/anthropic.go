package genai

import (
	"context"
	"fmt"
	"strings"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"
)

// DefaultAnthropicModel is used when no model is configured.
const DefaultAnthropicModel = "claude-opus-4-8"

type anthropicTextProvider struct {
	apiKey  string
	model   string
	baseURL string
}

func newAnthropicTextProvider(cfg ProviderConfig) anthropicTextProvider {
	model := strings.TrimSpace(cfg.Model)
	if model == "" {
		model = DefaultAnthropicModel
	}
	return anthropicTextProvider{apiKey: cfg.APIKey, model: model, baseURL: strings.TrimSpace(cfg.BaseURL)}
}

func (p anthropicTextProvider) Name() string { return ProviderAnthropic }

func (p anthropicTextProvider) GenerateText(ctx context.Context, req TextRequest) (TextResult, error) {
	if strings.TrimSpace(req.Prompt) == "" {
		return TextResult{}, fmt.Errorf("prompt is required")
	}
	maxTokens := req.MaxTokens
	if maxTokens <= 0 {
		maxTokens = 2048
	}

	opts := []option.RequestOption{option.WithAPIKey(p.apiKey)}
	if p.baseURL != "" {
		opts = append(opts, option.WithBaseURL(p.baseURL))
	}
	client := anthropic.NewClient(opts...)

	params := anthropic.MessageNewParams{
		Model:     anthropic.Model(p.model),
		MaxTokens: int64(maxTokens),
		Messages: []anthropic.MessageParam{
			anthropic.NewUserMessage(anthropic.NewTextBlock(req.Prompt)),
		},
	}
	if system := strings.TrimSpace(req.System); system != "" {
		params.System = []anthropic.TextBlockParam{{Text: system}}
	}

	resp, err := client.Messages.New(ctx, params)
	if err != nil {
		return TextResult{}, fmt.Errorf("anthropic generate text: %w", err)
	}

	var b strings.Builder
	for _, block := range resp.Content {
		if text, ok := block.AsAny().(anthropic.TextBlock); ok {
			b.WriteString(text.Text)
		}
	}
	out := strings.TrimSpace(b.String())
	if out == "" {
		return TextResult{}, fmt.Errorf("anthropic returned empty response")
	}
	return TextResult{Text: out, Model: p.model}, nil
}
