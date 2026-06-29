package safetygate

import (
	"regexp"
	"strings"
	"unicode/utf8"

	"github.com/escarface/sarepost/internal/domain"
)

// evaluateRule dispatches to the evaluator for the rule's kind and returns
// (passed, detail). detail is a short human-readable failure reason (empty on
// pass).
func evaluateRule(post domain.Post, rule domain.SafetyRule) (bool, string) {
	switch rule.Kind {
	case domain.RuleBannedTerms:
		return evaluateBannedTerms(post, rule)
	case domain.RuleLengthRange:
		return evaluateLengthRange(post, rule)
	case domain.RuleHashtagMax:
		return evaluateHashtagMax(post, rule)
	case domain.RuleLinkMax:
		return evaluateLinkMax(post, rule)
	case domain.RuleRequiredContains:
		return evaluateRequiredContains(post, rule)
	default:
		// Unknown kinds are treated as a non-blocking pass so a bad rule never
		// silently blocks the pipeline; the kind is still audited.
		return true, ""
	}
}

func evaluateBannedTerms(post domain.Post, rule domain.SafetyRule) (bool, string) {
	for _, raw := range rule.Params.BannedPatterns {
		pattern := strings.TrimSpace(raw)
		if pattern == "" {
			continue
		}
		re, err := regexp.Compile(pattern)
		if err != nil {
			// Invalid regex: fail safe (block) and report the bad pattern.
			return false, "invalid pattern '" + pattern + "'"
		}
		if match := re.FindString(post.Text); match != "" {
			return false, "matched '" + truncateSnippet(match, 40) + "'"
		}
	}
	return true, ""
}

func evaluateLengthRange(post domain.Post, rule domain.SafetyRule) (bool, string) {
	length := utf8.RuneCountInString(post.Text)
	if rule.Params.MaxLen > 0 && length > rule.Params.MaxLen {
		return false, "length " + itoa(length) + " > max " + itoa(rule.Params.MaxLen)
	}
	if rule.Params.MinLen > 0 && length < rule.Params.MinLen {
		return false, "length " + itoa(length) + " < min " + itoa(rule.Params.MinLen)
	}
	return true, ""
}

func evaluateHashtagMax(post domain.Post, rule domain.SafetyRule) (bool, string) {
	count := countHashtags(post.Text)
	if count > rule.Params.HashtagMax {
		return false, "hashtags " + itoa(count) + " > max " + itoa(rule.Params.HashtagMax)
	}
	return true, ""
}

func evaluateLinkMax(post domain.Post, rule domain.SafetyRule) (bool, string) {
	count := countLinks(post.Text)
	if count > rule.Params.LinkMax {
		return false, "links " + itoa(count) + " > max " + itoa(rule.Params.LinkMax)
	}
	return true, ""
}

func evaluateRequiredContains(post domain.Post, rule domain.SafetyRule) (bool, string) {
	if len(rule.Params.Needles) == 0 {
		return true, ""
	}
	for _, needle := range rule.Params.Needles {
		if strings.Contains(post.Text, needle) {
			return true, ""
		}
	}
	return false, "missing required substring"
}

var (
	hashtagRe = regexp.MustCompile(`(?im)[\x{FF03}#][\p{L}\p{N}_]+`)
	linkRe    = regexp.MustCompile(`(?i)\bhttps?://\S+`)
)

func countHashtags(text string) int {
	return len(hashtagRe.FindAllString(text, -1))
}

func countLinks(text string) int {
	return len(linkRe.FindAllString(text, -1))
}

func truncateSnippet(s string, max int) string {
	if max <= 0 || utf8.RuneCountInString(s) <= max {
		return s
	}
	r := []rune(s)
	if max < 4 {
		return string(r[:max])
	}
	return string(r[:max-3]) + "..."
}

// itoa avoids a strconv import for tiny integer formatting.
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	negative := n < 0
	if negative {
		n = -n
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	if negative {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}
