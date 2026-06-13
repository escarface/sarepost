package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	generationapp "github.com/escarface/sarepost/internal/application/generation"
	"github.com/escarface/sarepost/internal/db"
	"github.com/escarface/sarepost/internal/domain"
)

// handleGenerateText generates post copy and returns it for preview (JSON only).
func (s Server) handleGenerateText(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Prompt         string `json:"prompt"`
		Platform       string `json:"platform"`
		BrandProfileID string `json:"brand_profile_id"`
		MaxTokens      int    `json:"max_tokens"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, fmt.Errorf("invalid json body: %w", err))
		return
	}
	out, err := s.generationService().GenerateText(r.Context(), generationapp.GenerateTextInput{
		Prompt:         body.Prompt,
		Platform:       domain.Platform(strings.TrimSpace(strings.ToLower(body.Platform))),
		BrandProfileID: body.BrandProfileID,
		MaxTokens:      body.MaxTokens,
	})
	if err != nil {
		writeError(w, generationErrorStatus(err), err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"text":     out.Text,
		"model":    out.Model,
		"provider": out.Provider,
	})
}

// handleGenerateImage generates an image, persists it as media, and returns the
// media id and preview URL (JSON only) ready to attach to a post.
func (s Server) handleGenerateImage(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Prompt         string `json:"prompt"`
		BrandProfileID string `json:"brand_profile_id"`
		Size           string `json:"size"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, fmt.Errorf("invalid json body: %w", err))
		return
	}
	out, err := s.generationService().GenerateImage(r.Context(), generationapp.GenerateImageInput{
		Prompt:         body.Prompt,
		BrandProfileID: body.BrandProfileID,
		Size:           body.Size,
	})
	if err != nil {
		writeError(w, generationErrorStatus(err), err)
		return
	}
	media, err := s.persistGeneratedImage(r.Context(), out.Data, out.MimeType)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{
		"media_id":    media.ID,
		"preview_url": mediaContentURL(media.ID),
		"model":       out.Model,
		"provider":    out.Provider,
	})
}

// handleSaveGenerationProvider persists the text or image provider config.
// The kind is taken from the trailing path segment (.../text or .../image).
func (s Server) handleSaveGenerationProvider(w http.ResponseWriter, r *http.Request) {
	kind := strings.TrimPrefix(r.URL.Path, "/settings/generation/")
	kind = strings.Trim(kind, "/")
	update, returnTo, fromForm, err := parseProviderConfigUpdate(r)
	if err != nil {
		s.respondGenerationError(w, r, fromForm, returnTo, http.StatusBadRequest, err)
		return
	}
	svc := s.generationService()
	var view generationapp.ProviderConfigView
	switch kind {
	case "text":
		view, err = svc.SaveTextProviderConfig(r.Context(), update)
	case "image":
		view, err = svc.SaveImageProviderConfig(r.Context(), update)
	default:
		s.respondGenerationError(w, r, fromForm, returnTo, http.StatusNotFound, errors.New("unknown provider kind"))
		return
	}
	if err != nil {
		s.respondGenerationError(w, r, fromForm, returnTo, http.StatusBadRequest, err)
		return
	}
	if fromForm {
		http.Redirect(w, r, withQueryValue(returnTo, "gen_success", "generation settings saved"), http.StatusSeeOther)
		return
	}
	writeJSON(w, http.StatusOK, view)
}

