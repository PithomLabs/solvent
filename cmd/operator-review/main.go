// Command operator-review has two modes:
//
// 1. Enter a new belief (Track 2 baseline entry):
//
//	operator-review --dsn "$FABLE_DSN" --scenario <uuid> \
//	  --enter-claim "etcd v3.5.0 is approved for production deployment" \
//	  --claim-type postulated \
//	  --evidence-url "https://github.com/..." \
//	  --evidence-sha <64-hex-sha>
//
// 2. Review an existing belief (debt retirement, promotion, intent):
//
//	operator-review --dsn "$FABLE_DSN" --scenario <uuid> --belief <uuid> \
//	  --debt needInvariant --debt needToyCheck --debt needNullModel \
//	  --debt needObstruction --debt needFaithfulnessReview \
//	  --action "deploy etcd v3.5.28"
package main

import (
	"context"
	"database/sql"
	"flag"
	"fmt"
	"os"
	"regexp"
	"strings"

	"github.com/PithomLabs/solvent/internal/testdb"
	"github.com/PithomLabs/solvent/kernel"

	_ "github.com/jackc/pgx/v5/stdlib"
)

type stringSlice []string

func (s *stringSlice) String() string { return strings.Join(*s, ",") }
func (s *stringSlice) Set(v string) error {
	*s = append(*s, v)
	return nil
}

var hex64 = regexp.MustCompile(`^[0-9a-f]{64}$`)

func main() {
	dsn := flag.String("dsn", "", "CockroachDB connection string (env: FABLE_DSN)")
	scenarioID := flag.String("scenario", "", "scenario UUID (required)")
	beliefID := flag.String("belief", "", "belief UUID to review (mutually exclusive with --enter-claim)")
	action := flag.String("action", "", "action string for intent creation (optional; omit to skip intent)")

	// Entry mode flags (Track 2 baseline).
	enterClaim := flag.String("enter-claim", "", "claim text for entering a new belief (requires --claim-type)")
	claimType := flag.String("claim-type", "", "claim type: postulated (required with --enter-claim)")
	evidenceURL := flag.String("evidence-url", "", "evidence source URL (requires --evidence-sha)")
	evidenceSHA := flag.String("evidence-sha", "", "evidence content SHA256, 64 hex chars (requires --evidence-url)")

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

	// Validate mode: exactly one of --belief or --enter-claim.
	hasBelief := *beliefID != ""
	hasEnter := *enterClaim != ""
	if hasBelief == hasEnter {
		fail("exactly one of --belief or --enter-claim is required")
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

	if hasEnter {
		// Mode 1: Enter a new belief.
		runEnterMode(ctx, st, db, *scenarioID, *enterClaim, *claimType, *evidenceURL, *evidenceSHA)
	} else {
		// Mode 2: Review an existing belief.
		runReviewMode(ctx, st, db, *scenarioID, *beliefID, *action, debts)
	}
}

// runEnterMode enters a new belief and optionally attaches evidence.
func runEnterMode(ctx context.Context, st *kernel.Store, db *sql.DB, scenarioID, claim, claimType, evidenceURL, evidenceSHA string) {
	if claimType == "" {
		fail("--claim-type is required with --enter-claim")
	}
	if claimType != "postulated" {
		fail("--claim-type must be 'postulated' for the entry path")
	}

	beliefID, err := st.EnterBelief(ctx, scenarioID, claim, kernel.Postulated)
	if err != nil {
		fail(fmt.Sprintf("enter belief: %v", err))
	}
	fmt.Printf("BELIEF_ID=%s\n", beliefID)

	if evidenceURL != "" || evidenceSHA != "" {
		// Both must be provided together.
		if evidenceURL == "" {
			fail("--evidence-url is required with --evidence-sha")
		}
		if evidenceSHA == "" {
			fail("--evidence-sha is required with --evidence-url")
		}
		if !hex64.MatchString(evidenceSHA) {
			fail("--evidence-sha must be exactly 64 hex characters")
		}
		if err := st.AddEvidence(ctx, scenarioID, beliefID, "external_feed", evidenceURL, evidenceSHA); err != nil {
			fail(fmt.Sprintf("add evidence: %v", err))
		}
		fmt.Println("evidence: attached")
	}
}

// runReviewMode retires debts, promotes, and optionally creates an intent.
// It verifies the belief belongs to the given scenario before acting (F-8).
func runReviewMode(ctx context.Context, st *kernel.Store, db *sql.DB, scenarioID, beliefID, action string, debts stringSlice) {
	// Scenario/belief consistency pre-check (F-8).
	var beliefScenario string
	err := db.QueryRowContext(ctx,
		`SELECT scenario_id::STRING FROM belief WHERE id = $1`, beliefID,
	).Scan(&beliefScenario)
	if err == sql.ErrNoRows {
		fail(fmt.Sprintf("belief %s not found", beliefID))
	}
	if err != nil {
		fail(fmt.Sprintf("lookup belief: %v", err))
	}
	if beliefScenario != scenarioID {
		fail(fmt.Sprintf("scenario mismatch: belief %s belongs to scenario %s, not %s", beliefID, beliefScenario, scenarioID))
	}

	if len(debts) == 0 {
		fail("--debt is required (repeat for each debt item)")
	}

	for _, item := range debts {
		if err := st.RetireDebt(ctx, beliefID, item); err != nil {
			fail(fmt.Sprintf("retire %s: %v", item, err))
		}
		fmt.Printf("retired: %s\n", item)
	}

	if err := st.Promote(ctx, beliefID); err != nil {
		fail(fmt.Sprintf("promote: %v", err))
	}
	fmt.Println("promoted: ok")

	if action != "" {
		if err := st.IntentOnPromoted(ctx, scenarioID, beliefID, action); err != nil {
			fail(fmt.Sprintf("intent: %v", err))
		}
		fmt.Printf("intent: live (action=%q)\n", action)
	}

	auditCount, err := st.AuditLiveOnNonPromoted(ctx, scenarioID)
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
