package db

import (
	"context"
	"database/sql"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/escarface/sarepost/internal/domain"
)

func TestMigrationSafetyGateCreatesTableAndColumns(t *testing.T) {
	store := openTestStore(t)

	var tableCount int
	if err := store.db.QueryRowContext(t.Context(), `SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name='safety_rules'`).Scan(&tableCount); err != nil {
		t.Fatalf("query safety_rules table: %v", err)
	}
	if tableCount != 1 {
		t.Fatalf("expected safety_rules table to exist after migration, got count=%d", tableCount)
	}

	var count int
	if err := store.db.QueryRowContext(t.Context(), `SELECT COUNT(*) FROM safety_rules`).Scan(&count); err != nil {
		t.Fatalf("count safety rules: %v", err)
	}
	if count != 10 {
		t.Fatalf("expected 10 seeded default safety rules, got %d", count)
	}
}

func TestMigrationSafetyGateSeedsExpectedDefaults(t *testing.T) {
	store := openTestStore(t)

	type row struct {
		kind     string
		platform string
		severity string
		enabled  int
		params   string
	}
	rows, err := store.db.QueryContext(t.Context(), `
		SELECT kind, COALESCE(platform, ''), severity, enabled, params FROM safety_rules
	`)
	if err != nil {
		t.Fatalf("query safety rules: %v", err)
	}
	defer rows.Close()
	got := map[string]row{}
	for rows.Next() {
		var r row
		if err := rows.Scan(&r.kind, &r.platform, &r.severity, &r.enabled, &r.params); err != nil {
			t.Fatalf("scan rule: %v", err)
		}
		got[r.kind+"|"+r.platform] = r
	}
	if len(got) != 10 {
		t.Fatalf("expected 10 rules, got %d", len(got))
	}

	// length_range per platform (4), severity block, enabled, min_len 1.
	for _, p := range []domain.Platform{domain.PlatformX, domain.PlatformLinkedIn, domain.PlatformFacebook, domain.PlatformInstagram} {
		r, ok := got[string(domain.RuleLengthRange)+"|"+string(p)]
		if !ok {
			t.Fatalf("missing length_range rule for platform %s", p)
		}
		if r.severity != string(domain.SeverityBlock) || r.enabled != 1 {
			t.Fatalf("length_range %s must be block+enabled, got %+v", p, r)
		}
		if !strings.Contains(r.params, `"min_len":1`) {
			t.Fatalf("length_range %s params missing min_len:1: %s", p, r.params)
		}
	}

	// hashtag_max Instagram = 30.
	ig := got[string(domain.RuleHashtagMax)+"|"+string(domain.PlatformInstagram)]
	if !strings.Contains(ig.params, `"hashtag_max":30`) {
		t.Fatalf("instagram hashtag_max params wrong: %s", ig.params)
	}

	// link_max global (platform empty), max 1, block, enabled.
	linkRule, ok := got[string(domain.RuleLinkMax)+"|"]
	if !ok {
		t.Fatalf("missing global link_max rule")
	}
	if linkRule.enabled != 1 || !strings.Contains(linkRule.params, `"link_max":1`) {
		t.Fatalf("link_max global wrong: %+v", linkRule)
	}

	// banned_terms global, disabled by default, empty patterns.
	banned, ok := got[string(domain.RuleBannedTerms)+"|"]
	if !ok {
		t.Fatalf("missing global banned_terms rule")
	}
	if banned.enabled != 0 {
		t.Fatalf("banned_terms must be disabled by default, got enabled=%d", banned.enabled)
	}
	if strings.Contains(banned.params, "banned_patterns") {
		t.Fatalf("banned_terms default patterns must be empty, got %s", banned.params)
	}
}

