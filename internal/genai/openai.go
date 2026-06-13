package genai

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// Default OpenAI models used when none is configured.
const (
	DefaultOpenAITextModel  = "gpt-4o"
	DefaultOpenAIImageModel = "gpt-image-1"
	defaultOpenAIBaseURL    = "https://api.openai.com/v1"
)

func openAIHTTPClient() *http.Client {
	return &http.Client{Timeout: 120 * time.Second}
}

func openAIBaseURL(cfg ProviderConfig) string {
	if base := strings.TrimSpace(cfg.BaseURL); base != "" {
		return strings.TrimRight(base, "/")
	}
	return defaultOpenAIBaseURL
}

// --- text ---

type openAITextProvider struct {
	apiKey  string
	model   string
	baseURL string
}

func newOpenAITextProvider(cfg ProviderConfig) openAITextProvider {
	model := strings.TrimSpace(cfg.Model)
	if model == "" {
		model = DefaultOpenAITextModel
	}
	return openAITextProvider{apiKey: cfg.APIKey, model: model, baseURL: openAIBaseURL(cfg)}
}

func (p openAITextProvider) Name() string { return ProviderOpenAI }

func (p openAITextProvider) GenerateText(ctx context.Context, req TextRequest) (TextResult, error) {
	if strings.TrimSpace(req.Prompt) == "" {
		return TextResult{}, fmt.Errorf("prompt is required")
	}
	messages := make([]map[string]string, 0, 2)
	if system := strings.TrimSpace(req.System); system != "" {
		messages = append(messages, map[string]string{"role": "system", "content": system})
	}
	messages = append(messages, map[string]string{"role": "user", "content": req.Prompt})

	body := map[string]any{"model": p.model, "messages": messages}
	if req.MaxTokens > 0 {
		body["max_tokens"] = req.MaxTokens
	}

	var parsed struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := p.postJSON(ctx, "/chat/completions", body, &parsed); err != nil {
		return TextResult{}, err
	}
	if len(parsed.Choices) == 0 {
		return TextResult{}, fmt.Errorf("openai returned no choices")
	}
	out := strings.TrimSpace(parsed.Choices[0].Message.Content)
	if out == "" {
		return TextResult{}, fmt.Errorf("openai returned empty response")
	}
	return TextResult{Text: out, Model: p.model}, nil
}

func (p openAITextProvider) postJSON(ctx context.Context, path string, body any, out any) error {
	return openAIPostJSON(ctx, p.baseURL, p.apiKey, path, body, out)
}

// --- image ---

type openAIImageProvider struct {
	apiKey  string
	model   string
	baseURL string
}

func newOpenAIImageProvider(cfg ProviderConfig) openAIImageProvider {
	model := strings.TrimSpace(cfg.Model)
	if model == "" {
		model = DefaultOpenAIImageModel
	}
	return openAIImageProvider{apiKey: cfg.APIKey, model: model, baseURL: openAIBaseURL(cfg)}
}

func (p openAIImageProvider) Name() string { return ProviderOpenAI }

func (p openAIImageProvider) GenerateImage(ctx context.Context, req ImageRequest) (ImageResult, error) {
	if strings.TrimSpace(req.Prompt) == "" {
		return ImageResult{}, fmt.Errorf("prompt is required")
	}
	size := strings.TrimSpace(req.Size)
	if size == "" {
		size = "1024x1024"
	}
	body := map[string]any{"model": p.model, "prompt": req.Prompt, "size": size, "n": 1}

	var parsed struct {
		Data []struct {
			B64JSON string `json:"b64_json"`
		} `json:"data"`
	}
	if err := openAIPostJSON(ctx, p.baseURL, p.apiKey, "/images/generations", body, &parsed); err != nil {
		return ImageResult{}, err
	}
	if len(parsed.Data) == 0 || parsed.Data[0].B64JSON == "" {
		return ImageResult{}, fmt.Errorf("openai returned no image data")
	}
	raw, err := base64.StdEncoding.DecodeString(parsed.Data[0].B64JSON)
	if err != nil {
		return ImageResult{}, fmt.Errorf("decode openai image: %w", err)
	}
	return ImageResult{Data: raw, MimeType: "image/png", Model: p.model}, nil
}

func openAIPostJSON(ctx context.Context, baseURL, apiKey, path string, body any, out any) error {
	payload, err := json.Marshal(body)
	if err != nil {
		return err
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, baseURL+path, bytes.NewReader(payload))
	if err != nil {
		return err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+apiKey)

	resp, err := openAIHTTPClient().Do(httpReq)
	if err != nil {
		return fmt.Errorf("openai request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	data, err := io.ReadAll(io.LimitReader(resp.Body, 16<<20))
	if err != nil {
		return err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("openai request failed (%d): %s", resp.StatusCode, strings.TrimSpace(string(data)))
	}
	if err := json.Unmarshal(data, out); err != nil {
		return fmt.Errorf("decode openai response: %w", err)
	}
	return nil
}
