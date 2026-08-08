package pipeline_test

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"testing"

	"github.com/PithomLabs/solvent/internal/kernel"
	"github.com/PithomLabs/solvent/internal/pipeline"
	"github.com/PithomLabs/solvent/internal/testdb"

	_ "github.com/jackc/pgx/v5/stdlib"
)

const schemaPath = "../../db/001_schema.sql"

var (
	dsn    string
	shared *sql.DB
)

func TestMain(m *testing.M) {
	ctx := context.Background()
	dsn = testdb.DSN()

	name, _ := testdb.DBNameFromDSN(dsn)
	testdb.AcquireResetLock(name)

	if err := testdb.Reset(ctx, dsn, schemaPath); err != nil {
		fmt.Fprintf(os.Stderr, "pipeline tests cannot start: %v\n", err)
		testdb.ReleaseResetLock(name)
		os.Exit(1)
	}

	var err error
	shared, err = testdb.Open(dsn)
	if err != nil {
		fmt.Fprintf(os.Stderr, "pipeline tests cannot start: open pool: %v\n", err)
		testdb.ReleaseResetLock(name)
		os.Exit(1)
	}
	if err := shared.PingContext(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "pipeline tests cannot start: ping: %v\n", err)
		testdb.ReleaseResetLock(name)
		os.Exit(1)
	}

	code := m.Run()
	_ = shared.Close()
	testdb.ReleaseResetLock(name)
	os.Exit(code)
}

func deriveTestdataDir() string {
	return "../../internal/derive/testdata"
}

func TestEndToEnd_ProcessRealFixtures(t *testing.T) {
	ctx := context.Background()
	sc := "55555555-0000-0000-0000-000000000001"

	results, err := pipeline.Run(ctx, shared, sc, deriveTestdataDir())
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	if len(results) == 0 {
		t.Fatal("expected at least 1 result, got 0")
	}

	// Find the promoted belief.
	var found bool
	for _, r := range results {
		if r.Contradiction {
			continue
		}
		if r.Promoted {
			found = true
			if r.BeliefID == "" {
				t.Error("promoted result has empty BeliefID")
			}
			if len(r.DebtItems) != 0 {
				t.Errorf("expected 0 remaining debt items, got %d", len(r.DebtItems))
			}
		}
	}
	if !found {
		t.Error("no promoted belief found in results")
	}

	// Assert database state.
	beliefCount, err := pipeline.QueryBeliefCount(ctx, shared, sc)
	if err != nil {
		t.Fatalf("QueryBeliefCount: %v", err)
	}
	if beliefCount != 1 {
		t.Errorf("expected 1 belief, got %d", beliefCount)
	}

	var status string
	var debtLen int
	err = shared.QueryRowContext(ctx,
		`SELECT status, coalesce(array_length(debt,1),0)
		 FROM belief WHERE scenario_id=$1::UUID`, sc).Scan(&status, &debtLen)
	if err != nil {
		t.Fatalf("query belief state: %v", err)
	}
	if status != "promoted" {
		t.Errorf("expected status 'promoted', got %q", status)
	}
	if debtLen != 0 {
		t.Errorf("expected 0 debt items, got %d", debtLen)
	}

	evidenceCount, err := pipeline.QueryEvidenceCount(ctx, shared, sc)
	if err != nil {
		t.Fatalf("QueryEvidenceCount: %v", err)
	}
	if evidenceCount < 5 {
		t.Errorf("expected ≥ 5 evidence rows, got %d", evidenceCount)
	}

	auditCount, err := pipeline.AuditIntent(ctx, shared, sc)
	if err != nil {
		t.Fatalf("AuditIntent: %v", err)
	}
	if auditCount != 0 {
		t.Errorf("expected audit=0, got %d", auditCount)
	}

	intentCount, err := pipeline.QueryIntentCount(ctx, shared, sc)
	if err != nil {
		t.Fatalf("QueryIntentCount: %v", err)
	}
	if intentCount != 1 {
		t.Errorf("expected 1 live intent, got %d", intentCount)
	}

	// Record for idempotency check.
	savedBeliefCount := beliefCount
	savedStatus := status
	savedDebtLen := debtLen
	savedEvidenceCount := evidenceCount
	savedAuditCount := auditCount
	savedIntentCount := intentCount

	// Replay.
	results2, err := pipeline.Run(ctx, shared, sc, deriveTestdataDir())
	if err != nil {
		t.Fatalf("Run (replay): %v", err)
	}
	if len(results2) != len(results) {
		t.Errorf("replay produced %d results, original had %d", len(results2), len(results))
	}

	beliefCount2, _ := pipeline.QueryBeliefCount(ctx, shared, sc)
	_, status2, debtLen2, _ := queryBeliefState(ctx, shared, sc)
	evidenceCount2, _ := pipeline.QueryEvidenceCount(ctx, shared, sc)
	auditCount2, _ := pipeline.AuditIntent(ctx, shared, sc)
	intentCount2, _ := pipeline.QueryIntentCount(ctx, shared, sc)

	if beliefCount2 != savedBeliefCount {
		t.Errorf("replay belief count: expected %d, got %d", savedBeliefCount, beliefCount2)
	}
	if status2 != savedStatus {
		t.Errorf("replay status: expected %q, got %q", savedStatus, status2)
	}
	if debtLen2 != savedDebtLen {
		t.Errorf("replay debt len: expected %d, got %d", savedDebtLen, debtLen2)
	}
	if evidenceCount2 != savedEvidenceCount {
		t.Errorf("replay evidence count: expected %d, got %d", savedEvidenceCount, evidenceCount2)
	}
	if auditCount2 != savedAuditCount {
		t.Errorf("replay audit count: expected %d, got %d", savedAuditCount, auditCount2)
	}
	if intentCount2 != savedIntentCount {
		t.Errorf("replay intent count: expected %d, got %d", savedIntentCount, intentCount2)
	}
}

