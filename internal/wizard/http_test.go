package wizard_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/PithomLabs/solvent/internal/wizard"
	"github.com/PithomLabs/solvent/kernel"
)

// client drives the wizard over HTTP the way a browser does, carrying the cookie.
type client struct {
	t   *testing.T
	mux *http.ServeMux
	jar []*http.Cookie
}

func newClient(t *testing.T, s *wizard.Server) *client {
	t.Helper()
	mux := http.NewServeMux()
	s.Routes(mux)
	return &client{t: t, mux: mux}
}

func (c *client) do(method, path, body string) (*httptest.ResponseRecorder, map[string]any) {
	c.t.Helper()
	var r *http.Request
	if method == http.MethodPost {
		r = httptest.NewRequest(method, path, strings.NewReader(body))
		r.Header.Set("Content-Type", "application/json")
	} else {
		r = httptest.NewRequest(method, path, nil)
	}
	for _, ck := range c.jar {
		r.AddCookie(ck)
	}
	w := httptest.NewRecorder()
	c.mux.ServeHTTP(w, r)
	// Keep whatever cookie the server set, as a browser would.
	if set := w.Result().Cookies(); len(set) > 0 {
		c.jar = set
	}
	var out map[string]any
	if strings.HasPrefix(w.Header().Get("Content-Type"), "application/json") {
		_ = json.Unmarshal(w.Body.Bytes(), &out)
	}
	return w, out
}

// --- W-15: the shell renders, and writes nothing -------------------------------

func TestW15_ShellRendersWithoutSeeding(t *testing.T) {
	s := newServer(t, &fakeEmbedder{})
	c := newClient(t, s)

	w, _ := c.do(http.MethodGet, wizard.Prefix, "")
	if w.Code != http.StatusOK {
		t.Fatalf("GET %s = %d, want 200; body: %s", wizard.Prefix, w.Code, w.Body.String())
	}
	// The page is readable without becoming a writer. A GET that seeds makes every
	// crawler, uptime monitor and link preview a ledger author.
	if len(c.jar) != 0 {
		t.Error("GET set a cookie; the shell must not establish a scenario")
	}
	html := w.Body.String()
	for _, want := range []string{"Solvent", "stepper", "/api/state", "ASK", "DISCHARGE", "FALSIFY"} {
		if !strings.Contains(html, want) {
			t.Errorf("shell HTML missing %q", want)
		}
	}
	// Light palette, per the contract, and no webfont binaries.
	if !strings.Contains(html, "#FBFAF7") {
		t.Error("shell is not using the contract's paper colour")
	}
	if strings.Contains(html, ".woff") {
		t.Error("shell references a font binary; system stacks only")
	}

	// State is immediately renderable and is ASK — projected, not seeded.
	w2, st := c.do(http.MethodGet, wizard.Prefix+"/api/state", "")
	if w2.Code != http.StatusOK {
		t.Fatalf("GET state = %d: %s", w2.Code, w2.Body.String())
	}
	if st["screen"] != wizard.ScreenAsk {
		t.Errorf("screen = %v, want ASK", st["screen"])
	}
	if st["claim"] != wizard.DescendantClaim {
		t.Errorf("claim = %v, want the descendant", st["claim"])
	}
	if st["seeded"] != false {
		t.Errorf("seeded = %v, want false before any action", st["seeded"])
	}
	if len(c.jar) != 0 {
		t.Error("GET /api/state set a cookie; it must not create a scenario")
	}
}

// --- W-16: the two refusals over HTTP, then the full commit --------------------

