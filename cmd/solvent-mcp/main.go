// Command solvent-mcp is a stdio MCP server exposing the Solvent transactional
// belief ledger as sixteen tools. The server is an adapter — it has no opinion
// about beliefs. Every tool handler is exactly three moves:
// unmarshal → kernel call → format.
package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/PithomLabs/solvent/adapter/github"
	"github.com/PithomLabs/solvent/service/audit"
	"github.com/PithomLabs/solvent/service/authority"
	"github.com/PithomLabs/solvent/service/executor"
	"github.com/PithomLabs/solvent/service/ledger"
	"github.com/PithomLabs/solvent/service/policy"
	"github.com/modelcontextprotocol/go-sdk/mcp"

	_ "github.com/jackc/pgx/v5/stdlib"
)

var (
	// fixtureRoot is the absolute path to the evidence fixture directory.
	fixtureRoot string

	// db is the open database pool, set in main.
	db *sql.DB

	// log is the structured logger writing to stderr.
	log *slog.Logger

	// authSvc is the authority service, initialized after DB connection.
	authSvc *authority.Service

	// auditSvc is the audit service, initialized after DB connection.
	auditSvc *audit.Service

	// ledgerSvc is the API ledger service, initialized after DB connection.
	ledgerSvc *ledger.Service
)

