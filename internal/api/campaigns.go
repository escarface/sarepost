package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	campaignsapp "github.com/escarface/sarepost/internal/application/campaigns"
	"github.com/escarface/sarepost/internal/domain"
)

type campaignRequest struct {
	Name             string   `json:"name"`
	Objective        string   `json:"objective"`
	Status           string   `json:"status"`
	StartsAt         string   `json:"starts_at"`
	EndsAt           string   `json:"ends_at"`
	Notes            string   `json:"notes"`
	Tags             []string `json:"tags"`
	Timezone         string   `json:"timezone"`
	Audience         string   `json:"audience"`
	Tone             string   `json:"tone"`
	CTA              string   `json:"cta"`
	Restrictions     string   `json:"restrictions"`
	BrandProfileID   string   `json:"brand_profile_id"`
	BrandProfileName string   `json:"brand_profile"`
}

func (s Server) handleCreateCampaign(w http.ResponseWriter, r *http.Request) {
	req, fromForm, err := parseCampaignRequest(r)
	if err != nil {
		if fromForm {
			redirectCampaignForm(w, r, "", err.Error())
			return
		}
		writeError(w, http.StatusBadRequest, fmt.Errorf("invalid json body: %w", err))
		return
	}
	startsAt, err := parseOptionalRFC3339(req.StartsAt, "starts_at")
	if err != nil {
		if fromForm {
			redirectCampaignForm(w, r, "", err.Error())
			return
		}
		writeError(w, http.StatusBadRequest, err)
		return
	}
	endsAt, err := parseOptionalRFC3339(req.EndsAt, "ends_at")
	if err != nil {
		if fromForm {
			redirectCampaignForm(w, r, "", err.Error())
			return
		}
		writeError(w, http.StatusBadRequest, err)
		return
	}
	brandProfileID, err := s.resolveBrandProfileID(r.Context(), req.BrandProfileID, req.BrandProfileName)
	if err != nil {
		if fromForm {
			redirectCampaignForm(w, r, "", err.Error())
			return
		}
		writeError(w, http.StatusBadRequest, err)
		return
	}
	svc := campaignsapp.Service{Store: s.Store}
	campaign, err := svc.Create(r.Context(), campaignsapp.CreateInput{
		Name:           req.Name,
		Objective:      req.Objective,
		StartsAt:       startsAt,
		EndsAt:         endsAt,
		Notes:          req.Notes,
		Tags:           req.Tags,
		Timezone:       req.Timezone,
		Audience:       req.Audience,
		Tone:           req.Tone,
		CTA:            req.CTA,
		Restrictions:   req.Restrictions,
		BrandProfileID: brandProfileID,
	})
	if err != nil {
		if fromForm {
			redirectCampaignForm(w, r, "", err.Error())
			return
		}
		writeCampaignError(w, err)
		return
	}
	if fromForm {
		redirectCampaignForm(w, r, "campaign created", "")
		return
	}
	writeJSON(w, http.StatusCreated, campaign)
}

func (s Server) handleListCampaigns(w http.ResponseWriter, r *http.Request) {
	limit, err := parsePositiveLimit(r.URL.Query().Get("limit"), defaultMCPListLimit)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	filter := campaignsapp.ListFilter{
		Status: domain.CampaignStatus(strings.TrimSpace(r.URL.Query().Get("status"))),
		Tag:    strings.TrimSpace(r.URL.Query().Get("tag")),
		Limit:  limit,
	}
	svc := campaignsapp.Service{Store: s.Store}
	items, err := svc.List(r.Context(), filter)
	if err != nil {
		writeCampaignError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"count": len(items), "items": items})
}

func (s Server) handleCampaignActions(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/campaigns/")
	switch {
	case strings.HasSuffix(path, "/archive"):
		s.handleArchiveCampaign(w, r, strings.TrimSuffix(path, "/archive"))
	case strings.HasSuffix(path, "/drafts"):
		s.handleCreateCampaignDrafts(w, r, strings.TrimSuffix(path, "/drafts"))
	case strings.HasSuffix(path, "/calendar-drafts"):
		s.handleCreateCampaignCalendarDrafts(w, r, strings.TrimSuffix(path, "/calendar-drafts"))
	case strings.HasSuffix(path, "/posts"):
		s.handleAddCampaignPost(w, r, strings.TrimSuffix(path, "/posts"))
	default:
		s.handleUpdateCampaign(w, r, strings.TrimSuffix(path, "/"))
	}
}

