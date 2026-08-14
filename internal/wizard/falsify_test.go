package wizard_test

import (
	"context"
	"net/http"
	"strings"
	"testing"

	"github.com/PithomLabs/solvent/internal/corpus"
	"github.com/PithomLabs/solvent/internal/wizard"
	"github.com/PithomLabs/solvent/kernel"
)

// atFalsify drives a scenario all the way to the FALSIFY screen: refusals logged, all
// six checks discharged, promoted, authorised, live intent standing.
func atFalsify(t *testing.T, s *wizard.Server) (sid, bid string) {
	t.Helper()
	ctx := context.Background()
	sid = seeded(t, s)
	bid = beliefOf(t, s, sid)

	s.Promote(ctx, sid, bid)
	s.Authorize(ctx, sid, bid)
	selectHits(t, s, bid, "falsify setup", 2)
	for _, item := range kernel.FullDebt {
		if v := s.Discharge(ctx, sid, bid, item, operatorArtifacts[item]); !v.OK {
			t.Fatalf("setup: discharge %s: %+v", item, v)
		}
	}
	if v := s.Promote(ctx, sid, bid); !v.OK {
		t.Fatalf("setup: promote: %+v", v)
	}
	if v := s.Authorize(ctx, sid, bid); !v.OK {
		t.Fatalf("setup: authorize: %+v", v)
	}
	st, _ := s.State(ctx, sid)
	if st.Screen != wizard.ScreenFalsify {
		t.Fatalf("setup left screen = %q, want FALSIFY", st.Screen)
	}
	return sid, bid
}

// --- W-21: the URL allowlist ---------------------------------------------------

func TestW21_IssueURLAllowlist(t *testing.T) {
	ctx := context.Background()
	plantCorpus(t, 4)
	s := newServer(t, &fakeEmbedder{})
	sid, bid := atFalsify(t, s)

	bad := []string{
		"",
		"13001",
		"github.com/etcd-io/etcd/issues/13001",
		"http://github.com/etcd-io/etcd/issues/13001",              // not https
		"https://github.com/etcd-io/etcd/pull/13001",               // a PR, not an issue
		"https://github.com/etcd-io/etcd/issues/13001/",            // trailing slash
		"https://github.com/etcd-io/etcd/issues/abc",               // non-numeric
		"https://github.com/someone-else/etcd/issues/13001",        // wrong repo
		"https://evil.invalid/github.com/etcd-io/etcd/issues/1300", // prefix smuggling
		"https://github.com/etcd-io/etcd/issues/13001?x=1",         // query string
	}
	for _, u := range bad {
		if _, err := s.Introduce(ctx, sid, bid, u); err == nil {
			t.Errorf("accepted %q; the allowlist is not anchored", u)
		}
	}

	// A well-formed URL for an issue that exists in the corpus is accepted.
	good := "https://github.com/etcd-io/etcd/issues/13001"
	if _, err := s.Introduce(ctx, sid, bid, good); err != nil {
		t.Fatalf("rejected a valid corpus issue %q: %v", good, err)
	}
}

// --- W-22: resolution comes from the corpus, not the network -------------------

