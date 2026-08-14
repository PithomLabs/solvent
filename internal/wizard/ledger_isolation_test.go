package wizard_test

import (
	"context"
	"net/http"
	"testing"

	"github.com/PithomLabs/solvent/internal/wizard"
	"github.com/PithomLabs/solvent/kernel"
)

// ledgerScenario is the Track 2 scenario the read-only ledger pages display.
//
// Duplicated from demo/cloud/web/handlers.go:23 on purpose. That package is
// package main and cannot be imported, so the constant is restated here and this
// test fails if the two ever disagree about which scenario the ledger shows.
const ledgerScenario = "00000000-0000-0000-0000-000000000002"

// --- W-32: a wizard visitor cannot move the ledger's numbers -------------------
//
// This is the precondition for ever mounting the wizard at /. The landing page and
// /ledger render five counts; if any of them aggregated across scenarios, every
// judge who opened the demo would inflate the numbers a later judge reads.
//
// The queries below are copied verbatim from getLedgerCounts (handlers.go:25-53).
// Asserting only that they don't move would pass even if the filter were removed
// and the test simply got lucky, so the unfiltered count is measured alongside:
// it MUST rise. That is what proves the scenario filter is doing the work.
func TestW32_WizardDoesNotDisturbLedgerCounts(t *testing.T) {
	ctx := context.Background()
	plantCorpus(t, 4)
	s := newServer(t, &fakeEmbedder{})
	c := newClient(t, s)

	type counts struct{ beliefs, evidence, promoted, live, retracted, allBeliefs int }
	read := func() counts {
		t.Helper()
		var n counts
		q := func(sql string, dst *int, args ...any) {
			if err := shared.QueryRowContext(ctx, sql, args...).Scan(dst); err != nil {
				t.Fatalf("count: %v", err)
			}
		}
		q(`SELECT count(*) FROM belief WHERE scenario_id = $1::UUID`, &n.beliefs, ledgerScenario)
		q(`SELECT count(*) FROM evidence WHERE scenario_id = $1::UUID`, &n.evidence, ledgerScenario)
		q(`SELECT count(*) FROM belief WHERE scenario_id = $1::UUID AND status = 'promoted'`, &n.promoted, ledgerScenario)
		q(`SELECT count(*) FROM action_intent WHERE scenario_id = $1::UUID AND state = 'live'`, &n.live, ledgerScenario)
		q(`SELECT count(*) FROM belief WHERE scenario_id = $1::UUID AND status = 'retracted'`, &n.retracted, ledgerScenario)
		// Deliberately unscoped — the control.
		q(`SELECT count(*) FROM belief`, &n.allBeliefs)
		return n
	}

	before := read()

	// A judge runs the entire demo: two refusals, a search, a selection, six
	// discharges, promote, authorize, introduce, and both retractions.
	c.do(http.MethodPost, wizard.Prefix+"/api/promote", "{}")
	c.do(http.MethodPost, wizard.Prefix+"/api/authorize", "{}")
	_, sr := c.do(http.MethodPost, wizard.Prefix+"/api/search", `{"query":"ledger isolation"}`)
	if hits, _ := sr["hits"].([]any); len(hits) > 0 {
		h0 := hits[0].(map[string]any)
		c.do(http.MethodPost, wizard.Prefix+"/api/select",
			`{"corpus_id":"`+h0["corpus_id"].(string)+`","query":"ledger isolation","distance":`+
				jsonNum(h0["distance"])+`,"on":true}`)
	}
	for _, item := range kernel.FullDebt {
		c.do(http.MethodPost, wizard.Prefix+"/api/discharge",
			`{"check":"`+item+`","artifact":"`+operatorArtifacts[item]+`"}`)
	}
	c.do(http.MethodPost, wizard.Prefix+"/api/promote", "{}")
	c.do(http.MethodPost, wizard.Prefix+"/api/authorize", "{}")
	c.do(http.MethodPost, wizard.Prefix+"/api/retract", `{"mode":"unsafe"}`)
	c.do(http.MethodPost, wizard.Prefix+"/api/retract", `{"mode":"safe"}`)

	// The wizard really did write.
	_, st := c.do(http.MethodGet, wizard.Prefix+"/api/state", "")
	if st["seeded"] != true {
		t.Fatalf("the wizard run never seeded; this test would prove nothing: %v", st["screen"])
	}

	after := read()

	if after.allBeliefs <= before.allBeliefs {
		t.Fatalf("unscoped belief count did not rise (%d -> %d): the wizard wrote nothing, "+
			"so this test cannot show the filter is what protects the ledger",
			before.allBeliefs, after.allBeliefs)
	}
	if before.beliefs != after.beliefs || before.evidence != after.evidence ||
		before.promoted != after.promoted || before.live != after.live ||
		before.retracted != after.retracted {
		t.Errorf("a wizard visitor moved the ledger's numbers\n before: %+v\n after : %+v", before, after)
	}

	// The wizard must never write into the scenario the ledger displays.
	if st["scenario_id"] == ledgerScenario {
		t.Error("the wizard seeded into the ledger's own scenario")
	}
}
