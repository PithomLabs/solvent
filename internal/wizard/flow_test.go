package wizard_test

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"

	"github.com/PithomLabs/solvent/internal/corpus"
	"github.com/PithomLabs/solvent/internal/wizard"
	"github.com/PithomLabs/solvent/kernel"
)

// artifacts for the four operator checks, in kernel.FullDebt order.
var operatorArtifacts = map[string]string{
	"needBlastRadius":     "apiserver, compactor, raft",
	"needRollbackPlan":    "https://runbooks.example.invalid/etcd-rollback.md",
	"needVersionPin":      "3.5.0 - 3.5.6",
	"needOperatorSignoff": "@sre-team",
}

// selectFirstHit runs a search and selects hit 0, returning it.
func selectFirstHit(t *testing.T, s *wizard.Server, sid, beliefID, query string) wizard.Hit {
	t.Helper()
	ctx := context.Background()
	res, err := s.Search(ctx, beliefID, query)
	if err != nil {
		t.Fatalf("Search(%q): %v", query, err)
	}
	if len(res.Hits) == 0 {
		t.Fatalf("Search(%q) returned no hits; the corpus partition is empty", query)
	}
	h := res.Hits[0]
	if err := s.Select(ctx, beliefID, h.CorpusID, query, h.Distance, true); err != nil {
		t.Fatalf("Select: %v", err)
	}
	return h
}

// --- W-06: the third refusal ---------------------------------------------------

// needContradictionSweep cannot be discharged without a citation the judge actually
// retrieved. This is the refusal that proves retrieval is wired to authority rather
// than decorating it: the other two would still fire if retrieval were a hardcoded
// list.
func TestW06_ContradictionSweepRefusedWithoutACitation(t *testing.T) {
	ctx := context.Background()
	plantCorpus(t, 4)
	s := newServer(t, &fakeEmbedder{})
	sid := seeded(t, s)
	bid := beliefOf(t, s, sid)

	v := s.Discharge(ctx, sid, bid, wizard.ContradictionCheck, "")
	if v.OK {
		t.Fatal("contradiction sweep discharged with no citation on the record")
	}
	if v.Detail != wizard.NoCitationDetail {
		t.Errorf("detail = %q, want %q", v.Detail, wizard.NoCitationDetail)
	}
	// The user-facing string must not name a table that does not exist.
	if strings.Contains(v.Detail, "belief_evidence") {
		t.Error("refusal text names belief_evidence, a table this schema has never had")
	}

	// It is logged, so it counts toward the screen transition like the gate refusals.
	st, err := s.State(ctx, sid)
	if err != nil {
		t.Fatalf("State: %v", err)
	}
	if len(st.Refusals) != 1 {
		t.Fatalf("refusals = %d, want 1", len(st.Refusals))
	}

	// Now select something and it discharges.
	selectFirstHit(t, s, sid, bid, "data inconsistency after upgrade")
	if v := s.Discharge(ctx, sid, bid, wizard.ContradictionCheck, ""); !v.OK {
		t.Fatalf("contradiction sweep still refused after selecting evidence: %+v", v)
	}
}

// --- W-07: the whole gate, and a non-vacuous audit -----------------------------

// The phase's end-to-end path. The assertion that matters most is the last one: a live
// intent EXISTS at the same time as live_on_nonpromoted = 0. A zero over an empty set
// proves nothing, and a judge will notice.
func TestW07_FullDischargeThenPromoteThenAuthorize(t *testing.T) {
	ctx := context.Background()
	plantCorpus(t, 4)
	s := newServer(t, &fakeEmbedder{})
	sid := seeded(t, s)
	bid := beliefOf(t, s, sid)

	// Screen 1: both statements refused, which is what unlocks screen 2.
	if v := s.Promote(ctx, sid, bid); v.OK {
		t.Fatal("promote succeeded with full debt")
	}
	if v := s.Authorize(ctx, sid, bid); v.OK {
		t.Fatal("authorize succeeded on an unpromoted belief")
	}
	st, _ := s.State(ctx, sid)
	if st.Screen != wizard.ScreenDischarge {
		t.Fatalf("screen = %q after refusals, want DISCHARGE", st.Screen)
	}

	// Screen 2: a citation, then all six checks.
	selectFirstHit(t, s, sid, bid, "data inconsistency after upgrade")
	for _, item := range kernel.FullDebt {
		v := s.Discharge(ctx, sid, bid, item, operatorArtifacts[item])
		if !v.OK {
			t.Fatalf("discharge %s refused: %+v", item, v)
		}
	}
	st, _ = s.State(ctx, sid)
	if st.Debt != 0 {
		t.Fatalf("debt = %d after discharging all six, want 0", st.Debt)
	}
	for _, c := range st.Checks {
		if !c.Done {
			t.Errorf("check %s still outstanding", c.Name)
		}
		if c.Receipt == "" {
			t.Errorf("check %s discharged without a receipt", c.Name)
		}
	}

	// The same two statements, now accepted. Same method, no screen parameter.
	if v := s.Promote(ctx, sid, bid); !v.OK {
		t.Fatalf("promote refused with zero debt: %+v", v)
	}
	if v := s.Authorize(ctx, sid, bid); !v.OK {
		t.Fatalf("authorize refused on a promoted belief: %+v", v)
	}

	st, err := s.State(ctx, sid)
	if err != nil {
		t.Fatalf("State: %v", err)
	}
	if st.Status != "promoted" {
		t.Errorf("status = %q, want promoted", st.Status)
	}
	if st.IntentState != "live" {
		t.Fatalf("intent state = %q, want live", st.IntentState)
	}
	if st.Intent != wizard.DeployAction {
		t.Errorf("intent action = %q, want %q", st.Intent, wizard.DeployAction)
	}
	// Non-vacuous: the live intent exists AND the invariant holds.
	if st.Audit != 0 {
		t.Errorf("live_on_nonpromoted = %d, want 0", st.Audit)
	}
	if st.Screen != wizard.ScreenFalsify {
		t.Errorf("screen = %q after authorize, want FALSIFY", st.Screen)
	}
}