func TestW22_IssueResolvesFromCorpusOnly(t *testing.T) {
	ctx := context.Background()
	plantCorpus(t, 4)
	s := newServer(t, &fakeEmbedder{})
	sid, bid := atFalsify(t, s)

	// Well-formed, but not ingested. Must be refused rather than fetched.
	_, err := s.Introduce(ctx, sid, bid, "https://github.com/etcd-io/etcd/issues/999999")
	if err == nil {
		t.Fatal("introduced an issue that is not in the corpus")
	}
	if !strings.Contains(err.Error(), "not in the ingested corpus") {
		t.Errorf("error should say the issue is not ingested, got: %v", err)
	}

	intro, err := s.Introduce(ctx, sid, bid, "https://github.com/etcd-io/etcd/issues/13002")
	if err != nil {
		t.Fatalf("Introduce: %v", err)
	}
	if intro.IssueNumber != 13002 {
		t.Errorf("issue_number = %d, want 13002", intro.IssueNumber)
	}
	if intro.Title == "" || intro.URL == "" {
		t.Error("title and url must come from the corpus row")
	}
	if intro.Relation != corpus.RelationContradicts {
		t.Errorf("relation = %q, want contradicts", intro.Relation)
	}
	// Measured against the ANCESTOR's claim, and attached to the ancestor.
	//
	// #13766 refutes "no known data-integrity defects" on its face; it says nothing
	// directly about whether upgrading is safe. Citing the descendant would attach the
	// evidence one hop from what it contradicts and measure the distance against the
	// wrong sentence. It is also what makes the retraction Tier 1: the belief being
	// retracted is the ancestor.
	if intro.AgainstClaim != wizard.AncestorClaim {
		t.Errorf("against_claim = %q, want the ancestor's claim %q", intro.AgainstClaim, wizard.AncestorClaim)
	}
	if intro.Distance < 0 {
		t.Errorf("distance = %v, want a real measurement", intro.Distance)
	}

	// Written as a contradicts citation, on the ancestor.
	var rel, citedClaim string
	if err := shared.QueryRowContext(ctx, `
		SELECT c.relation, b.claim FROM belief_corpus_citation c
		  JOIN corpus_issue i ON i.id = c.corpus_id
		  JOIN belief b       ON b.id = c.belief_id
		 WHERE b.scenario_id = $1::UUID AND i.issue_number = 13002`, sid).Scan(&rel, &citedClaim); err != nil {
		t.Fatalf("read citation: %v", err)
	}
	if rel != corpus.RelationContradicts {
		t.Errorf("stored relation = %q, want contradicts", rel)
	}
	if citedClaim != wizard.AncestorClaim {
		t.Errorf("citation attached to %q, want the ancestor", citedClaim)
	}
	// The descendant keeps its own 'considered' receipt and gains no contradiction.
	var onDescendant int
	if err := shared.QueryRowContext(ctx, `
		SELECT count(*) FROM belief_corpus_citation
		 WHERE belief_id = $1::UUID AND relation = 'contradicts'`, bid).Scan(&onDescendant); err != nil {
		t.Fatalf("count descendant contradictions: %v", err)
	}
	if onDescendant != 0 {
		t.Errorf("descendant carries %d contradicts citations, want 0", onDescendant)
	}
}

// --- W-23: the unsafe retract is refused and rolls back ------------------------

// The demo's climax. Retracting the ANCESTOR while the judge's own intent is live must
// be refused by the database, with nothing moved.
func TestW23_UnsafeRetractIsRefusedAndRollsBack(t *testing.T) {
	ctx := context.Background()
	plantCorpus(t, 4)
	s := newServer(t, &fakeEmbedder{})
	sid, bid := atFalsify(t, s)
	if _, err := s.Introduce(ctx, sid, bid, "https://github.com/etcd-io/etcd/issues/13003"); err != nil {
		t.Fatalf("Introduce: %v", err)
	}

	before, _ := s.State(ctx, sid)

	v := s.Retract(ctx, sid, bid, wizard.ModeUnsafe)
	if v.OK {
		t.Fatal("the unsafe retract COMMITTED; live_requires_promoted did not fire")
	}
	if !v.CascadeRefusal() {
		t.Fatalf("not classified as the cascade refusal: sqlstate=%q constraint=%q", v.SQLState, v.Constraint)
	}

	after, _ := s.State(ctx, sid)
	if after.Status != before.Status {
		t.Errorf("descendant status moved %q -> %q on a refused retract", before.Status, after.Status)
	}
	if after.AncestorStatus != before.AncestorStatus {
		t.Errorf("ancestor status moved %q -> %q on a refused retract", before.AncestorStatus, after.AncestorStatus)
	}
	if after.IntentState != "live" {
		t.Errorf("intent state = %q after a refused retract, want live", after.IntentState)
	}
	if after.Audit != 0 {
		t.Errorf("audit = %d, want 0", after.Audit)
	}
	if after.Screen != wizard.ScreenFalsify {
		t.Errorf("screen = %q, want FALSIFY", after.Screen)
	}

	// And it is on the record with its real constraint name.
	var found bool
	for _, r := range after.Refusals {
		if r.Statement == wizard.StmtRetractUnsafe && r.Constraint == "live_requires_promoted" {
			found = true
		}
	}
	if !found {
		t.Error("the cascade refusal was not logged with its constraint name")
	}
}

