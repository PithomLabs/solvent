// Verification-only. This package is M0's executable specification, not production
// infrastructure. It must never be imported by the kernel package (D-023) and is frozen
// at the close of M0 (O1).
package m0

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
)

// Probe is one M0 acceptance check and its receipt. Every field is recorded, whether
// the probe passed or failed: opinions are not receipts.
type Probe struct {
	ID         string
	Criterion  string
	Statement  string
	Expected   string
	Observed   string
	SQLState   string
	Constraint string
	RawError   string
	Detail     string // multi-line block rendered under "Details" (SHOW CREATE, diffs)
	Elapsed    time.Duration
	Pass       bool
}

// Runner accumulates probes and remembers the first failure. It never panics and
// never stops early on its own: the caller decides, so that a failure late in the
// D-series still yields a complete transcript.
type Runner struct {
	db     *sql.DB
	probes []Probe
	first  *Probe
}

func NewRunner(db *sql.DB) *Runner { return &Runner{db: db} }

func (r *Runner) DB() *sql.DB { return r.db }

func (r *Runner) Record(p Probe) {
	r.probes = append(r.probes, p)
	if !p.Pass && r.first == nil {
		cp := p
		r.first = &cp
	}
}

func (r *Runner) Probes() []Probe { return r.probes }

// FirstFailure returns the earliest failing probe, or nil if M0 is green.
func (r *Runner) FirstFailure() *Probe { return r.first }

// --- error classification: pgx SQLSTATE, never substring matching on error text ---

// pgErrOf extracts the structured PostgreSQL error. D-019 pins pgx/v5 precisely
// because *pgconn.PgError carries Code and ConstraintName as fields, and satisfies
// the interface{ SQLState() string } that the kernel classifies through.
func pgErrOf(err error) *pgconn.PgError {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		return pgErr
	}
	return nil
}

func sqlStateOf(err error) string {
	if pgErr := pgErrOf(err); pgErr != nil {
		return pgErr.Code
	}
	return ""
}

func constraintOf(err error) string {
	if pgErr := pgErrOf(err); pgErr != nil {
		return pgErr.ConstraintName
	}
	return ""
}

func rawOf(err error) string {
	if err == nil {
		return ""
	}
	return collapse(err.Error())
}

// collapse makes a driver message safe for a Markdown table cell without altering
// its content: newlines become " / ", pipes are escaped.
func collapse(s string) string {
	s = strings.ReplaceAll(s, "\r\n", "\n")
	s = strings.ReplaceAll(s, "\n", " / ")
	s = strings.ReplaceAll(s, "|", "\\|")
	return strings.TrimSpace(s)
}

// timed runs fn and returns how long it took.
func timed(fn func() error) (time.Duration, error) {
	start := time.Now()
	err := fn()
	return time.Since(start), err
}

// ExpectFailure runs stmt inside its own transaction, which is always rolled back,
// and asserts the driver reported wantState (and, if given, wantConstraint).
//
// The pass gate is the SQLSTATE, because that is what contract §7 specifies. The
// constraint name is checked when supplied but tolerates CockroachDB placing it in
// the message rather than the structured field — either location counts, and the
// transcript records which one it was.
func (r *Runner) ExpectFailure(ctx context.Context, id, criterion, wantState, wantConstraint, stmt string) {
	var capturedErr error
	elapsed, _ := timed(func() error {
		tx, err := r.db.BeginTx(ctx, nil)
		if err != nil {
			capturedErr = err
			return nil
		}
		defer func() { _ = tx.Rollback() }()
		_, capturedErr = tx.ExecContext(ctx, stmt)
		return nil
	})

	p := Probe{
		ID:         id,
		Criterion:  criterion,
		Statement:  collapse(stmt),
		Expected:   expectationText(wantState, wantConstraint),
		SQLState:   sqlStateOf(capturedErr),
		Constraint: constraintOf(capturedErr),
		RawError:   rawOf(capturedErr),
		Elapsed:    elapsed,
	}

	switch {
	case capturedErr == nil:
		p.Observed = "statement SUCCEEDED — expected refusal"
	case p.SQLState == "":
		p.Observed = "error carried no SQLSTATE (not a *pgconn.PgError)"
	default:
		p.Observed = fmt.Sprintf("refused with SQLSTATE %s", p.SQLState)
	}

	p.Pass = capturedErr != nil && p.SQLState == wantState && constraintSatisfied(wantConstraint, p)
	if p.Pass && wantConstraint != "" {
		p.Observed += fmt.Sprintf(", constraint %s (%s)", wantConstraint, constraintSource(wantConstraint, p))
	}
	if !p.Pass && capturedErr != nil && p.SQLState == wantState && wantConstraint != "" {
		p.Observed += fmt.Sprintf(", but constraint %q not named anywhere in the error", wantConstraint)
	}
	if p.RawError != "" {
		p.Detail = "raw driver error:\n\n    " + strings.ReplaceAll(p.RawError, " / ", "\n    ")
	}
	r.Record(p)
}

