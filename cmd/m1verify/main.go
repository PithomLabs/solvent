// Command m1verify prepares every statement in the kernel's SQL catalog against a
// live, schema-applied database, then deallocates it.
//
// This is a SQL compile check, not an invariant check. Preparation runs
// CockroachDB's optimizer build, so it resolves relation names, column names and
// placeholder types — catching 42P01, 42703, 42P18 and 42804 at M1 instead of M2. It
// cannot catch constraint violations, row counts, or any semantics: 23514 and 23503
// arise only at execution. Proving behavior is M2's job.
//
// Nothing is executed. No transaction is opened. No row is written.
package main

import (
	"context"
	"database/sql"
	"errors"
	"flag"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib" // D-019: pgx/v5 is the sole approved driver

	"github.com/PithomLabs/solvent/internal/kernel"
)

type result struct {
	Name     string
	Prepared bool
	SQLState string
	Message  string
	Elapsed  time.Duration
}

func main() {
	dsn := flag.String("dsn", os.Getenv("FABLE_DSN"), "CockroachDB connection string")
	out := flag.String("out", "docs/M1_PREPARE.md", "transcript output path")
	skip := flag.Bool("skip", false, "skip the sweep (no cluster available) — reported loudly, never green")
	flag.Parse()

	if *skip {
		fmt.Fprintln(os.Stderr, "M1 PREPARE SWEEP SKIPPED — this does NOT count as green (M1-R9).")
		fmt.Fprintln(os.Stderr, "  criterion: all catalog statements prepare against a schema-applied database")
		fmt.Fprintln(os.Stderr, "  observed:  sweep skipped by explicit --skip")
		fmt.Fprintln(os.Stderr, "  expected:  8 statements prepared")
		os.Exit(2)
	}
	if *dsn == "" {
		fail("no DSN: pass --dsn or set FABLE_DSN")
	}
	if err := run(*dsn, *out); err != nil {
		fail(err.Error())
	}
}

func run(dsn, out string) error {
	ctx := context.Background()

	db, err := sql.Open("pgx", dsn)
	if err != nil {
		return fmt.Errorf("open: %w", err)
	}
	defer func() { _ = db.Close() }()
	if err := db.PingContext(ctx); err != nil {
		return fmt.Errorf("ping: %w", err)
	}

	// One connection for the whole sweep: prepared statement names are session-scoped,
	// so pinning the session keeps PREPARE and DEALLOCATE paired.
	conn, err := db.Conn(ctx)
	if err != nil {
		return fmt.Errorf("conn: %w", err)
	}
	defer func() { _ = conn.Close() }()

	catalog := kernel.SQLCatalog()
	results := make([]result, 0, len(catalog))
	var firstFailure *result

	for _, stmt := range catalog {
		r := prepareOne(ctx, conn, stmt)
		results = append(results, r)
		if !r.Prepared && firstFailure == nil {
			cp := r
			firstFailure = &cp
		}
	}

	if err := writeTranscript(out, dsn, results); err != nil {
		return err
	}

	if firstFailure != nil {
		return fmt.Errorf("M1 BLOCKED at statement %s\n  criterion: every kernel statement prepares against the frozen schema\n  observed:  SQLSTATE %s — %s\n  expected:  prepared with no error",
			firstFailure.Name, dash(firstFailure.SQLState), firstFailure.Message)
	}

	fmt.Printf("PREPARE sweep green — %d/%d statements prepared. Transcript: %s\n",
		len(results), len(catalog), out)
	return nil
}