func TestW16_EndToEndOverHTTP(t *testing.T) {
	plantCorpus(t, 4)
	s := newServer(t, &fakeEmbedder{})
	c := newClient(t, s)
	c.do(http.MethodGet, wizard.Prefix, "")

	// A refusal is a 200 carrying a verdict, not an HTTP error.
	w, v := c.do(http.MethodPost, wizard.Prefix+"/api/promote", "{}")
	if w.Code != http.StatusOK {
		t.Fatalf("promote HTTP status = %d, want 200 (a refusal is not a transport error)", w.Code)
	}
	if v["ok"] == true {
		t.Fatal("promote succeeded with full debt")
	}
	if v["sqlstate"] != "23514" || v["constraint"] != "promoted_is_debt_free" {
		t.Errorf("promote verdict = %v", v)
	}

	_, v = c.do(http.MethodPost, wizard.Prefix+"/api/authorize", "{}")
	if v["sqlstate"] != "23503" || v["constraint"] != "gate" {
		t.Errorf("authorize verdict = %v", v)
	}

	// Screen advanced because refusals are on the record.
	_, st := c.do(http.MethodGet, wizard.Prefix+"/api/state", "")
	if st["screen"] != wizard.ScreenDischarge {
		t.Fatalf("screen = %v after two refusals, want DISCHARGE", st["screen"])
	}

	// Search, select the first hit.
	_, sr := c.do(http.MethodPost, wizard.Prefix+"/api/search", `{"query":"http flow"}`)
	hits, _ := sr["hits"].([]any)
	if len(hits) == 0 {
		t.Fatalf("no hits: %v", sr)
	}
	// Two selections: the provenance check and the contradiction sweep each consume
	// their own citation, so one hit is not enough to clear the full debt.
	if len(hits) < 2 {
		t.Fatalf("hits = %d, need 2 for the two retrieval checks", len(hits))
	}
	for _, raw := range hits[:2] {
		h := raw.(map[string]any)
		sel := `{"corpus_id":"` + h["corpus_id"].(string) + `","query":"http flow","distance":` +
			jsonNum(h["distance"]) + `,"on":true}`
		if w, _ := c.do(http.MethodPost, wizard.Prefix+"/api/select", sel); w.Code != http.StatusOK {
			t.Fatalf("select = %d: %s", w.Code, w.Body.String())
		}
	}

	// The contradiction sweep is discharged from the citation; the rest take artifacts.
	for _, item := range kernel.FullDebt {
		body := `{"check":"` + item + `","artifact":"` + operatorArtifacts[item] + `"}`
		_, dv := c.do(http.MethodPost, wizard.Prefix+"/api/discharge", body)
		if dv["ok"] != true {
			t.Fatalf("discharge %s over HTTP: %v", item, dv)
		}
	}

	// Same endpoints, now accepted.
	if _, pv := c.do(http.MethodPost, wizard.Prefix+"/api/promote", "{}"); pv["ok"] != true {
		t.Fatalf("promote after discharge: %v", pv)
	}
	if _, av := c.do(http.MethodPost, wizard.Prefix+"/api/authorize", "{}"); av["ok"] != true {
		t.Fatalf("authorize after promote: %v", av)
	}

	_, st = c.do(http.MethodGet, wizard.Prefix+"/api/state", "")
	if st["screen"] != wizard.ScreenFalsify {
		t.Errorf("screen = %v after authorize, want FALSIFY", st["screen"])
	}
	if st["intent_state"] != "live" {
		t.Errorf("intent_state = %v, want live", st["intent_state"])
	}
	if n, _ := st["live_on_nonpromoted"].(float64); n != 0 {
		t.Errorf("live_on_nonpromoted = %v, want 0 while a live intent exists", st["live_on_nonpromoted"])
	}
}

// --- W-17: two judges are isolated by cookie ----------------------------------

