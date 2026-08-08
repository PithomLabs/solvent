package belief_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/PithomLabs/solvent/internal/belief"
	"github.com/PithomLabs/solvent/internal/derive"
	"github.com/PithomLabs/solvent/internal/intent"
	"github.com/PithomLabs/solvent/internal/normalize"
)

// integrationScenario uses a different prefix to avoid collisions with unit tests.
func integrationScenario(n int) string {
	return fmt.Sprintf("44444444-0000-0000-0000-%012x", n)
}

func TestIntegration_ProcessToIntent(t *testing.T) {
	ctx := context.Background()
	sc := integrationScenario(1)

	// Step 1: Process a derived belief with enough evidence to promote.
	b := derive.DerivedBelief{
		Claim:          "etcd v3.5.x is vulnerable to CVE-2024-24790",
		Classification: "derived",
		SupportingEvidence: []normalize.NormalizedEvidence{
			{SourceType: "kev_entry", SourceURL: "https://nvd.nist.gov/vuln/detail/CVE-2024-24790", ProvenanceClass: "external_feed", ContentSHA256: "int01", Assertion: "etcd v3.5.x is vulnerable to CVE-2024-24790"},
			{SourceType: "release", SourceURL: "https://github.com/etcd-io/etcd/releases/tag/v3.5.15", ProvenanceClass: "external_feed", ContentSHA256: "int02", Assertion: "release v3.5.15 published"},
			{SourceType: "maintainer_comment", SourceURL: "https://github.com/etcd-io/etcd/issues/17234#ic1", ProvenanceClass: "external_feed", ContentSHA256: "int03", Assertion: "fix released"},
			{SourceType: "maintainer_comment", SourceURL: "https://github.com/etcd-io/etcd/issues/17234#ic2", ProvenanceClass: "external_feed", ContentSHA256: "int04", Assertion: "tested and confirmed"},
			{SourceType: "maintainer_comment", SourceURL: "https://github.com/etcd-io/etcd/issues/17234#ic3", ProvenanceClass: "external_feed", ContentSHA256: "int05", Assertion: "no regression observed"},
			{SourceType: "maintainer_comment", SourceURL: "https://github.com/etcd-io/etcd/issues/17234#ic4", ProvenanceClass: "external_feed", ContentSHA256: "int06", Assertion: "security review completed"},
		},
	}
	if err := belief.Process(ctx, shared, sc, b); err != nil {
		t.Fatalf("Process: %v", err)
	}

	// Step 2: Verify belief is promoted.
	var status string
	_ = shared.QueryRowContext(ctx,
		`SELECT status FROM belief WHERE scenario_id=$1::UUID`, sc).Scan(&status)
	if status != "promoted" {
		t.Fatalf("expected promoted, got %q", status)
	}

	// Step 3: Find belief ID and propose intent.
	var beliefID string
	_ = shared.QueryRowContext(ctx,
		`SELECT id FROM belief WHERE scenario_id=$1::UUID`, sc).Scan(&beliefID)

	if err := intent.Propose(ctx, shared, sc, beliefID, "upgrade"); err != nil {
		t.Fatalf("Propose: %v", err)
	}

	// Step 4: Audit passes.
	n, err := intent.Audit(ctx, shared, sc)
	if err != nil {
		t.Fatalf("Audit: %v", err)
	}
	if n != 0 {
		t.Errorf("expected 0 live intents on non-promoted, got %d", n)
	}
}

