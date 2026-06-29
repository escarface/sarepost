package domain

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"sort"
	"strings"
	"time"
)

// SafetyRuleKind enumerates the deterministic rule kinds evaluated by the
// auto-approve safety gate.
type SafetyRuleKind string

const (
	RuleBannedTerms      SafetyRuleKind = "banned_terms"
	RuleLengthRange      SafetyRuleKind = "length_range"
	RuleHashtagMax       SafetyRuleKind = "hashtag_max"
	RuleLinkMax          SafetyRuleKind = "link_max"
	RuleRequiredContains SafetyRuleKind = "required_contains"
)

// SafetyRuleSeverity controls how a failing rule affects the verdict.
type SafetyRuleSeverity string

const (
	SeverityBlock  SafetyRuleSeverity = "block"
	SeverityReview SafetyRuleSeverity = "review"
)

// SafetyRuleScope limits rule applicability. The first slice ships global
// rules only; per-campaign overrides are deferred.
type SafetyRuleScope string

const (
	ScopeGlobal SafetyRuleScope = "global"
)

// SafetyRuleParams is the typed, JSON-serializable parameter set for a rule.
// Only the fields relevant to a rule's Kind are populated; the rest stay
// zero-valued and are omitted from JSON.
type SafetyRuleParams struct {
	BannedPatterns []string `json:"banned_patterns,omitempty"`
	MinLen         int      `json:"min_len,omitempty"`
	MaxLen         int      `json:"max_len,omitempty"`
	HashtagMax     int      `json:"hashtag_max,omitempty"`
	LinkMax        int      `json:"link_max,omitempty"`
	Needles        []string `json:"needles,omitempty"`
}

// SafetyRule is a persistent, admin-managed deterministic rule evaluated by the
// safety gate.
type SafetyRule struct {
	ID        string
	Name      string
	Kind      SafetyRuleKind
	Params    SafetyRuleParams
	Scope     SafetyRuleScope
	Platform  *Platform
	Severity  SafetyRuleSeverity
	Enabled   bool
	CreatedAt time.Time
	UpdatedAt time.Time
}

// AppliesTo reports whether an enabled rule should be evaluated for a post on
// the given platform. Disabled rules never apply (short-circuit, zero cost).
func (r SafetyRule) AppliesTo(platform Platform) bool {
	if !r.Enabled {
		return false
	}
	if r.Platform == nil {
		return true
	}
	return *r.Platform == platform
}

// SafetyVerdictStatus is the outcome of evaluating a post against the rule set.
type SafetyVerdictStatus string

const (
	VerdictApproved    SafetyVerdictStatus = "approved"
	VerdictNeedsManual SafetyVerdictStatus = "needs_manual_review"
	VerdictSkipped     SafetyVerdictStatus = "skipped"
)

// AuditOutcome is the per-rule pass/fail/skipped result used to build the
// auditable AutoApprovedReason string.
type AuditOutcome string

const (
	AuditPass    AuditOutcome = "pass"
	AuditFail    AuditOutcome = "fail"
	AuditSkipped AuditOutcome = "skipped"
)

// AuditEntry pairs a rule with its evaluation outcome for audit logging.
type AuditEntry struct {
	RuleID  string
	Kind    SafetyRuleKind
	Outcome AuditOutcome
}

// SafetyVerdict is the result of Evaluate.
type SafetyVerdict struct {
	Status        SafetyVerdictStatus
	FailedBlocks  []string // rule ids with severity=block that failed
	Notes         []string // severity=review failures (non-blocking)
	BlockedReason string   // human-readable reason string for BlockedReason column
	AuditedAs     string   // machine-readable audit summary for AutoApprovedReason
}

// BuildAuditedAs constructs the semicolon-separated `<kind>:<outcome>` audit
// string ordered by rule id (stable regardless of evaluation order).
func BuildAuditedAs(entries []AuditEntry) string {
	if len(entries) == 0 {
		return ""
	}
	ordered := make([]AuditEntry, len(entries))
	copy(ordered, entries)
	sort.Sort(byRuleID(ordered))
	parts := make([]string, 0, len(ordered))
	for _, e := range ordered {
		parts = append(parts, fmt.Sprintf("%s:%s", e.Kind, e.Outcome))
	}
	return strings.Join(parts, ";")
}

type byRuleID []AuditEntry

func (b byRuleID) Len() int           { return len(b) }
func (b byRuleID) Less(i, j int) bool { return b[i].RuleID < b[j].RuleID }
func (b byRuleID) Swap(i, j int)      { b[i], b[j] = b[j], b[i] }

// NewSafetyRuleID generates a fresh rule identifier with the sft_ prefix.
func NewSafetyRuleID() (string, error) {
	var b [10]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	return fmt.Sprintf("sft_%s", hex.EncodeToString(b[:])), nil
}
