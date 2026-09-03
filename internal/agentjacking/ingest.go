package agentjacking

import (
	"context"
	"database/sql"
	"fmt"
	"os"

	"github.com/PithomLabs/solvent/internal/belief"
	"github.com/PithomLabs/solvent/internal/view"
	"github.com/PithomLabs/solvent/kernel"
)

// Result reports what the demo orchestration did. It carries no judgment: the
// debt, promotion, and authorization outcomes below are whatever the existing
// Solvent machinery decided.
type Result struct {
	BeliefID         string   `json:"belief_id"`
	Claim            string   `json:"claim"`
	ProvenanceClass  string   `json:"provenance_class"`
	SourceType       string   `json:"source_type"`
	SourceURL        string   `json:"source_url"`
	ContentSHA256    string   `json:"content_sha256"`
	MessageRaw       string   `json:"message_raw"`
	MessageClean     string   `json:"message_clean"`
	EmbeddedCommands []string `json:"embedded_commands"`
	Status           string   `json:"status"`
	DebtRemaining    []string `json:"debt_remaining"`
}

// Ingest is demo orchestration and nothing more: parse the fixture, build the
// generic evidence and belief, hand them to belief.Process, and read back the
// resulting ledger state. It makes no decisions of its own — not whether the
// evidence retires debt, not whether the belief is promotable, not whether any
// action is authorized, and not whether an embedded command is dangerous.
// Every one of those judgments belongs to the existing Solvent layers.
func Ingest(ctx context.Context, db *sql.DB, scenarioID string, fixturePath string) (Result, error) {
	raw, err := readFixture(fixturePath)
	if err != nil {
		return Result{}, err
	}
	ev, err := Parse(raw)
	if err != nil {
		return Result{}, err
	}
	evidence, err := BuildEvidence(raw, ev)
	if err != nil {
		return Result{}, err
	}

	st := kernel.New(db)
	// belief.Process is idempotent but does not return the belief ID; ensure it
	// first so the demo can address the belief by ID.
	b := Belief(evidence)
	beliefID, err := st.EnsureBelief(ctx, scenarioID, b.Claim, kernel.Derived)
	if err != nil {
		return Result{}, fmt.Errorf("agentjacking: ensure belief: %w", err)
	}
	if err := belief.Process(ctx, db, scenarioID, b); err != nil {
		return Result{}, fmt.Errorf("agentjacking: belief process: %w", err)
	}

	snap, err := view.GetSnapshot(ctx, db, scenarioID, view.SnapshotOpts{BeliefID: beliefID})
	if err != nil {
		return Result{}, fmt.Errorf("agentjacking: read back ledger: %w", err)
	}
	if len(snap.Beliefs) != 1 {
		return Result{}, fmt.Errorf("agentjacking: belief %s not found after ingest", beliefID)
	}
	stored := snap.Beliefs[0]

	debt := stored.Debt
	if debt == nil {
		debt = []string{}
	}
	return Result{
		BeliefID:         beliefID,
		Claim:            stored.Claim,
		ProvenanceClass:  evidence.ProvenanceClass,
		SourceType:       evidence.SourceType,
		SourceURL:        evidence.SourceURL,
		ContentSHA256:    evidence.ContentSHA256,
		MessageRaw:       ev.Message,
		MessageClean:     cleanMessage(ev.Message),
		EmbeddedCommands: DetectCommands(ev.Message),
		Status:           stored.Status,
		DebtRemaining:    debt,
	}, nil
}

// readFixture loads the raw fixture bytes. The bytes are hashed as-is, so the
// file on disk is the artifact the ledger's evidence row points at.
func readFixture(path string) ([]byte, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("agentjacking: read fixture %s: %w", path, err)
	}
	return raw, nil
}
