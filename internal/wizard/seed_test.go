package wizard_test

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"
	_ "github.com/jackc/pgx/v5/stdlib" // D-019: pgx/v5 is the sole approved driver

	"github.com/PithomLabs/solvent/internal/corpus"
	"github.com/PithomLabs/solvent/internal/testdb"
	"github.com/PithomLabs/solvent/internal/wizard"
	"github.com/PithomLabs/solvent/kernel"
)

// 003 alters a 002 table, so all three are applied in order.
var schemaPaths = []string{
	"../../db/001_schema.sql",
	"../../db/002_corpus.sql",
	"../../db/003_wizard.sql", "../../db/004_debt_vocabulary.sql",
}

// Scenario namespace. 1111..6666 belong to the kernel/belief/intent/pipeline suites,
// 7777 to the kernel's lifecycle example, 8888/9999/aaaa/bbbb to corpus and
// corpus-ingest, cccc/dddd to demoseed. eeee is the corpus partition this suite
// plants its own embedded rows into; every wizard ledger scenario is minted fresh at
// runtime, as production does.
const testCorpusScenario = "eeeeeeee-eeee-eeee-eeee-eeeeeeeeeeee"

var shared *sql.DB

func TestMain(m *testing.M) {
	ctx := context.Background()
	dsn := testdb.DSN()

	name, _ := testdb.DBNameFromDSN(dsn)
	testdb.AcquireResetLock(name)

	if err := testdb.Reset(ctx, dsn, schemaPaths...); err != nil {
		fmt.Fprintf(os.Stderr, "wizard suite cannot start: %v\n", err)
		testdb.ReleaseResetLock(name)
		os.Exit(1)
	}
	var err error
	shared, err = testdb.Open(dsn)
	if err != nil {
		fmt.Fprintf(os.Stderr, "wizard suite cannot start: open pool: %v\n", err)
		testdb.ReleaseResetLock(name)
		os.Exit(1)
	}
	code := m.Run()
	_ = shared.Close()
	testdb.ReleaseResetLock(name)
	os.Exit(code)
}

// --- fixtures -----------------------------------------------------------------

// fakeEmbedder returns deterministic vectors and never touches the network.
//
// It lives in a _test.go file on purpose: the production code path has exactly one
// embedder, and a fake that could be linked into the binary would be a second
// implementation waiting to be selected by accident. The same discipline
// internal/corpus applies to HashEmbed.
type fakeEmbedder struct {
	calls int
	vec   []float32
}

func (f *fakeEmbedder) Embed(_ context.Context, text string) ([]float32, int, error) {
	f.calls++
	if f.vec != nil {
		return f.vec, len(text), nil
	}
	return planted(len(text) % 5), len(text), nil
}

// planted builds a deterministic unit-ish vector of the production width.
func planted(seed int) []float32 {
	v := make([]float32, corpus.Dim)
	for i := range v {
		v[i] = float32((i*7+seed*131)%1000) / 1000.0
	}
	return v
}

// newServer builds a Server against the shared pool with a fake embedder.
func newServer(t *testing.T, emb wizard.QueryEmbedder) *wizard.Server {
	t.Helper()
	s, err := wizard.New(shared, wizard.Options{
		CorpusScenario: testCorpusScenario,
		Embedder:       emb,
		TopK:           4,
	})
	if err != nil {
		t.Fatalf("wizard.New: %v", err)
	}
	return s
}

// seeded mints a fresh scenario and seeds it, as the lazy path does in production.
func seeded(t *testing.T, s *wizard.Server) string {
	t.Helper()
	ctx := context.Background()
	sid, err := s.NewScenario(ctx)
	if err != nil {
		t.Fatalf("NewScenario: %v", err)
	}
	if err := s.Seed(ctx, sid); err != nil {
		t.Fatalf("Seed: %v", err)
	}
	return sid
}

// plantCorpus inserts embedded corpus rows into the test corpus partition.
func plantCorpus(t *testing.T, n int) {
	t.Helper()
	ctx := context.Background()
	for i := 1; i <= n; i++ {
		id, _, err := corpus.Insert(ctx, shared, corpus.Issue{
			ScenarioID:    testCorpusScenario,
			IssueNumber:   13000 + i,
			Title:         fmt.Sprintf("planted issue %d", 13000+i),
			Body:          "body",
			State:         "closed",
			URL:           fmt.Sprintf("https://github.com/etcd-io/etcd/issues/%d", 13000+i),
			ContentSHA256: fmt.Sprintf("%064x", i),
			Embedding:     planted(i),
		})
		if err != nil && !strings.Contains(err.Error(), "duplicate") {
			t.Fatalf("plant corpus row %d: %v", i, err)
		}
		_ = id
	}
}

