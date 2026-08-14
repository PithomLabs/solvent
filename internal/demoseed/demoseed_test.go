package demoseed_test

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"
	_ "github.com/jackc/pgx/v5/stdlib" // D-019: pgx/v5 is the sole approved driver

	"github.com/PithomLabs/solvent/internal/demoseed"
	"github.com/PithomLabs/solvent/internal/testdb"
	"github.com/PithomLabs/solvent/kernel"
)

var schemaPaths = []string{"../../db/001_schema.sql", "../../db/002_corpus.sql", "../../db/003_wizard.sql", "../../db/004_debt_vocabulary.sql"}

// Scenario namespace for this suite. 1111..6666 belong to the kernel, belief,
// intent and pipeline suites; 7777 is the kernel's lifecycle example; 8888/9999/
// aaaa/bbbb belong to the corpus and corpus-ingest suites.
const (
	scenarioSeed = "cccccccc-cccc-cccc-cccc-cccccccccccc" // edge filing
	scenarioTier = "dddddddd-dddd-dddd-dddd-dddddddddddd" // the Tier 1 refusal
)

var shared *sql.DB

func TestMain(m *testing.M) {
	ctx := context.Background()
	dsn := testdb.DSN()

	name, _ := testdb.DBNameFromDSN(dsn)
	testdb.AcquireResetLock(name)

	if err := testdb.Reset(ctx, dsn, schemaPaths...); err != nil {
		fmt.Fprintf(os.Stderr, "demoseed suite cannot start: %v\n", err)
		testdb.ReleaseResetLock(name)
		os.Exit(1)
	}
	var err error
	shared, err = testdb.Open(dsn)
	if err != nil {
		fmt.Fprintf(os.Stderr, "demoseed suite cannot start: open pool: %v\n", err)
		testdb.ReleaseResetLock(name)
		os.Exit(1)
	}
	code := m.Run()
	_ = shared.Close()
	testdb.ReleaseResetLock(name)
	os.Exit(code)
}

// enterClaim creates a belief with the exact claim text the edge matches on.
func enterClaim(t *testing.T, sid, claim string, ct kernel.ClaimType) string {
	t.Helper()
	id, err := kernel.New(shared).EnterBelief(context.Background(), sid, claim, ct)
	if err != nil {
		t.Fatalf("EnterBelief(%q): %v", claim, err)
	}
	return id
}

func promote(t *testing.T, id string) {
	t.Helper()
	st := kernel.New(shared)
	ctx := context.Background()
	for _, d := range kernel.FullDebt {
		if err := st.RetireDebt(ctx, id, d); err != nil {
			t.Fatalf("RetireDebt(%s): %v", d, err)
		}
	}
	if err := st.Promote(ctx, id); err != nil {
		t.Fatalf("Promote: %v", err)
	}
}

// --- D-01: the edge is filed, and filing it twice is a no-op -------------------

func TestD01_FileDerivationEdgeIsIdempotent(t *testing.T) {
	ctx := context.Background()
	enterClaim(t, scenarioSeed, demoseed.ParentClaim, kernel.Derived)
	enterClaim(t, scenarioSeed, demoseed.ChildClaim, kernel.Accommodated)

	filed, err := demoseed.FileDerivationEdge(ctx, shared, scenarioSeed)
	if err != nil {
		t.Fatalf("first FileDerivationEdge: %v", err)
	}
	if !filed {
		t.Error("first call reported no new edge; want one filed")
	}

	// The demo seed is re-run freely and the Track 2 walkthrough invokes the
	// pipeline twice, so a second call must be silent rather than a constraint error.
	filed2, err := demoseed.FileDerivationEdge(ctx, shared, scenarioSeed)
	if err != nil {
		t.Fatalf("second FileDerivationEdge: %v", err)
	}
	if filed2 {
		t.Error("second call filed another edge; the primary key should have absorbed it")
	}

	ok, err := demoseed.EdgeExists(ctx, shared, scenarioSeed)
	if err != nil {
		t.Fatalf("EdgeExists: %v", err)
	}
	if !ok {
		t.Error("EdgeExists says the edge is absent after filing it")
	}

	var n int
	if err := shared.QueryRowContext(ctx, `
		SELECT count(*) FROM belief_edge e JOIN belief p ON p.id = e.parent_id
		 WHERE p.scenario_id = $1::UUID`, scenarioSeed).Scan(&n); err != nil {
		t.Fatalf("count edges: %v", err)
	}
	if n != 1 {
		t.Errorf("edges in scenario = %d, want exactly 1", n)
	}
}

