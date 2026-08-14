// Verification-only. This package is M0's executable specification, not production
// infrastructure. It must never be imported by the kernel package (D-023) and is frozen
// at the close of M0 (O1).
package m0

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"
)

// expectedTables is contract §2, as amended in Phase 2 (D-P2-3).
//
// The first four are the frozen ledger and are unchanged. The two corpus tables
// were authorised by the Phase 0 table waiver (D-P0-1): external issues cannot
// live in evidence, because evidence.belief_id is NOT NULL, and a corpus row
// exists before any belief is formed about it.
//
// The order here is the catalog's, not a grouping: RecordTableCount compares this
// slice to "ORDER BY table_name" with position-by-position equality, so it must be
// sorted. Corpus tables are marked in the trailing comments.
var expectedTables = []string{
	"action_intent",
	"belief",
	"belief_corpus_citation", // corpus layer
	"belief_edge",
	"corpus_issue", // corpus layer
	"evidence",
	"refusal_log", // wizard layer, db/003_wizard.sql -- approved Phase 5 addition
}

// ApplyDDL runs db/001_schema.sql statement by statement. The DDL is frozen; this
// function may not rewrite it, only execute it. Any error aborts M0 (B1).
func ApplyDDL(ctx context.Context, r *Runner, path string) error {
	raw, err := os.ReadFile(path)
	if err != nil {
		r.Record(Probe{
			ID:        "B1",
			Criterion: "db/001_schema.sql applies with zero errors (contract §7 M0)",
			Statement: "read " + path,
			Expected:  "file readable, every statement applies",
			Observed:  "could not read the schema file",
			RawError:  collapse(err.Error()),
		})
		return err
	}

	stmts := splitStatements(string(raw))

	var failedAt string
	var capturedErr error
	start := time.Now()
	for _, stmt := range stmts {
		if _, e := r.DB().ExecContext(ctx, stmt); e != nil {
			failedAt, capturedErr = stmt, e
			break
		}
	}
	elapsed := time.Since(start)

	p := Probe{
		ID:         "B1",
		Criterion:  "db/001_schema.sql applies with zero errors (contract §7 M0)",
		Statement:  fmt.Sprintf("apply %s (%d statements)", path, len(stmts)),
		Expected:   fmt.Sprintf("all %d statements apply", len(stmts)),
		SQLState:   sqlStateOf(capturedErr),
		Constraint: constraintOf(capturedErr),
		RawError:   rawOf(capturedErr),
		Elapsed:    elapsed,
		Pass:       capturedErr == nil,
	}
	if capturedErr == nil {
		p.Observed = fmt.Sprintf("all %d statements applied", len(stmts))
	} else {
		p.Observed = fmt.Sprintf("failed with SQLSTATE %s", p.SQLState)
		p.Detail = "failing statement:\n\n    " + strings.ReplaceAll(failedAt, "\n", "\n    ") +
			"\n\nraw driver error:\n\n    " + strings.ReplaceAll(p.RawError, " / ", "\n    ")
	}
	r.Record(p)
	return capturedErr
}

// SnapshotSchema captures SHOW CREATE TABLE for every table, sorted by name (N4).
// The returned string is the comparison unit for B2 vs B4 (D-025).
func SnapshotSchema(ctx context.Context, db *sql.DB) (string, error) {
	names, err := tableNames(ctx, db)
	if err != nil {
		return "", err
	}

	var b strings.Builder
	for _, name := range names {
		var tbl, create string
		// SHOW CREATE TABLE does not accept placeholders; the name comes from the
		// catalog, not from user input.
		row := db.QueryRowContext(ctx, fmt.Sprintf("SHOW CREATE TABLE %s", quoteIdent(name)))
		if err := row.Scan(&tbl, &create); err != nil {
			return "", fmt.Errorf("SHOW CREATE TABLE %s: %w", name, err)
		}
		b.WriteString(create)
		b.WriteString("\n\n")
	}
	return strings.TrimRight(b.String(), "\n") + "\n", nil
}

// RecordSchemaSnapshot runs B2: capture the DDL CockroachDB actually stored.
func RecordSchemaSnapshot(ctx context.Context, r *Runner) string {
	start := time.Now()
	snap, err := SnapshotSchema(ctx, r.DB())
	elapsed := time.Since(start)

	p := Probe{
		ID:        "B2",
		Criterion: "SHOW CREATE TABLE preserves the load-bearing constraints (gate FK ON UPDATE CASCADE, UNIQUE(id,status), both CHECKs, partial index)",
		Statement: "SHOW CREATE TABLE for all tables, sorted by name",
		Expected:  "snapshot captured; contains ON UPDATE CASCADE, belief_id_status_key, promoted_is_debt_free, live_requires_promoted, live_intents",
		RawError:  rawOf(err),
		Elapsed:   elapsed,
	}

	required := []string{
		"ON UPDATE CASCADE",
		"belief_id_status_key",
		"promoted_is_debt_free",
		"live_requires_promoted",
		"live_intents",
	}
	var missing []string
	for _, want := range required {
		if !strings.Contains(snap, want) {
			missing = append(missing, want)
		}
	}

	switch {
	case err != nil:
		p.Observed = "snapshot failed"
	case len(missing) > 0:
		p.Observed = "snapshot captured but missing: " + strings.Join(missing, ", ")
	default:
		p.Observed = "snapshot captured; all five markers present"
		p.Pass = true
	}
	p.Detail = "```sql\n" + snap + "```"
	r.Record(p)
	return snap
}