// prepareOne prepares a single statement and immediately deallocates it. The
// statement is never executed.
func prepareOne(ctx context.Context, conn *sql.Conn, stmt kernel.NamedSQL) result {
	name := "verify_" + stmt.Name

	start := time.Now()
	_, err := conn.ExecContext(ctx, fmt.Sprintf("PREPARE %s AS %s", name, stmt.SQL))
	elapsed := time.Since(start)

	if err != nil {
		return result{
			Name:     stmt.Name,
			SQLState: sqlStateOf(err),
			Message:  collapse(err.Error()),
			Elapsed:  elapsed,
		}
	}

	// Deallocate so the sweep leaves no session state behind.
	if _, derr := conn.ExecContext(ctx, "DEALLOCATE "+name); derr != nil {
		return result{
			Name:     stmt.Name,
			SQLState: sqlStateOf(derr),
			Message:  "prepared, but DEALLOCATE failed: " + collapse(derr.Error()),
			Elapsed:  elapsed,
		}
	}
	return result{Name: stmt.Name, Prepared: true, Elapsed: elapsed}
}

func writeTranscript(out, dsn string, results []result) error {
	var b strings.Builder
	b.WriteString("# M1 PREPARE Transcript\n\n")
	b.WriteString("Every statement in `kernel.SQLCatalog()`, prepared against a schema-applied\n")
	b.WriteString("database and immediately deallocated. **Nothing was executed; no row was written.**\n\n")
	b.WriteString("This is a SQL compile check, not an invariant check: it proves relations, columns,\n")
	b.WriteString("placeholder types and statement structure. It proves no semantics — 23514 and 23503\n")
	b.WriteString("arise only at execution, which is M2's gate.\n\n")

	b.WriteString("<!-- volatile:begin -->\n\n")
	host, _ := os.Hostname()
	b.WriteString("| run fact | value |\n|---|---|\n")
	b.WriteString("| generated_at | " + time.Now().UTC().Format(time.RFC3339) + " |\n")
	b.WriteString("| host | " + host + " |\n")
	b.WriteString("| dsn | " + redact(dsn) + " |\n\n")
	b.WriteString("<!-- volatile:end -->\n\n")

	passed := 0
	for _, r := range results {
		if r.Prepared {
			passed++
		}
	}
	b.WriteString("## Verdict\n\n")
	if passed == len(results) {
		b.WriteString(fmt.Sprintf("**GREEN** — %d/%d statements prepared, none skipped.\n\n", passed, len(results)))
	} else {
		b.WriteString(fmt.Sprintf("**BLOCKED** — %d/%d prepared.\n\n", passed, len(results)))
	}

	b.WriteString("## Statements\n\n")
	b.WriteString("| statement | prepared | sqlstate | message | elapsed_ms |\n")
	b.WriteString("|---|---|---|---|---|\n")
	for _, r := range results {
		status := "PASS"
		if !r.Prepared {
			status = "**FAIL**"
		}
		b.WriteString(fmt.Sprintf("| `verify_%s` | %s | %s | %s | %d |\n",
			r.Name, status, dash(r.SQLState), dash(r.Message), r.Elapsed.Milliseconds()))
	}
	b.WriteString("\n## Prepared SQL\n\n")
	for _, stmt := range kernel.SQLCatalog() {
		b.WriteString("### " + stmt.Name + "\n\n```sql\n" + strings.TrimSpace(dedent(stmt.SQL)) + "\n```\n\n")
	}

	if dir := filepath.Dir(out); dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return err
		}
	}
	return os.WriteFile(out, []byte(b.String()), 0o644)
}

type sqlStater interface{ SQLState() string }

func sqlStateOf(err error) string {
	var s sqlStater
	if errors.As(err, &s) {
		return s.SQLState()
	}
	return ""
}

func collapse(s string) string {
	s = strings.ReplaceAll(s, "\r\n", "\n")
	s = strings.ReplaceAll(s, "\n", " / ")
	s = strings.ReplaceAll(s, "|", "\\|")
	return strings.TrimSpace(s)
}

// dedent strips the leading tabs the Go raw string literals carry, so the
// transcript renders the SQL readably.
func dedent(s string) string {
	lines := strings.Split(s, "\n")
	for i, l := range lines {
		lines[i] = strings.TrimPrefix(l, "\t\t")
	}
	return strings.Join(lines, "\n")
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

func dash(s string) string {
	if s == "" {
		return "—"
	}
	return s
}

func fail(msg string) {
	fmt.Fprintln(os.Stderr, msg)
	os.Exit(1)
}