// handleSaveBrandProfile creates or updates a brand profile.
func (s Server) handleSaveBrandProfile(w http.ResponseWriter, r *http.Request) {
	fromForm := requestIsForm(r)
	returnTo := "/?view=settings"
	if fromForm {
		if err := r.ParseForm(); err != nil {
			s.respondGenerationError(w, r, true, returnTo, http.StatusBadRequest, fmt.Errorf("invalid form: %w", err))
			return
		}
		returnTo = sanitizedReturnToOrDefault(r.FormValue("return_to"), returnTo)
	}
	var update generationapp.BrandProfileUpdate
	if fromForm {
		update = generationapp.BrandProfileUpdate{
			ID:           r.FormValue("id"),
			Name:         r.FormValue("name"),
			SystemPrompt: r.FormValue("system_prompt"),
			Tone:         r.FormValue("tone"),
			ImageStyle:   r.FormValue("image_style"),
		}
	} else {
		var body struct {
			ID           string `json:"id"`
			Name         string `json:"name"`
			SystemPrompt string `json:"system_prompt"`
			Tone         string `json:"tone"`
			ImageStyle   string `json:"image_style"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeError(w, http.StatusBadRequest, fmt.Errorf("invalid json body: %w", err))
			return
		}
		update = generationapp.BrandProfileUpdate(body)
	}
	profile, err := s.generationService().SaveBrandProfile(r.Context(), update)
	if err != nil {
		s.respondGenerationError(w, r, fromForm, returnTo, http.StatusBadRequest, err)
		return
	}
	if fromForm {
		http.Redirect(w, r, withQueryValue(returnTo, "gen_success", "brand profile saved"), http.StatusSeeOther)
		return
	}
	writeJSON(w, http.StatusOK, profile)
}

// handleDeleteBrandProfile removes a brand profile.
func (s Server) handleDeleteBrandProfile(w http.ResponseWriter, r *http.Request) {
	fromForm := requestIsForm(r)
	returnTo := "/?view=settings"
	id := ""
	if fromForm {
		if err := r.ParseForm(); err != nil {
			s.respondGenerationError(w, r, true, returnTo, http.StatusBadRequest, fmt.Errorf("invalid form: %w", err))
			return
		}
		returnTo = sanitizedReturnToOrDefault(r.FormValue("return_to"), returnTo)
		id = r.FormValue("id")
	} else {
		var body struct {
			ID string `json:"id"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeError(w, http.StatusBadRequest, fmt.Errorf("invalid json body: %w", err))
			return
		}
		id = body.ID
	}
	if err := s.generationService().DeleteBrandProfile(r.Context(), id); err != nil {
		s.respondGenerationError(w, r, fromForm, returnTo, http.StatusBadRequest, err)
		return
	}
	if fromForm {
		http.Redirect(w, r, withQueryValue(returnTo, "gen_success", "brand profile deleted"), http.StatusSeeOther)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"deleted": true})
}

func parseProviderConfigUpdate(r *http.Request) (generationapp.ProviderConfigUpdate, string, bool, error) {
	fromForm := requestIsForm(r)
	returnTo := "/?view=settings"
	if fromForm {
		if err := r.ParseForm(); err != nil {
			return generationapp.ProviderConfigUpdate{}, returnTo, true, fmt.Errorf("invalid form: %w", err)
		}
		returnTo = sanitizedReturnToOrDefault(r.FormValue("return_to"), returnTo)
		return generationapp.ProviderConfigUpdate{
			Provider: r.FormValue("provider"),
			Model:    r.FormValue("model"),
			APIKey:   r.FormValue("api_key"),
			KeepKey:  true,
		}, returnTo, true, nil
	}
	var body struct {
		Provider string `json:"provider"`
		Model    string `json:"model"`
		APIKey   string `json:"api_key"`
		KeepKey  bool   `json:"keep_key"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		return generationapp.ProviderConfigUpdate{}, returnTo, false, fmt.Errorf("invalid json body: %w", err)
	}
	return generationapp.ProviderConfigUpdate(body), returnTo, false, nil
}

func (s Server) persistGeneratedImage(ctx context.Context, data []byte, mimeType string) (domain.Media, error) {
	if len(data) == 0 {
		return domain.Media{}, errors.New("generated image is empty")
	}
	mediaID, err := db.NewID("med")
	if err != nil {
		return domain.Media{}, err
	}
	storageDir := filepath.Join(s.DataDir, "media")
	if err := os.MkdirAll(storageDir, 0o755); err != nil {
		return domain.Media{}, err
	}
	ext := ".png"
	if mimeType == "image/jpeg" {
		ext = ".jpg"
	}
	storagePath := filepath.Join(storageDir, mediaID+"_generated"+ext)
	if err := os.WriteFile(storagePath, data, 0o644); err != nil {
		return domain.Media{}, err
	}
	media, err := s.Store.CreateMedia(ctx, domain.Media{
		ID:           mediaID,
		Kind:         "image",
		OriginalName: "generated" + ext,
		StoragePath:  storagePath,
		MimeType:     mimeType,
		SizeBytes:    int64(len(data)),
	})
	if err != nil {
		_ = removeFileQuiet(storagePath)
		return domain.Media{}, err
	}
	return media, nil
}

func (s Server) respondGenerationError(w http.ResponseWriter, r *http.Request, fromForm bool, returnTo string, code int, err error) {
	if fromForm {
		http.Redirect(w, r, withQueryValue(returnTo, "gen_error", err.Error()), http.StatusSeeOther)
		return
	}
	writeError(w, code, err)
}

func generationErrorStatus(err error) int {
	if errors.Is(err, generationapp.ErrProviderNotConfigured) {
		return http.StatusBadRequest
	}
	return http.StatusBadGateway
}

func requestIsForm(r *http.Request) bool {
	contentType := strings.ToLower(r.Header.Get("content-type"))
	return strings.Contains(contentType, "application/x-www-form-urlencoded") || strings.Contains(contentType, "multipart/form-data")
}

func sanitizedReturnToOrDefault(raw, fallback string) string {
	if rt := sanitizeReturnTo(strings.TrimSpace(raw)); rt != "" {
		return rt
	}
	return fallback
}
