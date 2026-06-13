package genai

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestOpenAIImageProvider_RoutesByReference(t *testing.T) {
	pngB64 := base64.StdEncoding.EncodeToString([]byte("generated"))

	tests := []struct {
		name     string
		ref      []byte
		wantPath string
	}{
		{name: "no reference uses generations", ref: nil, wantPath: "/images/generations"},
		{name: "reference uses edits", ref: []byte("REFBYTES"), wantPath: "/images/edits"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var gotPath, gotContentType string
			var gotBody []byte
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				gotPath = r.URL.Path
				gotContentType = r.Header.Get("Content-Type")
				gotBody, _ = io.ReadAll(r.Body)
				_ = json.NewEncoder(w).Encode(map[string]any{
					"data": []map[string]string{{"b64_json": pngB64}},
				})
			}))
			defer srv.Close()

			p := newOpenAIImageProvider(ProviderConfig{APIKey: "sk-test", Model: "gpt-image-1", BaseURL: srv.URL})
			res, err := p.GenerateImage(context.Background(), ImageRequest{
				Prompt: "a banner", RefImage: tc.ref, RefImageMime: "image/png",
			})
			if err != nil {
				t.Fatalf("generate image: %v", err)
			}
			if string(res.Data) != "generated" {
				t.Errorf("unexpected image bytes: %q", res.Data)
			}
			if gotPath != tc.wantPath {
				t.Errorf("path = %q, want %q", gotPath, tc.wantPath)
			}
			if tc.ref != nil {
				if !strings.HasPrefix(gotContentType, "multipart/form-data") {
					t.Errorf("expected multipart content-type, got %q", gotContentType)
				}
				if !strings.Contains(string(gotBody), "REFBYTES") {
					t.Errorf("reference bytes not in multipart body")
				}
			} else if gotContentType != "application/json" {
				t.Errorf("expected json content-type, got %q", gotContentType)
			}
		})
	}
}
