package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	generationapp "github.com/escarface/sarepost/internal/application/generation"
	"github.com/escarface/sarepost/internal/db"
	"github.com/escarface/sarepost/internal/domain"
)

func TestContentPlanHTTPFlowPreviewsCreatesAndEnqueues(t *testing.T) {
	tempDir := t.TempDir()
	store, err := db.Open(filepath.Join(tempDir, "plans.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer store.Close()
	srv := Server{Store: store, DataDir: tempDir}
	account := createTestAccount(t, store)
	profile, err := srv.generationService().SaveBrandProfile(t.Context(), generationapp.BrandProfileUpdate{Name: "Sare Digital"})
	if err != nil {
		t.Fatalf("save profile: %v", err)
	}
	body := map[string]any{
		"name": "July plan", "objective": "Educate", "from": "2026-07-06T00:00:00+02:00", "to": "2026-07-12T23:59:00+02:00", "timezone": "Europe/Madrid",
		"blocks": []map[string]any{{"brand_profile_id": profile.ID, "account_ids": []string{account.ID}, "weekdays": []string{"monday", "wednesday"}, "slots": []string{"09:00", "17:00"}, "generate_images": true}},
	}

	preview := performJSONRequest(t, srv, http.MethodPost, "/content-plans/preview", body)
	if preview.Code != http.StatusOK {
		t.Fatalf("preview status=%d body=%s", preview.Code, preview.Body.String())
	}
	var previewOut struct {
		ItemCount    int `json:"item_count"`
		VariantCount int `json:"variant_count"`
		ImageCount   int `json:"image_count"`
	}
	if err := json.Unmarshal(preview.Body.Bytes(), &previewOut); err != nil {
		t.Fatalf("decode preview: %v", err)
	}
	if previewOut.ItemCount != 4 || previewOut.VariantCount != 4 || previewOut.ImageCount != 4 {
		t.Fatalf("unexpected preview: %#v", previewOut)
	}

	created := performJSONRequest(t, srv, http.MethodPost, "/content-plans", body)
	if created.Code != http.StatusCreated {
		t.Fatalf("create status=%d body=%s", created.Code, created.Body.String())
	}
	var plan struct {
		ID     string `json:"id"`
		Status string `json:"status"`
		Items  []any  `json:"items"`
	}
	if err := json.Unmarshal(created.Body.Bytes(), &plan); err != nil {
		t.Fatalf("decode plan: %v", err)
	}
	if plan.ID == "" || plan.Status != "draft" || len(plan.Items) != 4 {
		t.Fatalf("unexpected plan: %#v", plan)
	}

	got := performJSONRequest(t, srv, http.MethodGet, "/content-plans/"+plan.ID, nil)
	if got.Code != http.StatusOK {
		t.Fatalf("get status=%d body=%s", got.Code, got.Body.String())
	}
	queued := performJSONRequest(t, srv, http.MethodPost, "/content-plans/"+plan.ID+"/generate", map[string]any{})
	if queued.Code != http.StatusAccepted {
		t.Fatalf("generate status=%d body=%s", queued.Code, queued.Body.String())
	}
}

func TestContentPlanHTTPUpdatesDraftConfigurationBeforeGeneration(t *testing.T) {
	tempDir := t.TempDir()
	store, err := db.Open(filepath.Join(tempDir, "update-plan.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer store.Close()
	srv := Server{Store: store, DataDir: tempDir}
	account := createTestAccount(t, store)
	profile, err := srv.generationService().SaveBrandProfile(t.Context(), generationapp.BrandProfileUpdate{Name: "Update Brand"})
	if err != nil {
		t.Fatalf("profile: %v", err)
	}
	body := map[string]any{"name": "Original", "from": "2026-07-06T00:00:00Z", "to": "2026-07-06T23:59:00Z", "timezone": "UTC", "blocks": []map[string]any{{"brand_profile_id": profile.ID, "account_ids": []string{account.ID}, "weekdays": []string{"monday"}, "slots": []string{"09:00"}}}}
	created := performJSONRequest(t, srv, http.MethodPost, "/content-plans", body)
	var plan domain.ContentPlan
	_ = json.Unmarshal(created.Body.Bytes(), &plan)
	body["name"] = "Revised"
	body["blocks"].([]map[string]any)[0]["slots"] = []string{"09:00", "17:00"}
	updated := performJSONRequest(t, srv, http.MethodPatch, "/content-plans/"+plan.ID, body)
	if updated.Code != http.StatusOK {
		t.Fatalf("update status=%d body=%s", updated.Code, updated.Body.String())
	}
	var got domain.ContentPlan
	if err := json.Unmarshal(updated.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if got.Name != "Revised" || len(got.Items) != 2 {
		t.Fatalf("unexpected updated plan: %#v", got)
	}
}

func TestContentPlanHTTPEditsAndSchedulesReadyVariant(t *testing.T) {
	tempDir := t.TempDir()
	store, err := db.Open(filepath.Join(tempDir, "schedule-plan.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer store.Close()
	srv := Server{Store: store, DataDir: tempDir, DefaultMaxRetries: 3}
	account := createTestAccount(t, store)
	when := time.Now().UTC().Add(2 * time.Hour).Round(time.Second)
	plan, err := store.CreateContentPlan(t.Context(), domain.ContentPlan{
		Name: "Review", StartsAt: when, EndsAt: when, Status: domain.ContentPlanStatusReview,
		Blocks: []domain.ContentPlanBlock{{ID: "block_http", AccountIDs: []string{account.ID}}},
		Items:  []domain.ContentPlanItem{{ID: "item_http", BlockID: "block_http", PlannedAt: when, Variants: []domain.ContentPlanVariant{{ID: "variant_http", AccountID: account.ID, Text: "Initial", Status: domain.ContentPlanVariantReady, PlannedAt: when}}}},
	})
	if err != nil {
		t.Fatalf("create plan: %v", err)
	}

	edited := performJSONRequest(t, srv, http.MethodPatch, "/content-plans/"+plan.ID+"/variants/variant_http", map[string]any{"text": "Edited post", "planned_at": when.Format(time.RFC3339)})
	if edited.Code != http.StatusOK {
		t.Fatalf("edit status=%d body=%s", edited.Code, edited.Body.String())
	}
	scheduled := performJSONRequest(t, srv, http.MethodPost, "/content-plans/"+plan.ID+"/schedule", map[string]any{"variant_ids": []string{"variant_http"}})
	if scheduled.Code != http.StatusOK {
		t.Fatalf("schedule status=%d body=%s", scheduled.Code, scheduled.Body.String())
	}
	var result struct {
		Scheduled []struct {
			VariantID string      `json:"variant_id"`
			Post      domain.Post `json:"post"`
		} `json:"scheduled"`
		Conflicts []any `json:"conflicts"`
	}
	if err := json.Unmarshal(scheduled.Body.Bytes(), &result); err != nil {
		t.Fatalf("decode result: %v", err)
	}
	if len(result.Scheduled) != 1 || result.Scheduled[0].VariantID != "variant_http" || result.Scheduled[0].Post.Text != "Edited post" || len(result.Conflicts) != 0 {
		t.Fatalf("unexpected schedule result: %#v", result)
	}
	loaded, _ := store.GetContentPlan(t.Context(), plan.ID)
	if loaded.Status != domain.ContentPlanStatusScheduled || loaded.Items[0].Variants[0].PostID == "" {
		t.Fatalf("expected materialized plan: %#v", loaded)
	}
}

func TestContentPlanHTTPRegeneratesWholeEditorialItem(t *testing.T) {
	tempDir := t.TempDir()
	store, err := db.Open(filepath.Join(tempDir, "regenerate-plan.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer store.Close()
	srv := Server{Store: store, DataDir: tempDir}
	account := createTestAccount(t, store)
	when := time.Now().UTC().Add(time.Hour)
	plan, err := store.CreateContentPlan(t.Context(), domain.ContentPlan{Name: "Regenerate", StartsAt: when, EndsAt: when, Status: domain.ContentPlanStatusReview, Blocks: []domain.ContentPlanBlock{{ID: "regen_block", AccountIDs: []string{account.ID}}}, Items: []domain.ContentPlanItem{{ID: "regen_item", BlockID: "regen_block", PlannedAt: when, Idea: "Old angle", Variants: []domain.ContentPlanVariant{{ID: "regen_variant", AccountID: account.ID, Text: "Old copy", Status: domain.ContentPlanVariantFailed}}}}})
	if err != nil {
		t.Fatalf("create plan: %v", err)
	}
	w := performJSONRequest(t, srv, http.MethodPost, "/content-plans/"+plan.ID+"/regenerate", map[string]any{"item_ids": []string{"regen_item"}})
	if w.Code != http.StatusAccepted {
		t.Fatalf("regenerate status=%d body=%s", w.Code, w.Body.String())
	}
	loaded, _ := store.GetContentPlan(t.Context(), plan.ID)
	if loaded.Status != domain.ContentPlanStatusQueued || loaded.Items[0].Idea != "" || loaded.Items[0].Variants[0].Status != domain.ContentPlanVariantPending {
		t.Fatalf("unexpected regenerated plan: %#v", loaded)
	}
}

func TestContentPlanHTTPSchedulesValidVariantsAndKeepsConflictsReviewable(t *testing.T) {
	tempDir := t.TempDir()
	store, err := db.Open(filepath.Join(tempDir, "partial-plan.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer store.Close()
	srv := Server{Store: store, DataDir: tempDir, DefaultMaxRetries: 3}
	account := createTestAccount(t, store)
	when := time.Now().UTC().Add(6 * time.Hour).Round(time.Second)
	plan, err := store.CreateContentPlan(t.Context(), domain.ContentPlan{Name: "Partial", StartsAt: when, EndsAt: when, Status: domain.ContentPlanStatusReview, Blocks: []domain.ContentPlanBlock{{ID: "partial_block", AccountIDs: []string{account.ID}}}, Items: []domain.ContentPlanItem{
		{ID: "partial_item_a", BlockID: "partial_block", PlannedAt: when, Variants: []domain.ContentPlanVariant{{ID: "partial_variant_a", AccountID: account.ID, Text: "First", Status: domain.ContentPlanVariantReady, PlannedAt: when}}},
		{ID: "partial_item_b", BlockID: "partial_block", PlannedAt: when.Add(2 * time.Minute), Variants: []domain.ContentPlanVariant{{ID: "partial_variant_b", AccountID: account.ID, Text: "Second", Status: domain.ContentPlanVariantReady, PlannedAt: when.Add(2 * time.Minute)}}},
	}})
	if err != nil {
		t.Fatalf("create plan: %v", err)
	}
	w := performJSONRequest(t, srv, http.MethodPost, "/content-plans/"+plan.ID+"/schedule", map[string]any{"variant_ids": []string{"partial_variant_a", "partial_variant_b"}})
	if w.Code != http.StatusOK {
		t.Fatalf("schedule status=%d body=%s", w.Code, w.Body.String())
	}
	var result struct {
		Scheduled []any `json:"scheduled"`
		Conflicts []struct {
			VariantID string `json:"variant_id"`
		} `json:"conflicts"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(result.Scheduled) != 1 || len(result.Conflicts) != 1 || result.Conflicts[0].VariantID != "partial_variant_b" {
		t.Fatalf("unexpected partial result: %#v", result)
	}
	loaded, _ := store.GetContentPlan(t.Context(), plan.ID)
	if loaded.Status != domain.ContentPlanStatusPartiallyScheduled || loaded.Items[1].Variants[0].Status != domain.ContentPlanVariantReady {
		t.Fatalf("conflict was not kept reviewable: %#v", loaded)
	}
}

func TestMCPContentPlanToolsCoverLifecycle(t *testing.T) {
	tempDir := t.TempDir()
	store, err := db.Open(filepath.Join(tempDir, "mcp-plans.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer store.Close()
	srv := Server{Store: store, DataDir: tempDir, DefaultMaxRetries: 3}
	account := createTestAccount(t, store)
	profile, err := srv.generationService().SaveBrandProfile(t.Context(), generationapp.BrandProfileUpdate{Name: "MCP Brand"})
	if err != nil {
		t.Fatalf("profile: %v", err)
	}
	input := mcpContentPlanInput{Name: "MCP plan", Objective: "Parity", From: "2026-07-06T00:00:00Z", To: "2026-07-06T23:59:00Z", Timezone: "UTC", Blocks: []mcpContentPlanBlockInput{{BrandProfileID: profile.ID, AccountIDs: []string{account.ID}, Weekdays: []string{"monday"}, Slots: []string{"09:00"}}}}
	if _, out, err := srv.mcpPreviewContentPlanTool(t.Context(), nil, input); err != nil || out.Preview.VariantCount != 1 {
		t.Fatalf("preview out=%#v err=%v", out, err)
	}
	_, created, err := srv.mcpCreateContentPlanTool(t.Context(), nil, input)
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	input.Name = "MCP revised"
	if _, updated, err := srv.mcpUpdateContentPlanTool(t.Context(), nil, mcpContentPlanUpdateInput{PlanID: created.Plan.ID, mcpContentPlanInput: input}); err != nil || updated.Plan.Name != "MCP revised" {
		t.Fatalf("update=%#v err=%v", updated, err)
	}
	if _, listed, err := srv.mcpListContentPlansTool(t.Context(), nil, mcpListContentPlansInput{Limit: 10}); err != nil || listed.Count != 1 {
		t.Fatalf("list=%#v err=%v", listed, err)
	}
	if _, got, err := srv.mcpGetContentPlanTool(t.Context(), nil, mcpContentPlanIDInput{PlanID: created.Plan.ID}); err != nil || got.Plan.ID != created.Plan.ID {
		t.Fatalf("get=%#v err=%v", got, err)
	}
	if _, job, err := srv.mcpGenerateContentPlanTool(t.Context(), nil, mcpContentPlanIDInput{PlanID: created.Plan.ID}); err != nil || job.Job.ID == "" {
		t.Fatalf("generate=%#v err=%v", job, err)
	}
	if _, canceled, err := srv.mcpCancelContentPlanTool(t.Context(), nil, mcpContentPlanIDInput{PlanID: created.Plan.ID}); err != nil || canceled.Plan.Status != domain.ContentPlanStatusCanceled {
		t.Fatalf("cancel=%#v err=%v", canceled, err)
	}
}

func TestMCPContentPlanToolsEditRetryAndSchedule(t *testing.T) {
	tempDir := t.TempDir()
	store, err := db.Open(filepath.Join(tempDir, "mcp-editorial-plan.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer store.Close()
	srv := Server{Store: store, DataDir: tempDir, DefaultMaxRetries: 3}
	account := createTestAccount(t, store)
	when := time.Now().UTC().Add(4 * time.Hour).Round(time.Second)
	plan, err := store.CreateContentPlan(t.Context(), domain.ContentPlan{Name: "MCP editorial", StartsAt: when, EndsAt: when, Status: domain.ContentPlanStatusReview, Blocks: []domain.ContentPlanBlock{{ID: "mcp_block", AccountIDs: []string{account.ID}}}, Items: []domain.ContentPlanItem{{ID: "mcp_item", BlockID: "mcp_block", PlannedAt: when, Variants: []domain.ContentPlanVariant{{ID: "mcp_ready", AccountID: account.ID, Text: "Ready", Status: domain.ContentPlanVariantReady, PlannedAt: when}, {ID: "mcp_failed", AccountID: account.ID, Text: "", Status: domain.ContentPlanVariantFailed, PlannedAt: when.Add(time.Hour)}}}}})
	if err != nil {
		t.Fatalf("create plan: %v", err)
	}
	if _, edited, err := srv.mcpUpdateContentPlanVariantTool(t.Context(), nil, mcpContentPlanVariantUpdateInput{PlanID: plan.ID, VariantID: "mcp_ready", Text: "Edited MCP", PlannedAt: when.Format(time.RFC3339)}); err != nil || edited.Plan.Items[0].Variants[0].Text != "Edited MCP" {
		t.Fatalf("edit=%#v err=%v", edited, err)
	}
	if _, scheduled, err := srv.mcpScheduleContentPlanTool(t.Context(), nil, mcpContentPlanVariantsInput{PlanID: plan.ID, VariantIDs: []string{"mcp_ready"}}); err != nil || len(scheduled.Result.Scheduled) != 1 {
		t.Fatalf("schedule=%#v err=%v", scheduled, err)
	}
	if _, retried, err := srv.mcpRetryContentPlanTool(t.Context(), nil, mcpContentPlanVariantsInput{PlanID: plan.ID, VariantIDs: []string{"mcp_failed"}}); err != nil || retried.Job.ID == "" {
		t.Fatalf("retry=%#v err=%v", retried, err)
	}
}

func performJSONRequest(t *testing.T, srv Server, method, path string, body any) *httptest.ResponseRecorder {
	t.Helper()
	var payload []byte
	if body != nil {
		payload, _ = json.Marshal(body)
	}
	req := httptest.NewRequest(method, path, bytes.NewReader(payload))
	req.Header.Set("content-type", "application/json")
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)
	return w
}
