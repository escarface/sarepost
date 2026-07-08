package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"

	contentsourcesapp "github.com/escarface/sarepost/internal/application/contentsources"
	"github.com/escarface/sarepost/internal/domain"
)

type contentSourceRequest struct {
	Title          string   `json:"title"`
	Body           string   `json:"body"`
	SourceURL      string   `json:"source_url"`
	CampaignID     string   `json:"campaign_id"`
	BrandProfileID string   `json:"brand_profile_id"`
	Tags           []string `json:"tags"`
	Status         string   `json:"status"`
}

func (s Server) contentSourceService() contentsourcesapp.Service {
	return contentsourcesapp.Service{Store: s.Store, Generator: s.generationService()}
}

func (s Server) handleCreateContentSource(w http.ResponseWriter, r *http.Request) {
	var req contentSourceRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, fmt.Errorf("invalid json body: %w", err))
		return
	}
	source, err := s.contentSourceService().Create(r.Context(), contentsourcesapp.CreateInput{
		Title:          req.Title,
		Body:           req.Body,
		SourceURL:      req.SourceURL,
		CampaignID:     req.CampaignID,
		BrandProfileID: req.BrandProfileID,
		Tags:           req.Tags,
	})
	if err != nil {
		writeContentSourceError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, source)
}

func (s Server) handleListContentSources(w http.ResponseWriter, r *http.Request) {
	limit, err := parsePositiveLimit(r.URL.Query().Get("limit"), defaultMCPListLimit)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	filter := domain.ContentSourceListFilter{
		Status:          domain.ContentSourceStatus(strings.TrimSpace(r.URL.Query().Get("status"))),
		IncludeArchived: parseBoolQuery(r.URL.Query().Get("include_archived")),
		Tag:             strings.TrimSpace(r.URL.Query().Get("tag")),
		Limit:           limit,
	}
	items, err := s.contentSourceService().List(r.Context(), filter)
	if err != nil {
		writeContentSourceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"count": len(items), "items": items})
}

func (s Server) handleGetContentSource(w http.ResponseWriter, r *http.Request) {
	source, err := s.contentSourceService().Get(r.Context(), r.PathValue("id"))
	if err != nil {
		writeContentSourceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, source)
}

func (s Server) handleUpdateContentSource(w http.ResponseWriter, r *http.Request) {
	var req contentSourceRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, fmt.Errorf("invalid json body: %w", err))
		return
	}
	source, err := s.contentSourceService().Update(r.Context(), contentsourcesapp.UpdateInput{
		ID:             r.PathValue("id"),
		Title:          req.Title,
		Body:           req.Body,
		SourceURL:      req.SourceURL,
		CampaignID:     req.CampaignID,
		BrandProfileID: req.BrandProfileID,
		Tags:           req.Tags,
		Status:         domain.ContentSourceStatus(strings.TrimSpace(req.Status)),
	})
	if err != nil {
		writeContentSourceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, source)
}

func (s Server) handleArchiveContentSource(w http.ResponseWriter, r *http.Request) {
	source, err := s.contentSourceService().Archive(r.Context(), r.PathValue("id"))
	if err != nil {
		writeContentSourceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, source)
}

func (s Server) handleGenerateContentSourceAngles(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Count        int    `json:"count"`
		Instructions string `json:"instructions"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, fmt.Errorf("invalid json body: %w", err))
		return
	}
	out, err := s.contentSourceService().GenerateAngles(r.Context(), contentsourcesapp.GenerateAnglesInput{
		ID:           r.PathValue("id"),
		Count:        req.Count,
		Instructions: req.Instructions,
	})
	if err != nil {
		writeContentSourceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func writeContentSourceError(w http.ResponseWriter, err error) {
	status := http.StatusBadRequest
	if errors.Is(err, contentsourcesapp.ErrSourceNotFound) {
		status = http.StatusNotFound
	}
	writeError(w, status, err)
}

func parseBoolQuery(raw string) bool {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "1", "true", "yes", "on":
		return true
	default:
		return false
	}
}