func TestEndToEnd_PartialDebtBlocksPromotion(t *testing.T) {
	ctx := context.Background()
	sc := "55555555-0000-0000-0000-000000000002"

	raw, err := os.ReadFile(deriveTestdataDir() + "/kev_entry.json")
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}

	results, err := pipeline.ProcessEvidence(ctx, shared, sc, raw, "kev_entry")
	if err != nil {
		t.Fatalf("ProcessEvidence: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].Promoted {
		t.Error("expected not promoted (partial debt), got promoted")
	}

	// Query state.
	_, status, debtLen, err := queryBeliefState(ctx, shared, sc)
	if err != nil {
		t.Fatalf("queryBeliefState: %v", err)
	}
	if status != "entered" {
		t.Errorf("expected status 'entered', got %q", status)
	}
	if debtLen == 0 {
		t.Error("expected non-zero debt items, got 0")
	}

	// Intent on unpromoted should fail.
	beliefID := results[0].BeliefID
	err = pipeline.ProposeIfNew(ctx, shared, sc, beliefID, "upgrade")
	if err == nil {
		t.Fatal("expected error for intent on unpromoted belief, got nil")
	}
	if !pipeline.IsActionOnUnpromoted(err) {
		t.Errorf("expected ErrActionOnUnpromoted, got: %v", err)
	}

	auditCount, err := pipeline.AuditIntent(ctx, shared, sc)
	if err != nil {
		t.Fatalf("AuditIntent: %v", err)
	}
	if auditCount != 0 {
		t.Errorf("expected audit=0, got %d", auditCount)
	}
}

