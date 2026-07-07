package db

import (
	"context"
	"strconv"
	"time"
)

// safetySweepLeaseKey is the settings row holding the active sweep lease
// expiry as an integer epoch-nanosecond string. Reusing the settings table
// avoids a new migration.
const safetySweepLeaseKey = "safety_sweep_lease"

// ClaimSafetySweep atomically acquires the safety-sweep lease for the given
// duration. It returns true when the caller now owns the lease, false when an
// active lease blocks the claim.
//
// The lease expiry is stored as an integer epoch-nanosecond count (decimal
// string in the TEXT settings column) and compared numerically via CAST.
// RFC3339Nano strings were used previously, but RFC3339Nano drops trailing
// zeros so lexicographic string ordering is NOT chronological — e.g.
// "19.1Z" (100ms) sorts after "19.12Z" (120ms). Numeric epoch comparison is
// correct. SQLite serializes writes, so two concurrent callers cannot both
// observe an expired lease (REQ-WORKER-HOOK).
func (s *Store) ClaimSafetySweep(ctx context.Context, leaseDuration time.Duration) (bool, error) {
	if leaseDuration <= 0 {
		leaseDuration = 30 * time.Second
	}
	nowNs := time.Now().UTC().UnixNano()
	expiryNs := nowNs + int64(leaseDuration)
	result, err := s.db.ExecContext(ctx, `
		INSERT INTO settings (key, value) VALUES (?, ?)
		ON CONFLICT(key) DO UPDATE SET value = excluded.value
		WHERE CAST(settings.value AS INTEGER) <= ?
	`, safetySweepLeaseKey, strconv.FormatInt(expiryNs, 10), strconv.FormatInt(nowNs, 10))
	if err != nil {
		return false, err
	}
	rows, _ := result.RowsAffected()
	return rows == 1, nil
}

// ReleaseSafetySweep frees the safety-sweep lease so the next tick can claim
// immediately instead of waiting for the lease duration to elapse. It sets the
// stored expiry to 0 (the distant past), which any subsequent ClaimSafetySweep
// observes as expired. Called by the worker on sweep completion (success,
// no-work, AND error paths) so the effective cadence honors the configured
// interval rather than the lease duration (R2-C2).
func (s *Store) ReleaseSafetySweep(ctx context.Context) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO settings (key, value) VALUES (?, '0')
		ON CONFLICT(key) DO UPDATE SET value = '0'
	`, safetySweepLeaseKey)
	return err
}