// ExpectOK runs stmt in its own committed transaction and asserts it succeeds.
func (r *Runner) ExpectOK(ctx context.Context, id, criterion, stmt string) {
	var capturedErr error
	elapsed, _ := timed(func() error {
		_, capturedErr = r.db.ExecContext(ctx, stmt)
		return nil
	})

	p := Probe{
		ID:         id,
		Criterion:  criterion,
		Statement:  collapse(stmt),
		Expected:   "statement succeeds",
		SQLState:   sqlStateOf(capturedErr),
		Constraint: constraintOf(capturedErr),
		RawError:   rawOf(capturedErr),
		Elapsed:    elapsed,
		Pass:       capturedErr == nil,
	}
	if capturedErr == nil {
		p.Observed = "succeeded"
	} else {
		p.Observed = fmt.Sprintf("FAILED with SQLSTATE %s", p.SQLState)
		p.Detail = "raw driver error:\n\n    " + strings.ReplaceAll(p.RawError, " / ", "\n    ")
	}
	r.Record(p)
}

// ExpectValue runs a single-value query and compares the result to want.
func (r *Runner) ExpectValue(ctx context.Context, id, criterion, want, query string) string {
	var got string
	var capturedErr error
	elapsed, _ := timed(func() error {
		capturedErr = r.db.QueryRowContext(ctx, query).Scan(&got)
		return nil
	})

	p := Probe{
		ID:        id,
		Criterion: criterion,
		Statement: collapse(query),
		Expected:  want,
		Observed:  got,
		SQLState:  sqlStateOf(capturedErr),
		RawError:  rawOf(capturedErr),
		Elapsed:   elapsed,
		Pass:      capturedErr == nil && got == want,
	}
	if capturedErr != nil {
		p.Observed = "query failed"
		p.Detail = "raw driver error:\n\n    " + strings.ReplaceAll(p.RawError, " / ", "\n    ")
	}
	r.Record(p)
	return got
}

// RecordInfo captures a value that is a receipt rather than an assertion — it passes
// as long as the query ran and matched the supplied predicate.
func (r *Runner) RecordInfo(ctx context.Context, id, criterion, expected, query string, ok func(string) bool) string {
	var got string
	var capturedErr error
	elapsed, _ := timed(func() error {
		capturedErr = r.db.QueryRowContext(ctx, query).Scan(&got)
		return nil
	})

	p := Probe{
		ID:        id,
		Criterion: criterion,
		Statement: collapse(query),
		Expected:  expected,
		Observed:  collapse(got),
		SQLState:  sqlStateOf(capturedErr),
		RawError:  rawOf(capturedErr),
		Elapsed:   elapsed,
		Pass:      capturedErr == nil && ok(got),
	}
	if capturedErr != nil {
		p.Observed = "query failed"
		p.Detail = "raw driver error:\n\n    " + strings.ReplaceAll(p.RawError, " / ", "\n    ")
	}
	r.Record(p)
	return got
}

func expectationText(wantState, wantConstraint string) string {
	if wantConstraint == "" {
		return "refused, SQLSTATE " + wantState
	}
	return fmt.Sprintf("refused, SQLSTATE %s, constraint %s", wantState, wantConstraint)
}

func constraintSatisfied(want string, p Probe) bool {
	if want == "" {
		return true
	}
	return p.Constraint == want || strings.Contains(p.RawError, want)
}

// constraintSource records *where* the constraint name was found. If CockroachDB
// leaves the structured field empty and only names the constraint in the message,
// that is itself a finding worth carrying into the transcript.
func constraintSource(want string, p Probe) string {
	if p.Constraint == want {
		return "structured field"
	}
	return "message text only; ConstraintName field empty"
}
