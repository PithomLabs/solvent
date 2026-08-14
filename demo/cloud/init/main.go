// Command solvent-init executes the verified Beat 7 / Track 2 walkthrough
// against the cloud database.
//
// It replays the exact local walkthrough sequence:
//
//	Step 1: pipeline.Run (track2 fixtures — postmortem + release)
//	Step 2: EnterBelief + AddEvidence (postulated baseline)
//	Step 3: RetireDebt × N → Promote → IntentOnPromoted
//	Step 4: pipeline.Run (track2 fixtures, no reset — literal fidelity)
//	Step 5: RetractCascade (falsify)
//
// Idempotent: if the canonical Track 2 state already exists, verifies and exits.
// Safe on restart: never drops before checking.
package main

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"os"

	"github.com/PithomLabs/solvent/internal/demoseed"
	"github.com/PithomLabs/solvent/internal/pipeline"
	"github.com/PithomLabs/solvent/internal/testdb"
	"github.com/PithomLabs/solvent/kernel"
	_ "github.com/jackc/pgx/v5/stdlib"
)

const (
	scenarioID          = "00000000-0000-0000-0000-000000000002"
	baselineClaim       = "etcd v3.5.0 is approved for production deployment (decision as of 2021-06-16)"
	baselineEvidenceURL = "https://github.com/etcd-io/etcd/releases/tag/v3.5.0"
	baselineEvidenceSHA = "f47656dfaad45b2ecdb32c3169b8897b153d7a8b2453ba8e7c34a2dcde609ce1"
	deployAction        = "deploy etcd v3.5.0"
	schemaPath          = "db/001_schema.sql"
	corpusSchemaPath    = "db/002_corpus.sql"
	wizardSchemaPath    = "db/003_wizard.sql"
	debtSchemaPath      = "db/004_debt_vocabulary.sql"
	fixtureDir          = "internal/derive/testdata/etcd_real/track2"
)

func main() {
	dsn := os.Getenv("FABLE_DSN")
	if dsn == "" {
		log.Fatal("FABLE_DSN not set")
	}

	ctx := context.Background()

	// 1. Connect and check canonical state.
	db, err := testdb.Open(dsn)
	if err != nil {
		log.Fatalf("open: %v", err)
	}
	if err := db.PingContext(ctx); err != nil {
		log.Fatalf("ping: %v", err)
	}

	// 1. Bring the schema up to date. This runs on EVERY start, not just cold ones.
	//
	// The previous shape only applied the schema when no tables existed, so a
	// container redeployed against an already-migrated database skipped migration
	// entirely and could never acquire tables added after that database was first
	// created. That is the cloud-only drift class that produced the stale-schema
	// incidents; db/002_corpus.sql and db/003_wizard.sql are idempotent by
	// construction (every statement is IF NOT EXISTS), so applying them
	// unconditionally is a no-op when they are present and a migration when not.
	//
	// Ordering matters: 002 references belief(id) and 003 alters a 002 table, so the
	// frozen DDL comes first and 003 comes last.
	if tablesExist(ctx, db) {
		fmt.Println("Base tables present. Ensuring corpus and wizard schema are current...")
		if err := testdb.ApplySchema(ctx, dsn, corpusSchemaPath, wizardSchemaPath, debtSchemaPath); err != nil {
			log.Fatalf("apply corpus/wizard schema: %v", err)
		}
	} else {
		fmt.Println("No tables found. Applying schema...")
		db.Close()
		if err := testdb.ApplySchema(ctx, dsn, schemaPath, corpusSchemaPath, wizardSchemaPath, debtSchemaPath); err != nil {
			log.Fatalf("apply schema: %v", err)
		}
		db, err = testdb.Open(dsn)
		if err != nil {
			log.Fatalf("reopen: %v", err)
		}
	}

	// 2. Check canonical state.
	if canonicalStateExists(ctx, db, scenarioID) {
		if err := verifyTrack2(ctx, db, scenarioID); err != nil {
			fmt.Printf("Canonical state exists but verification failed: %v\n", err)
			fmt.Println("Resetting partial state and re-seeding...")
			resetAndSeed(ctx, db, dsn)
			return
		}
		db.Close()
		fmt.Println("Already seeded and verified. No-op.")
		return
	}

	// 3. This scenario holds rows but no canonical state: refuse to guess.
	if !isDatabaseEmpty(ctx, db, scenarioID) {
		db.Close()
		log.Fatalf("SCENARIO %s NON-EMPTY BUT NO CANONICAL STATE: STOP — refusing to destroy unknown state", scenarioID)
	}

	// 4. Seed.
	seed(ctx, db)
}