// --- W-24: 23514 alone is not the cascade refusal ------------------------------

// promoted_is_debt_free is also 23514. Classifying on the code alone would let screen
// 1's refusal be narrated as screen 3's climax.
func TestW24_DebtFreeRefusalIsNotTheCascadeRefusal(t *testing.T) {
	ctx := context.Background()
	plantCorpus(t, 2)
	s := newServer(t, &fakeEmbedder{})
	sid := seeded(t, s)
	bid := beliefOf(t, s, sid)

	promote := s.Promote(ctx, sid, bid)
	if promote.SQLState != "23514" {
		t.Fatalf("expected a 23514 from promote, got %q", promote.SQLState)
	}
	if promote.Constraint != "promoted_is_debt_free" {
		t.Fatalf("expected promoted_is_debt_free, got %q", promote.Constraint)
	}
	if promote.CascadeRefusal() {
		t.Error("a 23514 · promoted_is_debt_free was classified as the cascade refusal")
	}

	// And the real one is.
	cascade := wizard.Verdict{SQLState: "23514", Constraint: "live_requires_promoted"}
	if !cascade.CascadeRefusal() {
		t.Error("23514 · live_requires_promoted was not classified as the cascade refusal")
	}
	// A commit is never a refusal, whatever it carries.
	ok := wizard.Verdict{OK: true, SQLState: "23514", Constraint: "live_requires_promoted"}
	if ok.CascadeRefusal() {
		t.Error("an OK verdict was classified as a refusal")
	}
}

// --- W-25: the safe cascade commits, and crosses the edge ----------------------

func TestW25_SafeCascadeCommits(t *testing.T) {
	ctx := context.Background()
	plantCorpus(t, 4)
	s := newServer(t, &fakeEmbedder{})
	sid, bid := atFalsify(t, s)
	if _, err := s.Introduce(ctx, sid, bid, "https://github.com/etcd-io/etcd/issues/13004"); err != nil {
		t.Fatalf("Introduce: %v", err)
	}
	// The refusal first, as the demo does.
	if v := s.Retract(ctx, sid, bid, wizard.ModeUnsafe); v.OK {
		t.Fatal("unsafe retract committed")
	}

	v := s.Retract(ctx, sid, bid, wizard.ModeSafe)
	if !v.OK {
		t.Fatalf("cancel-then-retract was refused: %+v", v)
	}
	// Both beliefs: the traversal crossed the seeded edge. That is what makes this Tier 1
	// rather than a single-belief retraction.
	if !strings.Contains(v.Detail, "2 belief(s)") {
		t.Errorf("detail = %q, want it to report 2 beliefs retracted", v.Detail)
	}

	st, err := s.State(ctx, sid)
	if err != nil {
		t.Fatalf("State: %v", err)
	}
	if st.Status != "retracted" {
		t.Errorf("descendant status = %q, want retracted", st.Status)
	}
	if st.AncestorStatus != "retracted" {
		t.Errorf("ancestor status = %q, want retracted", st.AncestorStatus)
	}
	if st.IntentState != "cancelled" {
		t.Errorf("intent state = %q, want cancelled", st.IntentState)
	}
	if st.Audit != 0 {
		t.Errorf("live_on_nonpromoted = %d, want 0", st.Audit)
	}
	if st.Screen != wizard.ScreenFalsify {
		t.Errorf("screen = %q, want terminal FALSIFY", st.Screen)
	}

	// Reload lands on the same terminal state.
	again, err := s.State(ctx, sid)
	if err != nil {
		t.Fatalf("State (reload): %v", err)
	}
	if again.Screen != st.Screen || again.Status != st.Status {
		t.Errorf("reload changed the terminal state: %q/%q -> %q/%q",
			st.Screen, st.Status, again.Screen, again.Status)
	}
}

// --- W-26: the chronology is the full causal trail -----------------------------