// --- W-01: the seed is the state at the START of screen 1 ----------------------

// The correction this whole phase turns on. The original prompt described ancestor
// promoted, descendant promoted, one live intent — which is the state at screen 3.
// Seeded that way, /api/state matches the FALSIFY row immediately and the judge never
// sees ASK or DISCHARGE.
//
// The seed must therefore be: ancestor promoted and debt-free, descendant entered with
// all six debts, and NO action_intent.
func TestW01_SeedIsPreFalsification(t *testing.T) {
	ctx := context.Background()
	s := newServer(t, &fakeEmbedder{})
	sid := seeded(t, s)

	st, err := s.SeedVerify(ctx, sid)
	if err != nil {
		t.Fatalf("SeedVerify: %v", err)
	}
	if st.AncestorStatus != "promoted" {
		t.Errorf("ancestor status = %q, want promoted", st.AncestorStatus)
	}
	if st.AncestorDebt != 0 {
		t.Errorf("ancestor debt = %d, want 0", st.AncestorDebt)
	}
	if st.DescStatus != "entered" {
		t.Errorf("descendant status = %q, want entered", st.DescStatus)
	}
	if st.DescDebt != len(kernel.FullDebt) {
		t.Errorf("descendant debt = %d, want %d", st.DescDebt, len(kernel.FullDebt))
	}
	if st.Edges != 1 {
		t.Errorf("edges = %d, want exactly 1", st.Edges)
	}
	if st.Intents != 0 {
		t.Errorf("intents = %d, want 0 — the judge creates the only one, on screen 2", st.Intents)
	}

	// And the screen that follows from it.
	full, err := s.State(ctx, sid)
	if err != nil {
		t.Fatalf("State: %v", err)
	}
	if full.Screen != wizard.ScreenAsk {
		t.Fatalf("screen = %q on a fresh scenario, want ASK; the seed has put the judge past the first two screens", full.Screen)
	}
	// The belief on screen must be the DESCENDANT. If this is the ancestor, the intent
	// will land on the ancestor and screen 3 degrades from Tier 1 to Tier 2 silently.
	if full.Claim != wizard.DescendantClaim {
		t.Errorf("screen 1 drives %q, want the descendant %q", full.Claim, wizard.DescendantClaim)
	}
	if full.Ancestor != wizard.AncestorClaim {
		t.Errorf("ancestor = %q, want %q", full.Ancestor, wizard.AncestorClaim)
	}
}

// --- W-02: the ancestor's provenance is recorded honestly ----------------------

// The seeded ancestor is a decision that predates the ledger. It gets exactly one
// evidence row, operator-asserted, with an internal reference rather than an invented
// URL — not dressed up as something retrieved.
func TestW02_AncestorCarriesOperatorAssertedProvenance(t *testing.T) {
	ctx := context.Background()
	s := newServer(t, &fakeEmbedder{})
	sid := seeded(t, s)

	var class, ref string
	var n int
	if err := shared.QueryRowContext(ctx, `
		SELECT count(*) FROM evidence WHERE scenario_id = $1::UUID`, sid).Scan(&n); err != nil {
		t.Fatalf("count evidence: %v", err)
	}
	if n != 1 {
		t.Fatalf("evidence rows = %d, want exactly 1 (the ancestor's)", n)
	}
	if err := shared.QueryRowContext(ctx, `
		SELECT provenance_class, coalesce(source_url, '') FROM evidence
		 WHERE scenario_id = $1::UUID`, sid).Scan(&class, &ref); err != nil {
		t.Fatalf("read evidence: %v", err)
	}
	if class != "operator_asserted" {
		t.Errorf("provenance_class = %q, want operator_asserted", class)
	}
	if strings.HasPrefix(ref, "http") {
		t.Errorf("source_url = %q — an operator-asserted row must not carry an invented URL", ref)
	}
	if !strings.HasPrefix(ref, "solvent:") {
		t.Errorf("source_url = %q, want an internal solvent: reference", ref)
	}
}

// --- W-03: seeding twice is refused, not merged --------------------------------

