// Package intent provides action intent wiring for the Transactional Belief Ledger.
//
// It wraps kernel.IntentOnPromoted and kernel.AuditLiveOnNonPromoted as thin
// translation functions. The kernel owns correctness.
package intent

import (
	"context"
	"database/sql"

	"github.com/PithomLabs/solvent/internal/kernel"
)

// Propose records a live action intent on a promoted belief.
// Returns ErrActionOnUnpromoted if the belief is not currently promoted.
func Propose(ctx context.Context, db *sql.DB, scenarioID, beliefID, action string) error {
	st := kernel.New(db)
	return st.IntentOnPromoted(ctx, scenarioID, beliefID, action)
}

// Audit returns the count of live intents whose belief is not promoted.
// Must always return 0 in committed state (invariant I-5).
func Audit(ctx context.Context, db *sql.DB, scenarioID string) (int, error) {
	st := kernel.New(db)
	return st.AuditLiveOnNonPromoted(ctx, scenarioID)
}
