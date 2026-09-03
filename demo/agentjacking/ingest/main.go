// Command agentjacking-ingest runs the Agentjacking demo adapter against a
// local Solvent ledger: it reads a Sentry-style fixture, converts it to generic
// external_feed evidence through internal/agentjacking, and prints the
// resulting ledger state.
//
// Demo-boundary tooling. It never executes payload content, never contacts any
// Sentry or npm endpoint, and the only network it touches is the local
// CockroachDB.
//
// The optional --reset clears only the given scenario's rows, mirroring the
// scenario-scoped delete set in internal/wizard/http_test.go. It exists so the
// demo can be re-run cleanly; it is disposable and is not used by any
// production package.
package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"flag"
	"fmt"
	"os"

	"github.com/PithomLabs/solvent/internal/agentjacking"
	"github.com/PithomLabs/solvent/internal/testdb"
	_ "github.com/jackc/pgx/v5/stdlib"
)

func main() {
	dsn := flag.String("dsn", "", "CockroachDB connection string (env: FABLE_DSN)")
	scenarioID := flag.String("scenario", "", "scenario UUID (required)")
	fixturePath := flag.String("fixture", "", "path to the Sentry event fixture (required)")
	reset := flag.Bool("reset", false, "clear only this scenario's rows before ingesting")
	flag.Parse()

	if *dsn == "" {
		*dsn = os.Getenv("FABLE_DSN")
	}
	if *dsn == "" {
		fail("no DSN: pass --dsn or set FABLE_DSN")
	}
	if *scenarioID == "" || *fixturePath == "" {
		fail("--scenario and --fixture are required")
	}

	db, err := testdb.Open(*dsn)
	if err != nil {
		fail(fmt.Sprintf("open: %v", err))
	}
	defer func() { _ = db.Close() }()

	ctx := context.Background()
	if *reset {
		if err := resetScenario(ctx, db, *scenarioID); err != nil {
			fail(fmt.Sprintf("reset scenario: %v", err))
		}
		fmt.Println("reset: scenario rows cleared")
	}

	res, err := agentjacking.Ingest(ctx, db, *scenarioID, *fixturePath)
	if err != nil {
		fail(err.Error())
	}

	out, err := json.MarshalIndent(res, "", "  ")
	if err != nil {
		fail(err.Error())
	}
	fmt.Println(string(out))
}

// resetScenario deletes every ledger row belonging to one scenario, in child
// order. This mirrors the delete set in internal/wizard/http_test.go and stays
// strictly scenario-scoped: other scenarios are untouched, and the database
// itself is never dropped.
func resetScenario(ctx context.Context, db *sql.DB, scenarioID string) error {
	stmts := []string{
		`DELETE FROM belief_corpus_citation WHERE belief_id IN (SELECT id FROM belief WHERE scenario_id = $1::UUID)`,
		`DELETE FROM refusal_log WHERE scenario_id = $1::UUID`,
		`DELETE FROM action_intent WHERE scenario_id = $1::UUID`,
		`DELETE FROM belief_edge WHERE parent_id IN (SELECT id FROM belief WHERE scenario_id = $1::UUID)`,
		`DELETE FROM evidence WHERE scenario_id = $1::UUID`,
		`DELETE FROM belief WHERE scenario_id = $1::UUID`,
	}
	for _, stmt := range stmts {
		if _, err := db.ExecContext(ctx, stmt, scenarioID); err != nil {
			return fmt.Errorf("%s: %w", stmt, err)
		}
	}
	return nil
}

func fail(msg string) {
	fmt.Fprintln(os.Stderr, "agentjacking-ingest: "+msg)
	os.Exit(1)
}