func (s Server) handleUpdateCampaign(w http.ResponseWriter, r *http.Request, id string) {
	var req campaignRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, fmt.Errorf("invalid json body: %w", err))
		return
	}
	startsAt, err := parseOptionalRFC3339(req.StartsAt, "starts_at")
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	endsAt, err := parseOptionalRFC3339(req.EndsAt, "ends_at")
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	brandProfileID, err := s.resolveBrandProfileID(r.Context(), req.BrandProfileID, req.BrandProfileName)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	svc := campaignsapp.Service{Store: s.Store}
	campaign, err := svc.Update(r.Context(), campaignsapp.UpdateInput{
		ID:             id,
		Name:           req.Name,
		Objective:      req.Objective,
		Status:         domain.CampaignStatus(strings.TrimSpace(req.Status)),
		StartsAt:       startsAt,
		EndsAt:         endsAt,
		Notes:          req.Notes,
		Tags:           req.Tags,
		Timezone:       req.Timezone,
		Audience:       req.Audience,
		Tone:           req.Tone,
		CTA:            req.CTA,
		Restrictions:   req.Restrictions,
		BrandProfileID: brandProfileID,
	})
	if err != nil {
		writeCampaignError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, campaign)
}

func (s Server) handleArchiveCampaign(w http.ResponseWriter, r *http.Request, id string) {
	svc := campaignsapp.Service{Store: s.Store}
	campaign, err := svc.Archive(r.Context(), id)
	if err != nil {
		if isFormRequest(r) {
			redirectCampaignForm(w, r, "", err.Error())
			return
		}
		writeCampaignError(w, err)
		return
	}
	if isFormRequest(r) {
		redirectCampaignForm(w, r, "campaign archived", "")
		return
	}
	writeJSON(w, http.StatusOK, campaign)
}

func (s Server) handleAddCampaignPost(w http.ResponseWriter, r *http.Request, campaignID string) {
	var req struct {
		PostID           string   `json:"post_id"`
		EditorialStatus  string   `json:"editorial_status"`
		RequiresApproval bool     `json:"requires_approval"`
		Tags             []string `json:"tags"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, fmt.Errorf("invalid json body: %w", err))
		return
	}
	status := domain.EditorialStatus(strings.TrimSpace(req.EditorialStatus))
	if status == "" {
		status = domain.EditorialStatusDrafting
	}
	if err := s.Store.AddPostToCampaign(r.Context(), req.PostID, campaignID, status, req.RequiresApproval, req.Tags); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	post, err := s.Store.GetPost(r.Context(), req.PostID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"post": post})
}

func (s Server) handleCreateCampaignDrafts(w http.ResponseWriter, r *http.Request, campaignID string) {
	var req struct {
		AccountID         string   `json:"account_id"`
		AccountIDs        []string `json:"account_ids"`
		Idea              string   `json:"idea"`
		VariantsPerPost   int      `json:"variants_per_post"`
		BrandProfileID    string   `json:"brand_profile_id"`
		BrandProfileName  string   `json:"brand_profile"`
		EditorialStatus   string   `json:"editorial_status"`
		RequiresApproval  *bool    `json:"requires_approval"`
		Tags              []string `json:"tags"`
		IdempotencyPrefix string   `json:"idempotency_prefix"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, fmt.Errorf("invalid json body: %w", err))
		return
	}
	accountIDs := append([]string(nil), req.AccountIDs...)
	if strings.TrimSpace(req.AccountID) != "" {
		accountIDs = append(accountIDs, strings.TrimSpace(req.AccountID))
	}
	brandProfileID, err := s.resolveBrandProfileID(r.Context(), req.BrandProfileID, req.BrandProfileName)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	out, err := campaignsapp.DraftService{
		Store:             s.Store,
		Generator:         s.generationService(),
		Registry:          s.providerRegistry(),
		DefaultMaxRetries: s.DefaultMaxRetries,
	}.CreateDrafts(r.Context(), campaignsapp.CreateDraftsInput{
		CampaignID:        campaignID,
		AccountIDs:        accountIDs,
		Idea:              req.Idea,
		VariantsPerPost:   req.VariantsPerPost,
		BrandProfileID:    brandProfileID,
		EditorialStatus:   domain.EditorialStatus(strings.TrimSpace(req.EditorialStatus)),
		RequiresApproval:  req.RequiresApproval,
		Tags:              req.Tags,
		IdempotencyPrefix: req.IdempotencyPrefix,
	})
	if err != nil {
		writeCampaignDraftError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, out)
}