func TestW03_SeedRefusesANonEmptyScenario(t *testing.T) {
	ctx := context.Background()
	s := newServer(t, &fakeEmbedder{})
	sid := seeded(t, s)

	err := s.Seed(ctx, sid)
	if err == nil {
		t.Fatal("seeding an already-seeded scenario succeeded; want a refusal — two ancestors is an ambiguous graph")
	}
	if !strings.Contains(err.Error(), "already holds") {
		t.Errorf("error should say the scenario is already populated, got: %v", err)
	}
}

// --- W-04: scenarios are isolated ---------------------------------------------

// Two judges hitting the deployed demo at once must not see each other. This is also
// what makes the refusal_log scenario filter load-bearing: the log is append-only
// across every scenario that has ever existed.
func TestW04_ScenariosAreIsolated(t *testing.T) {
	ctx := context.Background()
	s := newServer(t, &fakeEmbedder{})

	a := seeded(t, s)
	b := seeded(t, s)
	if a == b {
		t.Fatal("NewScenario returned the same id twice")
	}

	// Refuse a promotion in A only.
	if v := s.Promote(ctx, a, beliefOf(t, s, a)); v.OK {
		t.Fatal("promote with six outstanding debts succeeded; the schema should have refused")
	}

	sa, err := s.State(ctx, a)
	if err != nil {
		t.Fatalf("State(a): %v", err)
	}
	sb, err := s.State(ctx, b)
	if err != nil {
		t.Fatalf("State(b): %v", err)
	}
	if len(sa.Refusals) != 1 {
		t.Errorf("scenario a refusals = %d, want 1", len(sa.Refusals))
	}
	if len(sb.Refusals) != 0 {
		t.Errorf("scenario b refusals = %d, want 0 — refusal_log is not scenario-filtered", len(sb.Refusals))
	}
	if sa.Screen != wizard.ScreenDischarge {
		t.Errorf("scenario a screen = %q, want DISCHARGE after a logged refusal", sa.Screen)
	}
	if sb.Screen != wizard.ScreenAsk {
		t.Errorf("scenario b screen = %q, want ASK — it has refused nothing", sb.Screen)
	}
}

func beliefOf(t *testing.T, s *wizard.Server, sid string) string {
	t.Helper()
	st, err := s.State(context.Background(), sid)
	if err != nil {
		t.Fatalf("State: %v", err)
	}
	return st.BeliefID
}

// --- W-05: the two gate refusals, logged --------------------------------------

// Screen 1's whole purpose. Both buttons are always enabled, both are attempted, and
// both are refused by the schema rather than by the application.
func TestW05_GateRefusalsAreLoggedWithSQLSTATEAndConstraint(t *testing.T) {
	ctx := context.Background()
	s := newServer(t, &fakeEmbedder{})
	sid := seeded(t, s)
	bid := beliefOf(t, s, sid)

	promote := s.Promote(ctx, sid, bid)
	if promote.OK {
		t.Fatal("Promote succeeded with six outstanding debts")
	}
	if promote.SQLState != "23514" {
		t.Errorf("promote sqlstate = %q, want 23514", promote.SQLState)
	}
	if promote.Constraint != "promoted_is_debt_free" {
		t.Errorf("promote constraint = %q, want promoted_is_debt_free", promote.Constraint)
	}

	authorize := s.Authorize(ctx, sid, bid)
	if authorize.OK {
		t.Fatal("Authorize succeeded against a belief that is not promoted")
	}
	if authorize.SQLState != "23503" {
		t.Errorf("authorize sqlstate = %q, want 23503", authorize.SQLState)
	}
	if authorize.Constraint != "gate" {
		t.Errorf("authorize constraint = %q, want gate", authorize.Constraint)
	}

	st, err := s.State(ctx, sid)
	if err != nil {
		t.Fatalf("State: %v", err)
	}
	if len(st.Refusals) != 2 {
		t.Fatalf("logged refusals = %d, want 2", len(st.Refusals))
	}
	// The log survives the aborted transactions that produced it — which is the only
	// reason screen 1 has an exit condition at all.
	for _, r := range st.Refusals {
		if r.SQLState == "" {
			t.Errorf("refusal %q logged without a sqlstate", r.Statement)
		}
	}
	if st.Status != "entered" {
		t.Errorf("belief status = %q after two refusals, want entered — nothing should have moved", st.Status)
	}
}

// sqlStateOf is the extraction the handlers use, mirrored here so a test failure names
// the code rather than a wrapped string.
func sqlStateOf(err error) string {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		return pgErr.Code
	}
	return ""
}
