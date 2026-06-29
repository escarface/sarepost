package db

import (
	"context"
	"time"
)

// safetySweepLeaseKey is the settings row holding the active sweep lease
// expiry (RFC3339Nano). Reusing the settings table avoids a new migration.
const safetySweepLeaseKey = "safety_sweep_lease"

// ClaimSafetySweep atomically acquires the safety-sweep lease for the given
// duration. It returns true when the caller now owns the lease, false when an
// active lease blocks the claim. The lease is stored as an RFC3339Nano
// timestamp in the existing settings table; RFC3339Nano strings sort
// lexicographically in time order, so the ON CONFLICT WHERE comparison is
// correct. SQLite serializes writes, so two concurrent callers cannot both
// observe an expired lease (REQ-WORKER-HOOK).
func (s *Store) ClaimSafetySweep(ctx context.Context, leaseDuration time.Duration) (bool, error) {
	if leaseDuration <= 0 {
		leaseDuration = 30 * time.Second
	}
	now := time.Now().UTC()
	expiry := now.Add(leaseDuration).Format(time.RFC3339Nano)
	nowFmt := now.Format(time.RFC3339Nano)
	result, err := s.db.ExecContext(ctx, `
		INSERT INTO settings (key, value) VALUES (?, ?)
		ON CONFLICT(key) DO UPDATE SET value = excluded.value
		WHERE settings.value <= ?
	`, safetySweepLeaseKey, expiry, nowFmt)
	if err != nil {
		return false, err
	}
	rows, _ := result.RowsAffected()
	return rows == 1, nil
}
