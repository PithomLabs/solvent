// Command solvent is the minimum integration layer for Wave 4.
//
// It processes evidence fixtures through the full pipeline:
//
//	normalize → derive → belief.Process → ProposeIfNew → intent.Audit
//
// and prints a deterministic transcript proving pipeline correctness.
//
// Intent proposal is replay-idempotent under sequential execution;
// concurrent duplicate-intent prevention is deferred.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/PithomLabs/solvent/internal/pipeline"
	"github.com/PithomLabs/solvent/internal/testdb"

	_ "github.com/jackc/pgx/v5/stdlib"
)

func main() {
	dsn := flag.String("dsn", testdb.DSN(), "CockroachDB connection string")
	schema := flag.String("schema", "db/001_schema.sql", "path to the frozen DDL")
	scenario := flag.String("scenario", "", "scenario ID (required for deterministic replay)")
	reset := flag.Bool("reset", false, "drop and recreate the target database first")
	out := flag.String("out", "", "transcript output path (default: stdout)")
	flag.Parse()

	if *dsn == "" {
		fail("no DSN: pass --dsn or set FABLE_DSN")
	}
	if *scenario == "" {
		fail("no scenario ID: pass --scenario (required for deterministic replay)")
	}

	fixtureDir := filepath.Join("internal", "derive", "testdata")
	if _, err := os.Stat(fixtureDir); os.IsNotExist(err) {
		fail("fixture directory not found: " + fixtureDir)
	}

	ctx := context.Background()

	if *reset {
		if err := testdb.Reset(ctx, *dsn, *schema); err != nil {
			fail(fmt.Sprintf("reset: %v", err))
		}
	}

	db, err := testdb.Open(*dsn)
	if err != nil {
		fail(fmt.Sprintf("open: %v", err))
	}
	defer func() { _ = db.Close() }()
	if err := db.PingContext(ctx); err != nil {
		fail(fmt.Sprintf("ping: %v", err))
	}

	results, err := pipeline.Run(ctx, db, *scenario, fixtureDir)
	if err != nil {
		fail(fmt.Sprintf("pipeline: %v", err))
	}

	// Audit.
	auditCount, err := pipeline.AuditIntent(ctx, db, *scenario)
	if err != nil {
		fail(fmt.Sprintf("audit: %v", err))
	}

	// Transcript.
	var b strings.Builder
	b.WriteString("SOLVENT TRANSCRIPT\n")
	b.WriteString("==================\n\n")
	b.WriteString(fmt.Sprintf("scenario: %s\n", *scenario))
	b.WriteString(fmt.Sprintf("results:  %d\n\n", len(results)))

	for i, r := range results {
		b.WriteString(fmt.Sprintf("--- result %d ---\n", i+1))
		b.WriteString(fmt.Sprintf("  source:       %s\n", r.Normalized.SourceType))
		b.WriteString(fmt.Sprintf("  source_url:   %s\n", r.Normalized.SourceURL))

		if r.Contradiction {
			b.WriteString("  contradiction: true\n")
			b.WriteString("  ledger_mutation: none\n")
			b.WriteString("\n")
			continue
		}

		for _, d := range r.Beliefs {
			b.WriteString(fmt.Sprintf("  claim:        %s\n", d.Claim))
			b.WriteString(fmt.Sprintf("  classification: %s\n", d.Classification))
		}

		if r.BeliefID != "" {
			b.WriteString(fmt.Sprintf("  belief_id:    %s\n", r.BeliefID))
			b.WriteString(fmt.Sprintf("  promoted:     %v\n", r.Promoted))
			b.WriteString(fmt.Sprintf("  debt_items:   %d\n", len(r.DebtItems)))
			b.WriteString(fmt.Sprintf("  intent:       %s\n", intentState(r)))
		}
		b.WriteString("\n")
	}

	b.WriteString("--- audit ---\n")
	b.WriteString(fmt.Sprintf("audit_live_on_nonpromoted: %d\n", auditCount))
	b.WriteString("\n")

	if auditCount != 0 {
		b.WriteString("VERDICT: FAIL (audit ≠ 0)\n")
	} else {
		b.WriteString("VERDICT: PASS\n")
	}

	transcript := b.String()

	if *out != "" {
		if dir := filepath.Dir(*out); dir != "." {
			if err := os.MkdirAll(dir, 0o755); err != nil {
				fail(fmt.Sprintf("mkdir: %v", err))
			}
		}
		if err := os.WriteFile(*out, []byte(transcript), 0o644); err != nil {
			fail(fmt.Sprintf("write: %v", err))
		}
		fmt.Printf("Transcript written to %s\n", *out)
	} else {
		fmt.Print(transcript)
	}
}

func intentState(r pipeline.Result) string {
	if r.IntentState != "" {
		return r.IntentState
	}
	if r.Promoted {
		return "live"
	}
	return ""
}

func fail(msg string) {
	fmt.Fprintln(os.Stderr, msg)
	os.Exit(1)
}