func TestW17_TwoClientsGetSeparateScenarios(t *testing.T) {
	s := newServer(t, &fakeEmbedder{})
	a, b := newClient(t, s), newClient(t, s)

	// Both act, so both hold a real scenario id. If only one did, the other's id would
	// be the empty string and the inequality below would hold for the wrong reason.
	a.do(http.MethodPost, wizard.Prefix+"/api/promote", "{}")
	b.do(http.MethodPost, wizard.Prefix+"/api/search", `{"query":"anything"}`)

	_, sa := a.do(http.MethodGet, wizard.Prefix+"/api/state", "")
	_, sb := b.do(http.MethodGet, wizard.Prefix+"/api/state", "")
	if sa["scenario_id"] == sb["scenario_id"] {
		t.Fatal("two clients share a scenario")
	}
	if sa["screen"] != wizard.ScreenDischarge {
		t.Errorf("client a screen = %v, want DISCHARGE", sa["screen"])
	}
	if sb["screen"] != wizard.ScreenAsk {
		t.Errorf("client b screen = %v, want ASK — a's refusal leaked", sb["screen"])
	}
	if sa["scenario_id"] == "" || sb["scenario_id"] == "" {
		t.Fatalf("a scenario id is empty; both judges must have acted: a=%v b=%v",
			sa["scenario_id"], sb["scenario_id"])
	}
}

// --- W-18: reset issues a new scenario and never truncates ---------------------

func TestW18_ResetMintsAFreshScenario(t *testing.T) {
	s := newServer(t, &fakeEmbedder{})
	c := newClient(t, s)
	c.do(http.MethodGet, wizard.Prefix, "")
	c.do(http.MethodPost, wizard.Prefix+"/api/promote", "{}")

	_, before := c.do(http.MethodGet, wizard.Prefix+"/api/state", "")
	_, rst := c.do(http.MethodPost, wizard.Prefix+"/api/reset", "{}")
	if rst["scenario_id"] == nil || rst["scenario_id"] == before["scenario_id"] {
		t.Fatalf("reset did not mint a new scenario: %v", rst)
	}
	_, after := c.do(http.MethodGet, wizard.Prefix+"/api/state", "")
	if after["screen"] != wizard.ScreenAsk {
		t.Errorf("screen after reset = %v, want ASK", after["screen"])
	}

	// The abandoned scenario is still intact — reset abandons, it does not TRUNCATE.
	var n int
	if err := shared.QueryRowContext(t.Context(),
		`SELECT count(*) FROM belief WHERE scenario_id = $1::UUID`, before["scenario_id"]).Scan(&n); err != nil {
		t.Fatalf("count old scenario: %v", err)
	}
	if n == 0 {
		t.Error("reset destroyed the previous scenario; it must only abandon it")
	}
}

// --- W-19: GET-only endpoints reject POST and vice versa ----------------------

func TestW19_MethodsAreEnforced(t *testing.T) {
	s := newServer(t, &fakeEmbedder{})
	c := newClient(t, s)
	c.do(http.MethodGet, wizard.Prefix, "")

	for _, p := range []string{"/api/promote", "/api/authorize", "/api/discharge", "/api/select", "/api/search", "/api/reset"} {
		if w, _ := c.do(http.MethodGet, wizard.Prefix+p, ""); w.Code != http.StatusMethodNotAllowed {
			t.Errorf("GET %s = %d, want 405", p, w.Code)
		}
	}
}

func jsonNum(v any) string {
	b, _ := json.Marshal(v)
	return string(b)
}

// --- W-20: empty lists are [] and never null -----------------------------------

// Caught by driving the real endpoint, not by a Go test: a nil slice marshals to
// `null`, and the shell does `S.refusals.length` on it. On a freshly seeded scenario —
// the first thing every judge sees — that is a TypeError and a blank page.
//
// Asserted on the raw JSON bytes on purpose. Unmarshalling into a Go map turns both
// `null` and `[]` into something a test can iterate, which is exactly how the bug hid.
func TestW20_EmptyCollectionsSerialiseAsArrays(t *testing.T) {
	plantCorpus(t, 2)
	s := newServer(t, &fakeEmbedder{})
	c := newClient(t, s)
	c.do(http.MethodGet, wizard.Prefix, "")

	w, _ := c.do(http.MethodGet, wizard.Prefix+"/api/state", "")
	body := w.Body.String()
	for _, field := range []string{"refusals", "citations", "checks"} {
		if strings.Contains(body, `"`+field+`":null`) {
			t.Errorf(`%q serialised as null; the browser calls .length on it`, field)
		}
	}
	if !strings.Contains(body, `"refusals":[]`) {
		t.Error(`fresh scenario should report "refusals":[]`)
	}

	// Same for a search that matches nothing.
	w2, _ := c.do(http.MethodPost, wizard.Prefix+"/api/search", `{"query":"anything"}`)
	if strings.Contains(w2.Body.String(), `"hits":null`) {
		t.Error(`"hits" serialised as null`)
	}
}

