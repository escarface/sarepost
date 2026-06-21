package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	contentplansapp "github.com/escarface/sarepost/internal/application/contentplans"
)

type contentPlanRequest struct {
	Name      string                    `json:"name"`
	Objective string                    `json:"objective"`
	From      string                    `json:"from"`
	To        string                    `json:"to"`
	Timezone  string                    `json:"timezone"`
	Blocks    []contentPlanBlockRequest `json:"blocks"`
}

type contentPlanBlockRequest struct {
	BrandProfileID string   `json:"brand_profile_id"`
	CampaignID     string   `json:"campaign_id"`
	AccountIDs     []string `json:"account_ids"`
	Instructions   string   `json:"instructions"`
	Weekdays       []string `json:"weekdays"`
	Slots          []string `json:"slots"`
	GenerateImages bool     `json:"generate_images"`
	ImagePrompt    string   `json:"image_prompt"`
	ForceWebSearch bool     `json:"force_web_search"`
}

func (s Server) contentPlanService() contentplansapp.Service {
	return contentplansapp.Service{
		Store: s.Store, Profiles: s.generationService(),
		Scheduler: contentplansapp.PostScheduler{Store: s.Store, Registry: s.providerRegistry(), DefaultMaxRetries: s.DefaultMaxRetries},
	}
}

