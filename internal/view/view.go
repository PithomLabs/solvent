// Package view provides read-only projections for the transactional belief
// ledger. Every query is a SELECT — no INSERT, UPDATE, DELETE, CREATE, or DROP.
//
// This package exists so the MCP layer can present current state without
// containing any query of its own.
package view

import (
	"context"
	"database/sql"
	"strings"
)

// Belief is a read-only projection of a belief row.
type Belief struct {
	ID        string   `json:"id"`
	Claim     string   `json:"claim"`
	ClaimType string   `json:"claim_type"`
	Status    string   `json:"status"`
	Debt      []string `json:"debt"`
}

// Evidence is a read-only projection of an evidence row.
type Evidence struct {
	BeliefID        string `json:"belief_id"`
	SourceURL       string `json:"source_url"`
	ProvenanceClass string `json:"provenance_class"`
	ContentSHA256   string `json:"content_sha256"`
}

// Intent is a read-only projection of an action_intent row.
type Intent struct {
	BeliefID string `json:"belief_id"`
	Action   string `json:"action"`
	State    string `json:"state"`
}

// Snapshot is a complete read-only view of a scenario's ledger state.
type Snapshot struct {
	Beliefs                []Belief   `json:"beliefs"`
	Evidence               []Evidence `json:"evidence,omitempty"`
	Intents                []Intent   `json:"intents"`
	AuditLiveOnNonPromoted int        `json:"audit_live_on_nonpromoted"`
}

// SnapshotOpts controls which parts of the snapshot are populated.
type SnapshotOpts struct {
	BeliefID        string // optional: filter to a single belief
	IncludeEvidence bool   // whether to include evidence rows
}

// GetSnapshot returns a read-only view of the ledger for a scenario.
//
// All queries are SELECT only. The audit count is the return of
// pipeline.AuditIntent, which wraps kernel.AuditLiveOnNonPromoted.
func GetSnapshot(ctx context.Context, db *sql.DB, scenarioID string, opts SnapshotOpts) (*Snapshot, error) {
	snap := &Snapshot{}

	// Beliefs.
	if opts.BeliefID != "" {
		row := db.QueryRowContext(ctx,
			`SELECT id, claim, claim_type, status, debt::STRING
			 FROM belief WHERE scenario_id=$1::UUID AND id=$2::UUID`,
			scenarioID, opts.BeliefID)
		b, err := scanBelief(row)
		if err != nil {
			return nil, err
		}
		snap.Beliefs = []Belief{*b}
	} else {
		rows, err := db.QueryContext(ctx,
			`SELECT id, claim, claim_type, status, debt::STRING
			 FROM belief WHERE scenario_id=$1::UUID ORDER BY claim`,
			scenarioID)
		if err != nil {
			return nil, err
		}
		defer rows.Close()
		for rows.Next() {
			b, err := scanBelief(rows)
			if err != nil {
				return nil, err
			}
			snap.Beliefs = append(snap.Beliefs, *b)
		}
		if err := rows.Err(); err != nil {
			return nil, err
		}
	}

	// Evidence (only when requested).
	if opts.IncludeEvidence {
		rows, err := db.QueryContext(ctx,
			`SELECT belief_id, source_url, provenance_class, content_sha256
			 FROM evidence WHERE scenario_id=$1::UUID ORDER BY belief_id, ingested_at`,
			scenarioID)
		if err != nil {
			return nil, err
		}
		defer rows.Close()
		for rows.Next() {
			var e Evidence
			if err := rows.Scan(&e.BeliefID, &e.SourceURL, &e.ProvenanceClass, &e.ContentSHA256); err != nil {
				return nil, err
			}
			snap.Evidence = append(snap.Evidence, e)
		}
		if err := rows.Err(); err != nil {
			return nil, err
		}
	}

	// Intents.
	rows, err := db.QueryContext(ctx,
		`SELECT belief_id, action, state
		 FROM action_intent WHERE scenario_id=$1::UUID ORDER BY belief_id`,
		scenarioID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var i Intent
		if err := rows.Scan(&i.BeliefID, &i.Action, &i.State); err != nil {
			return nil, err
		}
		snap.Intents = append(snap.Intents, i)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	return snap, nil
}

// scanner is an interface for scanning a single row.
type scanner interface {
	Scan(dest ...any) error
}

// scanBelief reads a belief row from a scanner.
func scanBelief(s scanner) (*Belief, error) {
	var b Belief
	var debtRaw string
	if err := s.Scan(&b.ID, &b.Claim, &b.ClaimType, &b.Status, &debtRaw); err != nil {
		return nil, err
	}
	b.Debt = parsePGArray(debtRaw)
	return &b, nil
}

// parsePGArray parses a PostgreSQL text array literal like {a,b,c} into a string slice.
func parsePGArray(s string) []string {
	s = strings.TrimSpace(s)
	if s == "" || s == "{}" {
		return []string{}
	}
	s = strings.TrimPrefix(s, "{")
	s = strings.TrimSuffix(s, "}")
	if s == "" {
		return []string{}
	}
	return strings.Split(s, ",")
}
