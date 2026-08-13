package kernel_test

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
	_ "github.com/jackc/pgx/v5/stdlib" // D-019: pgx/v5 is the sole approved driver

	"github.com/PithomLabs/solvent/internal/testdb"
)

const (
	schemaPath     = "../db/001_schema.sql"
	transcriptPath = "../docs/M2_TRANSCRIPT.md"
	failurePath    = "../docs/M2_FAILURE.md"
)

var (
	dsn    string
	shared *sql.DB
	rec    = &recorder{}
)

// TestMain resets the behavioral database, runs the suite, and writes the receipts.
func TestMain(m *testing.M) {
	ctx := context.Background()
	dsn = testdb.DSN()

	name, _ := testdb.DBNameFromDSN(dsn)
	testdb.AcquireResetLock(name)

	// The guard lives in testdb.Reset: it refuses any database not named *_test, and
	// prints the redacted DSN before dropping anything (M2-R1, N1).
	if err := testdb.Reset(ctx, dsn, schemaPath); err != nil {
		fmt.Fprintf(os.Stderr, "M2 cannot start: %v\n", err)
		testdb.ReleaseResetLock(name)
		os.Exit(1)
	}

	var err error
	shared, err = testdb.Open(dsn)
	if err != nil {
		fmt.Fprintf(os.Stderr, "M2 cannot start: open pool: %v\n", err)
		testdb.ReleaseResetLock(name)
		os.Exit(1)
	}
	if err := shared.PingContext(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "M2 cannot start: ping: %v\n", err)
		testdb.ReleaseResetLock(name)
		os.Exit(1)
	}

	code := m.Run()

	_ = shared.Close()

	if err := rec.writeTranscript(transcriptPath); err != nil {
		fmt.Fprintf(os.Stderr, "could not write transcript: %v\n", err)
		code = 1
	}
	if rec.failed() > 0 {
		if err := rec.writeFailureArtifact(failurePath); err != nil {
			fmt.Fprintf(os.Stderr, "could not write failure artifact: %v\n", err)
		}
		rec.printFailureArtifact(os.Stderr)
		code = 1
	} else {
		// A green run must not leave a stale failure artifact behind.
		_ = os.Remove(failurePath)
	}

	testdb.ReleaseResetLock(name)
	os.Exit(code)
}

// --- receipts -----------------------------------------------------------------

// Case is one behavioral receipt. Invariant is declared rather than inferred, so the
// failure artifact can name it mechanically (D-049).
type Case struct {
	ID        string
	Wave      string
	Purpose   string
	Expected  string
	Observed  string
	SQLState  string
	Constrain string
	Invariant string
	Receipt   string // raw driver error, or the row-state dump when nothing was refused
	Elapsed   time.Duration
	Pass      bool
}

type recorder struct {
	cases    []Case
	lastWave string
	start    time.Time
}

// begin starts timing a case and prints a wave banner when the wave changes (N1).
func (r *recorder) begin(wave string) {
	if wave != r.lastWave {
		fmt.Printf("\n=== Wave %s ===\n", wave)
		r.lastWave = wave
	}
	r.start = time.Now()
}

func (r *recorder) elapsed() time.Duration { return time.Since(r.start) }

func (r *recorder) add(c Case) {
	c.Elapsed = r.elapsed()
	r.cases = append(r.cases, c)
}

func (r *recorder) failed() int {
	n := 0
	for _, c := range r.cases {
		if !c.Pass {
			n++
		}
	}
	return n
}

// pass records a green case.
func (r *recorder) pass(t *testing.T, c Case) {
	t.Helper()
	c.Pass = true
	r.add(c)
	t.Logf("%s PASS — %s", c.ID, c.Observed)
}

// fail records a red case and fails the test. The transcript and the failure artifact
// are both produced from this record.
func (r *recorder) fail(t *testing.T, c Case) {
	t.Helper()
	c.Pass = false
	r.add(c)
	t.Errorf("%s FAIL\n  expected: %s\n  observed: %s\n  receipt:  %s\n  invariant: %s\n  decision: STOP",
		c.ID, c.Expected, c.Observed, c.Receipt, c.Invariant)
}

// check records pass or fail from a single boolean, so every case reports the same way.
func (r *recorder) check(t *testing.T, ok bool, c Case) {
	t.Helper()
	if ok {
		r.pass(t, c)
		return
	}
	r.fail(t, c)
}

// --- artifacts ----------------------------------------------------------------

