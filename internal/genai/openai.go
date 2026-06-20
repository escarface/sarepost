package genai

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/textproto"
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
	if req.WebSearchRequired {
		return p.generateTextWithWebSearch(ctx, req)
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

func (p openAITextProvider) generateTextWithWebSearch(ctx context.Context, req TextRequest) (TextResult, error) {
	input := make([]map[string]any, 0, 2)
	if system := strings.TrimSpace(req.System); system != "" {
		input = append(input, map[string]any{
			"role":    "system",
			"content": []map[string]string{{"type": "input_text", "text": system}},
		})
	}
	input = append(input, map[string]any{
		"role":    "user",
		"content": []map[string]string{{"type": "input_text", "text": req.Prompt}},
	})

	body := map[string]any{
		"model":       p.model,
		"input":       input,
		"tools":       []map[string]any{{"type": "web_search"}},
		"tool_choice": "required",
	}
	if req.MaxTokens > 0 {
		body["max_output_tokens"] = req.MaxTokens
	}

	var parsed struct {
		OutputText string `json:"output_text"`
	}
	if err := p.postJSON(ctx, "/responses", body, &parsed); err != nil {
		return TextResult{}, err
	}
	out := strings.TrimSpace(parsed.OutputText)
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

type openAIImageResponse struct {
	Data []struct {
		B64JSON string `json:"b64_json"`
	} `json:"data"`
}

func (p openAIImageProvider) GenerateImage(ctx context.Context, req ImageRequest) (ImageResult, error) {
	if strings.TrimSpace(req.Prompt) == "" {
		return ImageResult{}, fmt.Errorf("prompt is required")
	}
	size := strings.TrimSpace(req.Size)
	if size == "" {
		size = "1024x1024"
	}

	var parsed openAIImageResponse
	var err error
	if len(req.RefImage) > 0 {
		// Image-to-image: the reference guides the visual style.
		err = p.postImageEdit(ctx, req, size, &parsed)
	} else {
		body := map[string]any{"model": p.model, "prompt": req.Prompt, "size": size, "n": 1}
		err = openAIPostJSON(ctx, p.baseURL, p.apiKey, "/images/generations", body, &parsed)
	}
	if err != nil {
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

// postImageEdit calls /images/edits with the reference image as multipart form data.
func (p openAIImageProvider) postImageEdit(ctx context.Context, req ImageRequest, size string, out any) error {
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	fields := map[string]string{"model": p.model, "prompt": req.Prompt, "size": size, "n": "1"}
	for k, v := range fields {
		if err := mw.WriteField(k, v); err != nil {
			return err
		}
	}
	refMime := strings.TrimSpace(req.RefImageMime)
	if refMime == "" {
		refMime = "image/png"
	}
	filename := "reference" + extensionForMime(refMime)
	header := make(textproto.MIMEHeader)
	header.Set("Content-Disposition", fmt.Sprintf(`form-data; name="image"; filename=%q`, filename))
	header.Set("Content-Type", refMime)
	part, err := mw.CreatePart(header)
	if err != nil {
		return err
	}
	if _, err := part.Write(req.RefImage); err != nil {
		return err
	}
	if err := mw.Close(); err != nil {
		return err
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, p.baseURL+"/images/edits", &buf)
	if err != nil {
		return err
	}
	httpReq.Header.Set("Content-Type", mw.FormDataContentType())
	httpReq.Header.Set("Authorization", "Bearer "+p.apiKey)

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

func extensionForMime(mime string) string {
	switch strings.ToLower(strings.TrimSpace(mime)) {
	case "image/jpeg", "image/jpg":
		return ".jpg"
	case "image/webp":
		return ".webp"
	default:
		return ".png"
	}
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