// RecordTableCount runs B3: contract §2 as amended (D-P2-3) permits the four frozen
// ledger tables plus the two corpus tables, and nothing else.
func RecordTableCount(ctx context.Context, r *Runner) {
	start := time.Now()
	names, err := tableNames(ctx, r.DB())
	elapsed := time.Since(start)

	p := Probe{
		ID:        "B3",
		Criterion: "Exactly the contracted tables exist: the four frozen ledger tables, the two corpus tables, and the wizard refusal log (contract §2 as amended, D-P2-3 and Phase 5)",
		Statement: "SELECT table_name FROM information_schema.tables WHERE table_schema='public' AND table_type='BASE TABLE' ORDER BY table_name",
		Expected:  strings.Join(expectedTables, ", "),
		Observed:  strings.Join(names, ", "),
		RawError:  rawOf(err),
		Elapsed:   elapsed,
		Pass:      err == nil && equalStrings(names, expectedTables),
	}
	r.Record(p)
}

// RecordSchemaStability runs B4 (N2 / D-025): re-snapshot after the D-series and
// prove the cascade did not rewrite catalog metadata.
func RecordSchemaStability(ctx context.Context, r *Runner, before string) {
	start := time.Now()
	after, err := SnapshotSchema(ctx, r.DB())
	elapsed := time.Since(start)

	p := Probe{
		ID:        "B4",
		Criterion: "Schema metadata is byte-identical before and after the D-series cascade (D-025)",
		Statement: "SHOW CREATE TABLE for all tables, re-run after D5",
		Expected:  "byte-identical to the B2 snapshot",
		RawError:  rawOf(err),
		Elapsed:   elapsed,
	}
	switch {
	case err != nil:
		p.Observed = "re-snapshot failed"
	case after == before:
		p.Observed = "identical — cascade did not rewrite metadata"
		p.Pass = true
	default:
		p.Observed = "DIFFERS — CockroachDB rewrote catalog metadata during cascade"
		p.Detail = "unified diff (B2 → B4):\n\n```diff\n" + unifiedDiff(before, after) + "```"
	}
	r.Record(p)
}

func tableNames(ctx context.Context, db *sql.DB) ([]string, error) {
	rows, err := db.QueryContext(ctx, `
		SELECT table_name FROM information_schema.tables
		WHERE table_schema = 'public' AND table_type = 'BASE TABLE'
		ORDER BY table_name`)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var names []string
	for rows.Next() {
		var n string
		if err := rows.Scan(&n); err != nil {
			return nil, err
		}
		names = append(names, n)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	sort.Strings(names) // N4: never rely on server ordering for a deterministic artifact
	return names, nil
}

// splitStatements strips line comments and splits on semicolons. The frozen DDL
// contains no dollar-quoting or string literals with semicolons, so this is exact
// for db/001_schema.sql; it is deliberately not a general SQL parser.
func splitStatements(sqlText string) []string {
	var cleaned strings.Builder
	for _, line := range strings.Split(sqlText, "\n") {
		if idx := strings.Index(line, "--"); idx >= 0 {
			line = line[:idx]
		}
		cleaned.WriteString(line)
		cleaned.WriteString("\n")
	}

	var out []string
	for _, part := range strings.Split(cleaned.String(), ";") {
		if s := strings.TrimSpace(part); s != "" {
			out = append(out, s)
		}
	}
	return out
}

func quoteIdent(s string) string {
	return `"` + strings.ReplaceAll(s, `"`, `""`) + `"`
}

func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// unifiedDiff is a minimal line diff, adequate for reporting an unexpected catalog
// change. It is not a general diff implementation.
func unifiedDiff(before, after string) string {
	b := strings.Split(before, "\n")
	a := strings.Split(after, "\n")

	var out strings.Builder
	max := len(b)
	if len(a) > max {
		max = len(a)
	}
	for i := 0; i < max; i++ {
		var bl, al string
		if i < len(b) {
			bl = b[i]
		}
		if i < len(a) {
			al = a[i]
		}
		if bl == al {
			continue
		}
		if i < len(b) {
			out.WriteString("- " + bl + "\n")
		}
		if i < len(a) {
			out.WriteString("+ " + al + "\n")
		}
	}
	return out.String()
}
