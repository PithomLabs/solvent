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

	"github.com/PithomLabs/solvent/internal/kernel"
	"github.com/PithomLabs/solvent/internal/pipeline"
	"github.com/PithomLabs/solvent/internal/testdb"
	_ "github.com/jackc/pgx/v5/stdlib"
)

const (
	scenarioID          = "00000000-0000-0000-0000-000000000002"
	baselineClaim       = "etcd v3.5.0 is approved for production deployment (decision as of 2021-06-16)"
	baselineEvidenceURL = "https://github.com/etcd-io/etcd/releases/tag/v3.5.0"
	baselineEvidenceSHA = "f47656dfaad45b2ecdb32c3169b8897b153d7a8b2453ba8e7c34a2dcde609ce1"
	deployAction        = "deploy etcd v3.5.0"
	schemaPath          = "db/001_schema.sql"
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

	// 2. Check if tables exist.
	if tablesExist(ctx, db) {
		if !isDatabaseEmpty(ctx, db) {
			db.Close()
			log.Fatal("DATABASE NON-EMPTY BUT NO CANONICAL STATE: STOP — refusing to destroy unknown state")
		}
		fmt.Println("Tables exist but empty. Schema already applied, skipping to seed...")
	} else {
		fmt.Println("No tables found. Applying schema...")
		db.Close()
		if err := testdb.ApplySchema(ctx, dsn, schemaPath); err != nil {
			log.Fatalf("apply schema: %v", err)
		}
		db, err = testdb.Open(dsn)
		if err != nil {
			log.Fatalf("reopen: %v", err)
		}
	}

	// 3. Seed.
	seed(ctx, db)
}

func resetAndSeed(ctx context.Context, db *sql.DB, dsn string) {
	// Truncate all tables (works without defaultdb access).
	fmt.Println("Truncating all tables...")
	for _, table := range []string{"action_intent", "evidence", "belief_edge", "belief"} {
		if _, err := db.ExecContext(ctx, fmt.Sprintf("TRUNCATE %s CASCADE", table)); err != nil {
			fmt.Printf("Warning: truncate %s: %v (may not exist yet)\n", table, err)
		}
	}

	// Apply schema (idempotent — tolerates duplicate_table).
	if err := testdb.ApplySchema(ctx, dsn, schemaPath); err != nil {
		fmt.Printf("Warning: apply schema: %v\n", err)
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

func isDatabaseEmpty(ctx context.Context, db *sql.DB) bool {
	var b, e, i int
	db.QueryRowContext(ctx, `SELECT count(*) FROM belief`).Scan(&b)
	db.QueryRowContext(ctx, `SELECT count(*) FROM evidence`).Scan(&e)
	db.QueryRowContext(ctx, `SELECT count(*) FROM action_intent`).Scan(&i)
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

	fmt.Println("Verification: PASSED (3 beliefs, retracted baseline, cancelled intent, audit=0)")
	return nil
}
