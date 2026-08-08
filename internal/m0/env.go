// Verification-only. This package is M0's executable specification, not production
// infrastructure. It must never be imported by internal/kernel (D-023) and is frozen
// at the close of M0 (O1).
package m0

import (
	"context"
	"fmt"
	"strings"
	"time"
)

// RunEnvironment executes the A-series: the receipts that say which CockroachDB this
// was, and that READ COMMITTED is genuinely available rather than silently upgraded.
func RunEnvironment(ctx context.Context, r *Runner) {
	r.RecordInfo(ctx,
		"A1",
		"CockroachDB version recorded (contract §6 M0: confirm cluster version)",
		"CockroachDB v26.x",
		"SELECT version()",
		func(v string) bool { return strings.HasPrefix(v, "CockroachDB") },
	)

	r.ExpectValue(ctx,
		"A2",
		"READ COMMITTED is available on this cluster (contract §7 M0)",
		"true",
		"SHOW CLUSTER SETTING sql.txn.read_committed_isolation.enabled",
	)

	// A3 is the assertion that matters. A2 reads a setting; A3 proves a transaction
	// actually runs at READ COMMITTED and is not upgraded to SERIALIZABLE underneath
	// us — which is precisely what M3's cell 1 depends on.
	runA3(ctx, r)

	r.RecordInfo(ctx,
		"A4",
		"Default isolation level recorded",
		"serializable",
		"SHOW default_transaction_isolation",
		func(v string) bool { return v != "" },
	)
}

func runA3(ctx context.Context, r *Runner) {
	const stmt = "BEGIN TRANSACTION ISOLATION LEVEL READ COMMITTED; SHOW transaction_isolation"

	var got string
	var capturedErr error
	start := time.Now()

	tx, err := r.DB().BeginTx(ctx, nil)
	if err != nil {
		capturedErr = err
	} else {
		if _, e := tx.ExecContext(ctx, "SET TRANSACTION ISOLATION LEVEL READ COMMITTED"); e != nil {
			capturedErr = e
		} else {
			capturedErr = tx.QueryRowContext(ctx, "SHOW transaction_isolation").Scan(&got)
		}
		_ = tx.Rollback()
	}
	elapsed := time.Since(start)

	p := Probe{
		ID:        "A3",
		Criterion: "A transaction opened as READ COMMITTED reports READ COMMITTED (not silently upgraded)",
		Statement: stmt,
		Expected:  "read committed",
		Observed:  got,
		SQLState:  sqlStateOf(capturedErr),
		RawError:  rawOf(capturedErr),
		Elapsed:   elapsed,
		Pass:      capturedErr == nil && got == "read committed",
	}
	if capturedErr != nil {
		p.Observed = "failed to open or read the transaction"
		p.Detail = "raw driver error:\n\n    " + strings.ReplaceAll(p.RawError, " / ", "\n    ")
	} else if !p.Pass {
		p.Observed = fmt.Sprintf("%q — the transaction was upgraded", got)
	}
	r.Record(p)
}