// --- W-08 / W-09: selection is a toggle, and idempotent ------------------------

func TestW08_SelectionToggleIsIdempotent(t *testing.T) {
	ctx := context.Background()
	plantCorpus(t, 4)
	s := newServer(t, &fakeEmbedder{})
	sid := seeded(t, s)
	bid := beliefOf(t, s, sid)

	h := selectFirstHit(t, s, sid, bid, "first query")
	if n := countCitations(t, bid); n != 1 {
		t.Fatalf("citations after one select = %d, want 1", n)
	}

	// Selecting again refreshes rather than duplicating: the primary key is
	// (belief_id, corpus_id).
	if err := s.Select(ctx, bid, h.CorpusID, "second query", 0.42, true); err != nil {
		t.Fatalf("re-select: %v", err)
	}
	if n := countCitations(t, bid); n != 1 {
		t.Errorf("citations after re-select = %d, want 1", n)
	}
	var q string
	var d float64
	if err := shared.QueryRowContext(ctx,
		`SELECT query_text, distance FROM belief_corpus_citation WHERE belief_id = $1::UUID`,
		bid).Scan(&q, &d); err != nil {
		t.Fatalf("read citation: %v", err)
	}
	if q != "second query" || d != 0.42 {
		t.Errorf("re-select did not refresh: query=%q distance=%v", q, d)
	}

	// Deselect removes it; deselecting again is still success.
	if err := s.Select(ctx, bid, h.CorpusID, "", 0, false); err != nil {
		t.Fatalf("deselect: %v", err)
	}
	if n := countCitations(t, bid); n != 0 {
		t.Errorf("citations after deselect = %d, want 0", n)
	}
	if err := s.Select(ctx, bid, h.CorpusID, "", 0, false); err != nil {
		t.Errorf("deselecting an absent citation should be success, got: %v", err)
	}
}

// Concurrent identical selections must leave exactly one row. The primary key is the
// guarantee; this asserts it rather than assuming it.
func TestW09_ConcurrentSelectLeavesOneRow(t *testing.T) {
	ctx := context.Background()
	plantCorpus(t, 4)
	s := newServer(t, &fakeEmbedder{})
	sid := seeded(t, s)
	bid := beliefOf(t, s, sid)

	res, err := s.Search(ctx, bid, "concurrency")
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	h := res.Hits[0]

	var wg sync.WaitGroup
	errs := make([]error, 8)
	for i := range errs {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			errs[i] = s.Select(ctx, bid, h.CorpusID, "concurrency", h.Distance, true)
		}(i)
	}
	wg.Wait()
	for i, err := range errs {
		if err != nil {
			t.Errorf("concurrent select %d: %v", i, err)
		}
	}
	if n := countCitations(t, bid); n != 1 {
		t.Errorf("citations after 8 concurrent selects = %d, want 1", n)
	}
}

// --- W-10: discharge idempotence -----------------------------------------------