// --- D-02: a missing claim is reported, never silently skipped -----------------

// Filing the edge into a scenario whose fixtures have not been ingested must fail
// loudly. Silently doing nothing would leave the Tier 1 demo path unavailable while
// the seed reported success — the exact failure this package exists to prevent.
func TestD02_MissingClaimsAreReported(t *testing.T) {
	const empty = "eeeeeeee-eeee-eeee-eeee-eeeeeeeeeeee"

	_, err := demoseed.FileDerivationEdge(context.Background(), shared, empty)
	if err == nil {
		t.Fatal("filing an edge with no beliefs present succeeded; want an error")
	}
	var missing *demoseed.ErrClaimsMissing
	if !errors.As(err, &missing) {
		t.Fatalf("error is not *ErrClaimsMissing: %v", err)
	}
	if missing.FoundParent || missing.FoundChild {
		t.Errorf("reported parent=%t child=%t; want both false", missing.FoundParent, missing.FoundChild)
	}
}

// --- D-03: the Tier 1 refusal, on the real seeded topology ---------------------

// This is the test that matters. proof/act6_tier_probe.log established that
// CockroachDB re-evaluates live_requires_promoted on a row modified by the gate FK's
// ON UPDATE CASCADE. That probe built its own fixtures; this asserts the same
// refusal over the topology the DEMO actually seeds — parent and child matched by
// the real claim text, joined by the real edge, with a live intent on the child.
//
// The operation under test is demoseed.SQLRetractWithoutCancel, NOT
// kernel.RetractCascade. The kernel cancels strictly before retracting (I-8), so it
// can never produce this refusal — it is safe by construction. The refusal belongs
// to the naive thing a normal application does, which is what the FALSIFY screen's
// first button issues. Chasing it through RetractCascade instead reads as "Tier 1 is
// broken" and it is not.
//
// Retracting the ANCESTOR must be refused. The recursive traversal is application
// logic; what the database enforces is that the traversal cannot finish having left
// a live intent behind.
func TestD03_AncestorRetractionIsRefusedWhileDescendantIntentIsLive(t *testing.T) {
	ctx := context.Background()
	st := kernel.New(shared)

	parent := enterClaim(t, scenarioTier, demoseed.ParentClaim, kernel.Derived)
	child := enterClaim(t, scenarioTier, demoseed.ChildClaim, kernel.Accommodated)

	if _, err := demoseed.FileDerivationEdge(ctx, shared, scenarioTier); err != nil {
		t.Fatalf("FileDerivationEdge: %v", err)
	}

	// Both promoted; the DESCENDANT carries the live intent, as the DISCHARGE screen
	// will leave it.
	promote(t, parent)
	promote(t, child)
	if err := st.IntentOnPromoted(ctx, scenarioTier, child, "deploy etcd v3.5.0"); err != nil {
		t.Fatalf("IntentOnPromoted: %v", err)
	}

	// Retract the ancestor without cancelling. The traversal reaches the child, the
	// gate FK rewrites the child's intent to belief_status='retracted', and the CHECK
	// detonates.
	_, err := shared.ExecContext(ctx, demoseed.SQLRetractWithoutCancel, parent, scenarioTier)
	if err == nil {
		t.Fatal("ancestor retraction succeeded with a live intent on the descendant; " +
			"Tier 1 is not enforced and the FALSIFY screen's climax does not exist")
	}

	// Assert on the structured error, not on message text. The constraint name is a
	// field pgx carries; it does not appear in Error(), which is why the probe's psql
	// transcript prints "CONSTRAINT NAME:" on its own line.
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) {
		t.Fatalf("error is not a *pgconn.PgError: %v", err)
	}
	if pgErr.Code != "23514" {
		t.Errorf("SQLSTATE = %s, want 23514 (check constraint violation): %v", pgErr.Code, err)
	}
	if pgErr.ConstraintName != "live_requires_promoted" {
		t.Errorf("constraint = %q, want live_requires_promoted", pgErr.ConstraintName)
	}

	// Rolled back atomically: both beliefs still promoted, the intent still live.
	for name, id := range map[string]string{"parent": parent, "child": child} {
		var status string
		if err := shared.QueryRowContext(ctx,
			`SELECT status FROM belief WHERE id = $1::UUID`, id).Scan(&status); err != nil {
			t.Fatalf("read %s status: %v", name, err)
		}
		if status != "promoted" {
			t.Errorf("%s status = %q after a refused retraction, want promoted", name, status)
		}
	}

	// And the audit invariant never moved.
	n, err := st.AuditLiveOnNonPromoted(ctx, scenarioTier)
	if err != nil {
		t.Fatalf("audit: %v", err)
	}
	if n != 0 {
		t.Errorf("audit = %d, want 0", n)
	}
}

