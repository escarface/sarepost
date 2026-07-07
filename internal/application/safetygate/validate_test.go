package safetygate

import (
	"strings"
	"testing"

	"github.com/escarface/sarepost/internal/domain"
)

// TestValidateRuleRejectsUnknownKindAndSeverity pins the use-case-boundary
// guard that stops a typo'd rule from being persisted: an unknown Kind makes
// the evaluator's default return PASS (rule never applies), and an unknown
// Severity routes failures into non-blocking Notes (rule never blocks). Both
// must be rejected at the application boundary so every surface stays
// consistent.
func TestValidateRuleRejectsUnknownKindAndSeverity(t *testing.T) {
	cases := []struct {
		name    string
		rule    domain.SafetyRule
		wantErr string
	}{
		{
			name:    "unknown kind typo banned_term",
			rule:    domain.SafetyRule{Kind: domain.SafetyRuleKind("banned_term"), Severity: domain.SeverityBlock},
			wantErr: "kind",
		},
		{
			name:    "unknown kind gibberish",
			rule:    domain.SafetyRule{Kind: domain.SafetyRuleKind("nope"), Severity: domain.SeverityBlock},
			wantErr: "kind",
		},
		{
			name:    "unknown severity typo blok",
			rule:    domain.SafetyRule{Kind: domain.RuleBannedTerms, Severity: domain.SafetyRuleSeverity("blok")},
			wantErr: "severity",
		},
		{
			name:    "unknown severity gibberish",
			rule:    domain.SafetyRule{Kind: domain.RuleHashtagMax, Severity: domain.SafetyRuleSeverity("warn")},
			wantErr: "severity",
		},
		{
			name: "valid banned_terms block",
			rule: domain.SafetyRule{Kind: domain.RuleBannedTerms, Severity: domain.SeverityBlock},
		},
		{
			name: "valid length_range review",
			rule: domain.SafetyRule{Kind: domain.RuleLengthRange, Severity: domain.SeverityReview},
		},
		{
			name: "valid hashtag_max block",
			rule: domain.SafetyRule{Kind: domain.RuleHashtagMax, Severity: domain.SeverityBlock},
		},
		{
			name: "valid link_max review",
			rule: domain.SafetyRule{Kind: domain.RuleLinkMax, Severity: domain.SeverityReview},
		},
		{
			name: "valid required_contains block",
			rule: domain.SafetyRule{Kind: domain.RuleRequiredContains, Severity: domain.SeverityBlock},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := ValidateRule(tc.rule)
			if tc.wantErr == "" {
				if err != nil {
					t.Fatalf("expected no error, got %v", err)
				}
				return
			}
			if err == nil {
				t.Fatalf("expected error mentioning %q, got nil", tc.wantErr)
			}
			if !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("error %q must mention %q", err.Error(), tc.wantErr)
			}
		})
	}
}
