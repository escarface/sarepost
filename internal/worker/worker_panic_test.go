package worker

import (
	"context"
	"testing"
	"time"

	"github.com/escarface/sarepost/internal/db"
	"github.com/escarface/sarepost/internal/domain"
	"github.com/escarface/sarepost/internal/postflow"
)

// panicPublishProvider is a postflow.Provider whose Publish always panics, used
// to prove the worker goroutine-level recover keeps the loop alive across a
// panicking publish tick.
type panicPublishProvider struct {
	platform domain.Platform
}

func (p panicPublishProvider) Platform() domain.Platform { return p.platform }

func (p panicPublishProvider) ValidateDraft(context.Context, domain.SocialAccount, postflow.Draft) ([]string, error) {
	return nil, nil
}

func (p panicPublishProvider) Publish(context.Context, domain.SocialAccount, postflow.Credentials, domain.Post, postflow.PublishOptions) (postflow.PublishResult, error) {
	panic("simulated provider panic during publish")
}

func (p panicPublishProvider) RefreshIfNeeded(context.Context, domain.SocialAccount, postflow.Credentials) (postflow.Credentials, bool, error) {
	return postflow.Credentials{}, false, nil
}

// TestSafeRunRecoversPanicAndDoesNotPropagate proves the per-tick recover
// wrapper catches a panic and does not propagate it. (R2-W1 component.)
func TestSafeRunRecoversPanicAndDoesNotPropagate(t *testing.T) {
	w := Worker{}
	called := false
	w.safeRun("test component", func() {
		called = true
		panic("boom")
	})
	if !called {
		t.Fatalf("safeRun must invoke the wrapped function")
	}
	// If the panic propagated, the test would have already aborted.
}

// TestStartSurvivesPanickingTickAndContinues proves the worker goroutine does
// not silently die when a tick panics: a panicking provider.Publish is recovered
// by the goroutine-level wrapper, and the goroutine stays alive for subsequent
// ticks until the context is cancelled. (R2-W1 end-to-end regression.)
func TestStartSurvivesPanickingTickAndContinues(t *testing.T) {
	store := openWorkerTestStore(t)
	cipher := newWorkerTestCipher(t)
	account := createWorkerTestAccount(t, store)
	// Seed a due post so the publish cycle reaches provider.Publish, which
	// panics. The post is created without a campaign so it remains claimable.
	if _, err := store.CreatePost(context.Background(), db.CreatePostParams{
		Post: domain.Post{
			AccountID:   account.ID,
			Platform:    account.Platform,
			Text:        "panic tick post",
			Status:      domain.PostStatusScheduled,
			ScheduledAt: time.Now().UTC().Add(-1 * time.Minute),
			MaxAttempts: 3,
		},
	}); err != nil {
		t.Fatalf("create post: %v", err)
	}

	registry := postflow.NewProviderRegistry(panicPublishProvider{platform: domain.PlatformX})
	w := Worker{
		Store:               store,
		Registry:            registry,
		Cipher:              cipher,
		Interval:            15 * time.Millisecond,
		RetryBackoff:        1 * time.Second,
		SafetySweepInterval: 50 * time.Millisecond,
	}

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		w.Start(ctx)
		close(done)
	}()

	// Let several ticks fire. The first publish tick panics inside
	// provider.Publish. If the goroutine-level recover were absent, the
	// goroutine would die and `done` would close immediately.
	select {
	case <-done:
		t.Fatalf("worker goroutine died from a panicking tick (panic was not recovered)")
	case <-time.After(120 * time.Millisecond):
		// goroutine still alive after the panicking tick and several more ticks.
	}

	cancel()
	select {
	case <-done:
	case <-time.After(500 * time.Millisecond):
		t.Fatalf("worker did not stop cleanly after context cancellation")
	}
}