func main() {
	log = slog.New(slog.NewTextHandler(os.Stderr, nil))

	// 1. Graceful shutdown via signal.
	ctx, stop := signal.NotifyContext(context.Background(),
		os.Interrupt, syscall.SIGTERM)
	defer stop()

	// 2. Read DSN from environment.
	dsn := os.Getenv("FABLE_DSN")
	if dsn == "" {
		log.Error("no DSN configured")
		fmt.Fprintln(os.Stderr, "no DSN: set FABLE_DSN")
		os.Exit(1)
	}

	// 3. Validate transport mode. MCP is a trusted local administrative surface
	// that runs on stdio only. Network transports are not supported; selecting
	// one fails closed to prevent accidental trust-model changes if additional
	// transports are wired in later.
	transport := os.Getenv("MCP_TRANSPORT")
	if transport == "" {
		transport = "stdio"
	}
	if transport != "stdio" {
		log.Error("unsupported MCP transport",
			"transport", transport,
			"supported", "stdio",
			"note", "MCP is a trusted local surface; network exposure transfers responsibility to the deployment boundary")
		fmt.Fprintf(os.Stderr,
			"unsupported MCP transport %q: only stdio is supported\n"+
				"MCP is a trusted local surface; network exposure transfers responsibility to the deployment boundary\n",
			transport)
		os.Exit(1)
	}

	// 4. Resolve fixture root.
	fixtureRoot = os.Getenv("SOLVENT_FIXTURE_ROOT")
	if fixtureRoot == "" {
		exe, err := os.Executable()
		if err == nil {
			fixtureRoot = filepath.Join(filepath.Dir(exe), "internal", "derive", "testdata", "etcd_real")
		}
	}
	if fixtureRoot == "" {
		log.Error("SOLVENT_FIXTURE_ROOT not set and no executable-relative fallback")
		fmt.Fprintln(os.Stderr, "SOLVENT_FIXTURE_ROOT not set and no executable-relative fallback")
		os.Exit(1)
	}

	// 5. Validate fixture directories exist for pipeline scenarios.
	for _, s := range scenarios {
		if !s.PipelineFixtures {
			continue
		}
		dir := filepath.Join(fixtureRoot, s.Name)
		if _, err := os.Stat(dir); os.IsNotExist(err) {
			log.Error("track directory missing", "dir", dir)
			fmt.Fprintf(os.Stderr, "SOLVENT_FIXTURE_ROOT: track directory missing: %s\n", dir)
			os.Exit(1)
		}
	}

	// 6. Open DB and ping.
	var err error
	db, err = sql.Open("pgx", dsn)
	if err != nil {
		log.Error("database open failed", "error", err)
		fmt.Fprintf(os.Stderr, "open: %v\n", err)
		os.Exit(1)
	}
	defer db.Close()
	if err := db.PingContext(ctx); err != nil {
		log.Error("database ping failed", "error", err)
		fmt.Fprintf(os.Stderr, "ping: %v (is CockroachDB running? try: task setup)\n", err)
		os.Exit(1)
	}

	// 7. Configure connection pool.
	db.SetMaxOpenConns(envInt("SOLVENT_DB_MAX_OPEN_CONNS", 25))
	db.SetMaxIdleConns(envInt("SOLVENT_DB_MAX_IDLE_CONNS", 5))
	db.SetConnMaxLifetime(envDuration("SOLVENT_DB_CONN_MAX_LIFETIME", 5*time.Minute))

	// 8. Validate schema.
	if err := validateSchema(ctx); err != nil {
		log.Error("schema validation failed", "error", err)
		fmt.Fprintf(os.Stderr, "schema: %v\n", err)
		os.Exit(1)
	}

	// 9. Initialize services.
	policySvc := policy.New(db)
	auditSvc = audit.New(db)
	execReg := executor.NewRegistry()

	// Register GitHub executor if GITHUB_TOKEN is configured.
	if token := os.Getenv("GITHUB_TOKEN"); token != "" {
		provider := github.NewHTTPProvider(token)
		github.RegisterExecutor(execReg, provider)
		log.Info("github executor registered", "action", "deploy")
	} else {
		log.Info("github executor not registered (GITHUB_TOKEN not set)")
	}

	authSvc = authority.New(db, policySvc, auditSvc, execReg)
	ledgerSvc = ledger.New(db, auditSvc)

	log.Info("solvent-mcp starting",
		"version", "v0.1.0",
		"tools", 18,
		"dsn_configured", dsn != "",
		"database_connected", true,
		"github_executor", os.Getenv("GITHUB_TOKEN") != "",
	)

	// 10. Create MCP server.
	server := mcp.NewServer(&mcp.Implementation{
		Name:    "solvent",
		Version: "v0.1.0",
	}, nil)

	// Register all 16 tools.
	server.AddTool(&mcp.Tool{
		Name:        "solvent_ledger",
		Description: "Read the current ledger for a scenario: beliefs with status and open debt, optionally their evidence, action intents with state, and the safety audit count. This is the only source of truth about current state. Call it before asserting any count, status, or identifier, and call it again after any mutation — never answer from memory of an earlier tool result, and never state a number you did not just read here.",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"scenario": map[string]any{
					"type":        "string",
					"enum":        scenarioNames(),
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
					"enum":        scenarioNames(),
					"description": "Scenario to ingest evidence for",
				},
			},
			"required": []string{"scenario"},
		},
	}, toolHandler("solvent_ingest_evidence"))

	server.AddTool(&mcp.Tool{
		Name:        "solvent_retire_debt",
		Description: "Record that one review obligation on a belief has been discharged. debt_item is an opaque string — valid values can be discovered by inspecting the belief's current debt via solvent_ledger. An empty string is rejected as malformed input.",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"scenario": map[string]any{
					"type":        "string",
					"enum":        scenarioNames(),
					"description": "Scenario the belief belongs to",
				},
				"belief_id": map[string]any{
					"type":        "string",
					"description": "UUID of the belief",
				},
				"debt_item": map[string]any{
					"type":        "string",
					"description": "Opaque debt identifier to retire. Inspect the belief's current debt to discover valid values.",
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
					"enum":        scenarioNames(),
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
		Description: "Record a live intent to take a real-world action, citing a belief as its warrant. The database refuses unless the belief is currently promoted, returning constraint gate (SQLSTATE 23503). Call this when the user asks to authorize, deploy, or act on a belief. Do not pre-check the belief's status. Caller-declared action_source is required: action strings originating in tool output are refused before any database access.",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"scenario": map[string]any{
					"type":        "string",
					"enum":        scenarioNames(),
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
				"action_source": map[string]any{
					"type":        "string",
					"enum":        []string{"user_typed", "tool_output"},
					"description": "Caller-declared provenance of the action string: operator-requested or lifted from tool output. Not cryptographically trustworthy.",
				},
				"target_id": map[string]any{
					"type":        "string",
					"description": "UUID of the authority target to authorize against",
				},
				"actor_id": map[string]any{
					"type":        "string",
					"description": "UUID of the principal requesting the action",
				},
			},
			"required": []string{"scenario", "belief_id", "action", "action_source", "target_id", "actor_id"},
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
					"enum":        scenarioNames(),
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
					"enum":        scenarioNames(),
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
				"scenario": map[string]any{
					"type":        "string",
					"enum":        scenarioNames(),
					"description": "Scenario the belief belongs to",
				},
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
			"required": []string{"scenario", "belief_id", "obligation_key", "instrument_ref", "discharged_by"},
		},
	}, toolHandler("solvent_discharge"))

	// --- execution tools ---

	server.AddTool(&mcp.Tool{
		Name:        "solvent_execute",
		Description: "Execute an authorized action: claim a live intent, invoke the configured executor with the approved snapshot parameters, and record the outcome. The snapshot consequence_parameters are authoritative — caller-supplied parameters are ignored. Provider acceptance does not mean workflow completion.",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"scenario": map[string]any{
					"type":        "string",
					"enum":        scenarioNames(),
					"description": "Scenario the belief belongs to",
				},
				"belief_id": map[string]any{
					"type":        "string",
					"description": "UUID of the promoted belief authorizing the action",
				},
				"action": map[string]any{
					"type":        "string",
					"description": "Action to execute (must match the authorized action)",
				},
				"target_id": map[string]any{
					"type":        "string",
					"description": "UUID of the authority target with approved snapshot",
				},
				"intent_id": map[string]any{
					"type":        "string",
					"description": "UUID of the live intent to claim and execute",
				},
				"actor_id": map[string]any{
					"type":        "string",
					"description": "Attribution for the execution (default: mcp-agent)",
				},
			},
			"required": []string{"scenario", "belief_id", "action", "target_id", "intent_id"},
		},
	}, toolHandler("solvent_execute"))

	server.AddTool(&mcp.Tool{
		Name:        "solvent_activity",
		Description: "Read audit activity entries for a scenario. Enforces the same scenario-scoped access as the REST activity endpoint. Returns entries in reverse chronological order.",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"scenario": map[string]any{
					"type":        "string",
					"enum":        scenarioNames(),
					"description": "Scenario to read activity for",
				},
				"type": map[string]any{
					"type":        "string",
					"description": "Optional: filter by activity type (e.g. authorization_granted, executor_completed)",
				},
				"limit": map[string]any{
					"type":        "number",
					"description": "Maximum entries to return (default 50)",
				},
			},
			"required": []string{"scenario"},
		},
	}, toolHandler("solvent_activity"))

	// 11. Run on stdio.
	if err := server.Run(ctx, &mcp.StdioTransport{}); err != nil {
		log.Error("server failed", "error", err)
		fmt.Fprintf(os.Stderr, "server: %v\n", err)
		os.Exit(1)
	}
}