// --- D-04: the kernel's own path succeeds, and traverses the edge --------------

// The refusal must be specific to the naive order, not a blanket block on ancestor
// retraction. kernel.RetractCascade cancels before retracting, so the same topology
// that refuses D-03 commits here — with a LIVE intent going in, untouched by hand.
//
// `retracted == 2` is the load-bearing assertion: it proves the kernel's recursive
// traversal actually walked the seeded edge, rather than retracting the ancestor
// alone. Before this edge existed, the demo could not have shown that at all.
func TestD04_KernelRetractCascadeTraversesTheEdgeAndSucceeds(t *testing.T) {
	ctx := context.Background()
	st := kernel.New(shared)
	const sid = "dddddddd-0000-0000-0000-000000000002"

	parent := enterClaim(t, sid, demoseed.ParentClaim, kernel.Derived)
	child := enterClaim(t, sid, demoseed.ChildClaim, kernel.Accommodated)
	if _, err := demoseed.FileDerivationEdge(ctx, shared, sid); err != nil {
		t.Fatalf("FileDerivationEdge: %v", err)
	}
	promote(t, parent)
	promote(t, child)
	if err := st.IntentOnPromoted(ctx, sid, child, "deploy etcd v3.5.0"); err != nil {
		t.Fatalf("IntentOnPromoted: %v", err)
	}

	retracted, err := st.RetractCascade(ctx, sid, parent)
	if err != nil {
		t.Fatalf("kernel.RetractCascade was refused; it cancels before retracting and must not be: %v", err)
	}
	// Both the ancestor and the descendant reached through the edge.
	if retracted != 2 {
		t.Errorf("retracted %d belief(s), want 2 (ancestor + descendant via the edge)", retracted)
	}

	var cancelled int
	if err := shared.QueryRowContext(ctx,
		`SELECT count(*) FROM action_intent WHERE scenario_id = $1::UUID AND state = 'cancelled'`,
		sid).Scan(&cancelled); err != nil {
		t.Fatalf("count cancelled: %v", err)
	}
	if cancelled != 1 {
		t.Errorf("cancelled intents = %d, want 1 (the cascade must have cancelled it)", cancelled)
	}

	n, err := st.AuditLiveOnNonPromoted(ctx, sid)
	if err != nil {
		t.Fatalf("audit: %v", err)
	}
	if n != 0 {
		t.Errorf("audit = %d, want 0", n)
	}
}