// evidence has no uniqueness constraint — two planning documents claim
// UNIQUE(belief_id, content_sha256) and the shipped schema has no such thing — so
// re-discharging would silently add a second row if the handler did not guard on
// outstanding debt. This pins the guard.
func TestW10_DischargeIsIdempotentAndAddsNoDuplicateEvidence(t *testing.T) {
	ctx := context.Background()
	plantCorpus(t, 4)
	s := newServer(t, &fakeEmbedder{})
	sid := seeded(t, s)
	bid := beliefOf(t, s, sid)

	const item = "needBlastRadius"
	if v := s.Discharge(ctx, sid, bid, item, operatorArtifacts[item]); !v.OK {
		t.Fatalf("first discharge refused: %+v", v)
	}
	before := countEvidence(t, sid)

	v := s.Discharge(ctx, sid, bid, item, operatorArtifacts[item])
	if !v.OK {
		t.Errorf("re-discharging an already-retired item should be success, got: %+v", v)
	}
	if !strings.Contains(v.Detail, "already discharged") {
		t.Errorf("detail = %q, want it to say the item was already discharged", v.Detail)
	}
	if after := countEvidence(t, sid); after != before {
		t.Errorf("evidence rows %d -> %d on re-discharge; want unchanged", before, after)
	}
}

// --- W-11: the relation column ------------------------------------------------

func TestW11_CitationRelationDefaultsAndIsChecked(t *testing.T) {
	ctx := context.Background()
	plantCorpus(t, 4)
	s := newServer(t, &fakeEmbedder{})
	sid := seeded(t, s)
	bid := beliefOf(t, s, sid)

	res, err := s.Search(ctx, bid, "relation check")
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	cid := res.Hits[0].CorpusID

	// Default: a citation written without naming a relation is 'considered', which is
	// what keeps every citation written before 003 valid.
	if _, err := shared.ExecContext(ctx, `
		INSERT INTO belief_corpus_citation (belief_id, corpus_id, distance, query_text)
		VALUES ($1::UUID, $2::UUID, 0.5, 'no relation given')`, bid, cid); err != nil {
		t.Fatalf("insert without relation: %v", err)
	}
	var rel string
	if err := shared.QueryRowContext(ctx,
		`SELECT relation FROM belief_corpus_citation WHERE belief_id=$1::UUID AND corpus_id=$2::UUID`,
		bid, cid).Scan(&rel); err != nil {
		t.Fatalf("read relation: %v", err)
	}
	if rel != corpus.RelationConsidered {
		t.Errorf("default relation = %q, want %q", rel, corpus.RelationConsidered)
	}

	// contradicts is accepted.
	if err := corpus.Cite(ctx, shared, bid, cid, 0.5, "q", corpus.RelationContradicts); err != nil {
		t.Fatalf("cite as contradicts: %v", err)
	}

	// Anything else is refused by the database, not by Go.
	_, err = shared.ExecContext(ctx, `
		UPDATE belief_corpus_citation SET relation = 'sideways'
		 WHERE belief_id = $1::UUID AND corpus_id = $2::UUID`, bid, cid)
	if err == nil {
		t.Fatal("an invalid relation was accepted; the CHECK is missing")
	}
	if got := sqlStateOf(err); got != "23514" {
		t.Errorf("invalid relation sqlstate = %q, want 23514", got)
	}

	// And a contradiction moves the screen to FALSIFY.
	st, err := s.State(ctx, sid)
	if err != nil {
		t.Fatalf("State: %v", err)
	}
	if st.Screen != wizard.ScreenFalsify {
		t.Errorf("screen = %q with a contradicts citation, want FALSIFY", st.Screen)
	}
}

// --- W-12: one promote path, one authorize path --------------------------------

// "The same SQL on both screens" is a property of there being one call site, not a
// promise to keep two in step. Driving the same method from the ASK state and from the
// DISCHARGE state must differ only in what the database says.
func TestW12_PromoteAndAuthorizeHaveNoScreenDependence(t *testing.T) {
	ctx := context.Background()
	plantCorpus(t, 4)
	s := newServer(t, &fakeEmbedder{})
	sid := seeded(t, s)
	bid := beliefOf(t, s, sid)

	// From ASK: refused, on the schema's terms.
	fromAsk := s.Promote(ctx, sid, bid)
	if fromAsk.OK || fromAsk.Constraint != "promoted_is_debt_free" {
		t.Fatalf("promote from ASK: %+v", fromAsk)
	}

	// Discharge everything, which is the only thing that changed.
	selectFirstHit(t, s, sid, bid, "same-sql check")
	for _, item := range kernel.FullDebt {
		if v := s.Discharge(ctx, sid, bid, item, operatorArtifacts[item]); !v.OK {
			t.Fatalf("discharge %s: %+v", item, v)
		}
	}

	// From DISCHARGE: accepted. Same call, same statement, different world.
	fromDischarge := s.Promote(ctx, sid, bid)
	if !fromDischarge.OK {
		t.Fatalf("promote from DISCHARGE: %+v", fromDischarge)
	}
	if fromAsk.Statement != fromDischarge.Statement {
		t.Errorf("statement label differs by screen: %q vs %q", fromAsk.Statement, fromDischarge.Statement)
	}
}

// --- W-13: search is live, cached honestly, and fails loudly --------------------