func resetAndSeed(ctx context.Context, db *sql.DB, dsn string) {
	// Truncate the ledger (works without defaultdb access).
	//
	// TRUNCATE belief CASCADE reaches belief_corpus_citation through its foreign
	// key, which is correct: a reset scenario should lose its citations.
	// corpus_issue is deliberately absent from this list and survives -- the corpus
	// is durable institutional memory, not scenario state.
	fmt.Println("Truncating ledger tables...")
	for _, table := range []string{"action_intent", "evidence", "belief_edge", "belief"} {
		if _, err := db.ExecContext(ctx, fmt.Sprintf("TRUNCATE %s CASCADE", table)); err != nil {
			fmt.Printf("Warning: truncate %s: %v (may not exist yet)\n", table, err)
		}
	}

	// Only the corpus and wizard layers are re-applied here. The frozen DDL is not
	// idempotent (no IF NOT EXISTS), so applying it against tables that were just
	// truncated rather than dropped fails on the first statement and would stop the
	// whole sequence before reaching 002 and 003.
	if err := testdb.ApplySchema(ctx, dsn, corpusSchemaPath, wizardSchemaPath, debtSchemaPath); err != nil {
		fmt.Printf("Warning: apply corpus/wizard schema: %v\n", err)
	}

	// Re-seed.
	seed(ctx, db)
}

