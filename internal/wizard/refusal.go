package wizard

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5/pgconn"
)

// Verdict is what the database said, on its way to the screen.
//
// A refusal and a commit are the same shape deliberately: the UI renders both through
// one primitive, and the only difference the judge sees is which colour the engine's
// own words are printed in.
type Verdict struct {
	OK         bool   `json:"ok"`
	Statement  string `json:"statement"`
	SQLState   string `json:"sqlstate,omitempty"`
	Constraint string `json:"constraint,omitempty"`
	Detail     string `json:"detail,omitempty"`
}

// The statements refusal_log's CHECK permits.
const (
	StmtPromote       = "promote"
	StmtAuthorize     = "authorize"
	StmtDischarge     = "discharge"
	StmtRetractUnsafe = "retract_unsafe"
)

// refuse turns a driver error into a Verdict and records it.
//
// # Why the log is written in a separate statement
//
// A gate refusal aborts its transaction, so anything written inside that transaction
// rolls back with it — including a row saying it happened. The refusal therefore has
// to be recorded after the fact, from the caught error, on its own. That ordering is
// not an implementation detail: without it screen 1 has no exit condition, because the
// only trace of the refusal would be a message already sent to a browser that can be
// reloaded away.
//
// # Why the constraint name is read from the struct field
//
// pgx does not put it in Error(). errors.As to *pgconn.PgError and read .ConstraintName
// or it is simply absent, which is why the probe transcripts print "CONSTRAINT NAME:"
// on a line of their own. A verdict without the constraint name is a verdict that has
// lost the interesting half.
func (s *Server) refuse(ctx context.Context, scenarioID, statement string, err error, detail string) Verdict {
	v := Verdict{Statement: statement, Detail: detail}

	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		v.SQLState = pgErr.Code
		v.Constraint = pgErr.ConstraintName
		if v.Detail == "" {
			v.Detail = pgErr.Message
		}
	} else if err != nil {
		// Not a database refusal: a bug, a context cancellation, a dropped connection.
		// Recorded with no SQLSTATE so the two are never conflated on screen.
		v.Detail = err.Error()
	}

	if logErr := s.logRefusal(ctx, scenarioID, statement, v); logErr != nil {
		// The refusal itself is the news; failing to file it must not replace it.
		v.Detail = fmt.Sprintf("%s (refusal_log write failed: %v)", v.Detail, logErr)
	}
	return v
}

func (s *Server) logRefusal(ctx context.Context, scenarioID, statement string, v Verdict) error {
	sqlstate := v.SQLState
	if sqlstate == "" {
		sqlstate = "00000" // successful-completion: "no SQLSTATE was reported", not a refusal code
	}
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO refusal_log (scenario_id, statement, sqlstate, constraint_name, detail)
		VALUES ($1::UUID, $2::STRING, $3::STRING, $4::STRING, $5::STRING)`,
		scenarioID, statement, sqlstate, v.Constraint, v.Detail)
	if err != nil {
		return fmt.Errorf("wizard: log refusal: %w", err)
	}
	return nil
}

func (s *Server) refusals(ctx context.Context, scenarioID string) ([]Refusal, error) {
	// Scoped to the scenario. refusal_log is append-only across every judge who has
	// ever opened the demo; without this predicate the second visitor would arrive on
	// screen 2 having refused nothing.
	rows, err := s.db.QueryContext(ctx, `
		SELECT statement, sqlstate, coalesce(constraint_name, ''), coalesce(detail, ''),
		       logged_at::STRING
		  FROM refusal_log
		 WHERE scenario_id = $1::UUID
		 ORDER BY logged_at`, scenarioID)
	if err != nil {
		return nil, fmt.Errorf("wizard: read refusals: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var out []Refusal
	for rows.Next() {
		var r Refusal
		if err := rows.Scan(&r.Statement, &r.SQLState, &r.Constraint, &r.Detail, &r.LoggedAt); err != nil {
			return nil, fmt.Errorf("wizard: scan refusal: %w", err)
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// Promote attempts promotion and reports what the database decided.
//
// There is no debt check here, and adding one would end the demo's argument. The
// six outstanding items are exactly why the schema refuses, and the refusal is the
// product. Screens 1 and 2 call this same method with no screen parameter, so
// "the same SQL on both screens" is a property of there being one call site rather
// than a promise to keep two in step.
func (s *Server) Promote(ctx context.Context, scenarioID, beliefID string) Verdict {
	if err := s.kern.Promote(ctx, beliefID); err != nil {
		return s.refuse(ctx, scenarioID, StmtPromote, err, "")
	}
	return Verdict{OK: true, Statement: StmtPromote, Detail: "COMMIT"}
}

// Authorize attempts to record intent on the belief, and reports what came back.
//
// Same discipline as Promote: no precondition, one call site, the composite gate FK
// does the refusing.
func (s *Server) Authorize(ctx context.Context, scenarioID, beliefID string) Verdict {
	if err := s.kern.IntentOnPromoted(ctx, scenarioID, beliefID, DeployAction); err != nil {
		return s.refuse(ctx, scenarioID, StmtAuthorize, err, "")
	}
	return Verdict{OK: true, Statement: StmtAuthorize, Detail: "COMMIT"}
}