func TestMigrationSafetyGateAddsCampaignPostAuditColumns(t *testing.T) {
	store := openTestStore(t)

	campaign, err := store.CreateCampaign(t.Context(), domain.Campaign{Name: "safety campaign", Status: domain.CampaignStatusActive})
	if err != nil {
		t.Fatalf("create campaign: %v", err)
	}
	created, err := store.CreatePost(t.Context(), CreatePostParams{
		Post: domain.Post{
			AccountID:       mustSeedSafetyAccount(t, store, domain.PlatformX, "safety-acct"),
			Platform:        domain.PlatformX,
			Text:            "eligible post",
			Status:          domain.PostStatusScheduled,
			ScheduledAt:     testNow(),
			MaxAttempts:     3,
			EditorialStatus: domain.EditorialStatusNeedsReview,
		},
		CampaignID:       campaign.ID,
		EditorialStatus:  domain.EditorialStatusNeedsReview,
		RequiresApproval: true,
	})
	if err != nil {
		t.Fatalf("create campaign post: %v", err)
	}

	var autoApproved, blockedReason, editorial string
	var requiresApproval int
	err = store.db.QueryRowContext(t.Context(), `
		SELECT auto_approved_reason, blocked_reason, editorial_status, requires_approval
		FROM campaign_posts WHERE post_id = ?
	`, created.Post.ID).Scan(&autoApproved, &blockedReason, &editorial, &requiresApproval)
	if err != nil {
		t.Fatalf("query campaign_posts audit columns: %v", err)
	}
	if autoApproved != "" || blockedReason != "" {
		t.Fatalf("new audit columns must default empty, got auto=%q blocked=%q", autoApproved, blockedReason)
	}
	if editorial != string(domain.EditorialStatusNeedsReview) {
		t.Fatalf("editorial_status changed by migration: %q", editorial)
	}
	if requiresApproval != 1 {
		t.Fatalf("requires_approval changed by migration: %d", requiresApproval)
	}
}

func TestMigrationSafetyGateIdempotentReentry(t *testing.T) {
	store := openTestStore(t)

	// Re-run the migration Up function directly on an already-migrated DB.
	// It must not error and must not duplicate seeded rows.
	tx, err := store.db.BeginTx(t.Context(), nil)
	if err != nil {
		t.Fatalf("begin tx: %v", err)
	}
	if err := migrationAddSafetyGate(t.Context(), tx); err != nil {
		_ = tx.Rollback()
		t.Fatalf("re-run migrationAddSafetyGate: %v", err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatalf("commit: %v", err)
	}

	var count int
	if err := store.db.QueryRowContext(t.Context(), `SELECT COUNT(*) FROM safety_rules`).Scan(&count); err != nil {
		t.Fatalf("count rules after re-entry: %v", err)
	}
	if count != 10 {
		t.Fatalf("idempotent re-entry duplicated rules: got %d, want 10", count)
	}
}

func TestMigrationSafetyGateAppliesOnLegacyDBWithExistingPosts(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "legacy_safety.db")
	legacyDB, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("open legacy db: %v", err)
	}
	defer legacyDB.Close()
	if err := seedLegacySchemaV1(t.Context(), legacyDB, "acc_legacy_safety", "pst_legacy_safety"); err != nil {
		t.Fatalf("seed legacy: %v", err)
	}
	if err := legacyDB.Close(); err != nil {
		t.Fatalf("close legacy: %v", err)
	}

	store, err := Open(dbPath)
	if err != nil {
		t.Fatalf("open migrated: %v", err)
	}
	defer store.Close()

	post, err := store.GetPost(context.Background(), "pst_legacy_safety")
	if err != nil {
		t.Fatalf("get legacy post: %v", err)
	}
	if post.Text != "legacy scheduled post" {
		t.Fatalf("legacy post text changed: %q", post.Text)
	}

	var migrationCount int
	if err := store.db.QueryRowContext(t.Context(), `SELECT COUNT(*) FROM schema_migrations`).Scan(&migrationCount); err != nil {
		t.Fatalf("count migrations: %v", err)
	}
	if migrationCount != len(dbMigrations) {
		t.Fatalf("expected %d migrations applied, got %d", len(dbMigrations), migrationCount)
	}

	var ruleCount int
	if err := store.db.QueryRowContext(t.Context(), `SELECT COUNT(*) FROM safety_rules`).Scan(&ruleCount); err != nil {
		t.Fatalf("count rules on legacy-migrated db: %v", err)
	}
	if ruleCount != 10 {
		t.Fatalf("legacy-migrated db expected 10 rules, got %d", ruleCount)
	}
}

func platformKey(p *domain.Platform) string {
	if p == nil {
		return ""
	}
	return strings.TrimSpace(string(*p))
}

func mustSeedSafetyAccount(t *testing.T, store *Store, platform domain.Platform, externalID string) string {
	t.Helper()
	account, err := store.UpsertAccount(t.Context(), UpsertAccountParams{
		Platform:          platform,
		DisplayName:       "Safety Test " + string(platform),
		ExternalAccountID: externalID,
		AuthMethod:        domain.AuthMethodStatic,
		Status:            domain.AccountStatusConnected,
	})
	if err != nil {
		t.Fatalf("seed account: %v", err)
	}
	return account.ID
}

func testNow() time.Time {
	return time.Now().UTC().Add(10 * time.Minute).Truncate(time.Second)
}