func seed(ctx context.Context, db *sql.DB) {
	defer db.Close()
	st := kernel.New(db)

	// Step 1: Ingest track2 fixtures.
	fmt.Println("Step 1: Ingesting track2 fixtures...")
	results1, err := pipeline.Run(ctx, db, scenarioID, fixtureDir)
	if err != nil {
		log.Fatalf("Step 1 pipeline.Run: %v", err)
	}
	fmt.Printf("Step 1: processed %d fixture results\n", len(results1))

	// Step 1b: file the one derivation edge. It must come after the fixtures, which
	// are what create the two claims it connects.
	//
	// This is what makes the FALSIFY & AUDIT screen able to demonstrate the stronger
	// of the two refusals proof/act6_tier_probe.log measured: retracting an ANCESTOR
	// is refused, not merely retracting the belief that directly carries the intent.
	fmt.Println("Step 1b: Filing the derivation edge...")
	filed, err := demoseed.FileDerivationEdge(ctx, db, scenarioID)
	if err != nil {
		log.Fatalf("Step 1b FileDerivationEdge: %v", err)
	}
	fmt.Printf("Step 1b: edge %q --%s--> %q (newly filed: %t)\n",
		demoseed.ParentClaim, demoseed.Kind, demoseed.ChildClaim, filed)

	// Step 2: Enter baseline postulated belief.
	fmt.Println("Step 2: Entering baseline belief...")
	beliefID, err := st.EnterBelief(ctx, scenarioID, baselineClaim, kernel.Postulated)
	if err != nil {
		log.Fatalf("Step 2 EnterBelief: %v", err)
	}
	fmt.Printf("Step 2: BELIEF_ID=%s\n", beliefID)

	if err := st.AddEvidence(ctx, scenarioID, beliefID, "external_feed",
		baselineEvidenceURL, baselineEvidenceSHA); err != nil {
		log.Fatalf("Step 2 AddEvidence: %v", err)
	}
	fmt.Println("Step 2: evidence attached")

	// Step 3: Retire all FullDebt items, promote, create intent.
	// Matches Beat 7 exactly — no DB scan needed, FullDebt is the canonical set.
	fmt.Println("Step 3: Retiring debts, promoting, creating intent...")

	for _, d := range kernel.FullDebt {
		if err := st.RetireDebt(ctx, beliefID, d); err != nil {
			log.Fatalf("Step 3 RetireDebt(%s): %v", d, err)
		}
		fmt.Printf("Step 3: retired: %s\n", d)
	}

	if err := st.Promote(ctx, beliefID); err != nil {
		log.Fatalf("Step 3 Promote: %v", err)
	}
	fmt.Println("Step 3: promoted: ok")

	if err := st.IntentOnPromoted(ctx, scenarioID, beliefID, deployAction); err != nil {
		log.Fatalf("Step 3 IntentOnPromoted: %v", err)
	}
	fmt.Printf("Step 3: intent: live (action=%q)\n", deployAction)

	// Step 4: Re-run pipeline (literal fidelity).
	fmt.Println("Step 4: Re-running pipeline (literal fidelity)...")
	results4, err := pipeline.Run(ctx, db, scenarioID, fixtureDir)
	if err != nil {
		log.Fatalf("Step 4 pipeline.Run: %v", err)
	}
	fmt.Printf("Step 4: processed %d fixture results (idempotent)\n", len(results4))

	// Step 5: Falsify.
	fmt.Println("Step 5: Falsifying baseline belief...")
	retracted, err := st.RetractCascade(ctx, scenarioID, beliefID)
	if err != nil {
		log.Fatalf("Step 5 RetractCascade: %v", err)
	}
	fmt.Printf("Step 5: cascade: retracted %d belief(s)\n", retracted)

	// Verify.
	fmt.Println("Verifying final state...")
	if err := verifyTrack2(ctx, db, scenarioID); err != nil {
		log.Fatalf("VERIFICATION FAILED: %v", err)
	}

	fmt.Println("\n=== Track 2 walkthrough complete ===")
}

func canonicalStateExists(ctx context.Context, db *sql.DB, sid string) bool {
	var count int
	err := db.QueryRowContext(ctx,
		`SELECT count(*) FROM belief WHERE scenario_id = $1::UUID`, sid).Scan(&count)
	if err != nil {
		return false
	}
	return count > 0
}

func tablesExist(ctx context.Context, db *sql.DB) bool {
	var one int
	err := db.QueryRowContext(ctx,
		`SELECT 1 FROM information_schema.tables WHERE table_schema = 'public' AND table_name = 'belief' LIMIT 1`).Scan(&one)
	return err == nil
}

// isDatabaseEmpty reports whether THIS SCENARIO holds no ledger rows.
//
// # Why the scenario filter is load-bearing
//
// This function guards the log.Fatal below it, and it used to count belief,
// evidence and action_intent across every scenario. That made the initializer
// refuse to seed because of rows it had no business caring about — a leftover
// track1 run, a test scenario, an aborted seed. And the failure was not loud:
// Dockerfile chains solvent-init and solvent-web, so a dead initializer still let
// the web process start and serve a completely empty ledger, with every count on
// the landing page rendering 0. A plausible-looking wrong answer.
//
// The question the guard actually needs to ask is "is there unknown state in the
// scenario I am about to seed?", so that is the question it now asks. Rows in other
// scenarios are someone else's business and are left strictly alone.
//
// Errors are still tolerated rather than returned: an unreadable table means we
// cannot claim the scenario is empty, so a scan failure must NOT read as "empty".
// Each count defaults to -1 for exactly that reason.
//
// # This guard is now defence-in-depth, and cannot currently fire
//
// Scoped this way it is unreachable, and that is worth stating rather than leaving
// for someone to rediscover. evidence.belief_id and action_intent.belief_id are both
// NOT NULL foreign keys into belief, so a scenario with zero beliefs necessarily has
// zero evidence and zero intents. The caller only reaches this check when
// canonicalStateExists found no beliefs — at which point all three counts are
// already guaranteed to be zero.
//
// It is kept because a scan error must still refuse (the -1 defaults above), and
// because a future schema that let evidence outlive its belief would make the check
// live again. The real protection against partial state is verifyTrack2, which
// checks the shape rather than merely the absence.
//
// Note for whoever touches that path next: when verifyTrack2 fails, the caller runs
// resetAndSeed, whose TRUNCATE is NOT scenario-scoped and will take other scenarios
// with it. That is a separate defect from the one fixed here and is deliberately
// untouched.
func isDatabaseEmpty(ctx context.Context, db *sql.DB, sid string) bool {
	b, e, i := -1, -1, -1
	_ = db.QueryRowContext(ctx, `SELECT count(*) FROM belief WHERE scenario_id = $1::UUID`, sid).Scan(&b)
	_ = db.QueryRowContext(ctx, `SELECT count(*) FROM evidence WHERE scenario_id = $1::UUID`, sid).Scan(&e)
	_ = db.QueryRowContext(ctx, `SELECT count(*) FROM action_intent WHERE scenario_id = $1::UUID`, sid).Scan(&i)
	return b == 0 && e == 0 && i == 0
}