func (s Server) handlePreviewContentPlan(w http.ResponseWriter, r *http.Request) {
	in, err := decodeContentPlanRequest(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	preview, err := s.contentPlanService().Preview(r.Context(), in)
	if err != nil {
		writeContentPlanError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, preview)
}

func (s Server) handleCreateContentPlan(w http.ResponseWriter, r *http.Request) {
	in, err := decodeContentPlanRequest(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	plan, _, err := s.contentPlanService().Create(r.Context(), in)
	if err != nil {
		writeContentPlanError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, plan)
}

func (s Server) handleGetContentPlan(w http.ResponseWriter, r *http.Request) {
	plan, err := s.Store.GetContentPlan(r.Context(), strings.TrimSpace(r.PathValue("id")))
	if err != nil {
		writeError(w, http.StatusNotFound, err)
		return
	}
	writeJSON(w, http.StatusOK, plan)
}

func (s Server) handleUpdateContentPlan(w http.ResponseWriter, r *http.Request) {
	in, err := decodeContentPlanRequest(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	plan, _, err := s.contentPlanService().Update(r.Context(), r.PathValue("id"), in)
	if err != nil {
		writeContentPlanError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, plan)
}

func (s Server) handleListContentPlans(w http.ResponseWriter, r *http.Request) {
	plans, err := s.Store.ListContentPlans(r.Context(), 100)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": plans})
}

func (s Server) handleGenerateContentPlan(w http.ResponseWriter, r *http.Request) {
	job, err := s.contentPlanService().Generate(r.Context(), strings.TrimSpace(r.PathValue("id")))
	if err != nil {
		writeContentPlanError(w, err)
		return
	}
	writeJSON(w, http.StatusAccepted, job)
}

func (s Server) handleCancelContentPlan(w http.ResponseWriter, r *http.Request) {
	if err := s.contentPlanService().Cancel(r.Context(), r.PathValue("id")); err != nil {
		writeContentPlanError(w, err)
		return
	}
	plan, _ := s.Store.GetContentPlan(r.Context(), r.PathValue("id"))
	writeJSON(w, http.StatusOK, plan)
}

func (s Server) handleRetryContentPlan(w http.ResponseWriter, r *http.Request) {
	var req struct {
		VariantIDs []string `json:"variant_ids"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, fmt.Errorf("invalid json body: %w", err))
		return
	}
	job, err := s.contentPlanService().Retry(r.Context(), r.PathValue("id"), req.VariantIDs)
	if err != nil {
		writeContentPlanError(w, err)
		return
	}
	writeJSON(w, http.StatusAccepted, job)
}

func (s Server) handleRegenerateContentPlan(w http.ResponseWriter, r *http.Request) {
	var req struct {
		VariantIDs []string `json:"variant_ids"`
		ItemIDs    []string `json:"item_ids"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, fmt.Errorf("invalid json body: %w", err))
		return
	}
	job, err := s.contentPlanService().Regenerate(r.Context(), r.PathValue("id"), req.VariantIDs, req.ItemIDs)
	if err != nil {
		writeContentPlanError(w, err)
		return
	}
	writeJSON(w, http.StatusAccepted, job)
}

func (s Server) handleScheduleContentPlan(w http.ResponseWriter, r *http.Request) {
	var req struct {
		VariantIDs []string `json:"variant_ids"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, fmt.Errorf("invalid json body: %w", err))
		return
	}
	result, err := s.contentPlanService().Schedule(r.Context(), r.PathValue("id"), req.VariantIDs)
	if err != nil {
		writeContentPlanError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (s Server) handleUpdateContentPlanVariant(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Text      string `json:"text"`
		PlannedAt string `json:"planned_at"`
		MediaID   string `json:"media_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, fmt.Errorf("invalid json body: %w", err))
		return
	}
	plannedAt, err := time.Parse(time.RFC3339, strings.TrimSpace(req.PlannedAt))
	if err != nil {
		writeError(w, http.StatusBadRequest, errors.New("planned_at must be RFC3339"))
		return
	}
	if err := s.contentPlanService().UpdateVariant(r.Context(), r.PathValue("id"), r.PathValue("variant_id"), req.Text, plannedAt, req.MediaID); err != nil {
		writeContentPlanError(w, err)
		return
	}
	plan, err := s.Store.GetContentPlan(r.Context(), r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, plan)
}

func decodeContentPlanRequest(r *http.Request) (contentplansapp.CreateInput, error) {
	var req contentPlanRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return contentplansapp.CreateInput{}, fmt.Errorf("invalid json body: %w", err)
	}
	from, err := time.Parse(time.RFC3339, strings.TrimSpace(req.From))
	if err != nil {
		return contentplansapp.CreateInput{}, errors.New("from must be RFC3339")
	}
	to, err := time.Parse(time.RFC3339, strings.TrimSpace(req.To))
	if err != nil {
		return contentplansapp.CreateInput{}, errors.New("to must be RFC3339")
	}
	in := contentplansapp.CreateInput{Name: req.Name, Objective: req.Objective, From: from, To: to, Timezone: req.Timezone}
	for index, block := range req.Blocks {
		weekdays := make([]time.Weekday, 0, len(block.Weekdays))
		for _, raw := range block.Weekdays {
			weekday, ok := parseContentPlanWeekday(raw)
			if !ok {
				return contentplansapp.CreateInput{}, fmt.Errorf("block %d has invalid weekday %q", index+1, raw)
			}
			weekdays = append(weekdays, weekday)
		}
		in.Blocks = append(in.Blocks, contentplansapp.BlockInput{
			BrandProfileID: block.BrandProfileID, CampaignID: block.CampaignID,
			AccountIDs: block.AccountIDs, Instructions: block.Instructions,
			Weekdays: weekdays, Slots: block.Slots, GenerateImages: block.GenerateImages,
			ImagePrompt: block.ImagePrompt, ForceWebSearch: block.ForceWebSearch,
		})
	}
	return in, nil
}

func parseContentPlanWeekday(raw string) (time.Weekday, bool) {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "sunday":
		return time.Sunday, true
	case "monday":
		return time.Monday, true
	case "tuesday":
		return time.Tuesday, true
	case "wednesday":
		return time.Wednesday, true
	case "thursday":
		return time.Thursday, true
	case "friday":
		return time.Friday, true
	case "saturday":
		return time.Saturday, true
	default:
		return 0, false
	}
}

func writeContentPlanError(w http.ResponseWriter, err error) {
	status := http.StatusBadRequest
	if errors.Is(err, contentplansapp.ErrPlanNotDraft) || errors.Is(err, contentplansapp.ErrPlanNotReviewable) {
		status = http.StatusConflict
	}
	writeError(w, status, err)
}
