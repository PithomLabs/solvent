// Command solvent is the minimum integration layer for Wave 4.
//
// It processes evidence fixtures through the full pipeline:
//
//	normalize → derive → belief.Process → intent.Audit
//
// and prints a deterministic transcript proving pipeline correctness.
package main

import (
	"context"
	"flag"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/PithomLabs/solvent/internal/demoseed"
	"github.com/PithomLabs/solvent/internal/pipeline"
	"github.com/PithomLabs/solvent/internal/testdb"
	"github.com/PithomLabs/solvent/kernel"

	_ "github.com/jackc/pgx/v5/stdlib"
)

func main() {
	dsn := flag.String("dsn", "", "CockroachDB connection string (env: FABLE_DSN)")
	schema := flag.String("schema", "db/001_schema.sql", "path to the frozen DDL")
	scenario := flag.String("scenario", "", "scenario ID (required for deterministic replay)")
	reset := flag.Bool("reset", false, "drop and recreate the target database first")
	fixtures := flag.String("fixtures", "", "path to evidence fixture directory (default: internal/derive/testdata relative to repo root)")
	out := flag.String("out", "", "transcript output path (default: stdout)")
	falsify := flag.String("falsify", "", "belief ID to retract atomically (RetractCascade); mutually exclusive with --scenario pipeline")
	fileDemoEdge := flag.Bool("file-demo-edge", false, "after the pipeline runs, file the demo derivation edge (see internal/demoseed)")
	flag.Parse()

	// Resolve DSN: flag > env > error.
	if *dsn == "" {
		*dsn = os.Getenv("FABLE_DSN")
	}
	if *dsn == "" {
		fail("no DSN: pass --dsn or set FABLE_DSN")
	}

	// Falsify mode: retract a belief and its cascade atomically.
	if *falsify != "" {
		if *scenario == "" {
			fail("--falsify requires --scenario to scope the cascade")
		}
		runFalsify(*dsn, resolveSchemaPaths(*schema, ""), *scenario, *falsify, *reset)
		return
	}

	// Normal pipeline mode requires --scenario.
	if *scenario == "" {
		fail("no scenario ID: pass --scenario (required for deterministic replay)")
	}

	// Resolve fixture directory.
	fixtureDir := *fixtures
	if fixtureDir == "" {
		fixtureDir = defaultFixtureDir()
	}
	if _, err := os.Stat(fixtureDir); os.IsNotExist(err) {
		fail("fixture directory not found: " + fixtureDir)
	}

	schemaPaths := resolveSchemaPaths(*schema, fixtureDir)

	ctx := context.Background()

	if *reset {
		if err := cliReset(ctx, *dsn, schemaPaths); err != nil {
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

	// The derivation edge is filed here, behind an explicit flag, so the local demo
	// and the cloud initializer produce the same canonical state from the same code.
	// The pipeline creates the two claims it connects; nothing else does.
	if *fileDemoEdge {
		filed, err := demoseed.FileDerivationEdge(ctx, db, *scenario)
		if err != nil {
			fail(fmt.Sprintf("file-demo-edge: %v", err))
		}
		fmt.Printf("demo edge: %q --%s--> %q (newly filed: %t)\n",
			demoseed.ParentClaim, demoseed.Kind, demoseed.ChildClaim, filed)
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

// runFalsify atomically retracts a belief and its cascade, then audits.
func runFalsify(dsn string, schemaPaths []string, scenarioID, beliefID string, reset bool) {
	ctx := context.Background()

	if reset {
		if err := cliReset(ctx, dsn, schemaPaths); err != nil {
			fail(fmt.Sprintf("reset: %v", err))
		}
	}

	db, err := testdb.Open(dsn)
	if err != nil {
		fail(fmt.Sprintf("open: %v", err))
	}
	defer func() { _ = db.Close() }()
	if err := db.PingContext(ctx); err != nil {
		fail(fmt.Sprintf("ping: %v", err))
	}

	st := kernel.New(db)

	retracted, err := st.RetractCascade(ctx, scenarioID, beliefID)
	if err != nil {
		fail(fmt.Sprintf("retract cascade: %v", err))
	}

	auditCount, err := pipeline.AuditIntent(ctx, db, scenarioID)
	if err != nil {
		fail(fmt.Sprintf("audit: %v", err))
	}

	var b strings.Builder
	b.WriteString("FALSIFY\n")
	b.WriteString(fmt.Sprintf("belief: %s\n", beliefID))
	b.WriteString(fmt.Sprintf("cascade: retracted %d belief(s)\n", retracted))
	b.WriteString(fmt.Sprintf("audit_live_on_nonpromoted: %d\n", auditCount))
	b.WriteString("\n")

	if auditCount != 0 {
		b.WriteString("VERDICT: FAIL (audit ≠ 0)\n")
	} else if retracted == 0 {
		b.WriteString("VERDICT: NO-OP\n")
	} else {
		b.WriteString("VERDICT: PASS\n")
	}

	fmt.Print(b.String())
}

// cliReset drops and recreates the database without the _test suffix guard.
// This is the CLI equivalent of testdb.Reset, for non-test databases.
func cliReset(ctx context.Context, dsn string, schemaPaths []string) error {
	u, err := url.Parse(dsn)
	if err != nil {
		return fmt.Errorf("parse DSN: %w", err)
	}
	name := strings.TrimPrefix(u.Path, "/")
	if name == "" {
		return fmt.Errorf("DSN carries no database name")
	}

	fmt.Printf("=== Solvent CLI === resetting database\n")
	fmt.Printf("    dsn:      %s\n", testdb.Redact(dsn))
	fmt.Printf("    database: %s  (DROP + CREATE + apply %s)\n", name, strings.Join(schemaPaths, ", "))

	admin := dsn
	adminURL, _ := url.Parse(dsn)
	adminURL.Path = "/defaultdb"
	admin = adminURL.String()

	adminDB, err := testdb.Open(admin)
	if err != nil {
		return err
	}
	defer func() { _ = adminDB.Close() }()

	if _, err := adminDB.ExecContext(ctx, fmt.Sprintf("DROP DATABASE IF EXISTS %q CASCADE", name)); err != nil {
		return fmt.Errorf("drop %s: %w", name, err)
	}
	if _, err := adminDB.ExecContext(ctx, fmt.Sprintf("CREATE DATABASE IF NOT EXISTS %q", name)); err != nil {
		return fmt.Errorf("create %s: %w", name, err)
	}

	return testdb.ApplySchema(ctx, dsn, schemaPaths...)
}

// resolveSchemaPaths locates the frozen DDL and, beside it, every layer above it.
//
// The frozen path is tried against CWD first, then relative to the fixture directory
// when one is given. Each later layer is appended when it exists next to
// db/001_schema.sql, in order, because --reset drops and recreates the database: miss a
// layer here and the reset silently removes tables the application still expects.
//
// That is not hypothetical. When db/003_wizard.sql was added, this function still knew
// only about 002, so `task demo:track2` — which resets through this path — rebuilt the
// database without the wizard layer, and the wizard's first query failed with
// `column "c.relation" does not exist`. Any new layer must be added here as well as to
// demo/cloud/init, the Taskfile, and every test suite's schemaPaths.
func resolveSchemaPaths(schema, fixtureDir string) []string {
	schemaPath := schema
	if _, err := os.Stat(schemaPath); os.IsNotExist(err) && fixtureDir != "" {
		relToFixture := filepath.Join(filepath.Dir(fixtureDir), "..", "..", schemaPath)
		if _, err2 := os.Stat(relToFixture); err2 == nil {
			schemaPath = relToFixture
		}
	}
	paths := []string{schemaPath}
	// Ordered: 002 references belief(id), and 003 alters a table 002 creates.
	for _, layer := range []string{"002_corpus.sql", "003_wizard.sql", "004_debt_vocabulary.sql"} {
		if p := filepath.Join(filepath.Dir(schemaPath), layer); fileExists(p) {
			paths = append(paths, p)
		}
	}
	return paths
}

// fileExists reports whether path names an existing file.
func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// defaultFixtureDir returns the default fixture directory path.
// It resolves relative to the current working directory first, then
// tries to find it relative to the module root by walking up from the
// binary's location.
func defaultFixtureDir() string {
	rel := filepath.Join("internal", "derive", "testdata")
	if _, err := os.Stat(rel); err == nil {
		return rel
	}
	// Fallback: try from the binary's directory.
	exe, err := os.Executable()
	if err == nil {
		p := filepath.Join(filepath.Dir(exe), rel)
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	return rel
}

func intentState(r pipeline.Result) string {
	return r.IntentState
}

func fail(msg string) {
	fmt.Fprintln(os.Stderr, msg)
	os.Exit(1)
}
