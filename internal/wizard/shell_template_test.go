package wizard_test

import (
	"os"
	"strings"
	"testing"
)

// --- W-35: screen 3's closing sentence may not guess at the citation list -------
//
// The sentence under "What just happened" names the issue that discharged
// needContradictionSweep. It once picked `S.citations.find(c => c.relation ===
// 'considered')` -- the first considered citation -- which was correct only while
// citations were ordered by distance and the nearest happened to be the sweep's. When
// each retrieval check began consuming its own citation the list became selection-
// ordered, "first" became the provenance check's issue, and the closing frame of the
// demo contradicted the receipt printed directly above it.
//
// There is no JS harness here, so this pins the source: the fragile pick must stay gone
// and the sentence must keep reading the check the server bound the citation to. It
// cannot prove the rendering -- TestW34 covers the values themselves -- only that the
// known-bad shortcut has not come back.
func TestW35_TerminalSentenceReadsTheSweepsOwnReceipt(t *testing.T) {
	src, err := os.ReadFile("templates/shell.html")
	if err != nil {
		t.Fatalf("read shell.html: %v", err)
	}
	shell := string(src)

	for _, banned := range []string{
		"S.citations || []).find",
		"c.relation === 'considered'",
	} {
		if strings.Contains(shell, banned) {
			t.Errorf("shell.html picks a single citation with %q; screen 3 must read the "+
				"check the server bound the citation to, not guess at the list", banned)
		}
	}

	for _, required := range []string{
		"c.item === 'needContradictionSweep'",
		"sweep.receipt_issue",
		"sweep.receipt_distance",
	} {
		if !strings.Contains(shell, required) {
			t.Errorf("shell.html no longer contains %q; the closing sentence must name the "+
				"sweep's own citation", required)
		}
	}

	// The wording the demo script depends on.
	if !strings.Contains(shell, "was discharged by citing") {
		t.Error("the 'What just happened' sentence changed shape; the demo script pins its wording")
	}
}
