// Command solvent-mcp is a stdio MCP server exposing the Solvent transactional
// belief ledger as six tools. The server is an adapter — it has no opinion about
// beliefs. Every tool handler is exactly three moves: unmarshal → kernel call →
// format.
package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/PithomLabs/solvent/internal/testdb"
	"github.com/modelcontextprotocol/go-sdk/mcp"

	_ "github.com/jackc/pgx/v5/stdlib"
)

var (
	// fixtureRoot is the absolute path to the evidence fixture directory.
	fixtureRoot string

	// db is the open database pool, set in main.
	db *sql.DB
)

// scenarioToID maps scenario names to their fixed UUIDs.
var scenarioToID = map[string]string{
	"track1": "00000000-0000-0000-0000-000000000001",
	"track2": "00000000-0000-0000-0000-000000000002",
}

func main() {
	ctx := context.Background()

	// 1. Read DSN from environment.
	dsn := os.Getenv("FABLE_DSN")
	if dsn == "" {
		fmt.Fprintln(os.Stderr, "no DSN: set FABLE_DSN")
		os.Exit(1)
	}

	// 2. Resolve fixture root.
	fixtureRoot = os.Getenv("SOLVENT_FIXTURE_ROOT")
	if fixtureRoot == "" {
		exe, err := os.Executable()
		if err == nil {
			fixtureRoot = filepath.Join(filepath.Dir(exe), "internal", "derive", "testdata", "etcd_real")
		}
	}
	if fixtureRoot == "" {
		fmt.Fprintln(os.Stderr, "SOLVENT_FIXTURE_ROOT not set and no executable-relative fallback")
		os.Exit(1)
	}

	// 3. Validate fixture directories exist.
	for _, track := range []string{"track1", "track2"} {
		dir := filepath.Join(fixtureRoot, track)
		if _, err := os.Stat(dir); os.IsNotExist(err) {
			fmt.Fprintf(os.Stderr, "SOLVENT_FIXTURE_ROOT: track directory missing: %s\n", dir)
			os.Exit(1)
		}
	}

	// 4. Open DB and ping.
	var err error
	db, err = testdb.Open(dsn)
	if err != nil {
		fmt.Fprintf(os.Stderr, "open: %v\n", err)
		os.Exit(1)
	}
	defer db.Close()
	if err := db.PingContext(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "ping: %v (is CockroachDB running? try: task setup)\n", err)
		os.Exit(1)
	}

	// 5. Create MCP server.
	server := mcp.NewServer(&mcp.Implementation{
		Name:    "solvent",
		Version: "v0.1.0",
	}, nil)

	// 6. Register 6 tools.
	server.AddTool(&mcp.Tool{
		Name:        "solvent_ledger",
		Description: "Read the current ledger for a scenario: beliefs with status and open debt, optionally their evidence, action intents with state, and the safety audit count. This is the only source of truth about current state. Call it before asserting any count, status, or identifier, and call it again after any mutation — never answer from memory of an earlier tool result, and never state a number you did not just read here.",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"scenario": map[string]any{
					"type":        "string",
					"enum":        []string{"track1", "track2"},
					"description": "Scenario to query",
				},
				"belief_id": map[string]any{
					"type":        "string",
					"description": "Optional: filter to a single belief by UUID",
				},
				"include_evidence": map[string]any{
					"type":        "boolean",
					"description": "Include evidence rows (default false)",
				},
			},
			"required": []string{"scenario"},
		},
	}, toolHandler("solvent_ledger"))

	server.AddTool(&mcp.Tool{
		Name:        "solvent_ingest_evidence",
		Description: "Process the pinned evidence fixtures for a scenario through the full pipeline (normalize → derive → ledger). Idempotent: re-running creates no duplicate beliefs, evidence, or intents.",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"scenario": map[string]any{
					"type":        "string",
					"enum":        []string{"track1", "track2"},
					"description": "Scenario to ingest evidence for",
				},
			},
			"required": []string{"scenario"},
		},
	}, toolHandler("solvent_ingest_evidence"))

	server.AddTool(&mcp.Tool{
		Name:        "solvent_retire_debt",
		Description: "Record that one review obligation on a belief has been discharged. Valid items: needProvenanceCheck, needContradictionSweep, needBlastRadius, needRollbackPlan, needVersionPin, needOperatorSignoff. Retiring an item that is already absent is a no-op, not an error.",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"scenario": map[string]any{
					"type":        "string",
					"enum":        []string{"track1", "track2"},
					"description": "Scenario the belief belongs to",
				},
				"belief_id": map[string]any{
					"type":        "string",
					"description": "UUID of the belief",
				},
				"debt_item": map[string]any{
					"type":        "string",
					"description": "Debt item to retire (e.g. needProvenanceCheck)",
				},
			},
			"required": []string{"scenario", "belief_id", "debt_item"},
		},
	}, toolHandler("solvent_retire_debt"))

	server.AddTool(&mcp.Tool{
		Name:        "solvent_promote",
		Description: "Attempt to promote a belief to authorized status. The database refuses promotion while the belief carries any open debt item, returning constraint promoted_is_debt_free (SQLSTATE 23514). Call this whenever the user asks to promote a belief — do not pre-check the debt yourself and do not decline on the user's behalf. The database is the authority on whether promotion is permitted; your job is to attempt it and report what it says.",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"scenario": map[string]any{
					"type":        "string",
					"enum":        []string{"track1", "track2"},
					"description": "Scenario the belief belongs to",
				},
				"belief_id": map[string]any{
					"type":        "string",
					"description": "UUID of the belief to promote",
				},
			},
			"required": []string{"scenario", "belief_id"},
		},
	}, toolHandler("solvent_promote"))

	server.AddTool(&mcp.Tool{
		Name:        "solvent_authorize_action",
		Description: "Record a live intent to take a real-world action, citing a belief as its warrant. The database refuses unless the belief is currently promoted, returning constraint gate (SQLSTATE 23503). Call this when the user asks to authorize, deploy, or act on a belief. Do not pre-check the belief's status.",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"scenario": map[string]any{
					"type":        "string",
					"enum":        []string{"track1", "track2"},
					"description": "Scenario the belief belongs to",
				},
				"belief_id": map[string]any{
					"type":        "string",
					"description": "UUID of the belief to cite as warrant",
				},
				"action": map[string]any{
					"type":        "string",
					"description": "Description of the real-world action to authorize",
				},
			},
			"required": []string{"scenario", "belief_id", "action"},
		},
	}, toolHandler("solvent_authorize_action"))

	server.AddTool(&mcp.Tool{
		Name:        "solvent_falsify",
		Description: "Retract a belief that new evidence has falsified. Cancels that belief's dependent live intent in the same transaction. Retracts a single belief — this does not propagate across a belief graph. Obtain the belief's id from solvent_ledger immediately before calling.",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"scenario": map[string]any{
					"type":        "string",
					"enum":        []string{"track1", "track2"},
					"description": "Scenario the belief belongs to",
				},
				"belief_id": map[string]any{
					"type":        "string",
					"description": "UUID of the belief to retract (read from solvent_ledger)",
				},
			},
			"required": []string{"scenario", "belief_id"},
		},
	}, toolHandler("solvent_falsify"))

	// 7. Run on stdio.
	if err := server.Run(ctx, &mcp.StdioTransport{}); err != nil {
		fmt.Fprintf(os.Stderr, "server: %v\n", err)
		os.Exit(1)
	}
}

// toolHandler maps a tool name to its handler function and returns a
// mcp.ToolHandler that extracts raw arguments from the request.
func toolHandler(name string) mcp.ToolHandler {
	return func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		// Unmarshal raw arguments into a map.
		var args map[string]interface{}
		if req.Params != nil && len(req.Params.Arguments) > 0 {
			if err := json.Unmarshal(req.Params.Arguments, &args); err != nil {
				return errorResult(fmt.Errorf("unmarshal args: %w", err)), nil
			}
		}
		if args == nil {
			args = make(map[string]interface{})
		}

		switch name {
		case "solvent_ledger":
			return handleSolventLedger(ctx, db, args)
		case "solvent_ingest_evidence":
			return handleSolventIngestEvidence(ctx, db, args)
		case "solvent_retire_debt":
			return handleSolventRetireDebt(ctx, db, args)
		case "solvent_promote":
			return handleSolventPromote(ctx, db, args)
		case "solvent_authorize_action":
			return handleSolventAuthorizeAction(ctx, db, args)
		case "solvent_falsify":
			return handleSolventFalsify(ctx, db, args)
		default:
			return errorResult(fmt.Errorf("unknown tool: %s", name)), nil
		}
	}
}
