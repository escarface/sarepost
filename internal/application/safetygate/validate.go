package safetygate

import (
	"fmt"

	"github.com/escarface/sarepost/internal/domain"
)

// ValidateRule enforces the domain invariants for a SafetyRule at the
// application/use-case boundary so every surface (HTTP, MCP, CLI) rejects
// invalid rules consistently (parity hard requirement).
//
// An unknown Kind makes the evaluator's default dispatch return PASS — the
// rule never applies. An unknown Severity routes failures into non-blocking
// Notes — the rule never blocks. A typo'd rule would therefore silently never
// block, defeating the safety gate. Validate before persisting.
func ValidateRule(rule domain.SafetyRule) error {
	if !validKind(rule.Kind) {
		return fmt.Errorf("invalid kind %q: must be one of banned_terms, length_range, hashtag_max, link_max, required_contains", rule.Kind)
	}
	if !validSeverity(rule.Severity) {
		return fmt.Errorf("invalid severity %q: must be one of block, review", rule.Severity)
	}
	return nil
}

func validKind(k domain.SafetyRuleKind) bool {
	switch k {
	case domain.RuleBannedTerms, domain.RuleLengthRange, domain.RuleHashtagMax, domain.RuleLinkMax, domain.RuleRequiredContains:
		return true
	}
	return false
}

func validSeverity(s domain.SafetyRuleSeverity) bool {
	switch s {
	case domain.SeverityBlock, domain.SeverityReview:
		return true
	}
	return false
}