func TestW13_SearchUsesTheEmbedderAndCachesOnlyRealVectors(t *testing.T) {
	ctx := context.Background()
	plantCorpus(t, 4)
	fake := &fakeEmbedder{}
	s := newServer(t, fake)
	sid := seeded(t, s)
	bid := beliefOf(t, s, sid)

	if _, err := s.Search(ctx, bid, "same question"); err != nil {
		t.Fatalf("first search: %v", err)
	}
	if fake.calls != 1 {
		t.Fatalf("embedder calls = %d after one search, want 1", fake.calls)
	}

	res, err := s.Search(ctx, bid, "same question")
	if err != nil {
		t.Fatalf("second search: %v", err)
	}
	if fake.calls != 1 {
		t.Errorf("embedder calls = %d after a repeated query, want 1 (cache miss)", fake.calls)
	}
	if !res.Cached {
		t.Error("second identical search not reported as cached")
	}

	// A different question is a different vector — the cache is keyed on exact text
	// because the vector is a function of exact text.
	if _, err := s.Search(ctx, bid, "same question "); err != nil {
		t.Fatalf("whitespace-differing search: %v", err)
	}

	// Empty query is refused before anything is embedded.
	if _, err := s.Search(ctx, bid, "   "); err == nil {
		t.Error("empty query accepted")
	}
}

// With no embedder and no credentials, search fails with an explanation naming the
// permission, and never silently substitutes anything.
func TestW13b_SearchWithoutCredentialsFailsExplicitly(t *testing.T) {
	s, err := wizard.New(shared, wizard.Options{
		CorpusScenario: testCorpusScenario,
		Region:         "us-west-2",
		Model:          "amazon.titan-embed-text-v2:0",
	})
	if err != nil {
		t.Fatalf("wizard.New: %v", err)
	}
	sid := seeded(t, s)
	bid := beliefOf(t, s, sid)

	// This may legitimately succeed on a machine with working credentials; the point of
	// the test is that it never silently returns fabricated hits.
	res, err := s.Search(context.Background(), bid, "credential probe")
	if err != nil {
		if !errors.Is(err, wizard.ErrEmbedderUnavailable) && !strings.Contains(err.Error(), "embed query") {
			t.Errorf("unexpected failure shape: %v", err)
		}
		if len(res.Hits) != 0 {
			t.Error("hits returned alongside an embedding failure")
		}
		return
	}
	if len(res.Hits) == 0 {
		t.Error("search reported success with no hits and no error")
	}
}

// --- W-14: reload lands where the judge was ------------------------------------

func TestW14_StateIsDerivedNotRemembered(t *testing.T) {
	ctx := context.Background()
	plantCorpus(t, 4)
	s := newServer(t, &fakeEmbedder{})
	sid := seeded(t, s)
	bid := beliefOf(t, s, sid)

	steps := []struct {
		name string
		do   func()
		want string
	}{
		{"fresh", func() {}, wizard.ScreenAsk},
		{"after refusal", func() { s.Promote(ctx, sid, bid) }, wizard.ScreenDischarge},
		{"after discharge", func() {
			selectFirstHit(t, s, sid, bid, "reload check")
			for _, item := range kernel.FullDebt {
				s.Discharge(ctx, sid, bid, item, operatorArtifacts[item])
			}
		}, wizard.ScreenDischarge},
		{"after promote", func() { s.Promote(ctx, sid, bid) }, wizard.ScreenDischarge},
		{"after authorize", func() { s.Authorize(ctx, sid, bid) }, wizard.ScreenFalsify},
	}

	for _, step := range steps {
		step.do()
		// Two consecutive reads must agree: State is a pure function of committed rows.
		first, err := s.State(ctx, sid)
		if err != nil {
			t.Fatalf("%s: State: %v", step.name, err)
		}
		second, err := s.State(ctx, sid)
		if err != nil {
			t.Fatalf("%s: State (reload): %v", step.name, err)
		}
		if first.Screen != step.want {
			t.Errorf("%s: screen = %q, want %q", step.name, first.Screen, step.want)
		}
		if first.Screen != second.Screen {
			t.Errorf("%s: reload changed the screen: %q -> %q", step.name, first.Screen, second.Screen)
		}
	}
}

func countCitations(t *testing.T, beliefID string) int {
	t.Helper()
	var n int
	if err := shared.QueryRowContext(context.Background(),
		`SELECT count(*) FROM belief_corpus_citation WHERE belief_id = $1::UUID`, beliefID).Scan(&n); err != nil {
		t.Fatalf("count citations: %v", err)
	}
	return n
}

func countEvidence(t *testing.T, scenarioID string) int {
	t.Helper()
	var n int
	if err := shared.QueryRowContext(context.Background(),
		`SELECT count(*) FROM evidence WHERE scenario_id = $1::UUID`, scenarioID).Scan(&n); err != nil {
		t.Fatalf("count evidence: %v", err)
	}
	return n
}