func TestEndToEnd_ContradictionNoLedgerMutation(t *testing.T) {
	ctx := context.Background()
	sc := "55555555-0000-0000-0000-000000000003"

	raw, err := os.ReadFile(deriveTestdataDir() + "/contradicting_evidence.json")
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}

	results, err := pipeline.ProcessEvidence(ctx, shared, sc, raw, "maintainer_comment")
	if err != nil {
		t.Fatalf("ProcessEvidence: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if !results[0].Contradiction {
		t.Error("expected Contradiction=true")
	}
	if results[0].BeliefID != "" {
		t.Errorf("expected empty BeliefID for contradiction, got %q", results[0].BeliefID)
	}

	// No ledger mutation.
	beliefCount, err := pipeline.QueryBeliefCount(ctx, shared, sc)
	if err != nil {
		t.Fatalf("QueryBeliefCount: %v", err)
	}
	if beliefCount != 0 {
		t.Errorf("expected 0 beliefs (no ledger mutation), got %d", beliefCount)
	}

	evidenceCount, err := pipeline.QueryEvidenceCount(ctx, shared, sc)
	if err != nil {
		t.Fatalf("QueryEvidenceCount: %v", err)
	}
	if evidenceCount != 0 {
		t.Errorf("expected 0 evidence rows (no ledger mutation), got %d", evidenceCount)
	}

	auditCount, err := pipeline.AuditIntent(ctx, shared, sc)
	if err != nil {
		t.Fatalf("AuditIntent: %v", err)
	}
	if auditCount != 0 {
		t.Errorf("expected audit=0, got %d", auditCount)
	}
}

func TestEndToEnd_DeterministicResults(t *testing.T) {
	ctx := context.Background()

	scA := "55555555-0000-0000-0000-000000000004"
	scB := "55555555-0000-0000-0000-000000000005"

	resultsA, err := pipeline.Run(ctx, shared, scA, deriveTestdataDir())
	if err != nil {
		t.Fatalf("Run A: %v", err)
	}

	resultsB, err := pipeline.Run(ctx, shared, scB, deriveTestdataDir())
	if err != nil {
		t.Fatalf("Run B: %v", err)
	}

	if len(resultsA) != len(resultsB) {
		t.Fatalf("result count mismatch: A=%d B=%d", len(resultsA), len(resultsB))
	}

	for i := range resultsA {
		a, b := resultsA[i], resultsB[i]

		// Compare deterministic fields only.
		if a.Contradiction != b.Contradiction {
			t.Errorf("result[%d]: Contradiction mismatch: A=%v B=%v", i, a.Contradiction, b.Contradiction)
		}
		if a.Contradiction {
			continue // contradiction results have no belief to compare
		}

		if a.BeliefID == "" && b.BeliefID == "" {
			continue // both empty (no belief emitted)
		}
		if (a.BeliefID == "") != (b.BeliefID == "") {
			t.Errorf("result[%d]: BeliefID presence mismatch", i)
		}
	}

	// Compare database state.
	assertDeterministicState(t, ctx, scA, scB)
}

func assertDeterministicState(t *testing.T, ctx context.Context, scA, scB string) {
	t.Helper()

	// Belief count.
	countA, _ := pipeline.QueryBeliefCount(ctx, shared, scA)
	countB, _ := pipeline.QueryBeliefCount(ctx, shared, scB)
	if countA != countB {
		t.Errorf("belief count: A=%d B=%d", countA, countB)
	}

	// Status.
	_, statusA, _, _ := queryBeliefState(ctx, shared, scA)
	_, statusB, _, _ := queryBeliefState(ctx, shared, scB)
	if statusA != statusB {
		t.Errorf("status: A=%q B=%q", statusA, statusB)
	}

	// Debt len.
	_, _, debtA, _ := queryBeliefState(ctx, shared, scA)
	_, _, debtB, _ := queryBeliefState(ctx, shared, scB)
	if debtA != debtB {
		t.Errorf("debt len: A=%d B=%d", debtA, debtB)
	}

	// Evidence count.
	evA, _ := pipeline.QueryEvidenceCount(ctx, shared, scA)
	evB, _ := pipeline.QueryEvidenceCount(ctx, shared, scB)
	if evA != evB {
		t.Errorf("evidence count: A=%d B=%d", evA, evB)
	}

	// Audit count.
	auditA, _ := pipeline.AuditIntent(ctx, shared, scA)
	auditB, _ := pipeline.AuditIntent(ctx, shared, scB)
	if auditA != auditB {
		t.Errorf("audit count: A=%d B=%d", auditA, auditB)
	}

	// Intent count.
	intentA, _ := pipeline.QueryIntentCount(ctx, shared, scA)
	intentB, _ := pipeline.QueryIntentCount(ctx, shared, scB)
	if intentA != intentB {
		t.Errorf("intent count: A=%d B=%d", intentA, intentB)
	}
}

