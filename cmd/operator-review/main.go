// Command operator-review retires debts on a belief and promotes it.
//
// This is the explicit demo action for debt retirement after the pipeline
// has run and promotion was refused. It is NOT a pipeline step — it is an
// operator-initiated action that calls kernel functions directly.
//
// Usage:
//
//	operator-review --dsn "$FABLE_DSN" --scenario <uuid> --belief <uuid> \
//	  --debt needInvariant --debt needToyCheck --debt needNullModel \
//	  --debt needObstruction --debt needFaithfulnessReview
//
// With intent creation:
//
//	operator-review --dsn "$FABLE_DSN" --scenario <uuid> --belief <uuid> \
//	  --action "deploy etcd v3.5.28" \
//	  --debt needInvariant --debt needToyCheck --debt needNullModel \
//	  --debt needObstruction --debt needFaithfulnessReview
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/PithomLabs/solvent/internal/kernel"
	"github.com/PithomLabs/solvent/internal/testdb"

	_ "github.com/jackc/pgx/v5/stdlib"
)

type stringSlice []string

func (s *stringSlice) String() string { return strings.Join(*s, ",") }
func (s *stringSlice) Set(v string) error {
	*s = append(*s, v)
	return nil
}

func main() {
	dsn := flag.String("dsn", "", "CockroachDB connection string (env: FABLE_DSN)")
	scenarioID := flag.String("scenario", "", "scenario UUID (required)")
	beliefID := flag.String("belief", "", "belief UUID to promote (required)")
	action := flag.String("action", "", "action string for intent creation (optional; omit to skip intent)")
	var debts stringSlice
	flag.Var(&debts, "debt", "debt items to retire (repeatable)")
	flag.Parse()

	if *dsn == "" {
		*dsn = os.Getenv("FABLE_DSN")
	}
	if *dsn == "" {
		fail("no DSN: pass --dsn or set FABLE_DSN")
	}
	if *scenarioID == "" {
		fail("--scenario is required")
	}
	if *beliefID == "" {
		fail("--belief is required")
	}
	if len(debts) == 0 {
		fail("--debt is required (repeat for each debt item)")
	}

	ctx := context.Background()

	db, err := testdb.Open(*dsn)
	if err != nil {
		fail(fmt.Sprintf("open: %v", err))
	}
	defer func() { _ = db.Close() }()
	if err := db.PingContext(ctx); err != nil {
		fail(fmt.Sprintf("ping: %v", err))
	}

	st := kernel.New(db)

	for _, item := range debts {
		if err := st.RetireDebt(ctx, *beliefID, item); err != nil {
			fail(fmt.Sprintf("retire %s: %v", item, err))
		}
		fmt.Printf("retired: %s\n", item)
	}

	if err := st.Promote(ctx, *beliefID); err != nil {
		fail(fmt.Sprintf("promote: %v", err))
	}
	fmt.Println("promoted: ok")

	if *action != "" {
		if err := st.IntentOnPromoted(ctx, *scenarioID, *beliefID, *action); err != nil {
			fail(fmt.Sprintf("intent: %v", err))
		}
		fmt.Printf("intent: live (action=%q)\n", *action)
	}

	auditCount, err := st.AuditLiveOnNonPromoted(ctx, *scenarioID)
	if err != nil {
		fail(fmt.Sprintf("audit: %v", err))
	}
	fmt.Printf("audit_live_on_nonpromoted: %d\n", auditCount)

	if auditCount != 0 {
		fail("VERDICT: FAIL (audit != 0)")
	}
	fmt.Println("VERDICT: PASS")
}

func fail(msg string) {
	fmt.Fprintln(os.Stderr, msg)
	os.Exit(1)
}
