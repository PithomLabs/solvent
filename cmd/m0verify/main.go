// Command m0verify is Milestone M0's connection smoke test and acceptance runner.
//
// It connects, optionally applies db/001_schema.sql, runs every M0 probe, and writes
// a deterministic transcript. It contains no kernel logic: M0 writes no kernel.
//
// Orchestration only — the probes live in internal/m0 (N1).
package main

import (
	"context"
	"database/sql"
	"flag"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib" // D-019: pgx/v5 is the sole approved driver

	"github.com/PithomLabs/solvent/internal/m0"
)

func main() {
	dsn := flag.String("dsn", os.Getenv("FABLE_DSN"), "CockroachDB connection string")
	schema := flag.String("schema", "db/001_schema.sql", "path to the frozen DDL")
	apply := flag.Bool("apply", false, "apply the schema before probing")
	reset := flag.Bool("reset", false, "drop and recreate the target database first")
	out := flag.String("out", "docs/M0_TRANSCRIPT.md", "transcript output path")
	noTimings := flag.Bool("no-timings", false, "omit elapsed_ms, for byte-stable diffs (E1)")
	flag.Parse()

	if *dsn == "" {
		fail("no DSN: pass --dsn or set FABLE_DSN")
	}
	if err := run(*dsn, *schema, *out, *apply, *reset, !*noTimings); err != nil {
		fail(err.Error())
	}
}

func run(dsn, schema, out string, apply, reset, showTimings bool) error {
	ctx := context.Background()

	if reset {
		if err := resetDatabase(ctx, dsn); err != nil {
			return fmt.Errorf("reset: %w", err)
		}
	}

	db, err := sql.Open("pgx", dsn)
	if err != nil {
		return fmt.Errorf("open: %w", err)
	}
	defer func() { _ = db.Close() }()
	if err := db.PingContext(ctx); err != nil {
		return fmt.Errorf("ping: %w", err)
	}

	runner := m0.NewRunner(db)

	m0.RunEnvironment(ctx, runner)

	if apply {
		if err := m0.ApplyDDL(ctx, runner, schema); err != nil {
			// B1 is already recorded as a failure; write the transcript and stop.
			_ = writeTranscript(out, dsn, schema, runner, showTimings)
			return reportFirstFailure(runner)
		}
	}

	before := m0.RecordSchemaSnapshot(ctx, runner)
	m0.RecordTableCount(ctx, runner)
	m0.RunGate(ctx, runner)
	m0.RecordSchemaStability(ctx, runner, before)

	if err := writeTranscript(out, dsn, schema, runner, showTimings); err != nil {
		return err
	}
	if runner.FirstFailure() != nil {
		return reportFirstFailure(runner)
	}
	fmt.Printf("M0 GREEN — %d probes passed. Transcript: %s\n", len(runner.Probes()), out)
	return nil
}

func writeTranscript(out, dsn, schema string, runner *m0.Runner, showTimings bool) error {
	host, _ := os.Hostname()
	header := m0.Header{
		GeneratedAt: time.Now().UTC().Format(time.RFC3339),
		Host:        host,
		DSN:         redact(dsn),
		SchemaPath:  schema,
	}
	if dir := filepath.Dir(out); dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return err
		}
	}
	return os.WriteFile(out, []byte(m0.Render(header, runner.Probes(), showTimings)), 0o644)
}

// reportFirstFailure prints exactly the three lines prompt_pkg02.md demands.
func reportFirstFailure(runner *m0.Runner) error {
	p := runner.FirstFailure()
	return fmt.Errorf("M0 BLOCKED at probe %s\n  criterion: %s\n  observed:  %s\n  expected:  %s",
		p.ID, p.Criterion, p.Observed, p.Expected)
}

// resetDatabase drops and recreates the DSN's database, connecting via defaultdb
// because a session cannot drop the database it is attached to.
func resetDatabase(ctx context.Context, dsn string) error {
	u, err := url.Parse(dsn)
	if err != nil {
		return err
	}
	name := strings.TrimPrefix(u.Path, "/")
	if name == "" {
		return fmt.Errorf("DSN has no database name")
	}

	admin := *u
	admin.Path = "/defaultdb"
	db, err := sql.Open("pgx", admin.String())
	if err != nil {
		return err
	}
	defer func() { _ = db.Close() }()

	if _, err := db.ExecContext(ctx, fmt.Sprintf("DROP DATABASE IF EXISTS %q CASCADE", name)); err != nil {
		return err
	}
	_, err = db.ExecContext(ctx, fmt.Sprintf("CREATE DATABASE %q", name))
	return err
}

func redact(dsn string) string {
	u, err := url.Parse(dsn)
	if err != nil {
		return "<unparseable>"
	}
	if u.User != nil {
		u.User = url.User(u.User.Username())
	}
	return u.String()
}

func fail(msg string) {
	fmt.Fprintln(os.Stderr, msg)
	os.Exit(1)
}