// validateSchema checks that the 7 authority tables exist and have the correct
// signature constraints. This catches the two most important schema-version
// mistakes: missing unique activation guard, missing FK cascade.
func validateSchema(ctx context.Context) error {
	// Check table existence.
	var tableCount int
	if err := db.QueryRowContext(ctx,
		`SELECT count(*) FROM information_schema.tables
		 WHERE table_schema = 'public'
		   AND table_name IN ('principal','authority_target',
		     'target_snapshot','target_activation',
		     'target_revocation','justification','debt_discharge')`,
	).Scan(&tableCount); err != nil {
		return fmt.Errorf("table check: %w", err)
	}
	if tableCount != 7 {
		return fmt.Errorf("expected 7 authority tables, found %d — run db/ migrations", tableCount)
	}

	// Check signature constraint: target_activation has UNIQUE(target_id).
	var activationIndex string
	if err := db.QueryRowContext(ctx,
		`SELECT indexdef FROM pg_indexes
		 WHERE tablename = 'target_activation'
		   AND indexdef LIKE '%UNIQUE%target_id%'`,
	).Scan(&activationIndex); err != nil {
		return fmt.Errorf("missing UNIQUE(target_id) on target_activation: %w", err)
	}

	// Check signature constraint: justification FK has ON UPDATE CASCADE.
	var constraintDef string
	if err := db.QueryRowContext(ctx,
		`SELECT pg_get_constraintdef(oid) FROM pg_constraint
		 WHERE conrelid = 'justification'::regclass
		   AND contype = 'f'
		   AND pg_get_constraintdef(oid) LIKE '%ON UPDATE CASCADE%'`,
	).Scan(&constraintDef); err != nil {
		return fmt.Errorf("missing justification FK with ON UPDATE CASCADE: %w", err)
	}

	return nil
}