func (s Server) handleCreateCampaignCalendarDrafts(w http.ResponseWriter, r *http.Request, campaignID string) {
	var req struct {
		AccountID         string   `json:"account_id"`
		AccountIDs        []string `json:"account_ids"`
		Idea              string   `json:"idea"`
		From              string   `json:"from"`
		Days              int      `json:"days"`
		PostsPerDay       int      `json:"posts_per_day"`
		Slots             []string `json:"slots"`
		BrandProfileID    string   `json:"brand_profile_id"`
		BrandProfileName  string   `json:"brand_profile"`
		EditorialStatus   string   `json:"editorial_status"`
		RequiresApproval  *bool    `json:"requires_approval"`
		Tags              []string `json:"tags"`
		IdempotencyPrefix string   `json:"idempotency_prefix"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, fmt.Errorf("invalid json body: %w", err))
		return
	}
	from, err := parseOptionalRFC3339PreserveLocation(req.From, "from")
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	accountIDs := append([]string(nil), req.AccountIDs...)
	if strings.TrimSpace(req.AccountID) != "" {
		accountIDs = append(accountIDs, strings.TrimSpace(req.AccountID))
	}
	brandProfileID, err := s.resolveBrandProfileID(r.Context(), req.BrandProfileID, req.BrandProfileName)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	out, err := campaignsapp.DraftService{
		Store:             s.Store,
		Generator:         s.generationService(),
		Registry:          s.providerRegistry(),
		DefaultMaxRetries: s.DefaultMaxRetries,
	}.CreateCalendarDrafts(r.Context(), campaignsapp.CreateCalendarDraftsInput{
		CampaignID:        campaignID,
		AccountIDs:        accountIDs,
		Idea:              req.Idea,
		From:              from,
		Days:              req.Days,
		PostsPerDay:       req.PostsPerDay,
		Slots:             req.Slots,
		BrandProfileID:    brandProfileID,
		EditorialStatus:   domain.EditorialStatus(strings.TrimSpace(req.EditorialStatus)),
		RequiresApproval:  req.RequiresApproval,
		Tags:              req.Tags,
		IdempotencyPrefix: req.IdempotencyPrefix,
	})
	if err != nil {
		writeCampaignDraftError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, out)
}

func (s Server) handleEditorialBacklog(w http.ResponseWriter, r *http.Request) {
	limit, err := parsePositiveLimit(r.URL.Query().Get("limit"), defaultMCPListLimit)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	from, err := parseOptionalRFC3339(r.URL.Query().Get("from"), "from")
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	to, err := parseOptionalRFC3339(r.URL.Query().Get("to"), "to")
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	items, err := s.Store.ListEditorialBacklog(r.Context(), domain.EditorialBacklogFilter{
		CampaignID:      strings.TrimSpace(r.URL.Query().Get("campaign_id")),
		Platform:        domain.Platform(strings.TrimSpace(r.URL.Query().Get("platform"))),
		EditorialStatus: domain.EditorialStatus(strings.TrimSpace(r.URL.Query().Get("editorial_status"))),
		Tag:             strings.TrimSpace(r.URL.Query().Get("tag")),
		From:            from,
		To:              to,
		Limit:           limit,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"count": len(items), "items": items})
}

func (s Server) handleApprovePost(w http.ResponseWriter, r *http.Request) {
	postID, err := extractPostIDFromPath(r.URL.Path, "approve")
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	if err := s.Store.ApprovePost(r.Context(), postID); err != nil {
		if isFormRequest(r) {
			redirectToReturn(w, r, "", err.Error())
			return
		}
		writeError(w, http.StatusConflict, err)
		return
	}
	post, err := s.Store.GetPost(r.Context(), postID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	if isFormRequest(r) {
		redirectToReturn(w, r, "post approved", "")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"id": postID, "status": string(post.EditorialStatus), "post": post})
}

func parseCampaignRequest(r *http.Request) (campaignRequest, bool, error) {
	ct := strings.ToLower(r.Header.Get("content-type"))
	if !strings.Contains(ct, "application/x-www-form-urlencoded") && !strings.Contains(ct, "multipart/form-data") {
		var req campaignRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			return campaignRequest{}, false, err
		}
		return req, false, nil
	}
	if err := r.ParseForm(); err != nil {
		return campaignRequest{}, true, err
	}
	return campaignRequest{
		Name:             strings.TrimSpace(r.FormValue("name")),
		Objective:        strings.TrimSpace(r.FormValue("objective")),
		Status:           strings.TrimSpace(r.FormValue("status")),
		StartsAt:         strings.TrimSpace(r.FormValue("starts_at")),
		EndsAt:           strings.TrimSpace(r.FormValue("ends_at")),
		Notes:            strings.TrimSpace(r.FormValue("notes")),
		Tags:             splitCSV(r.FormValue("tags")),
		Timezone:         strings.TrimSpace(r.FormValue("timezone")),
		Audience:         strings.TrimSpace(r.FormValue("audience")),
		Tone:             strings.TrimSpace(r.FormValue("tone")),
		CTA:              strings.TrimSpace(r.FormValue("cta")),
		Restrictions:     strings.TrimSpace(r.FormValue("restrictions")),
		BrandProfileID:   strings.TrimSpace(r.FormValue("brand_profile_id")),
		BrandProfileName: strings.TrimSpace(r.FormValue("brand_profile")),
	}, true, nil
}

func (s Server) resolveBrandProfileID(ctx context.Context, id string, name string) (string, error) {
	return s.generationService().ResolveBrandProfileID(ctx, id, name)
}

func isFormRequest(r *http.Request) bool {
	ct := strings.ToLower(r.Header.Get("content-type"))
	return strings.Contains(ct, "application/x-www-form-urlencoded") || strings.Contains(ct, "multipart/form-data")
}

func redirectCampaignForm(w http.ResponseWriter, r *http.Request, successMsg, errorMsg string) {
	redirectToReturn(w, r, successMsg, errorMsg)
}

func redirectToReturn(w http.ResponseWriter, r *http.Request, successMsg, errorMsg string) {
	returnTo := strings.TrimSpace(r.FormValue("return_to"))
	if returnTo == "" {
		returnTo = "/?view=campaigns"
	}
	values := make([]string, 0, 2)
	if strings.Contains(returnTo, "?") {
		values = append(values, returnTo+"&")
	} else {
		values = append(values, returnTo+"?")
	}
	if successMsg != "" {
		values = append(values, "success="+url.QueryEscape(successMsg))
	}
	if errorMsg != "" {
		values = append(values, "error="+url.QueryEscape(errorMsg))
	}
	http.Redirect(w, r, strings.Join(values, ""), http.StatusSeeOther)
}

func writeCampaignError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, campaignsapp.ErrNameRequired), errors.Is(err, campaignsapp.ErrCampaignIDRequired), errors.Is(err, campaignsapp.ErrInvalidStatus), errors.Is(err, campaignsapp.ErrInvalidTimezone):
		writeError(w, http.StatusBadRequest, err)
	case errors.Is(err, campaignsapp.ErrCampaignNotFound):
		writeError(w, http.StatusNotFound, err)
	default:
		writeError(w, http.StatusInternalServerError, err)
	}
}

func writeCampaignDraftError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, campaignsapp.ErrCampaignIDRequired), errors.Is(err, campaignsapp.ErrAccountIDsRequired):
		writeError(w, http.StatusBadRequest, err)
	case errors.Is(err, campaignsapp.ErrCampaignNotFound):
		writeError(w, http.StatusNotFound, err)
	case errors.Is(err, campaignsapp.ErrCampaignArchived):
		writeError(w, http.StatusConflict, err)
	default:
		writeError(w, http.StatusInternalServerError, err)
	}
}

func parseOptionalRFC3339(raw, field string) (time.Time, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return time.Time{}, nil
	}
	parsed, err := time.Parse(time.RFC3339, raw)
	if err != nil {
		return time.Time{}, fmt.Errorf("%s must be RFC3339: %w", field, err)
	}
	return parsed.UTC(), nil
}

func parseOptionalRFC3339PreserveLocation(raw, field string) (time.Time, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return time.Time{}, nil
	}
	parsed, err := time.Parse(time.RFC3339, raw)
	if err != nil {
		return time.Time{}, fmt.Errorf("%s must be RFC3339: %w", field, err)
	}
	return parsed, nil
}

func parsePositiveLimit(raw string, fallback int) (int, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return fallback, nil
	}
	var limit int
	if _, err := fmt.Sscanf(raw, "%d", &limit); err != nil || limit <= 0 {
		return 0, errors.New("limit must be a positive integer")
	}
	return clampMCPListLimit(limit), nil
}

func splitCSV(raw string) []string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		value := strings.TrimSpace(part)
		if value != "" {
			out = append(out, value)
		}
	}
	return out
}
