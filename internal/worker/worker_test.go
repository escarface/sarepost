package worker

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	generationapp "github.com/escarface/sarepost/internal/application/generation"
	"github.com/escarface/sarepost/internal/db"
	"github.com/escarface/sarepost/internal/domain"
	"github.com/escarface/sarepost/internal/genai"
	"github.com/escarface/sarepost/internal/postflow"
	"github.com/escarface/sarepost/internal/secure"
)

func TestLoadCredentialsWithoutRowReturnsEmpty(t *testing.T) {
	store := openWorkerTestStore(t)
	cipher := newWorkerTestCipher(t)
	account := createWorkerTestAccount(t, store)

	w := Worker{Store: store, Cipher: cipher}
	got, err := w.loadCredentials(t.Context(), account.ID)
	if err != nil {
		t.Fatalf("load credentials: %v", err)
	}
	if strings.TrimSpace(got.AccessToken) != "" {
		t.Fatalf("expected empty access token, got %q", got.AccessToken)
	}
}

func TestSaveAndLoadCredentialsRoundTrip(t *testing.T) {
	store := openWorkerTestStore(t)
	cipher := newWorkerTestCipher(t)
	account := createWorkerTestAccount(t, store)

	w := Worker{Store: store, Cipher: cipher}
	original := postflow.Credentials{
		AccessToken:  "access_123",
		RefreshToken: "refresh_123",
		TokenType:    "Bearer",
		Extra: map[string]string{
			"scope": "write",
		},
	}
	if err := (workerCredentialsStore{worker: w}).SaveCredentials(t.Context(), account.ID, original); err != nil {
		t.Fatalf("save credentials: %v", err)
	}
	loaded, err := w.loadCredentials(t.Context(), account.ID)
	if err != nil {
		t.Fatalf("load credentials: %v", err)
	}
	if loaded.AccessToken != original.AccessToken {
		t.Fatalf("unexpected access token: got=%q want=%q", loaded.AccessToken, original.AccessToken)
	}
	if loaded.RefreshToken != original.RefreshToken {
		t.Fatalf("unexpected refresh token: got=%q want=%q", loaded.RefreshToken, original.RefreshToken)
	}
	if loaded.TokenType != original.TokenType {
		t.Fatalf("unexpected token type: got=%q want=%q", loaded.TokenType, original.TokenType)
	}
	if loaded.Extra["scope"] != "write" {
		t.Fatalf("unexpected extra scope: got=%q", loaded.Extra["scope"])
	}
}

func TestRunOncePublishesDuePost(t *testing.T) {
	store := openWorkerTestStore(t)
	cipher := newWorkerTestCipher(t)
	account := createWorkerTestAccount(t, store)
	created, err := store.CreatePost(t.Context(), db.CreatePostParams{
		Post: domain.Post{
			AccountID:   account.ID,
			Platform:    account.Platform,
			Text:        "worker should publish this",
			Status:      domain.PostStatusScheduled,
			ScheduledAt: time.Now().UTC().Add(-1 * time.Minute),
			MaxAttempts: 3,
		},
	})
	if err != nil {
		t.Fatalf("create post: %v", err)
	}

	w := Worker{
		Store:        store,
		Registry:     postflow.NewProviderRegistry(postflow.NewMockProvider(domain.PlatformX)),
		Cipher:       cipher,
		Interval:     25 * time.Millisecond,
		RetryBackoff: 1 * time.Second,
	}
	w.runOnce(context.Background())

	post, err := store.GetPost(t.Context(), created.Post.ID)
	if err != nil {
		t.Fatalf("get post: %v", err)
	}
	if post.Status != domain.PostStatusPublished {
		t.Fatalf("expected status published after runOnce, got %s", post.Status)
	}
	if post.ExternalID == nil || strings.TrimSpace(*post.ExternalID) == "" {
		t.Fatalf("expected external_id to be set after publish")
	}
}