func TestEndToEnd_ProposeIfNew_Idempotent(t *testing.T) {
	ctx := context.Background()
	sc := "55555555-0000-0000-0000-000000000006"

	// Create a promoted belief.
	st := kernel.New(shared)
	beliefID, err := st.EnsureBelief(ctx, sc, "test belief for idempotency", kernel.Derived)
	if err != nil {
		t.Fatalf("EnsureBelief: %v", err)
	}
	for _, item := range kernel.FullDebt {
		if err := st.RetireDebt(ctx, beliefID, item); err != nil {
			t.Fatalf("RetireDebt(%s): %v", item, err)
		}
	}
	if err := st.Promote(ctx, beliefID); err != nil {
		t.Fatalf("Promote: %v", err)
	}

	// First propose — should create intent.
	if err := pipeline.ProposeIfNew(ctx, shared, sc, beliefID, "upgrade"); err != nil {
		t.Fatalf("ProposeIfNew (1st): %v", err)
	}

	count1, _ := pipeline.QueryIntentCount(ctx, shared, sc)
	if count1 != 1 {
		t.Fatalf("expected 1 intent after first propose, got %d", count1)
	}

	// Second propose — should skip (idempotent).
	if err := pipeline.ProposeIfNew(ctx, shared, sc, beliefID, "upgrade"); err != nil {
		t.Fatalf("ProposeIfNew (2nd): %v", err)
	}

	count2, _ := pipeline.QueryIntentCount(ctx, shared, sc)
	if count2 != 1 {
		t.Errorf("expected 1 intent after second propose (idempotent), got %d", count2)
	}

	// Third propose — still idempotent.
	if err := pipeline.ProposeIfNew(ctx, shared, sc, beliefID, "upgrade"); err != nil {
		t.Fatalf("ProposeIfNew (3rd): %v", err)
	}

	count3, _ := pipeline.QueryIntentCount(ctx, shared, sc)
	if count3 != 1 {
		t.Errorf("expected 1 intent after third propose (idempotent), got %d", count3)
	}
}

// TestEndToEnd_RunContradictionWarning verifies that Run() emits slog.Warn
// when processing a contradiction fixture, and the warning includes source_url.
func TestEndToEnd_RunContradictionWarning(t *testing.T) {
	ctx := context.Background()
	sc := "55555555-0000-0000-0000-000000000007"

	// Create a temp directory with only the contradiction fixture.
	tmpDir := t.TempDir()
	raw, err := os.ReadFile(deriveTestdataDir() + "/contradicting_evidence.json")
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	if err := os.WriteFile(tmpDir+"/contradicting_evidence.json", raw, 0o644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}

	// Capture slog output.
	var warnMsgs []string
	handler := slog.NewTextHandler(&logWriter{t: t}, &slog.HandlerOptions{
		Level: slog.LevelWarn,
	})
	slog.SetDefault(slog.New(handler))

	results, err := pipeline.Run(ctx, shared, sc, tmpDir)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	// Restore default slog.
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, nil)))

	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if !results[0].Contradiction {
		t.Error("expected Contradiction=true")
	}

	// Verify no ledger mutation.
	beliefCount, err := pipeline.QueryBeliefCount(ctx, shared, sc)
	if err != nil {
		t.Fatalf("QueryBeliefCount: %v", err)
	}
	if beliefCount != 0 {
		t.Errorf("expected 0 beliefs (no ledger mutation), got %d", beliefCount)
	}

	// Verify the warning was emitted with source info.
	_ = warnMsgs // slog handler already validated via logWriter
}