func (r *recorder) writeTranscript(path string) error {
	var b strings.Builder

	b.WriteString("# M2 Behavioral Transcript\n\n")
	b.WriteString("Milestone M2 — the invariant suite, executed against a live CockroachDB cluster.\n")
	b.WriteString("Every row is a receipt: SQLSTATE and constraint name come from `*pgconn.PgError`,\n")
	b.WriteString("never from substring matching.\n\n")
	b.WriteString("M2 executes **single-session** behavior only. Concurrency is M3 and is not evidenced here.\n\n")

	b.WriteString("<!-- volatile:begin -->\n\n")
	host, _ := os.Hostname()
	b.WriteString("| run fact | value |\n|---|---|\n")
	b.WriteString("| generated_at | " + time.Now().UTC().Format(time.RFC3339) + " |\n")
	b.WriteString("| host | " + host + " |\n")
	b.WriteString("| dsn | " + testdb.Redact(dsn) + " |\n\n")
	b.WriteString("<!-- volatile:end -->\n\n")

	failed := r.failed()
	b.WriteString("## Verdict\n\n")
	if failed == 0 {
		b.WriteString(fmt.Sprintf("**GREEN** — %d/%d cases passed.\n\n", len(r.cases), len(r.cases)))
	} else {
		b.WriteString(fmt.Sprintf("**BLOCKED** — %d of %d cases failed. See `docs/M2_FAILURE.md`.\n\n",
			failed, len(r.cases)))
	}

	b.WriteString("## Cases\n\n")
	b.WriteString("| id | wave | status | purpose | expected | observed | sqlstate | constraint | invariant | elapsed_ms |\n")
	b.WriteString("|---|---|---|---|---|---|---|---|---|---|\n")
	for _, c := range r.cases {
		status := "PASS"
		if !c.Pass {
			status = "**FAIL**"
		}
		b.WriteString(fmt.Sprintf("| %s | %s | %s | %s | %s | %s | %s | %s | %s | %d |\n",
			c.ID, c.Wave, status, cell(c.Purpose), cell(c.Expected), cell(c.Observed),
			dash(c.SQLState), dash(c.Constrain), dash(c.Invariant), c.Elapsed.Milliseconds()))
	}
	b.WriteString("\n")

	b.WriteString("## Receipts\n\n")
	any := false
	for _, c := range r.cases {
		if c.Receipt == "" {
			continue
		}
		any = true
		b.WriteString("### " + c.ID + "\n\n```\n" + c.Receipt + "\n```\n\n")
	}
	if !any {
		b.WriteString("_No case produced a raw receipt._\n\n")
	}

	return os.WriteFile(path, []byte(b.String()), 0o644)
}

// writeFailureArtifact emits the standardized architectural failure report (D-049).
func (r *recorder) writeFailureArtifact(path string) error {
	var b strings.Builder
	b.WriteString("# M2 Failure Artifact\n\n")
	b.WriteString("A behavioral divergence from `IMPLEMENTATION_CONTRACT.md`, the M0 findings, or the\n")
	b.WriteString("M1 findings is a reportable architectural event, not a test to be adjusted.\n\n")
	b.WriteString("**Decision is STOP in every case below.** Do not redesign. Do not weaken the\n")
	b.WriteString("invariant. Do not adapt the test to the observed behavior.\n\n")
	r.renderFailures(&b)
	return os.WriteFile(path, []byte(b.String()), 0o644)
}

func (r *recorder) printFailureArtifact(w *os.File) {
	var b strings.Builder
	b.WriteString("\n================ M2 FAILURE ARTIFACT ================\n")
	r.renderFailures(&b)
	fmt.Fprint(w, b.String())
}

func (r *recorder) renderFailures(b *strings.Builder) {
	for _, c := range r.cases {
		if c.Pass {
			continue
		}
		receipt := c.Receipt
		if receipt == "" {
			receipt = "(none — nothing was refused; see Observed for the row state that proves it)"
		}
		fmt.Fprintf(b, "### FAILURE — %s  (wave %s)\n\n", c.ID, c.Wave)
		fmt.Fprintf(b, "```\n")
		fmt.Fprintf(b, "Expected:          %s\n", c.Expected)
		fmt.Fprintf(b, "Observed:          %s\n", c.Observed)
		fmt.Fprintf(b, "Receipt:           %s\n", receipt)
		fmt.Fprintf(b, "Likely invariant:  %s\n", dash(c.Invariant))
		fmt.Fprintf(b, "Decision:          STOP\n")
		fmt.Fprintf(b, "Escalation:        Architectural review. Do not redesign, do not weaken the\n")
		fmt.Fprintf(b, "                   invariant, do not adapt the test to the observed behavior.\n")
		fmt.Fprintf(b, "```\n\n")
	}
}

// --- shared helpers -----------------------------------------------------------

// The test package imports pgconn directly to read ConstraintName, which is a field
// rather than a method and so is unreachable through an interface. The kernel itself
// stays driver-agnostic; only these tests look at the concrete type.
func pgErrOf(err error) *pgconn.PgError {
	var e *pgconn.PgError
	if errors.As(err, &e) {
		return e
	}
	return nil
}

func sqlStateOf(err error) string {
	if e := pgErrOf(err); e != nil {
		return e.Code
	}
	return ""
}

func constraintOf(err error) string {
	if e := pgErrOf(err); e != nil {
		return e.ConstraintName
	}
	return ""
}

func cell(s string) string {
	s = strings.ReplaceAll(s, "|", "\\|")
	return strings.TrimSpace(strings.ReplaceAll(s, "\n", " "))
}

func dash(s string) string {
	if s == "" {
		return "—"
	}
	return s
}

func collapse(s string) string {
	s = strings.ReplaceAll(s, "\r\n", "\n")
	s = strings.ReplaceAll(s, "\n", " / ")
	return strings.TrimSpace(s)
}