func verifyTrack2(ctx context.Context, db *sql.DB, sid string) error {
	var retractedCount int
	if err := db.QueryRowContext(ctx,
		`SELECT count(*) FROM belief WHERE scenario_id = $1::UUID AND status = 'retracted' AND claim = $2`,
		sid, baselineClaim).Scan(&retractedCount); err != nil {
		return fmt.Errorf("check retracted belief: %w", err)
	}
	if retractedCount != 1 {
		return fmt.Errorf("expected 1 retracted baseline belief, got %d", retractedCount)
	}

	var cancelledCount int
	if err := db.QueryRowContext(ctx,
		`SELECT count(*) FROM action_intent WHERE scenario_id = $1::UUID AND state = 'cancelled' AND action = $2`,
		sid, deployAction).Scan(&cancelledCount); err != nil {
		return fmt.Errorf("check cancelled intent: %w", err)
	}
	if cancelledCount != 1 {
		return fmt.Errorf("expected 1 cancelled intent, got %d", cancelledCount)
	}

	var auditCount int
	if err := db.QueryRowContext(ctx,
		`SELECT count(*) FROM action_intent a
		 JOIN belief b ON b.id = a.belief_id
		 WHERE a.state = 'live' AND b.status <> 'promoted' AND a.scenario_id = $1::UUID`,
		sid).Scan(&auditCount); err != nil {
		return fmt.Errorf("check audit: %w", err)
	}
	if auditCount != 0 {
		return fmt.Errorf("audit violation: %d live intents on non-promoted beliefs", auditCount)
	}

	var beliefCount int
	if err := db.QueryRowContext(ctx,
		`SELECT count(*) FROM belief WHERE scenario_id = $1::UUID`, sid).Scan(&beliefCount); err != nil {
		return fmt.Errorf("check belief count: %w", err)
	}
	if beliefCount != 3 {
		return fmt.Errorf("expected 3 beliefs, got %d", beliefCount)
	}

	// The derivation edge is part of the canonical state, not an optional extra: the
	// Tier 1 demo path does not exist without it. Checking it here also migrates an
	// already-deployed database — an older seed without the edge fails verification,
	// which routes through resetAndSeed and comes back with it.
	edge, err := demoseed.EdgeExists(ctx, db, sid)
	if err != nil {
		return fmt.Errorf("check derivation edge: %w", err)
	}
	if !edge {
		return fmt.Errorf("derivation edge %q --%s--> %q is missing",
			demoseed.ParentClaim, demoseed.Kind, demoseed.ChildClaim)
	}

	fmt.Println("Verification: PASSED (3 beliefs, 1 derivation edge, retracted baseline, cancelled intent, audit=0)")
	return nil
}