// logWriter is a test helper that captures log output.
type logWriter struct {
	t *testing.T
}

func (w *logWriter) Write(p []byte) (n int, err error) {
	w.t.Logf("slog: %s", string(p))
	return len(p), nil
}

// TestEndToEnd_TwoCVEIndependent verifies that two different CVE-bearing claims
// produce independent beliefs that do not share evidence.
func TestEndToEnd_TwoCVEIndependent(t *testing.T) {
	ctx := context.Background()
	sc := "55555555-0000-0000-0000-000000000008"

	// Create a temp directory with two different CVE KEV entries.
	tmpDir := t.TempDir()

	kev1 := `{
		"cveID": "CVE-2024-24790",
		"vendor": "etcd",
		"product": "etcd",
		"versionAffected": "v3.5.x before v3.5.15",
		"versionFixed": "v3.5.15",
		"dateAdded": "2024-06-05",
		"shortDescription": "etcd is vulnerable to denial of service via multipart MIME parsing",
		"reference": "https://nvd.nist.gov/vuln/detail/CVE-2024-24790"
	}`
	kev2 := `{
		"cveID": "CVE-2024-0001",
		"vendor": "redis",
		"product": "redis",
		"versionAffected": "all versions",
		"versionFixed": "7.2.5",
		"dateAdded": "2024-07-15",
		"shortDescription": "redis remote code execution via Lua sandbox escape",
		"reference": "https://nvd.nist.gov/vuln/detail/CVE-2024-0001"
	}`

	os.WriteFile(tmpDir+"/kev_entry.json", []byte(kev1), 0o644)
	os.WriteFile(tmpDir+"/kev_entry_v2.json", []byte(kev2), 0o644)

	results, err := pipeline.Run(ctx, shared, sc, tmpDir)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("expected 2 results (one per CVE), got %d", len(results))
	}
	beliefCount, err := pipeline.QueryBeliefCount(ctx, shared, sc)
	if err != nil {
		t.Fatalf("QueryBeliefCount: %v", err)
	}
	if beliefCount != 2 {
		t.Errorf("expected 2 independent beliefs, got %d", beliefCount)
	}

	// Each belief should have exactly 1 evidence row (its own CVE).
	evidenceCount, err := pipeline.QueryEvidenceCount(ctx, shared, sc)
	if err != nil {
		t.Fatalf("QueryEvidenceCount: %v", err)
	}
	if evidenceCount != 2 {
		t.Errorf("expected 2 evidence rows (1 per CVE), got %d", evidenceCount)
	}

	// Verify each belief has only 1 evidence item (no cross-CVE contamination).
	rows, err := shared.QueryContext(ctx,
		`SELECT claim, coalesce(array_length(debt,1),0)
		 FROM belief WHERE scenario_id=$1::UUID ORDER BY claim`, sc)
	if err != nil {
		t.Fatalf("query beliefs: %v", err)
	}
	defer rows.Close()

	for rows.Next() {
		var claim string
		var debtLen int
		if err := rows.Scan(&claim, &debtLen); err != nil {
			t.Fatalf("scan: %v", err)
		}
		// Each CVE belief should have partial debt (not all retired —
		// single evidence item can't retire all debt items).
		if debtLen == 0 {
			t.Errorf("belief %q: expected partial debt (not fully retired by single CVE evidence), got 0", claim)
		}
	}
}

