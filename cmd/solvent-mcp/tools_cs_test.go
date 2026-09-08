package main

import (
	"context"
	"strings"
	"testing"

	"github.com/PithomLabs/solvent/kernel"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// TestCS_Discharge_WrongScenario_MCP verifies that MCP solvent_discharge
// with a valid scenario name that doesn't contain the belief returns a tool
// error and zero durable side effects.
func TestCS_Discharge_WrongScenario_MCP(t *testing.T) {
	db := sharedDB
	ctx := context.Background()
	st := kernel.New(db)

	// Create belief in track1.
	bid, err := st.EnsureBelief(ctx, "00000000-0000-0000-0000-000000000001",
		"belief for MCP cross-scenario discharge", kernel.Derived)
	if err != nil {
		t.Fatalf("setup: ensure belief: %v", err)
	}

	// Read state before.
	var countBefore int
	_ = db.QueryRowContext(ctx,
		`SELECT count(*) FROM debt_discharge WHERE belief_id=$1::UUID`, bid).Scan(&countBefore)
	var debtBefore string
	_ = db.QueryRowContext(ctx,
		`SELECT array_to_string(debt, ',') FROM belief WHERE id=$1::UUID`, bid).Scan(&debtBefore)

	// Call discharge with track2 (valid scenario, but belief is in track1).
	args := map[string]interface{}{
		"scenario":       "track2",
		"belief_id":      bid,
		"obligation_key": "mcp-cs-obligation",
		"instrument_ref": "mcp-cs-instrument",
		"discharged_by":  "00000000-0000-0000-0000-000000000099",
	}

	result, err := handleSolventDischarge(ctx, db, args)
	if err != nil {
		t.Fatalf("handler returned Go error: %v", err)
	}
	if !result.IsError {
		t.Fatal("expected MCP error for wrong scenario, got success")
	}

	text := result.Content[0].(*mcp.TextContent).Text
	if !strings.Contains(text, "not found") {
		t.Errorf("error text missing 'not found': %s", text)
	}

	// Verify zero mutation.
	var countAfter int
	_ = db.QueryRowContext(ctx,
		`SELECT count(*) FROM debt_discharge WHERE belief_id=$1::UUID`, bid).Scan(&countAfter)
	var debtAfter string
	_ = db.QueryRowContext(ctx,
		`SELECT array_to_string(debt, ',') FROM belief WHERE id=$1::UUID`, bid).Scan(&debtAfter)
	if countBefore != countAfter {
		t.Errorf("discharge rows mutated: before=%d, after=%d", countBefore, countAfter)
	}
	if debtBefore != debtAfter {
		t.Errorf("debt mutated: before=%q, after=%q", debtBefore, debtAfter)
	}
}