func TestIntegration_PartialDebtBlocksPromotion(t *testing.T) {
	ctx := context.Background()
	sc := integrationScenario(2)

	// Only kev_entry evidence — retires needMap but not the rest.
	b := derive.DerivedBelief{
		Claim:          "etcd v3.5.x is vulnerable to CVE-2024-24790",
		Classification: "derived",
		SupportingEvidence: []normalize.NormalizedEvidence{
			{SourceType: "kev_entry", SourceURL: "https://nvd.nist.gov/vuln/detail/CVE-2024-24790", ProvenanceClass: "external_feed", ContentSHA256: "int07", Assertion: "etcd v3.5.x is vulnerable to CVE-2024-24790"},
		},
	}
	if err := belief.Process(ctx, shared, sc, b); err != nil {
		t.Fatalf("Process: %v", err)
	}

	var status string
	_ = shared.QueryRowContext(ctx,
		`SELECT status FROM belief WHERE scenario_id=$1::UUID`, sc).Scan(&status)
	if status != "entered" {
		t.Errorf("expected entered (debt not fully retired), got %q", status)
	}

	// Intent on unpromoted should fail.
	var beliefID string
	_ = shared.QueryRowContext(ctx,
		`SELECT id FROM belief WHERE scenario_id=$1::UUID`, sc).Scan(&beliefID)

	err := intent.Propose(ctx, shared, sc, beliefID, "upgrade")
	if err == nil {
		t.Error("expected error for intent on unpromoted belief, got nil")
	}
}

func TestIntegration_MultiScenarioIsolation(t *testing.T) {
	ctx := context.Background()

	b := derive.DerivedBelief{
		Claim:          "etcd v3.5.x is vulnerable to CVE-2024-24790",
		Classification: "derived",
		SupportingEvidence: []normalize.NormalizedEvidence{
			{SourceType: "kev_entry", SourceURL: "https://nvd.nist.gov/vuln/detail/CVE-2024-24790", ProvenanceClass: "external_feed", ContentSHA256: "int08"},
		},
	}

	scA := integrationScenario(3)
	scB := integrationScenario(4)

	if err := belief.Process(ctx, shared, scA, b); err != nil {
		t.Fatalf("Process A: %v", err)
	}
	if err := belief.Process(ctx, shared, scB, b); err != nil {
		t.Fatalf("Process B: %v", err)
	}

	var countA, countB int
	_ = shared.QueryRowContext(ctx, `SELECT count(*) FROM belief WHERE scenario_id=$1::UUID`, scA).Scan(&countA)
	_ = shared.QueryRowContext(ctx, `SELECT count(*) FROM belief WHERE scenario_id=$1::UUID`, scB).Scan(&countB)

	if countA != 1 || countB != 1 {
		t.Errorf("expected 1 belief per scenario, got A=%d B=%d", countA, countB)
	}

	// Audit on A should be independent of B.
	n, _ := intent.Audit(ctx, shared, scA)
	if n != 0 {
		t.Errorf("expected 0 on A, got %d", n)
	}
}

func TestIntegration_DeterminismAcrossReplays(t *testing.T) {
	ctx := context.Background()
	sc := integrationScenario(5)

	b := derive.DerivedBelief{
		Claim:          "etcd v3.5.x is vulnerable to CVE-2024-24790",
		Classification: "derived",
		SupportingEvidence: []normalize.NormalizedEvidence{
			{SourceType: "kev_entry", SourceURL: "https://nvd.nist.gov/vuln/detail/CVE-2024-24790", ProvenanceClass: "external_feed", ContentSHA256: "int09"},
		},
	}

	// Replay 100 times — idempotent.
	for i := 0; i < 100; i++ {
		if err := belief.Process(ctx, shared, sc, b); err != nil {
			t.Fatalf("iteration %d: %v", i, err)
		}
	}

	var count int
	_ = shared.QueryRowContext(ctx,
		`SELECT count(*) FROM belief WHERE scenario_id=$1::UUID`, sc).Scan(&count)
	if count != 1 {
		t.Errorf("expected 1 belief after 100 replays, got %d", count)
	}

	var evidenceCount int
	_ = shared.QueryRowContext(ctx,
		`SELECT count(*) FROM evidence WHERE scenario_id=$1::UUID`, sc).Scan(&evidenceCount)
	if evidenceCount != 1 {
		t.Errorf("expected 1 evidence after 100 replays, got %d", evidenceCount)
	}
}