// --- W-30: the projection is faithful, not decorative --------------------------
//
// The whole safety argument for a non-writing GET is that the projected screen 1 is
// indistinguishable from the seeded one. If they ever diverge, the page a judge sees
// changes under them on their first click -- so this asserts the equality rather than
// trusting the constants to stay in step.
func TestW30_ProjectedStateMatchesSeeded(t *testing.T) {
	ctx := context.Background()
	s := newServer(t, &fakeEmbedder{})

	projected, err := s.ProjectedState(ctx)
	if err != nil {
		t.Fatalf("ProjectedState: %v", err)
	}
	actual, err := s.State(ctx, seeded(t, s))
	if err != nil {
		t.Fatalf("State: %v", err)
	}

	if projected.Seeded || !actual.Seeded {
		t.Errorf("seeded flags wrong: projected=%v actual=%v", projected.Seeded, actual.Seeded)
	}
	// Ids cannot be projected -- no row exists to have one. Everything else must match.
	projected.Seeded, actual.Seeded = false, false
	projected.ScenarioID, actual.ScenarioID = "", ""
	projected.BeliefID, actual.BeliefID = "", ""

	pj, _ := json.Marshal(projected)
	rj, _ := json.Marshal(actual)
	if string(pj) != string(rj) {
		t.Errorf("projected screen 1 differs from seeded screen 1\n projected: %s\n seeded   : %s", pj, rj)
	}
}

// --- W-31: reading the wizard never writes to the ledger -----------------------

func TestW31_GetsWriteNothing(t *testing.T) {
	ctx := context.Background()
	s := newServer(t, &fakeEmbedder{})
	c := newClient(t, s)

	count := func() (b, e, ed int) {
		t.Helper()
		if err := shared.QueryRowContext(ctx, `
			SELECT (SELECT count(*) FROM belief), (SELECT count(*) FROM evidence),
			       (SELECT count(*) FROM belief_edge)`).Scan(&b, &e, &ed); err != nil {
			t.Fatalf("count: %v", err)
		}
		return
	}

	b0, e0, ed0 := count()
	// Twenty cookie-less reads, as a crawler sweeping the site would issue.
	for i := 0; i < 20; i++ {
		c.jar = nil
		if w, _ := c.do(http.MethodGet, wizard.Prefix, ""); w.Code != http.StatusOK {
			t.Fatalf("GET shell #%d = %d", i, w.Code)
		}
		c.jar = nil
		if w, _ := c.do(http.MethodGet, wizard.Prefix+"/api/state", ""); w.Code != http.StatusOK {
			t.Fatalf("GET state #%d = %d: %s", i, w.Code, w.Body.String())
		}
		c.jar = nil
		if w, _ := c.do(http.MethodGet, wizard.ProofPath, ""); w.Code != http.StatusOK {
			t.Fatalf("GET proof #%d = %d", i, w.Code)
		}
	}
	b1, e1, ed1 := count()
	if b0 != b1 || e0 != e1 || ed0 != ed1 {
		t.Errorf("60 GETs wrote to the ledger: beliefs %d->%d, evidence %d->%d, edges %d->%d",
			b0, b1, e0, e1, ed0, ed1)
	}

	// The first action does create, exactly once.
	c.jar = nil
	if w, _ := c.do(http.MethodPost, wizard.Prefix+"/api/promote", "{}"); w.Code == http.StatusServiceUnavailable {
		t.Fatalf("POST promote = 503: %s", w.Body.String())
	}
	b2, e2, ed2 := count()
	if b2 != b1+2 || e2 != e1+1 || ed2 != ed1+1 {
		t.Errorf("first action seeded wrong: beliefs %d->%d (want +2), evidence %d->%d (want +1), edges %d->%d (want +1)",
			b1, b2, e1, e2, ed1, ed2)
	}
	if len(c.jar) == 0 {
		t.Error("the first action set no cookie; the judge would lose their scenario")
	}
}

