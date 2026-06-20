package api

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/escarface/sarepost/internal/db"
	"github.com/escarface/sarepost/internal/domain"
)

func TestCreatePostFromFormRedirects(t *testing.T) {
	tempDir := t.TempDir()
	store, err := db.Open(filepath.Join(tempDir, "test.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer store.Close()

	srv := Server{Store: store, DataDir: tempDir, DefaultMaxRetries: 3}
	h := srv.Handler()

	form := url.Values{}
	form.Set("account_id", testAccountID(t, store))
	form.Set("text", "draft from form")
	req := httptest.NewRequest(http.MethodPost, "/posts", bytes.NewBufferString(form.Encode()))
	req.Header.Set("content-type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusSeeOther {
		t.Fatalf("expected 303, got %d", w.Code)
	}

	drafts, err := store.ListDrafts(t.Context())
	if err != nil {
		t.Fatalf("list drafts: %v", err)
	}
	if len(drafts) != 1 {
		t.Fatalf("expected 1 draft, got %d", len(drafts))
	}
}

func TestCreatePostFromFormPublishNowIgnoresScheduledAt(t *testing.T) {
	tempDir := t.TempDir()
	store, err := db.Open(filepath.Join(tempDir, "test.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer store.Close()

	srv := Server{Store: store, DataDir: tempDir, DefaultMaxRetries: 3}
	h := srv.Handler()

	form := url.Values{}
	form.Set("account_id", testAccountID(t, store))
	form.Set("text", "publish now from form")
	form.Set("intent", "publish_now")
	form.Set("scheduled_at_local", "2099-01-01T12:00")

	before := time.Now().UTC()
	req := httptest.NewRequest(http.MethodPost, "/posts", bytes.NewBufferString(form.Encode()))
	req.Header.Set("content-type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	after := time.Now().UTC()
	if w.Code != http.StatusSeeOther {
		t.Fatalf("expected 303, got %d", w.Code)
	}

	items, err := store.ListSchedule(t.Context(), before.Add(-1*time.Minute), after.Add(1*time.Minute))
	if err != nil {
		t.Fatalf("list schedule: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("expected one immediate scheduled post, got %d", len(items))
	}
	post := items[0]
	if post.Text != "publish now from form" {
		t.Fatalf("expected updated text, got %q", post.Text)
	}
	if post.ScheduledAt.Before(before.Add(-5*time.Second)) || post.ScheduledAt.After(after.Add(5*time.Second)) {
		t.Fatalf("expected publish_now scheduled_at near request time, got %s (before=%s after=%s)", post.ScheduledAt, before, after)
	}
}

func TestEditPostFromForm(t *testing.T) {
	tempDir := t.TempDir()
	store, err := db.Open(filepath.Join(tempDir, "test.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer store.Close()

	srv := Server{Store: store, DataDir: tempDir, DefaultMaxRetries: 3}
	h := srv.Handler()

	createPayload, _ := json.Marshal(map[string]any{
		"account_id":   testAccountID(t, store),
		"text":         "initial scheduled",
		"scheduled_at": time.Now().UTC().Add(2 * time.Hour).Format(time.RFC3339),
	})
	createReq := httptest.NewRequest(http.MethodPost, "/posts", bytes.NewReader(createPayload))
	createW := httptest.NewRecorder()
	h.ServeHTTP(createW, createReq)
	if createW.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d", createW.Code)
	}
	var created map[string]any
	if err := json.Unmarshal(createW.Body.Bytes(), &created); err != nil {
		t.Fatalf("decode create response: %v", err)
	}
	postID, _ := created["id"].(string)
	if postID == "" {
		t.Fatalf("missing post id")
	}

	editForm := url.Values{}
	editForm.Set("text", "edited and keep schedule")
	editReq := httptest.NewRequest(http.MethodPost, "/posts/"+postID+"/edit", bytes.NewBufferString(editForm.Encode()))
	editReq.Header.Set("content-type", "application/x-www-form-urlencoded")
	editW := httptest.NewRecorder()
	h.ServeHTTP(editW, editReq)
	if editW.Code != http.StatusSeeOther {
		t.Fatalf("expected 303, got %d", editW.Code)
	}

	post, err := store.GetPost(t.Context(), postID)
	if err != nil {
		t.Fatalf("get post: %v", err)
	}
	if post.Text != "edited and keep schedule" {
		t.Fatalf("expected updated text, got %q", post.Text)
	}
	if status := string(post.Status); status != "scheduled" {
		t.Fatalf("expected scheduled status to be preserved, got %s", status)
	}
	if post.ScheduledAt.IsZero() {
		t.Fatalf("expected scheduled_at to be preserved")
	}
}

func TestEditPostFromFormUpdatesAllGroupedPostIDs(t *testing.T) {
	tempDir := t.TempDir()
	store, err := db.Open(filepath.Join(tempDir, "test.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer store.Close()

	xAccount := createConnectedAccountForPlatform(t, store, domain.PlatformX, "x-edit-group-form")
	linkedinAccount := createConnectedAccountForPlatform(t, store, domain.PlatformLinkedIn, "li-edit-group-form")

	srv := Server{Store: store, DataDir: tempDir, DefaultMaxRetries: 3}
	h := srv.Handler()

	scheduledAt := time.Now().UTC().Add(3 * time.Hour).Truncate(time.Minute)
	createdIDs := make([]string, 0, 2)
	for _, accountID := range []string{xAccount.ID, linkedinAccount.ID} {
		createPayload, _ := json.Marshal(map[string]any{
			"account_id":   accountID,
			"text":         "same grouped edit text",
			"scheduled_at": scheduledAt.Format(time.RFC3339),
		})
		createReq := httptest.NewRequest(http.MethodPost, "/posts", bytes.NewReader(createPayload))
		createW := httptest.NewRecorder()
		h.ServeHTTP(createW, createReq)
		if createW.Code != http.StatusCreated {
			t.Fatalf("expected 201, got %d", createW.Code)
		}
		var created map[string]any
		if err := json.Unmarshal(createW.Body.Bytes(), &created); err != nil {
			t.Fatalf("decode create response: %v", err)
		}
		postID, _ := created["id"].(string)
		if postID == "" {
			t.Fatalf("missing post id")
		}
		createdIDs = append(createdIDs, postID)
	}

	editForm := url.Values{}
	editForm.Set("text", "edited everywhere")
	for _, postID := range createdIDs {
		editForm.Add("post_ids", postID)
	}
	editReq := httptest.NewRequest(http.MethodPost, "/posts/"+createdIDs[0]+"/edit", bytes.NewBufferString(editForm.Encode()))
	editReq.Header.Set("content-type", "application/x-www-form-urlencoded")
	editW := httptest.NewRecorder()
	h.ServeHTTP(editW, editReq)
	if editW.Code != http.StatusSeeOther {
		t.Fatalf("expected 303, got %d", editW.Code)
	}

	for _, postID := range createdIDs {
		post, err := store.GetPost(t.Context(), postID)
		if err != nil {
			t.Fatalf("get post %s: %v", postID, err)
		}
		if post.Text != "edited everywhere" {
			t.Fatalf("expected grouped edit to update post %s, got %q", postID, post.Text)
		}
	}
}

func TestEditPostFromFormPublishNowIgnoresScheduledAt(t *testing.T) {
	tempDir := t.TempDir()
	store, err := db.Open(filepath.Join(tempDir, "test.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer store.Close()

	srv := Server{Store: store, DataDir: tempDir, DefaultMaxRetries: 3}
	h := srv.Handler()

	createPayload, _ := json.Marshal(map[string]any{
		"account_id":   testAccountID(t, store),
		"text":         "initial scheduled",
		"scheduled_at": time.Now().UTC().Add(6 * time.Hour).Format(time.RFC3339),
	})
	createReq := httptest.NewRequest(http.MethodPost, "/posts", bytes.NewReader(createPayload))
	createW := httptest.NewRecorder()
	h.ServeHTTP(createW, createReq)
	if createW.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d", createW.Code)
	}
	var created map[string]any
	if err := json.Unmarshal(createW.Body.Bytes(), &created); err != nil {
		t.Fatalf("decode create response: %v", err)
	}
	postID, _ := created["id"].(string)
	if postID == "" {
		t.Fatalf("missing post id")
	}

	editForm := url.Values{}
	editForm.Set("text", "edited publish now")
	editForm.Set("intent", "publish_now")
	editForm.Set("scheduled_at_local", "2099-01-01T12:00")
	before := time.Now().UTC()
	editReq := httptest.NewRequest(http.MethodPost, "/posts/"+postID+"/edit", bytes.NewBufferString(editForm.Encode()))
	editReq.Header.Set("content-type", "application/x-www-form-urlencoded")
	editW := httptest.NewRecorder()
	h.ServeHTTP(editW, editReq)
	after := time.Now().UTC()
	if editW.Code != http.StatusSeeOther {
		t.Fatalf("expected 303, got %d", editW.Code)
	}

	post, err := store.GetPost(t.Context(), postID)
	if err != nil {
		t.Fatalf("get post: %v", err)
	}
	if post.Text != "edited publish now" {
		t.Fatalf("expected updated text, got %q", post.Text)
	}
	if post.ScheduledAt.Before(before.Add(-5*time.Second)) || post.ScheduledAt.After(after.Add(5*time.Second)) {
		t.Fatalf("expected publish_now scheduled_at near request time, got %s (before=%s after=%s)", post.ScheduledAt, before, after)
	}
}

func TestEditPostFromFormCanApproveAndScheduleCampaignPost(t *testing.T) {
	tempDir := t.TempDir()
	store, err := db.Open(filepath.Join(tempDir, "test.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer store.Close()

	srv := Server{Store: store, DataDir: tempDir, DefaultMaxRetries: 3}
	h := srv.Handler()

	accountID := testAccountID(t, store)
	campaign, err := store.CreateCampaign(t.Context(), domain.Campaign{Name: "Approval flow"})
	if err != nil {
		t.Fatalf("create campaign: %v", err)
	}
	created, err := store.CreatePost(t.Context(), db.CreatePostParams{
		Post: domain.Post{
			AccountID: accountID,
			Text:      "needs approval first",
			Status:    domain.PostStatusDraft,
		},
		CampaignID:       campaign.ID,
		EditorialStatus:  domain.EditorialStatusNeedsReview,
		RequiresApproval: true,
	})
	if err != nil {
		t.Fatalf("create post: %v", err)
	}

	editForm := url.Values{}
	editForm.Set("text", "approved from edit form")
	editForm.Set("intent", "schedule")
	editForm.Set("scheduled_at_local", "2026-07-01T10:00")
	editForm.Set("editorial_status", "approved")
	editForm.Set("requires_approval", "true")
	editReq := httptest.NewRequest(http.MethodPost, "/posts/"+created.Post.ID+"/edit", bytes.NewBufferString(editForm.Encode()))
	editReq.Header.Set("content-type", "application/x-www-form-urlencoded")
	editW := httptest.NewRecorder()
	h.ServeHTTP(editW, editReq)
	if editW.Code != http.StatusSeeOther {
		t.Fatalf("expected 303, got %d body=%s", editW.Code, editW.Body.String())
	}

	post, err := store.GetPost(t.Context(), created.Post.ID)
	if err != nil {
		t.Fatalf("get post: %v", err)
	}
	if post.Status != domain.PostStatusScheduled {
		t.Fatalf("expected scheduled status, got %s", post.Status)
	}
	if post.EditorialStatus != domain.EditorialStatusApproved {
		t.Fatalf("expected approved editorial status, got %s", post.EditorialStatus)
	}
	if post.RequiresApproval {
		t.Fatalf("expected requires approval to be cleared after approving in edit form")
	}
	if post.ApprovedAt == nil {
		t.Fatalf("expected approved_at to be set")
	}
}

func TestEditPostFromFormRejectsInvalidThreadSegmentsForPlatform(t *testing.T) {
	tempDir := t.TempDir()
	store, err := db.Open(filepath.Join(tempDir, "test.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer store.Close()

	account := createConnectedAccountForPlatform(t, store, domain.PlatformLinkedIn, "li-edit-thread-validation")
	media, err := store.CreateMedia(t.Context(), domain.Media{
		OriginalName: "followup.png",
		MimeType:     "image/png",
		SizeBytes:    128,
		StoragePath:  "uploads/followup.png",
	})
	if err != nil {
		t.Fatalf("create media: %v", err)
	}

	srv := Server{Store: store, DataDir: tempDir, DefaultMaxRetries: 3}
	h := srv.Handler()

	createPayload, _ := json.Marshal(map[string]any{
		"account_id":   account.ID,
		"text":         "initial scheduled",
		"scheduled_at": time.Now().UTC().Add(2 * time.Hour).Format(time.RFC3339),
	})
	createReq := httptest.NewRequest(http.MethodPost, "/posts", bytes.NewReader(createPayload))
	createW := httptest.NewRecorder()
	h.ServeHTTP(createW, createReq)
	if createW.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d", createW.Code)
	}
	var created map[string]any
	if err := json.Unmarshal(createW.Body.Bytes(), &created); err != nil {
		t.Fatalf("decode create response: %v", err)
	}
	postID, _ := created["id"].(string)
	if postID == "" {
		t.Fatalf("missing post id")
	}

	segmentsJSON, err := json.Marshal([]map[string]any{
		{"text": "edited root"},
		{"text": "invalid linkedin follow-up", "media_ids": []string{media.ID}},
	})
	if err != nil {
		t.Fatalf("marshal segments json: %v", err)
	}

	editForm := url.Values{}
	editForm.Set("text", "edited root")
	editForm.Set("segments_json", string(segmentsJSON))
	editReq := httptest.NewRequest(http.MethodPost, "/posts/"+postID+"/edit", bytes.NewBufferString(editForm.Encode()))
	editReq.Header.Set("content-type", "application/x-www-form-urlencoded")
	editW := httptest.NewRecorder()
	h.ServeHTTP(editW, editReq)
	if editW.Code != http.StatusSeeOther {
		t.Fatalf("expected 303, got %d", editW.Code)
	}
	location := editW.Header().Get("Location")
	if !strings.Contains(location, "linkedin+thread+comments+do+not+support+media") {
		t.Fatalf("expected redirect error for invalid linkedin follow-up media, got %q", location)
	}

	post, err := store.GetPost(t.Context(), postID)
	if err != nil {
		t.Fatalf("get post: %v", err)
	}
	if post.Text != "initial scheduled" {
		t.Fatalf("expected original text to remain after failed thread edit, got %q", post.Text)
	}
	threadPosts, err := store.ListThreadPosts(t.Context(), postID)
	if err != nil {
		t.Fatalf("list thread posts: %v", err)
	}
	if len(threadPosts) != 1 {
		t.Fatalf("expected invalid edit to avoid expanding thread, got %d posts", len(threadPosts))
	}
}

func TestCreateDraftWithoutScheduledAt(t *testing.T) {
	tempDir := t.TempDir()
	store, err := db.Open(filepath.Join(tempDir, "test.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer store.Close()

	srv := Server{Store: store, DataDir: tempDir, DefaultMaxRetries: 3}
	h := srv.Handler()

	body := map[string]any{
		"account_id": testAccountID(t, store),
		"text":       "idea de borrador",
	}
	payload, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/posts", bytes.NewReader(payload))
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d", w.Code)
	}
	var resp map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if status, _ := resp["status"].(string); status != "draft" {
		t.Fatalf("expected status=draft, got %q", status)
	}
}

func TestScheduleDraftPost(t *testing.T) {
	tempDir := t.TempDir()
	store, err := db.Open(filepath.Join(tempDir, "test.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer store.Close()

	srv := Server{Store: store, DataDir: tempDir, DefaultMaxRetries: 3}
	h := srv.Handler()

	createPayload, _ := json.Marshal(map[string]any{
		"account_id": testAccountID(t, store),
		"text":       "draft to schedule",
	})
	createReq := httptest.NewRequest(http.MethodPost, "/posts", bytes.NewReader(createPayload))
	createW := httptest.NewRecorder()
	h.ServeHTTP(createW, createReq)
	if createW.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d", createW.Code)
	}
	var created map[string]any
	if err := json.Unmarshal(createW.Body.Bytes(), &created); err != nil {
		t.Fatalf("decode create response: %v", err)
	}
	postID, _ := created["id"].(string)
	if postID == "" {
		t.Fatalf("missing post id")
	}

	schedulePayload, _ := json.Marshal(map[string]any{
		"scheduled_at": time.Now().UTC().Add(5 * time.Minute).Format(time.RFC3339),
	})
	scheduleReq := httptest.NewRequest(http.MethodPost, "/posts/"+postID+"/schedule", bytes.NewReader(schedulePayload))
	scheduleReq.Header.Set("content-type", "application/json")
	scheduleW := httptest.NewRecorder()
	h.ServeHTTP(scheduleW, scheduleReq)
	if scheduleW.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", scheduleW.Code)
	}
	var scheduled map[string]any
	if err := json.Unmarshal(scheduleW.Body.Bytes(), &scheduled); err != nil {
		t.Fatalf("decode schedule response: %v", err)
	}
	if status, _ := scheduled["status"].(string); status != "scheduled" {
		t.Fatalf("expected status=scheduled, got %q", status)
	}
}

func TestPreviewSchedulePostReturnsWarningsWithoutScheduling(t *testing.T) {
	tempDir := t.TempDir()
	store, err := db.Open(filepath.Join(tempDir, "test.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer store.Close()

	srv := Server{Store: store, DataDir: tempDir, DefaultMaxRetries: 3}
	h := srv.Handler()

	createPayload, _ := json.Marshal(map[string]any{
		"account_id":         testAccountID(t, store),
		"text":               "draft needing approval",
		"editorial_status":   "needs_review",
		"requires_approval":  true,
		"idempotency_key":    "preview-schedule-test",
		"idempotency_scope":  "api-test",
		"idempotency_target": "preview",
	})
	createReq := httptest.NewRequest(http.MethodPost, "/posts", bytes.NewReader(createPayload))
	createReq.Header.Set("content-type", "application/json")
	createW := httptest.NewRecorder()
	h.ServeHTTP(createW, createReq)
	if createW.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d body=%s", createW.Code, createW.Body.String())
	}
	var created struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(createW.Body.Bytes(), &created); err != nil {
		t.Fatalf("decode create response: %v", err)
	}
	if strings.TrimSpace(created.ID) == "" {
		t.Fatalf("missing post id")
	}
	campaign, err := store.CreateCampaign(t.Context(), domain.Campaign{Name: "Preview approval campaign"})
	if err != nil {
		t.Fatalf("create campaign: %v", err)
	}
	if err := store.AddPostToCampaign(t.Context(), created.ID, campaign.ID, domain.EditorialStatusNeedsReview, true, nil); err != nil {
		t.Fatalf("add post to campaign: %v", err)
	}

	previewPayload, _ := json.Marshal(map[string]any{
		"scheduled_at": time.Now().UTC().Add(2 * time.Hour).Format(time.RFC3339),
	})
	previewReq := httptest.NewRequest(http.MethodPost, "/posts/"+created.ID+"/preview-schedule", bytes.NewReader(previewPayload))
	previewReq.Header.Set("content-type", "application/json")
	previewW := httptest.NewRecorder()
	h.ServeHTTP(previewW, previewReq)
	if previewW.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", previewW.Code, previewW.Body.String())
	}
	var preview struct {
		PostID   string   `json:"post_id"`
		Text     string   `json:"text"`
		Warnings []string `json:"warnings"`
	}
	if err := json.Unmarshal(previewW.Body.Bytes(), &preview); err != nil {
		t.Fatalf("decode preview response: %v", err)
	}
	if preview.PostID != created.ID || preview.Text != "draft needing approval" {
		t.Fatalf("unexpected preview payload: %+v", preview)
	}
	if !containsString(preview.Warnings, "post requires approval before scheduling") {
		t.Fatalf("expected approval warning, got %+v", preview.Warnings)
	}
	post, err := store.GetPost(t.Context(), created.ID)
	if err != nil {
		t.Fatalf("get post: %v", err)
	}
	if post.Status != domain.PostStatusDraft || !post.ScheduledAt.IsZero() {
		t.Fatalf("preview should not schedule post, got status=%s scheduled_at=%s", post.Status, post.ScheduledAt)
	}
}

func TestScheduleEndpointReturnsCreatedPost(t *testing.T) {
	tempDir := t.TempDir()
	store, err := db.Open(filepath.Join(tempDir, "test.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer store.Close()

	srv := Server{Store: store, DataDir: tempDir, DefaultMaxRetries: 3}
	h := srv.Handler()

	scheduled := time.Now().UTC().Add(2 * time.Hour).Format(time.RFC3339)
	body := map[string]any{
		"account_id":   testAccountID(t, store),
		"text":         "post de prueba",
		"scheduled_at": scheduled,
	}
	payload, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/posts", bytes.NewReader(payload))
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d", w.Code)
	}

	req = httptest.NewRequest(http.MethodGet, "/schedule", nil)
	w = httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	if !bytes.Contains(w.Body.Bytes(), []byte("post de prueba")) {
		_ = os.WriteFile(filepath.Join(tempDir, "schedule_response.json"), w.Body.Bytes(), 0o644)
		t.Fatalf("expected schedule to include created post")
	}
}

func TestCreatePostWithIdempotencyKeyIsReplayed(t *testing.T) {
	tempDir := t.TempDir()
	store, err := db.Open(filepath.Join(tempDir, "test.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer store.Close()

	srv := Server{Store: store, DataDir: tempDir, DefaultMaxRetries: 3}
	h := srv.Handler()

	scheduled := time.Now().UTC().Add(2 * time.Hour).Format(time.RFC3339)
	body := map[string]any{
		"account_id":   testAccountID(t, store),
		"text":         "idempotent post",
		"scheduled_at": scheduled,
	}
	payload, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/posts", bytes.NewReader(payload))
	req.Header.Set("Idempotency-Key", "same-request-123")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d", w.Code)
	}
	var firstResp map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &firstResp); err != nil {
		t.Fatalf("decode first response: %v", err)
	}
	firstID, _ := firstResp["id"].(string)
	if firstID == "" {
		t.Fatalf("expected first response to include post id")
	}

	req = httptest.NewRequest(http.MethodPost, "/posts", bytes.NewReader(payload))
	req.Header.Set("Idempotency-Key", "same-request-123")
	w = httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 replay, got %d", w.Code)
	}
	var secondResp map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &secondResp); err != nil {
		t.Fatalf("decode second response: %v", err)
	}
	secondID, _ := secondResp["id"].(string)
	if secondID != firstID {
		t.Fatalf("expected same id on replay, got %q and %q", firstID, secondID)
	}
}

func TestDeletePostFromFormOnlyAllowsEditableStatuses(t *testing.T) {
	tempDir := t.TempDir()
	store, err := db.Open(filepath.Join(tempDir, "test.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer store.Close()

	srv := Server{Store: store, DataDir: tempDir, DefaultMaxRetries: 3}
	h := srv.Handler()
	accountID := testAccountID(t, store)

	scheduledPayload, _ := json.Marshal(map[string]any{
		"account_id":   accountID,
		"text":         "scheduled deletable",
		"scheduled_at": time.Now().UTC().Add(1 * time.Hour).Format(time.RFC3339),
	})
	scheduledReq := httptest.NewRequest(http.MethodPost, "/posts", bytes.NewReader(scheduledPayload))
	scheduledW := httptest.NewRecorder()
	h.ServeHTTP(scheduledW, scheduledReq)
	if scheduledW.Code != http.StatusCreated {
		t.Fatalf("expected 201 for scheduled post, got %d", scheduledW.Code)
	}
	var scheduledResp map[string]any
	if err := json.Unmarshal(scheduledW.Body.Bytes(), &scheduledResp); err != nil {
		t.Fatalf("decode scheduled response: %v", err)
	}
	scheduledID, _ := scheduledResp["id"].(string)
	if scheduledID == "" {
		t.Fatalf("expected scheduled post id")
	}

	deleteForm := url.Values{}
	deleteForm.Set("return_to", "/?view=calendar")
	deleteReq := httptest.NewRequest(http.MethodPost, "/posts/"+scheduledID+"/delete", bytes.NewBufferString(deleteForm.Encode()))
	deleteReq.Header.Set("content-type", "application/x-www-form-urlencoded")
	deleteW := httptest.NewRecorder()
	h.ServeHTTP(deleteW, deleteReq)
	if deleteW.Code != http.StatusSeeOther {
		t.Fatalf("expected 303 deleting scheduled post, got %d", deleteW.Code)
	}
	location := deleteW.Header().Get("Location")
	if !strings.Contains(location, "success=post+deleted") {
		t.Fatalf("expected success redirect, got %q", location)
	}
	if _, err := store.GetPost(t.Context(), scheduledID); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("expected deleted scheduled post, got err=%v", err)
	}

	publishedPayload, _ := json.Marshal(map[string]any{
		"account_id":   accountID,
		"text":         "published not deletable",
		"scheduled_at": time.Now().UTC().Add(2 * time.Hour).Format(time.RFC3339),
	})
	publishedReq := httptest.NewRequest(http.MethodPost, "/posts", bytes.NewReader(publishedPayload))
	publishedW := httptest.NewRecorder()
	h.ServeHTTP(publishedW, publishedReq)
	if publishedW.Code != http.StatusCreated {
		t.Fatalf("expected 201 for seed published post, got %d", publishedW.Code)
	}
	var publishedResp map[string]any
	if err := json.Unmarshal(publishedW.Body.Bytes(), &publishedResp); err != nil {
		t.Fatalf("decode published response: %v", err)
	}
	publishedID, _ := publishedResp["id"].(string)
	if publishedID == "" {
		t.Fatalf("expected published post id")
	}
	if err := store.MarkPublished(t.Context(), publishedID, "x-published-seed", ""); err != nil {
		t.Fatalf("mark published: %v", err)
	}

	publishedDeleteReq := httptest.NewRequest(http.MethodPost, "/posts/"+publishedID+"/delete", bytes.NewBufferString(deleteForm.Encode()))
	publishedDeleteReq.Header.Set("content-type", "application/x-www-form-urlencoded")
	publishedDeleteW := httptest.NewRecorder()
	h.ServeHTTP(publishedDeleteW, publishedDeleteReq)
	if publishedDeleteW.Code != http.StatusSeeOther {
		t.Fatalf("expected 303 when deleting published post, got %d", publishedDeleteW.Code)
	}
	publishedLocation := publishedDeleteW.Header().Get("Location")
	if !strings.Contains(publishedLocation, "error=post+not+deletable") {
		t.Fatalf("expected error redirect for published post, got %q", publishedLocation)
	}
	if _, err := store.GetPost(t.Context(), publishedID); err != nil {
		t.Fatalf("expected published post to remain, got %v", err)
	}
}

func TestDeletePostFromFormSupportsBulkIDs(t *testing.T) {
	tempDir := t.TempDir()
	store, err := db.Open(filepath.Join(tempDir, "test.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer store.Close()

	srv := Server{Store: store, DataDir: tempDir, DefaultMaxRetries: 3}
	h := srv.Handler()
	accountID := testAccountID(t, store)

	createScheduled := func(text string, offset time.Duration) string {
		payload, _ := json.Marshal(map[string]any{
			"account_id":   accountID,
			"text":         text,
			"scheduled_at": time.Now().UTC().Add(offset).Format(time.RFC3339),
		})
		req := httptest.NewRequest(http.MethodPost, "/posts", bytes.NewReader(payload))
		w := httptest.NewRecorder()
		h.ServeHTTP(w, req)
		if w.Code != http.StatusCreated {
			t.Fatalf("expected 201 for scheduled post, got %d", w.Code)
		}
		var out map[string]any
		if err := json.Unmarshal(w.Body.Bytes(), &out); err != nil {
			t.Fatalf("decode create response: %v", err)
		}
		id, _ := out["id"].(string)
		if id == "" {
			t.Fatalf("expected scheduled post id")
		}
		return id
	}

	postA := createScheduled("bulk delete a", 1*time.Hour)
	postB := createScheduled("bulk delete b", 2*time.Hour)

	deleteForm := url.Values{}
	deleteForm.Set("return_to", "/?view=publications")
	deleteForm.Add("ids", postA)
	deleteForm.Add("ids", postB)
	deleteReq := httptest.NewRequest(http.MethodPost, "/posts/"+postA+"/delete", bytes.NewBufferString(deleteForm.Encode()))
	deleteReq.Header.Set("content-type", "application/x-www-form-urlencoded")
	deleteW := httptest.NewRecorder()
	h.ServeHTTP(deleteW, deleteReq)
	if deleteW.Code != http.StatusSeeOther {
		t.Fatalf("expected 303 for bulk delete form, got %d", deleteW.Code)
	}
	location := deleteW.Header().Get("Location")
	if !strings.Contains(location, "success=deleted+2") {
		t.Fatalf("expected bulk delete success redirect, got %q", location)
	}

	if _, err := store.GetPost(t.Context(), postA); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("expected postA deleted, got err=%v", err)
	}
	if _, err := store.GetPost(t.Context(), postB); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("expected postB deleted, got err=%v", err)
	}
}
