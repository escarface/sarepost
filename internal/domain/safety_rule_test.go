package domain

import (
	"encoding/json"
	"reflect"
	"sort"
	"strings"
	"testing"
	"time"
)

func TestSafetyRuleKindEnumValues(t *testing.T) {
	cases := map[SafetyRuleKind]string{
		RuleBannedTerms:      "banned_terms",
		RuleLengthRange:      "length_range",
		RuleHashtagMax:       "hashtag_max",
		RuleLinkMax:          "link_max",
		RuleRequiredContains: "required_contains",
	}
	for kind, want := range cases {
		if string(kind) != want {
			t.Fatalf("rule kind %v = %q, want %q", kind, string(kind), want)
		}
	}
}

func TestSafetyRuleSeverityEnumValues(t *testing.T) {
	if string(SeverityBlock) != "block" {
		t.Fatalf("SeverityBlock = %q, want block", string(SeverityBlock))
	}
	if string(SeverityReview) != "review" {
		t.Fatalf("SeverityReview = %q, want review", string(SeverityReview))
	}
}

func TestSafetyVerdictStatusEnumValues(t *testing.T) {
	cases := map[SafetyVerdictStatus]string{
		VerdictApproved:    "approved",
		VerdictNeedsManual: "needs_manual_review",
	}
	for status, want := range cases {
		if string(status) != want {
			t.Fatalf("verdict status %v = %q, want %q", status, string(status), want)
		}
	}
}

func TestSafetyRuleParamsJSONRoundTrip(t *testing.T) {
	params := SafetyRuleParams{
		BannedPatterns: []string{"spam\\b", "buy now"},
		MinLen:         1,
		MaxLen:         280,
		HashtagMax:     30,
		LinkMax:        1,
		Needles:        []string{"cta:", "learn more"},
	}
	raw, err := json.Marshal(params)
	if err != nil {
		t.Fatalf("marshal params: %v", err)
	}
	var decoded SafetyRuleParams
	if err := json.Unmarshal(raw, &decoded); err != nil {
		t.Fatalf("unmarshal params: %v", err)
	}
	if !reflect.DeepEqual(decoded, params) {
		t.Fatalf("round-trip mismatch:\n got=%+v\nwant=%+v", decoded, params)
	}
}

func TestSafetyRuleParamsJSONRoundTripEmpty(t *testing.T) {
	var params SafetyRuleParams
	raw, err := json.Marshal(params)
	if err != nil {
		t.Fatalf("marshal empty params: %v", err)
	}
	var decoded SafetyRuleParams
	if err := json.Unmarshal(raw, &decoded); err != nil {
		t.Fatalf("unmarshal empty params: %v", err)
	}
	if !reflect.DeepEqual(decoded, params) {
		t.Fatalf("empty round-trip mismatch:\n got=%+v\nwant=%+v", decoded, params)
	}
}

func TestBuildAuditedAsOrderingStableByRuleID(t *testing.T) {
	// Entries provided out of order; output must be sorted by rule id.
	entries := []AuditEntry{
		{RuleID: "sft_zzz", Kind: RuleBannedTerms, Outcome: AuditPass},
		{RuleID: "sft_aaa", Kind: RuleLengthRange, Outcome: AuditPass},
		{RuleID: "sft_mmm", Kind: RuleHashtagMax, Outcome: AuditFail},
	}
	got := BuildAuditedAs(entries)
	want := "length_range:pass;hashtag_max:fail;banned_terms:pass"
	if got != want {
		t.Fatalf("BuildAuditedAs = %q, want %q", got, want)
	}

	// Verify the ordering helper is idempotent across shuffled inputs.
	shuffled := []AuditEntry{
		{RuleID: "sft_mmm", Kind: RuleHashtagMax, Outcome: AuditFail},
		{RuleID: "sft_aaa", Kind: RuleLengthRange, Outcome: AuditPass},
		{RuleID: "sft_zzz", Kind: RuleBannedTerms, Outcome: AuditPass},
	}
	if got2 := BuildAuditedAs(shuffled); got2 != want {
		t.Fatalf("BuildAuditedAs not stable across shuffle: got %q, want %q", got2, want)
	}
}

func TestBuildAuditedAsSkippedOutcome(t *testing.T) {
	entries := []AuditEntry{
		{RuleID: "sft_aaa", Kind: RuleLinkMax, Outcome: AuditSkipped},
	}
	if got := BuildAuditedAs(entries); got != "link_max:skipped" {
		t.Fatalf("BuildAuditedAs skipped = %q, want link_max:skipped", got)
	}
}

func TestBuildAuditedAsEmpty(t *testing.T) {
	if got := BuildAuditedAs(nil); got != "" {
		t.Fatalf("BuildAuditedAs(nil) = %q, want empty", got)
	}
}

func TestSafetyRuleAppliesToPlatform(t *testing.T) {
	now := time.Now().UTC()
	globalRule := SafetyRule{ID: "sft_g", Kind: RuleLinkMax, Severity: SeverityBlock, Enabled: true, CreatedAt: now, UpdatedAt: now}
	xPlatform := PlatformX
	xRule := SafetyRule{ID: "sft_x", Kind: RuleLengthRange, Platform: &xPlatform, Severity: SeverityBlock, Enabled: true, CreatedAt: now, UpdatedAt: now}

	if !globalRule.AppliesTo(PlatformX) {
		t.Fatalf("global rule should apply to x")
	}
	if !globalRule.AppliesTo(PlatformLinkedIn) {
		t.Fatalf("global rule should apply to linkedin")
	}
	if !xRule.AppliesTo(PlatformX) {
		t.Fatalf("x-scoped rule should apply to x")
	}
	if xRule.AppliesTo(PlatformLinkedIn) {
		t.Fatalf("x-scoped rule should not apply to linkedin")
	}
}

func TestSafetyRuleDisabledNeverApplies(t *testing.T) {
	rule := SafetyRule{ID: "sft_off", Kind: RuleBannedTerms, Severity: SeverityBlock, Enabled: false}
	if rule.AppliesTo(PlatformX) {
		t.Fatalf("disabled rule should not apply even when platform matches")
	}
}

func TestSortAuditEntriesByRuleID(t *testing.T) {
	entries := []AuditEntry{
		{RuleID: "sft_c", Kind: RuleHashtagMax, Outcome: AuditPass},
		{RuleID: "sft_a", Kind: RuleLinkMax, Outcome: AuditFail},
		{RuleID: "sft_b", Kind: RuleBannedTerms, Outcome: AuditPass},
	}
	sort.Sort(byRuleID(entries))
	if entries[0].RuleID != "sft_a" || entries[1].RuleID != "sft_b" || entries[2].RuleID != "sft_c" {
		t.Fatalf("sort by rule id wrong order: %+v", entries)
	}
}

func TestNewSafetyRuleIDFormat(t *testing.T) {
	id, err := NewSafetyRuleID()
	if err != nil {
		t.Fatalf("NewSafetyRuleID: %v", err)
	}
	if !strings.HasPrefix(id, "sft_") || len(id) <= len("sft_") {
		t.Fatalf("NewSafetyRuleID = %q, want sft_ prefix", id)
	}
	id2, _ := NewSafetyRuleID()
	if id == id2 {
		t.Fatalf("NewSafetyRuleID must be unique, got duplicate %q", id)
	}
}
