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
	raw, err := p.generateResponse(ctx, req, req.MaxTokens)
	if err != nil {
		return TextResult{}, err
	}
	out := strings.TrimSpace(extractOpenAIResponsesText(raw))
	if out == "" && openAIResponseIncompleteReason(raw) == "max_output_tokens" {
		retryLimit := req.MaxTokens * 2
		if retryLimit < 1200 {
			retryLimit = 1200
		}
		if retryLimit != req.MaxTokens {
			raw, err = p.generateResponse(ctx, req, retryLimit)
			if err != nil {
				return TextResult{}, err
			}
			out = strings.TrimSpace(extractOpenAIResponsesText(raw))
		}
	}
	if out == "" {
		return TextResult{}, fmt.Errorf("openai returned empty response: %s", describeOpenAIResponse(raw))
	}
	return TextResult{
		Text:          out,
		Model:         p.model,
		UsedWebSearch: req.WebSearchRequired,
		Sources:       extractOpenAIResponsesSources(raw),
	}, nil
}

func (p openAITextProvider) generateResponse(ctx context.Context, req TextRequest, maxTokens int) ([]byte, error) {
	body := map[string]any{
		"model": p.model,
		"input": req.Prompt,
	}
	if system := strings.TrimSpace(req.System); system != "" {
		body["instructions"] = system
	}
	if maxTokens > 0 {
		body["max_output_tokens"] = maxTokens
	}
	if effort := openAIReasoningEffort(p.model); effort != "" {
		body["reasoning"] = map[string]any{"effort": effort}
	}
	if req.WebSearchRequired {
		body["tools"] = []map[string]any{{"type": "web_search"}}
		body["tool_choice"] = "required"
	}
	return openAIPostJSONRaw(ctx, p.baseURL, p.apiKey, "/responses", body)
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
	data, err := openAIPostJSONRaw(ctx, baseURL, apiKey, path, body)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(data, out); err != nil {
		return fmt.Errorf("decode openai response: %w", err)
	}
	return nil
}

func openAIPostJSONRaw(ctx context.Context, baseURL, apiKey, path string, body any) ([]byte, error) {
	payload, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, baseURL+path, bytes.NewReader(payload))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+apiKey)

	resp, err := openAIHTTPClient().Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("openai request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	data, err := io.ReadAll(io.LimitReader(resp.Body, 16<<20))
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("openai request failed (%d): %s", resp.StatusCode, strings.TrimSpace(string(data)))
	}
	return data, nil
}

func extractOpenAIResponsesText(raw []byte) string {
	var parsed struct {
		OutputText string `json:"output_text"`
		Output     []struct {
			Content []struct {
				Type string `json:"type"`
				Text string `json:"text"`
			} `json:"content"`
		} `json:"output"`
	}
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return ""
	}
	if strings.TrimSpace(parsed.OutputText) != "" {
		return strings.TrimSpace(parsed.OutputText)
	}
	var b strings.Builder
	for _, item := range parsed.Output {
		for _, content := range item.Content {
			if strings.TrimSpace(content.Text) == "" {
				continue
			}
			if b.Len() > 0 {
				b.WriteString("\n")
			}
			b.WriteString(strings.TrimSpace(content.Text))
		}
	}
	return strings.TrimSpace(b.String())
}

func openAIResponseIncompleteReason(raw []byte) string {
	var parsed struct {
		IncompleteDetails struct {
			Reason string `json:"reason"`
		} `json:"incomplete_details"`
	}
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return ""
	}
	return strings.TrimSpace(parsed.IncompleteDetails.Reason)
}

func openAIReasoningEffort(model string) string {
	model = strings.ToLower(strings.TrimSpace(model))
	switch {
	case strings.HasPrefix(model, "gpt-5"), strings.HasPrefix(model, "o1"), strings.HasPrefix(model, "o3"), strings.HasPrefix(model, "o4"):
		return "minimal"
	default:
		return ""
	}
}

func describeOpenAIResponse(raw []byte) string {
	var parsed struct {
		Status            string `json:"status"`
		Error             any    `json:"error"`
		IncompleteDetails any    `json:"incomplete_details"`
		Output            []struct {
			Type    string `json:"type"`
			Status  string `json:"status"`
			Content []struct {
				Type string `json:"type"`
				Text string `json:"text"`
			} `json:"content"`
		} `json:"output"`
	}
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return "unparseable response"
	}
	itemTypes := make([]string, 0, len(parsed.Output))
	for _, item := range parsed.Output {
		label := strings.TrimSpace(item.Type)
		if status := strings.TrimSpace(item.Status); status != "" {
			label += ":" + status
		}
		itemTypes = append(itemTypes, label)
	}
	return fmt.Sprintf("status=%q output=%q error=%v incomplete_details=%v", parsed.Status, itemTypes, parsed.Error, parsed.IncompleteDetails)
}

func extractOpenAIResponsesSources(raw []byte) []TextSource {
	var parsed struct {
		Output []struct {
			Content []struct {
				Annotations []struct {
					Type  string `json:"type"`
					Title string `json:"title"`
					URL   string `json:"url"`
				} `json:"annotations"`
			} `json:"content"`
		} `json:"output"`
	}
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return nil
	}
	sources := make([]TextSource, 0)
	seen := make(map[string]struct{})
	for _, item := range parsed.Output {
		for _, content := range item.Content {
			for _, annotation := range content.Annotations {
				url := strings.TrimSpace(annotation.URL)
				if url == "" {
					continue
				}
				if _, ok := seen[url]; ok {
					continue
				}
				seen[url] = struct{}{}
				sources = append(sources, TextSource{
					Title: strings.TrimSpace(annotation.Title),
					URL:   url,
				})
			}
		}
	}
	if len(sources) == 0 {
		return nil
	}
	return sources
}
