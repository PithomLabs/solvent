package belief

import "regexp"

// DebtRule maps an evidence pattern to the debt items it retires.
// The mechanism is domain-agnostic; the entries are etcd-specific for the MVP.
type DebtRule struct {
	Match *regexp.Regexp // pattern matched against evidence.Assertion
	Items []string       // debt items retired by this evidence
}

// DebtMapping maps evidence source types to the debt rules they trigger.
// This table is the only place domain knowledge enters the wiring layer.
// It is a compile-time constant — no configuration, no runtime mutation.
var DebtMapping = map[string][]DebtRule{
	"kev_entry": {
		{Match: regexp.MustCompile(`(?i)vulnerable to`), Items: []string{"needMap"}},
	},
	"release": {
		{Match: regexp.MustCompile(`(?i)release`), Items: []string{"needMap", "needInvariant"}},
	},
	"maintainer_comment": {
		{Match: regexp.MustCompile(`(?i)\b(fixed|fix released|patch available)\b`), Items: []string{"needMap", "needInvariant"}},
		{Match: regexp.MustCompile(`(?i)\b(tested|confirmed)\b`), Items: []string{"needToyCheck", "needNullModel"}},
		{Match: regexp.MustCompile(`(?i)\bno regression\b`), Items: []string{"needNullModel", "needObstruction"}},
		{Match: regexp.MustCompile(`(?i)\b(security review|reviewed by)\b`), Items: []string{"needFaithfulnessReview"}},
	},
	"github_pr": {
		{Match: regexp.MustCompile(`(?i)\bfix\b`), Items: []string{"needMap", "needInvariant"}},
	},
	"github_advisory": {
		{Match: regexp.MustCompile(`(?i)vulnerable to`), Items: []string{"needMap"}},
	},
}

// DebtItemsForEvidence returns the debt items that the given evidence retires.
// Returns nil if no rule matches. When multiple rules match, the result is the
// union of all matched rules' items in table order, with duplicates removed.
func DebtItemsForEvidence(sourceType, assertion string) []string {
	rules, ok := DebtMapping[sourceType]
	if !ok {
		return nil
	}
	seen := make(map[string]bool)
	var result []string
	for _, r := range rules {
		if r.Match.MatchString(assertion) {
			for _, item := range r.Items {
				if !seen[item] {
					seen[item] = true
					result = append(result, item)
				}
			}
		}
	}
	return result
}
