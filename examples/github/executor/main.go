// Package executor demonstrates using the GitHub workflow executor adapter
// through the Solvent authorization kernel.
//
// This example shows the full flow: create principal → enter belief → promote →
// create authority target → approve → execute via GitHub Actions.
//
// Usage:
//
//	GITHUB_TOKEN=ghp_xxx FABLE_DSN=... go run . --repo org/repo --workflow deploy.yml --ref main
package executor

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/PithomLabs/solvent/adapter/github"
	"github.com/PithomLabs/solvent/internal/testdb"
	"github.com/PithomLabs/solvent/kernel"
	"github.com/PithomLabs/solvent/service/audit"
	"github.com/PithomLabs/solvent/service/authority"
	"github.com/PithomLabs/solvent/service/executor"
	"github.com/PithomLabs/solvent/service/policy"
	_ "github.com/jackc/pgx/v5/stdlib"
)

func main() {
	repo := flag.String("repo", "", "GitHub repository (owner/repo)")
	workflow := flag.String("workflow", "", "Workflow file name")
	ref := flag.String("ref", "main", "Git ref")
	flag.Parse()

	if *repo == "" || *workflow == "" {
		fmt.Fprintln(os.Stderr, "Usage: go run . --repo org/repo --workflow deploy.yml [--ref main]")
		os.Exit(1)
	}

	token := os.Getenv("GITHUB_TOKEN")
	if token == "" {
		fmt.Fprintln(os.Stderr, "GITHUB_TOKEN environment variable is required")
		os.Exit(1)
	}

	dsn := os.Getenv("FABLE_DSN")
	if dsn == "" {
		fmt.Fprintln(os.Stderr, "FABLE_DSN environment variable is required")
		os.Exit(1)
	}

	ctx := context.Background()
	db, err := testdb.Open(dsn)
	if err != nil {
		fmt.Fprintf(os.Stderr, "open database: %v\n", err)
		os.Exit(1)
	}
	defer db.Close()

	// Wire up services.
	pol := policy.New(db)
	aud := audit.New(db)
	reg := executor.NewRegistry()

	// Register the real GitHub executor.
	provider := github.NewHTTPProvider(token)
	github.RegisterExecutor(reg, provider)

	svc := authority.New(db, pol, aud, reg)
	st := kernel.New(db)

	// Create a scenario and principal.
	sid := "00000000-0000-0000-0000-000000000001"
	principalID, err := st.CreatePrincipal(ctx, "agent", "github-example")
	if err != nil {
		fmt.Fprintf(os.Stderr, "create principal: %v\n", err)
		os.Exit(1)
	}

	// Create a promoted belief.
	beliefID, err := st.EnterBelief(ctx, sid, "etcd v3.5.x is safe to deploy", kernel.Derived)
	if err != nil {
		fmt.Fprintf(os.Stderr, "enter belief: %v\n", err)
		os.Exit(1)
	}
	for _, item := range kernel.FullDebt {
		_ = st.RetireDebt(ctx, beliefID, item)
	}
	if err := st.Promote(ctx, beliefID); err != nil {
		fmt.Fprintf(os.Stderr, "promote belief: %v\n", err)
		os.Exit(1)
	}

	// Create consequence parameters from the CLI flags.
	params, _ := json.Marshal(map[string]string{
		"repo":     *repo,
		"workflow": *workflow,
		"ref":      *ref,
	})

	// Create and approve authority target.
	targetID, err := st.CreateTarget(ctx, principalID,
		"scenario", sid, "belief:"+beliefID,
		"solvent", "deploy", "execution", params, principalID)
	if err != nil {
		fmt.Fprintf(os.Stderr, "create target: %v\n", err)
		os.Exit(1)
	}
	if err := st.AttachJustification(ctx, targetID, beliefID, "promoted", principalID); err != nil {
		fmt.Fprintf(os.Stderr, "attach justification: %v\n", err)
		os.Exit(1)
	}
	if err := st.RequestAuthorization(ctx, targetID, principalID); err != nil {
		fmt.Fprintf(os.Stderr, "request authorization: %v\n", err)
		os.Exit(1)
	}
	if err := st.Approve(ctx, targetID, principalID); err != nil {
		fmt.Fprintf(os.Stderr, "approve: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Authority target approved: %s\n", targetID)

	// Create a live intent.
	var intentID string
	err = db.QueryRowContext(ctx, `
		INSERT INTO action_intent (scenario_id, belief_id, action)
		VALUES ($1::UUID, $2::UUID, 'deploy')
		RETURNING id`, sid, beliefID).Scan(&intentID)
	if err != nil {
		fmt.Fprintf(os.Stderr, "create intent: %v\n", err)
		os.Exit(1)
	}

	// Execute — this triggers the real GitHub workflow dispatch.
	result, err := svc.ExecuteAction(ctx, sid, beliefID, "deploy", targetID, principalID,
		intentID, map[string]interface{}{}, "execution", params)
	if err != nil {
		fmt.Fprintf(os.Stderr, "execute action: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Allowed: %v\n", result.Allowed)
	fmt.Printf("Success: %v\n", result.Success)
	if result.Output != "" {
		fmt.Printf("Output: %s\n", result.Output)
	}
	if result.Error != "" {
		fmt.Printf("Error: %s\n", result.Error)
	}
	fmt.Printf("Executed at: %s\n", result.ExecutedAt.Format(time.RFC3339))

	// Check intent state.
	var state string
	err = db.QueryRowContext(ctx, `SELECT state FROM action_intent WHERE id = $1::UUID`, intentID).Scan(&state)
	if err != nil {
		fmt.Fprintf(os.Stderr, "query intent state: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("Intent state: %s\n", state)
}
