// Package kernel is the transactional belief ledger.
//
// The invariants are the schema's, not this package's: db/001_schema.sql enforces
// I-1 through I-5 with CHECK constraints and a composite foreign key. This package's
// job is narrower and mechanical — route every write through crdb.ExecuteTx so
// serialization failures (40001) retry correctly, keep the cascade in ONE
// transaction with cancel strictly before retract (I-8), and name the database's
// refusals without ever substituting its own judgement for them.
//
// The package imports no driver. Errors are classified through
// interface{ SQLState() string }, which *pgconn.PgError satisfies; the driver is
// selected at the cmd/ edge.
//
// # Prerequisite
//
// New wraps an *sql.DB that is already pointed at a database with db/001_schema.sql
// applied. The kernel does not migrate, ping, or validate the schema — if the tables
// are absent, the first call fails with the driver's error rather than a friendlier
// one, because guessing at the caller's deployment is not this package's job.
//
// # Lifecycle
//
// A claim enters free, accumulates evidence, discharges debt, earns promotion, and
// only then may an action cite it. New evidence can reverse all of it:
//
//	evidence → belief → debt → promotion → intent → falsification → retraction → cancellation
//
// See the package example for that sequence executed end to end.
//
// # The two refusals
//
// Callers must handle two sentinels. Both originate in the schema; this package only
// names them, and the driver error is preserved underneath each with %w so that
// errors.As can still reach *pgconn.PgError for the SQLSTATE and constraint name.
//
//	ErrPromotionBlocked   — SQLSTATE 23514, CHECK promoted_is_debt_free.
//	                        Promote was called on a belief that still carries debt
//	                        or final-truth language.
//
//	ErrActionOnUnpromoted — SQLSTATE 23503, composite FK "gate".
//	                        IntentOnPromoted was called on a belief that is not
//	                        currently promoted. The row is not rejected by a
//	                        validation rule; it is unrepresentable.
//
// Neither is an error condition to be worked around. They are the product: the
// database, not the caller, decides whether an action has earned its authority.
//
// # Scenarios
//
// Every operation is scoped by a scenario_id the caller supplies. Scenarios are an
// isolation convention enforced by query predicate, not by a foreign key — there is
// no scenario table. RetractCascade will not traverse out of its scenario (D-032).
package kernel