func TestW26_RefusalChronology(t *testing.T) {
	ctx := context.Background()
	plantCorpus(t, 4)
	s := newServer(t, &fakeEmbedder{})
	sid, bid := atFalsify(t, s)
	if _, err := s.Introduce(ctx, sid, bid, "https://github.com/etcd-io/etcd/issues/13001"); err != nil {
		t.Fatalf("Introduce: %v", err)
	}
	s.Retract(ctx, sid, bid, wizard.ModeUnsafe)

	st, err := s.State(ctx, sid)
	if err != nil {
		t.Fatalf("State: %v", err)
	}

	// The three refusals the demo is built on, in the order they happened.
	want := []struct{ statement, sqlstate, constraint string }{
		{wizard.StmtPromote, "23514", "promoted_is_debt_free"},
		{wizard.StmtAuthorize, "23503", "gate"},
		{wizard.StmtRetractUnsafe, "23514", "live_requires_promoted"},
	}
	var got []wizard.Refusal
	for _, r := range st.Refusals {
		if r.Statement != wizard.StmtDischarge {
			got = append(got, r)
		}
	}
	if len(got) != len(want) {
		t.Fatalf("chronology has %d entries, want %d: %+v", len(got), len(want), got)
	}
	for i, w := range want {
		if got[i].Statement != w.statement || got[i].SQLState != w.sqlstate || got[i].Constraint != w.constraint {
			t.Errorf("chronology[%d] = %s %s %s, want %s %s %s",
				i, got[i].Statement, got[i].SQLState, got[i].Constraint, w.statement, w.sqlstate, w.constraint)
		}
		if got[i].LoggedAt == "" {
			t.Errorf("chronology[%d] has no timestamp", i)
		}
	}
}

// --- W-27: Screen 3 over HTTP, and /proof --------------------------------------

func TestW27_Screen3OverHTTP(t *testing.T) {
	plantCorpus(t, 4)
	s := newServer(t, &fakeEmbedder{})
	c := newClient(t, s)
	c.do(http.MethodGet, wizard.Prefix, "")

	// Drive to FALSIFY over HTTP.
	c.do(http.MethodPost, wizard.Prefix+"/api/promote", "{}")
	c.do(http.MethodPost, wizard.Prefix+"/api/authorize", "{}")
	_, sr := c.do(http.MethodPost, wizard.Prefix+"/api/search", `{"query":"http screen3"}`)
	hits, _ := sr["hits"].([]any)
	// Two selections: each retrieval check consumes its own citation.
	for i := 0; i < 2 && i < len(hits); i++ {
		h := hits[i].(map[string]any)
		c.do(http.MethodPost, wizard.Prefix+"/api/select",
			`{"corpus_id":"`+h["corpus_id"].(string)+`","query":"http screen3","distance":`+jsonNum(h["distance"])+`,"on":true}`)
	}
	for _, item := range kernel.FullDebt {
		c.do(http.MethodPost, wizard.Prefix+"/api/discharge",
			`{"check":"`+item+`","artifact":"`+operatorArtifacts[item]+`"}`)
	}
	c.do(http.MethodPost, wizard.Prefix+"/api/promote", "{}")
	c.do(http.MethodPost, wizard.Prefix+"/api/authorize", "{}")

	// A bad URL is a 400, not a 500.
	if w, _ := c.do(http.MethodPost, wizard.Prefix+"/api/introduce", `{"issue_url":"https://evil.invalid/x"}`); w.Code != http.StatusBadRequest {
		t.Errorf("bad issue URL = %d, want 400", w.Code)
	}

	_, iv := c.do(http.MethodPost, wizard.Prefix+"/api/introduce",
		`{"issue_url":"https://github.com/etcd-io/etcd/issues/13001"}`)
	if iv["relation"] != "contradicts" {
		t.Fatalf("introduce verdict = %v", iv)
	}

	// The refusal is a 200 carrying a verdict.
	w, rv := c.do(http.MethodPost, wizard.Prefix+"/api/retract", `{"mode":"unsafe"}`)
	if w.Code != http.StatusOK {
		t.Fatalf("unsafe retract HTTP = %d, want 200", w.Code)
	}
	if rv["ok"] == true {
		t.Fatal("unsafe retract committed over HTTP")
	}
	if rv["sqlstate"] != "23514" || rv["constraint"] != "live_requires_promoted" {
		t.Errorf("unsafe retract verdict = %v", rv)
	}

	_, sv := c.do(http.MethodPost, wizard.Prefix+"/api/retract", `{"mode":"safe"}`)
	if sv["ok"] != true {
		t.Fatalf("safe retract refused over HTTP: %v", sv)
	}

	_, st := c.do(http.MethodGet, wizard.Prefix+"/api/state", "")
	if st["status"] != "retracted" || st["ancestor_status"] != "retracted" {
		t.Errorf("terminal state = %v / %v", st["status"], st["ancestor_status"])
	}
	if n, _ := st["live_on_nonpromoted"].(float64); n != 0 {
		t.Errorf("audit = %v, want 0", st["live_on_nonpromoted"])
	}

	// An unknown mode is refused, not silently treated as one of the two.
	if _, uv := c.do(http.MethodPost, wizard.Prefix+"/api/retract", `{"mode":"whatever"}`); uv["ok"] == true {
		t.Error("an unknown retract mode succeeded")
	}
}