func TestRunOnceGeneratesQueuedContentPlanDurably(t *testing.T) {
	store := openWorkerTestStore(t)
	cipher := newWorkerTestCipher(t)
	account := createWorkerTestAccount(t, store)
	generation := generationapp.Service{Store: store, Cipher: cipher, Driver: genai.DriverMock}
	if _, err := generation.SaveTextProviderConfig(t.Context(), generationapp.ProviderConfigUpdate{Provider: genai.ProviderAnthropic, Model: "mock-text"}); err != nil {
		t.Fatalf("save text provider: %v", err)
	}
	profile, err := generation.SaveBrandProfile(t.Context(), generationapp.BrandProfileUpdate{Name: "Worker Brand"})
	if err != nil {
		t.Fatalf("save profile: %v", err)
	}
	when := time.Now().UTC().Add(24 * time.Hour)
	plan, err := store.CreateContentPlan(t.Context(), domain.ContentPlan{
		Name: "Worker plan", Objective: "Educate", StartsAt: when, EndsAt: when, Status: domain.ContentPlanStatusDraft,
		Blocks: []domain.ContentPlanBlock{{ID: "worker_block", BrandProfileID: profile.ID, AccountIDs: []string{account.ID}}},
		Items:  []domain.ContentPlanItem{{ID: "worker_item", BlockID: "worker_block", PlannedAt: when, Variants: []domain.ContentPlanVariant{{ID: "worker_variant", AccountID: account.ID, Platform: account.Platform, Status: domain.ContentPlanVariantPending, PlannedAt: when}}}},
	})
	if err != nil {
		t.Fatalf("create plan: %v", err)
	}
	if _, err := store.EnqueueContentPlanJob(t.Context(), plan.ID); err != nil {
		t.Fatalf("enqueue: %v", err)
	}

	w := Worker{Store: store, Registry: postflow.NewProviderRegistry(postflow.NewMockProvider(domain.PlatformX)), Cipher: cipher, Interval: time.Second, RetryBackoff: time.Second, DataDir: t.TempDir(), GenerationDriver: string(genai.DriverMock)}
	w.runOnce(t.Context())
	loaded, err := store.GetContentPlan(t.Context(), plan.ID)
	if err != nil {
		t.Fatalf("get plan: %v", err)
	}
	if loaded.Status != domain.ContentPlanStatusReview || loaded.Items[0].Idea == "" || loaded.Items[0].Variants[0].Status != domain.ContentPlanVariantReady || loaded.Items[0].Variants[0].Text == "" {
		t.Fatalf("unexpected generated plan: %#v", loaded)
	}
}

func TestWorkerMediaReaderLoadsPersistedReferenceImage(t *testing.T) {
	store := openWorkerTestStore(t)
	path := filepath.Join(t.TempDir(), "reference.png")
	if err := os.WriteFile(path, []byte("reference-image"), 0o644); err != nil {
		t.Fatalf("write image: %v", err)
	}
	media, err := store.CreateMedia(t.Context(), domain.Media{Kind: "image", OriginalName: "reference.png", StoragePath: path, MimeType: "image/png", SizeBytes: 15})
	if err != nil {
		t.Fatalf("create media: %v", err)
	}
	data, mimeType, err := (workerMediaReader{store: store}).ReadMedia(t.Context(), media.ID)
	if err != nil {
		t.Fatalf("read media: %v", err)
	}
	if string(data) != "reference-image" || mimeType != "image/png" {
		t.Fatalf("unexpected media data=%q mime=%q", data, mimeType)
	}
}

func TestWorkerMediaReaderReportsMissingRowsAndFiles(t *testing.T) {
	store := openWorkerTestStore(t)
	reader := workerMediaReader{store: store}
	if _, _, err := reader.ReadMedia(t.Context(), "med_missing"); err == nil {
		t.Fatal("expected missing media error")
	}
	media, err := store.CreateMedia(t.Context(), domain.Media{Kind: "image", OriginalName: "missing.png", StoragePath: filepath.Join(t.TempDir(), "missing.png"), MimeType: "image/png", SizeBytes: 1})
	if err != nil {
		t.Fatalf("create media: %v", err)
	}
	if _, _, err := reader.ReadMedia(t.Context(), media.ID); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("expected missing file error, got %v", err)
	}
}

func TestStartStopsWhenContextCancelled(t *testing.T) {
	store := openWorkerTestStore(t)
	cipher := newWorkerTestCipher(t)
	_ = createWorkerTestAccount(t, store)

	w := Worker{
		Store:        store,
		Registry:     postflow.NewProviderRegistry(postflow.NewMockProvider(domain.PlatformX)),
		Cipher:       cipher,
		Interval:     25 * time.Millisecond,
		RetryBackoff: 1 * time.Second,
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	done := make(chan struct{})
	go func() {
		w.Start(ctx)
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(500 * time.Millisecond):
		t.Fatalf("worker Start did not stop after context cancellation")
	}
}

func openWorkerTestStore(t *testing.T) *db.Store {
	t.Helper()
	store, err := db.Open(filepath.Join(t.TempDir(), "worker.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })
	return store
}

func createWorkerTestAccount(t *testing.T, store *db.Store) domain.SocialAccount {
	t.Helper()
	account, err := store.UpsertAccount(t.Context(), db.UpsertAccountParams{
		Platform:          domain.PlatformX,
		DisplayName:       "Worker X",
		ExternalAccountID: "worker_" + strings.ReplaceAll(t.Name(), "/", "_"),
		AuthMethod:        domain.AuthMethodStatic,
		Status:            domain.AccountStatusConnected,
	})
	if err != nil {
		t.Fatalf("upsert account: %v", err)
	}
	return account
}

func newWorkerTestCipher(t *testing.T) *secure.Cipher {
	t.Helper()
	key := make([]byte, 32)
	for i := range key {
		key[i] = byte(i + 1)
	}
	cipher, err := secure.NewCipher(key, 1)
	if err != nil {
		t.Fatalf("new cipher: %v", err)
	}
	return cipher
}
