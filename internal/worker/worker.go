package worker

import (
	"context"
	"database/sql"
	"errors"
	"log/slog"
	"os"
	"strings"
	"time"

	contentplansapp "github.com/escarface/sarepost/internal/application/contentplans"
	generationapp "github.com/escarface/sarepost/internal/application/generation"
	mediaapp "github.com/escarface/sarepost/internal/application/media"
	notificationsapp "github.com/escarface/sarepost/internal/application/notifications"
	publishcycle "github.com/escarface/sarepost/internal/application/publishcycle"
	safetygate "github.com/escarface/sarepost/internal/application/safetygate"
	"github.com/escarface/sarepost/internal/db"
	"github.com/escarface/sarepost/internal/genai"
	"github.com/escarface/sarepost/internal/postflow"
	"github.com/escarface/sarepost/internal/secure"
)

type Worker struct {
	Store               *db.Store
	Registry            *postflow.ProviderRegistry
	Cipher              *secure.Cipher
	Interval            time.Duration
	RetryBackoff        time.Duration
	DataDir             string
	GenerationDriver    string
	SafetySweepInterval time.Duration
	SafetyBatchSize     int
	SafetySweepLease    time.Duration
}

func (w Worker) Start(ctx context.Context) {
	ticker := time.NewTicker(w.Interval)
	defer ticker.Stop()

	sweepInterval := w.safetySweepInterval()
	sweepTicker := time.NewTicker(sweepInterval)
	defer sweepTicker.Stop()

	w.runOnce(ctx)
	w.runSafetySweep(ctx)
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			w.runOnce(ctx)
		case <-sweepTicker.C:
			w.runSafetySweep(ctx)
		}
	}
}

func (w Worker) safetySweepInterval() time.Duration {
	if w.SafetySweepInterval > 0 {
		return w.SafetySweepInterval
	}
	return 30 * time.Second
}

func (w Worker) safetySweepLease() time.Duration {
	if w.SafetySweepLease > 0 {
		return w.SafetySweepLease
	}
	return 2 * time.Minute
}

func (w Worker) safetyBatchSize() int {
	if w.SafetyBatchSize > 0 {
		return w.SafetyBatchSize
	}
	return 100
}

// runSafetySweep claims the DB-backed safety-sweep lease, then runs one
// safetygate.ApproveEligible pass. If the lease cannot be acquired (another
// sweep is in progress), it skips this tick. Mid-batch store errors are logged
// and the sweep continues on the next tick (REQ-WORKER-HOOK).
func (w Worker) runSafetySweep(ctx context.Context) {
	if w.Store == nil {
		return
	}
	held, err := w.Store.ClaimSafetySweep(ctx, w.safetySweepLease())
	if err != nil {
		slog.Default().Error("safety sweep lease claim failed", "error", err)
		return
	}
	if !held {
		return
	}
	svc := safetygate.Service{Store: w.Store, MaxBatchSize: w.safetyBatchSize()}
	summary, err := svc.ApproveEligible(ctx)
	if err != nil {
		slog.Default().Error("safety sweep failed", "error", err)
		return
	}
	if summary.Evaluated > 0 {
		slog.Default().Info("safety sweep",
			"evaluated", summary.Evaluated,
			"approved", summary.Approved,
			"blocked", summary.Blocked,
			"errors", len(summary.Errors),
		)
	}
}

func (w Worker) runOnce(ctx context.Context) {
	recovered, err := w.Store.RecoverStalePublishingPosts(ctx, 5*time.Minute)
	if err != nil {
		slog.Default().Error("worker stale publishing recovery failed", "error", err)
	} else if recovered > 0 {
		slog.Default().Warn("worker recovered stale publishing posts", "count", recovered)
	}

	runner := publishcycle.Runner{
		Store:           w.Store,
		Registry:        w.Registry,
		Credentials:     workerCredentialsStore{worker: w},
		FailureNotifier: notificationsapp.Service{Store: w.Store, Cipher: w.Cipher},
		RetryBackoff:    w.RetryBackoff,
		Interval:        w.Interval,
	}
	runner.RunOnce(ctx)
	w.runContentPlanOnce(ctx)
}

func (w Worker) runContentPlanOnce(ctx context.Context) {
	job, err := w.Store.ClaimContentPlanJob(ctx, 5*time.Minute)
	if errors.Is(err, db.ErrNoContentPlanJob) {
		return
	}
	if err != nil {
		slog.Default().Error("content plan job claim failed", "error", err)
		return
	}
	driver := genai.DriverMock
	if strings.EqualFold(strings.TrimSpace(w.GenerationDriver), "live") {
		driver = genai.DriverLive
	}
	generation := generationapp.Service{Store: w.Store, Cipher: w.Cipher, Driver: driver, MediaReader: workerMediaReader{store: w.Store}}
	runner := contentplansapp.Runner{
		Store: w.Store, Text: generation, Images: generation,
		Media: mediaapp.Service{GeneratedStore: w.Store, DataDir: w.DataDir}, MaxConcurrency: 2,
	}
	if err := runner.RunJob(ctx, job); err != nil {
		slog.Default().Error("content plan job failed", "job_id", job.ID, "plan_id", job.PlanID, "error", err)
	}
}

func (w Worker) loadCredentials(ctx context.Context, accountID string) (postflow.Credentials, error) {
	encrypted, err := w.Store.GetAccountCredentials(ctx, accountID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return postflow.Credentials{}, nil
		}
		return postflow.Credentials{}, err
	}
	var credentials postflow.Credentials
	if err := w.Cipher.DecryptJSON(encrypted.Ciphertext, encrypted.Nonce, &credentials); err != nil {
		return postflow.Credentials{}, err
	}
	if credentials.Extra == nil {
		credentials.Extra = make(map[string]string)
	}
	return credentials, nil
}

type workerCredentialsStore struct {
	worker Worker
}

type workerMediaReader struct{ store *db.Store }

func (r workerMediaReader) ReadMedia(ctx context.Context, mediaID string) ([]byte, string, error) {
	items, err := r.store.GetMediaByIDs(ctx, []string{strings.TrimSpace(mediaID)})
	if err != nil {
		return nil, "", err
	}
	if len(items) == 0 {
		return nil, "", sql.ErrNoRows
	}
	data, err := os.ReadFile(items[0].StoragePath)
	if err != nil {
		return nil, "", err
	}
	return data, items[0].MimeType, nil
}

func (w workerCredentialsStore) LoadCredentials(ctx context.Context, accountID string) (postflow.Credentials, error) {
	return w.worker.loadCredentials(ctx, accountID)
}

func (w workerCredentialsStore) SaveCredentials(ctx context.Context, accountID string, credentials postflow.Credentials) error {
	sealed, nonce, err := w.worker.Cipher.EncryptJSON(credentials)
	if err != nil {
		return err
	}
	return w.worker.Store.SaveAccountCredentials(ctx, accountID, db.EncryptedCredentials{
		Ciphertext: sealed,
		Nonce:      nonce,
		KeyVersion: w.worker.Cipher.KeyVersion(),
		UpdatedAt:  time.Now().UTC(),
	})
}