// --- W-28: the new debt vocabulary reaches a fresh seed ------------------------

// The migration's user-visible end. A freshly seeded belief must carry the six
// deployment obligations, in order — not the inherited physics vocabulary.
func TestW28_FreshSeedCarriesTheDeploymentVocabulary(t *testing.T) {
	ctx := context.Background()
	s := newServer(t, &fakeEmbedder{})
	sid := seeded(t, s)

	want := []string{
		"needProvenanceCheck", "needContradictionSweep", "needBlastRadius",
		"needRollbackPlan", "needVersionPin", "needOperatorSignoff",
	}

	var got string
	if err := shared.QueryRowContext(ctx, `
		SELECT array_to_string(debt, ',') FROM belief
		 WHERE scenario_id = $1::UUID AND claim = $2`, sid, wizard.DescendantClaim).Scan(&got); err != nil {
		t.Fatalf("read debt: %v", err)
	}
	if got != strings.Join(want, ",") {
		t.Errorf("fresh descendant debt =\n  %s\nwant\n  %s", got, strings.Join(want, ","))
	}

	// And the screen renders those names, with no parenthesised alias left over.
	st, err := s.State(ctx, sid)
	if err != nil {
		t.Fatalf("State: %v", err)
	}
	if len(st.Checks) != len(want) {
		t.Fatalf("checks = %d, want %d", len(st.Checks), len(want))
	}
	for i, c := range st.Checks {
		if c.Item != want[i] {
			t.Errorf("check[%d].Item = %q, want %q", i, c.Item, want[i])
		}
		if c.Name != c.Item {
			t.Errorf("check[%d] still shows a dual label: Name=%q Item=%q", i, c.Name, c.Item)
		}
		if strings.Contains(c.Name, "(") {
			t.Errorf("check[%d].Name = %q still carries an alias", i, c.Name)
		}
	}
}

// --- W-29: /proof renders the evidence -----------------------------------------

func TestW29_ProofPage(t *testing.T) {
	s := newServer(t, &fakeEmbedder{})
	c := newClient(t, s)

	w, _ := c.do(http.MethodGet, wizard.ProofPath, "")
	if w.Code != http.StatusOK {
		t.Fatalf("GET %s = %d, want 200", wizard.ProofPath, w.Code)
	}
	body := w.Body.String()

	// All three cells, with their observed results.
	for _, want := range []string{"AUDIT = 1", "40001 RETRY_SERIALIZABLE", "23503 · gate", "great-goat"} {
		if !strings.Contains(body, want) {
			t.Errorf("/proof missing %q", want)
		}
	}
	// The narration a Cockroach Labs judge checks first.
	if !strings.Contains(body, "detect → retry → refusal on fresh state") {
		t.Error("/proof does not narrate 40001 as detect -> retry -> refusal")
	}
	if strings.Contains(body, "40001 → transaction refused") {
		t.Error("/proof narrates 40001 as a refusal")
	}
	// Cell 3's scope, stated exactly.
	if !strings.Contains(body, "different rows in a disposable schema") {
		t.Error("/proof does not state cell 3's scope")
	}
	// The transcripts are actually embedded, not placeholders.
	if !strings.Contains(body, "cells_crdb.sh") {
		t.Error("/proof does not contain the isolation transcript")
	}
	if !strings.Contains(body, "TIER1") {
		t.Error("/proof does not contain the tier probe transcript")
	}
	// It touches no scenario: no cookie should be set.
	if len(w.Result().Cookies()) != 0 {
		t.Error("/proof set a cookie; it should be stateless")
	}
}
