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
	"github.com/PithomLabs/solvent/kernel"
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
		Name: "solvent_retire_debt",
		// The valid items are no longer restated in prose. They were, and the prose was
		// one of five hand-copies of kernel.FullDebt; the Phase 5 vocabulary rename had
		// to find every one of them. The enum below is generated from the kernel, so
		// this description can only describe behaviour, not enumerate values.
		Description: "Record that one review obligation on a belief has been discharged. debt_item must be one of the six items the database issued (see the enum). An unrecognised item is refused rather than silently ignored.",
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
					"type": "string",
					// Generated, not transcribed. Advertising the vocabulary to the agent
					// is only half the job: this SDK's low-level AddTool does not validate
					// arguments against the schema, so the enum is documentation and
					// handleSolventRetireDebt does the refusing.
					"enum":        kernel.FullDebt,
					"description": "Debt item to retire. Must be one of the six the database issued.",
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

	server.AddTool(&mcp.Tool{
		Name:        "solvent_explain",
		Description: "Explain whether beliefs in a scenario are promotable or authorizable. Read-only projection derived from the same SELECT-backed ledger as solvent_ledger (internal/view). Returns structured fields (can_promote, can_authorize, remaining debt, live intents, retracted state) plus a concise human summary. Never invents SQLSTATEs: real engine errors (23503 gate, 23514 promoted_is_debt_free/live_requires_promoted) are preserved only when actually emitted; predictions are labeled as predicted_* with predicted constraint names.",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"scenario": map[string]any{
					"type":        "string",
					"enum":        []string{"track1", "track2"},
					"description": "Scenario to explain",
				},
				"belief_id": map[string]any{
					"type":        "string",
					"description": "Optional: UUID of a single belief to explain; if omitted, all beliefs in the scenario are explained",
				},
				"include_evidence": map[string]any{
					"type":        "boolean",
					"description": "Include evidence rows in the explanation (default false)",
				},
			},
			"required": []string{"scenario"},
		},
	}, toolHandler("solvent_explain"))

	// --- Authority lifecycle tools ---

	server.AddTool(&mcp.Tool{
		Name:        "solvent_create_principal",
		Description: "Create a new principal (identity record). Returns the principal_id. MCP principal-ID fields are attribution inputs, not authentication proof — the v0 MCP server must be deployed as a trusted administrative surface.",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"principal_type": map[string]any{
					"type":        "string",
					"description": "Principal type (e.g. \"human\", \"agent\")",
				},
				"issuer": map[string]any{
					"type":        "string",
					"description": "Issuer or source of the principal identity",
				},
			},
			"required": []string{"principal_type", "issuer"},
		},
	}, toolHandler("solvent_create_principal"))

	server.AddTool(&mcp.Tool{
		Name:        "solvent_revoke_principal",
		Description: "Revoke a principal. The principal is recorded as revoked; existing FK references remain valid. Revocation is idempotent.",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"principal_id": map[string]any{
					"type":        "string",
					"description": "UUID of the principal to revoke",
				},
			},
			"required": []string{"principal_id"},
		},
	}, toolHandler("solvent_revoke_principal"))

	server.AddTool(&mcp.Tool{
		Name:        "solvent_create_target",
		Description: "Create an authority target proposal. No authority is granted until Approve is called. The created_by field records attribution, not caller identity.",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"principal_id": map[string]any{
					"type":        "string",
					"description": "UUID of the principal this target is for",
				},
				"resource_type": map[string]any{
					"type":        "string",
					"description": "Type of resource (e.g. \"cluster\", \"namespace\")",
				},
				"resource_id": map[string]any{
					"type":        "string",
					"description": "Identifier of the resource",
				},
				"scope": map[string]any{
					"type":        "string",
					"description": "Scope of authority (e.g. \"cluster\", \"namespace\")",
				},
				"action_namespace": map[string]any{
					"type":        "string",
					"description": "Namespace of the action (e.g. \"k8s\", \"db\")",
				},
				"action_name": map[string]any{
					"type":        "string",
					"description": "Name of the action (e.g. \"deploy\", \"rollback\")",
				},
				"consequence_type": map[string]any{
					"type":        "string",
					"description": "Type of consequence (e.g. \"state_change\", \"data_write\")",
				},
				"consequence_parameters": map[string]any{
					"type":        "string",
					"description": "JSON-encoded consequence parameters",
				},
				"created_by": map[string]any{
					"type":        "string",
					"description": "UUID of the principal creating this target (attribution, not auth)",
				},
			},
			"required": []string{"principal_id", "resource_type", "resource_id", "scope", "action_namespace", "action_name", "consequence_type", "consequence_parameters", "created_by"},
		},
	}, toolHandler("solvent_create_target"))

	server.AddTool(&mcp.Tool{
		Name:        "solvent_attach_justification",
		Description: "Attach a promoted belief as justification for an authority target. Idempotent: duplicate attachment is a no-op. The target must not be activated.",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"target_id": map[string]any{
					"type":        "string",
					"description": "UUID of the authority target",
				},
				"belief_id": map[string]any{
					"type":        "string",
					"description": "UUID of the promoted belief",
				},
				"belief_status": map[string]any{
					"type":        "string",
					"description": "Current status of the belief (typically \"promoted\")",
				},
				"attached_by": map[string]any{
					"type":        "string",
					"description": "UUID of the principal attaching the justification (attribution)",
				},
			},
			"required": []string{"target_id", "belief_id", "belief_status", "attached_by"},
		},
	}, toolHandler("solvent_attach_justification"))

	server.AddTool(&mcp.Tool{
		Name:        "solvent_request_authorization",
		Description: "Pin the current proposal and justification set for approval. Must be called before Approve. The hash is recomputed on each call.",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"target_id": map[string]any{
					"type":        "string",
					"description": "UUID of the authority target",
				},
				"requested_by": map[string]any{
					"type":        "string",
					"description": "UUID of the principal requesting authorization (attribution)",
				},
			},
			"required": []string{"target_id", "requested_by"},
		},
	}, toolHandler("solvent_request_authorization"))

	server.AddTool(&mcp.Tool{
		Name:        "solvent_approve",
		Description: "Approve an authority target. Atomically creates an immutable snapshot and activation. This is the ONLY operation that creates authority. Only call from a trusted administrative surface — the approved_by field is attribution, not caller authentication.",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"target_id": map[string]any{
					"type":        "string",
					"description": "UUID of the authority target to approve",
				},
				"approved_by": map[string]any{
					"type":        "string",
					"description": "UUID of the approving principal (attribution, not caller auth)",
				},
			},
			"required": []string{"target_id", "approved_by"},
		},
	}, toolHandler("solvent_approve"))

	server.AddTool(&mcp.Tool{
		Name:        "solvent_authorize",
		Description: "Read-only verification: check if an execution-time tuple matches the approved authority. Returns Allowed=true only on exact match with current belief state. Zero writes — no authorization_decision, no warrant, no receipt.",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"target_id": map[string]any{
					"type":        "string",
					"description": "UUID of the authority target to authorize against",
				},
				"principal_id": map[string]any{
					"type":        "string",
					"description": "Presented principal ID",
				},
				"resource_type": map[string]any{
					"type":        "string",
					"description": "Presented resource type",
				},
				"resource_id": map[string]any{
					"type":        "string",
					"description": "Presented resource ID",
				},
				"scope": map[string]any{
					"type":        "string",
					"description": "Presented scope",
				},
				"action_namespace": map[string]any{
					"type":        "string",
					"description": "Presented action namespace",
				},
				"action_name": map[string]any{
					"type":        "string",
					"description": "Presented action name",
				},
				"consequence_type": map[string]any{
					"type":        "string",
					"description": "Presented consequence type",
				},
				"consequence_parameters": map[string]any{
					"type":        "string",
					"description": "JSON-encoded consequence parameters",
				},
			},
			"required": []string{"target_id", "principal_id", "resource_type", "resource_id", "scope", "action_namespace", "action_name", "consequence_type", "consequence_parameters"},
		},
	}, toolHandler("solvent_authorize"))

	server.AddTool(&mcp.Tool{
		Name:        "solvent_revoke_target",
		Description: "Revoke an authority target. Inserts an immutable revocation fact. Does not delete the activation — revocation does not free the once-ever activation uniqueness.",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"target_id": map[string]any{
					"type":        "string",
					"description": "UUID of the authority target to revoke",
				},
				"revoked_by": map[string]any{
					"type":        "string",
					"description": "UUID of the principal revoking (attribution)",
				},
				"reason": map[string]any{
					"type":        "string",
					"description": "Reason for revocation",
				},
			},
			"required": []string{"target_id", "revoked_by", "reason"},
		},
	}, toolHandler("solvent_revoke_target"))

	server.AddTool(&mcp.Tool{
		Name:        "solvent_discharge",
		Description: "Record that a belief's debt obligation has been discharged by a principal. Idempotent: duplicate discharge (same belief + obligation + instrument + principal) is rejected.",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"belief_id": map[string]any{
					"type":        "string",
					"description": "UUID of the belief whose debt is discharged",
				},
				"obligation_key": map[string]any{
					"type":        "string",
					"description": "Key identifying the obligation (e.g. \"needProvenanceCheck\")",
				},
				"instrument_ref": map[string]any{
					"type":        "string",
					"description": "Reference to the discharge instrument (e.g. \"attestation-123\")",
				},
				"discharged_by": map[string]any{
					"type":        "string",
					"description": "UUID of the principal performing the discharge (attribution)",
				},
			},
			"required": []string{"belief_id", "obligation_key", "instrument_ref", "discharged_by"},
		},
	}, toolHandler("solvent_discharge"))

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
		case "solvent_explain":
			return handleSolventExplain(ctx, db, args)
		case "solvent_create_principal":
			return handleSolventCreatePrincipal(ctx, db, args)
		case "solvent_revoke_principal":
			return handleSolventRevokePrincipal(ctx, db, args)
		case "solvent_create_target":
			return handleSolventCreateTarget(ctx, db, args)
		case "solvent_attach_justification":
			return handleSolventAttachJustification(ctx, db, args)
		case "solvent_request_authorization":
			return handleSolventRequestAuthorization(ctx, db, args)
		case "solvent_approve":
			return handleSolventApprove(ctx, db, args)
		case "solvent_authorize":
			return handleSolventAuthorize(ctx, db, args)
		case "solvent_revoke_target":
			return handleSolventRevokeTarget(ctx, db, args)
		case "solvent_discharge":
			return handleSolventDischarge(ctx, db, args)
		default:
			return errorResult(fmt.Errorf("unknown tool: %s", name)), nil
		}
	}
}