// --- W-33: a cookie naming a destroyed scenario degrades, it does not break ----
//
// Scenarios do get destroyed under a live cookie: resetAndSeed's TRUNCATE is not
// scenario-scoped, and verification runs delete scenarios directly. Before this, State
// returned an error for an unseeded scenario, handleState turned it into a 500, and the
// client's bootstrap fetch painted a red error where screen 1 belongs.
func TestW33_DestroyedScenarioRecovers(t *testing.T) {
	ctx := context.Background()
	s := newServer(t, &fakeEmbedder{})
	c := newClient(t, s)

	// A bystander whose rows must survive the repair.
	bystander := seeded(t, s)

	// The judge acts once to get a real scenario and cookie, then it is destroyed.
	c.do(http.MethodPost, wizard.Prefix+"/api/promote", "{}")
	if len(c.jar) == 0 {
		t.Fatal("no cookie after the first action")
	}
	doomed := c.jar[0].Value
	for _, q := range []string{
		`DELETE FROM belief_corpus_citation WHERE belief_id IN (SELECT id FROM belief WHERE scenario_id = $1::UUID)`,
		`DELETE FROM refusal_log WHERE scenario_id = $1::UUID`,
		`DELETE FROM action_intent WHERE scenario_id = $1::UUID`,
		`DELETE FROM belief_edge WHERE parent_id IN (SELECT id FROM belief WHERE scenario_id = $1::UUID)`,
		`DELETE FROM evidence WHERE scenario_id = $1::UUID`,
		`DELETE FROM belief WHERE scenario_id = $1::UUID`,
	} {
		if _, err := shared.ExecContext(ctx, q, doomed); err != nil {
			t.Fatalf("destroy scenario: %v", err)
		}
	}

	// The stale cookie is still sent. State must project, not 500.
	w, st := c.do(http.MethodGet, wizard.Prefix+"/api/state", "")
	if w.Code != http.StatusOK {
		t.Fatalf("GET state on a destroyed scenario = %d, want 200: %s", w.Code, w.Body.String())
	}
	if st["screen"] != wizard.ScreenAsk || st["seeded"] != false {
		t.Errorf("screen = %v seeded = %v, want ASK/false", st["screen"], st["seeded"])
	}

	// The next action repairs it under a fresh id.
	if w, _ := c.do(http.MethodPost, wizard.Prefix+"/api/promote", "{}"); w.Code == http.StatusServiceUnavailable {
		t.Fatalf("POST after destruction = 503: %s", w.Body.String())
	}
	repaired := c.jar[0].Value
	if repaired == doomed {
		t.Error("repaired into the destroyed scenario id; it should mint a fresh one")
	}
	_, st2 := c.do(http.MethodGet, wizard.Prefix+"/api/state", "")
	if st2["seeded"] != true {
		t.Errorf("after repair seeded = %v, want true", st2["seeded"])
	}

	// The repair must not have used a TRUNCATE-shaped path.
	var n int
	if err := shared.QueryRowContext(ctx,
		`SELECT count(*) FROM belief WHERE scenario_id = $1::UUID`, bystander).Scan(&n); err != nil {
		t.Fatalf("count bystander: %v", err)
	}
	if n != 2 {
		t.Errorf("bystander scenario has %d beliefs, want 2; the repair destroyed someone else's work", n)
	}
}