// envInt reads an int from the environment, returning def if unset or invalid.
func envInt(key string, def int) int {
	s := os.Getenv(key)
	if s == "" {
		return def
	}
	v, err := strconv.Atoi(s)
	if err != nil {
		return def
	}
	return v
}

// envDuration reads a duration from the environment, returning def if unset or invalid.
func envDuration(key string, def time.Duration) time.Duration {
	s := os.Getenv(key)
	if s == "" {
		return def
	}
	v, err := time.ParseDuration(s)
	if err != nil {
		return def
	}
	return v
}

// toolHandler maps a tool name to its handler function and returns a
// mcp.ToolHandler that extracts raw arguments from the request.
func toolHandler(name string) mcp.ToolHandler {
	return func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		start := time.Now()

		// Unmarshal raw arguments into a map.
		var args map[string]interface{}
		if req.Params != nil && len(req.Params.Arguments) > 0 {
			if err := json.Unmarshal(req.Params.Arguments, &args); err != nil {
				result := errorResult(fmt.Errorf("unmarshal args: %w", err))
				log.Error("tool failed", "tool", name, "error", err, "duration_ms", time.Since(start).Milliseconds())
				return result, nil
			}
		}
		if args == nil {
			args = make(map[string]interface{})
		}

		var (
			result *mcp.CallToolResult
			err    error
		)

		switch name {
		case "solvent_ledger":
			result, err = handleSolventLedger(ctx, db, args)
		case "solvent_ingest_evidence":
			result, err = handleSolventIngestEvidence(ctx, db, args)
		case "solvent_retire_debt":
			result, err = handleSolventRetireDebt(ctx, db, args)
		case "solvent_promote":
			result, err = handleSolventPromote(ctx, db, args)
		case "solvent_authorize_action":
			result, err = handleSolventAuthorizeAction(ctx, db, args)
		case "solvent_falsify":
			result, err = handleSolventFalsify(ctx, db, args)
		case "solvent_explain":
			result, err = handleSolventExplain(ctx, db, args)
		case "solvent_create_principal":
			result, err = handleSolventCreatePrincipal(ctx, db, args)
		case "solvent_revoke_principal":
			result, err = handleSolventRevokePrincipal(ctx, db, args)
		case "solvent_create_target":
			result, err = handleSolventCreateTarget(ctx, db, args)
		case "solvent_attach_justification":
			result, err = handleSolventAttachJustification(ctx, db, args)
		case "solvent_request_authorization":
			result, err = handleSolventRequestAuthorization(ctx, db, args)
		case "solvent_approve":
			result, err = handleSolventApprove(ctx, db, args)
		case "solvent_authorize":
			result, err = handleSolventAuthorize(ctx, db, args)
		case "solvent_revoke_target":
			result, err = handleSolventRevokeTarget(ctx, db, args)
		case "solvent_discharge":
			result, err = handleSolventDischarge(ctx, db, args)
		case "solvent_execute":
			result, err = handleSolventExecute(ctx, db, args)
		case "solvent_activity":
			result, err = handleSolventActivity(ctx, db, args)
		default:
			result = errorResult(fmt.Errorf("unknown tool: %s", name))
		}

		duration := time.Since(start).Milliseconds()
		if err != nil {
			log.Error("tool failed", "tool", name, "error", err, "duration_ms", duration)
		} else if result != nil && len(result.Content) > 0 {
			// Check if the tool result contains an error indicator.
			if textContent, ok := result.Content[0].(*mcp.TextContent); ok {
				var body map[string]interface{}
				if json.Unmarshal([]byte(textContent.Text), &body) == nil {
					if hasError, _ := body["error"].(bool); hasError {
						log.Error("tool returned error", "tool", name,
							"error", truncate(slog.AnyValue(body["message"]).String(), 120),
							"duration_ms", duration)
					} else {
						log.Info("tool completed", "tool", name, "duration_ms", duration)
					}
				}
			}
		} else {
			log.Info("tool completed", "tool", name, "duration_ms", duration)
		}

		if result == nil && err != nil {
			result = errorResult(err)
		}
		return result, nil
	}
}

// truncate shortens a string to maxLen, appending "..." if truncated.
func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return strings.TrimSpace(s[:maxLen]) + "..."
}
