package agentjacking

import (
	"strings"
)

// cleanMessage strips the markdown dressing (heading markers, emphasis,
// backticks) from a Sentry message so the plain text can serve as the generic
// evidence assertion. It is deliberately demo-local: ~10 lines, not a parser,
// and it never rewrites message_raw — the verbatim text stays in the domain
// payload for human review.
func cleanMessage(s string) string {
	var b strings.Builder
	for _, line := range strings.Split(s, "\n") {
		b.WriteString(strings.TrimLeft(line, "#"))
		b.WriteString("\n")
	}
	out := b.String()
	out = strings.ReplaceAll(out, "**", "")
	out = strings.ReplaceAll(out, "`", "")
	// Collapse the blank runs the heading strip can leave behind.
	for strings.Contains(out, "\n\n\n") {
		out = strings.ReplaceAll(out, "\n\n\n", "\n\n")
	}
	return strings.TrimSpace(out)
}