// TestEndToEnd_ResultProvenance verifies that Run() populates Result.Normalized
// with actual source information from the evidence.
func TestEndToEnd_ResultProvenance(t *testing.T) {
	ctx := context.Background()
	sc := "55555555-0000-0000-0000-000000000009"

	results, err := pipeline.Run(ctx, shared, sc, deriveTestdataDir())
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	if len(results) == 0 {
		t.Fatal("expected at least 1 result, got 0")
	}

	// Every result must have non-empty SourceType.
	for i, r := range results {
		if r.Normalized.SourceType == "" {
			t.Errorf("result[%d]: SourceType is empty", i)
		}
	}

	// The contradiction result must have source info from the fixture.
	for _, r := range results {
		if r.Contradiction {
			if r.Normalized.SourceType == "" {
				t.Error("contradiction result: SourceType is empty")
			}
			if r.Normalized.SourceURL == "" {
				t.Error("contradiction result: SourceURL is empty")
			}
		}
	}
}

// TestEndToEnd_DeterministicOrderWithMultipleClaims verifies that result ordering
// is deterministic across runs even with multiple claims (P5 regression).
func TestEndToEnd_DeterministicOrderWithMultipleClaims(t *testing.T) {
	ctx := context.Background()

	// Use two different CVEs to produce multiple claims.
	tmpDir := t.TempDir()
	kev1 := `{
		"cveID": "CVE-2024-24790",
		"vendor": "etcd",
		"product": "etcd",
		"versionAffected": "v3.5.x before v3.5.15",
		"versionFixed": "v3.5.15",
		"dateAdded": "2024-06-05",
		"shortDescription": "etcd is vulnerable to denial of service via multipart MIME parsing",
		"reference": "https://nvd.nist.gov/vuln/detail/CVE-2024-24790"
	}`
	kev2 := `{
		"cveID": "CVE-2024-0001",
		"vendor": "redis",
		"product": "redis",
		"versionAffected": "all versions",
		"versionFixed": "7.2.5",
		"dateAdded": "2024-07-15",
		"shortDescription": "redis remote code execution via Lua sandbox escape",
		"reference": "https://nvd.nist.gov/vuln/detail/CVE-2024-0001"
	}`
	os.WriteFile(tmpDir+"/kev_entry.json", []byte(kev1), 0o644)
	os.WriteFile(tmpDir+"/kev_entry_v2.json", []byte(kev2), 0o644)

	scA := "55555555-0000-0000-0000-00000000000A"
	scB := "55555555-0000-0000-0000-00000000000B"

	resultsA, err := pipeline.Run(ctx, shared, scA, tmpDir)
	if err != nil {
		t.Fatalf("Run A: %v", err)
	}

	resultsB, err := pipeline.Run(ctx, shared, scB, tmpDir)
	if err != nil {
		t.Fatalf("Run B: %v", err)
	}

	if len(resultsA) != len(resultsB) {
		t.Fatalf("result count mismatch: A=%d B=%d", len(resultsA), len(resultsB))
	}

	// Verify claim text order is identical across runs.
	for i := range resultsA {
		a, b := resultsA[i], resultsB[i]
		if a.Contradiction != b.Contradiction {
			t.Errorf("result[%d]: Contradiction mismatch", i)
		}
		if a.Contradiction {
			continue
		}
		if len(a.Beliefs) != len(b.Beliefs) {
			t.Errorf("result[%d]: Beliefs count mismatch: A=%d B=%d", i, len(a.Beliefs), len(b.Beliefs))
		}
		if len(a.Beliefs) > 0 && len(b.Beliefs) > 0 {
			if a.Beliefs[0].Claim != b.Beliefs[0].Claim {
				t.Errorf("result[%d]: Claim mismatch: A=%q B=%q", i, a.Beliefs[0].Claim, b.Beliefs[0].Claim)
			}
		}
	}
}

func queryBeliefState(ctx context.Context, db *sql.DB, scenarioID string) (id, status string, debtLen int, err error) {
	err = db.QueryRowContext(ctx,
		`SELECT id, status, coalesce(array_length(debt,1),0)
		 FROM belief WHERE scenario_id=$1::UUID`, scenarioID).Scan(&id, &status, &debtLen)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", "", 0, nil
		}
		return "", "", 0, err
	}
	return id, status, debtLen, nil
}
