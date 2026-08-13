package belief_test

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"testing"

	"github.com/PithomLabs/solvent/internal/belief"
	"github.com/PithomLabs/solvent/internal/derive"
	"github.com/PithomLabs/solvent/internal/normalize"
	"github.com/PithomLabs/solvent/internal/testdb"
	"github.com/PithomLabs/solvent/kernel"

	_ "github.com/jackc/pgx/v5/stdlib"
)

// 002 carries the corpus layer. Applied everywhere so every environment has one
// schema shape; these packages do not use the corpus tables themselves.
var schemaPaths = []string{"../../db/001_schema.sql", "../../db/002_corpus.sql"}

var (
	dsn    string
	shared *sql.DB
)

func TestMain(m *testing.M) {
	ctx := context.Background()
	dsn = testdb.DSN()

	name, _ := testdb.DBNameFromDSN(dsn)
	testdb.AcquireResetLock(name)

	if err := testdb.Reset(ctx, dsn, schemaPaths...); err != nil {
		fmt.Fprintf(os.Stderr, "belief tests cannot start: %v\n", err)
		testdb.ReleaseResetLock(name)
		os.Exit(1)
	}

	var err error
	shared, err = testdb.Open(dsn)
	if err != nil {
		fmt.Fprintf(os.Stderr, "belief tests cannot start: open pool: %v\n", err)
		testdb.ReleaseResetLock(name)
		os.Exit(1)
	}
	if err := shared.PingContext(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "belief tests cannot start: ping: %v\n", err)
		testdb.ReleaseResetLock(name)
		os.Exit(1)
	}

	code := m.Run()
	_ = shared.Close()
	testdb.ReleaseResetLock(name)
	os.Exit(code)
}

func scenario(n int) string {
	return fmt.Sprintf("22222222-0000-0000-0000-%012x", n)
}

func TestProcess_EnterNewBelief(t *testing.T) {
	ctx := context.Background()
	sc := scenario(1)

	b := derive.DerivedBelief{
		Claim:          "etcd v3.5.x is vulnerable to CVE-2024-24790",
		Classification: "derived",
		SupportingEvidence: []normalize.NormalizedEvidence{
			{
				SourceType:      "kev_entry",
				SourceURL:       "https://nvd.nist.gov/vuln/detail/CVE-2024-24790",
				ProvenanceClass: "external_feed",
				ContentSHA256:   "deadbeef01",
			},
		},
	}

	err := belief.Process(ctx, shared, sc, b)
	if err != nil {
		t.Fatalf("Process returned error: %v", err)
	}

	var count int
	err = shared.QueryRowContext(ctx,
		`SELECT count(*) FROM belief WHERE scenario_id=$1::UUID AND claim=$2`, sc, b.Claim).Scan(&count)
	if err != nil {
		t.Fatalf("query belief: %v", err)
	}
	if count != 1 {
		t.Errorf("expected 1 belief, got %d", count)
	}

	var evidenceCount int
	err = shared.QueryRowContext(ctx,
		`SELECT count(*) FROM evidence WHERE scenario_id=$1::UUID`, sc).Scan(&evidenceCount)
	if err != nil {
		t.Fatalf("query evidence: %v", err)
	}
	if evidenceCount != 1 {
		t.Errorf("expected 1 evidence row, got %d", evidenceCount)
	}
}

func TestProcess_ExistingBelief(t *testing.T) {
	ctx := context.Background()
	sc := scenario(2)
	claim := "etcd v3.5.14 is vulnerable to CVE-2024-24790"

	b1 := derive.DerivedBelief{
		Claim:          claim,
		Classification: "derived",
		SupportingEvidence: []normalize.NormalizedEvidence{
			{
				SourceType:      "kev_entry",
				SourceURL:       "https://nvd.nist.gov/vuln/detail/CVE-2024-24790",
				ProvenanceClass: "external_feed",
				ContentSHA256:   "deadbeef02",
			},
		},
	}
	if err := belief.Process(ctx, shared, sc, b1); err != nil {
		t.Fatalf("first Process: %v", err)
	}

	b2 := derive.DerivedBelief{
		Claim:          claim,
		Classification: "derived",
		SupportingEvidence: []normalize.NormalizedEvidence{
			{
				SourceType:      "release",
				SourceURL:       "https://github.com/etcd-io/etcd/releases/tag/v3.5.15",
				ProvenanceClass: "external_feed",
				ContentSHA256:   "deadbeef03",
			},
		},
	}
	if err := belief.Process(ctx, shared, sc, b2); err != nil {
		t.Fatalf("second Process: %v", err)
	}

	var beliefCount int
	_ = shared.QueryRowContext(ctx,
		`SELECT count(*) FROM belief WHERE scenario_id=$1::UUID AND claim=$2`, sc, claim).Scan(&beliefCount)

	var evidenceCount int
	_ = shared.QueryRowContext(ctx,
		`SELECT count(*) FROM evidence WHERE scenario_id=$1::UUID`, sc).Scan(&evidenceCount)

	if beliefCount != 1 {
		t.Errorf("expected 1 belief (dedup), got %d", beliefCount)
	}
	if evidenceCount != 2 {
		t.Errorf("expected 2 evidence rows, got %d", evidenceCount)
	}
}

func TestProcess_Promotion(t *testing.T) {
	ctx := context.Background()
	sc := scenario(3)

	b := derive.DerivedBelief{
		Claim:          "etcd v3.5.x is vulnerable to CVE-2024-24790",
		Classification: "derived",
		SupportingEvidence: []normalize.NormalizedEvidence{
			{
				SourceType:      "kev_entry",
				SourceURL:       "https://nvd.nist.gov/vuln/detail/CVE-2024-24790",
				ProvenanceClass: "external_feed",
				ContentSHA256:   "deadbeef04",
				Assertion:       "etcd v3.5.x is vulnerable to CVE-2024-24790",
			},
			{
				SourceType:      "release",
				SourceURL:       "https://github.com/etcd-io/etcd/releases/tag/v3.5.15",
				ProvenanceClass: "external_feed",
				ContentSHA256:   "deadbeef05",
				Assertion:       "release v3.5.15 published",
			},
			{
				SourceType:      "maintainer_comment",
				SourceURL:       "https://github.com/etcd-io/etcd/issues/17234#issuecomment-220123456",
				ProvenanceClass: "external_feed",
				ContentSHA256:   "deadbeef06",
				Assertion:       "fix released in v3.5.15",
			},
			{
				SourceType:      "maintainer_comment",
				SourceURL:       "https://github.com/etcd-io/etcd/issues/17234#issuecomment-220123457",
				ProvenanceClass: "external_feed",
				ContentSHA256:   "deadbeef07",
				Assertion:       "tested and confirmed fix",
			},
			{
				SourceType:      "maintainer_comment",
				SourceURL:       "https://github.com/etcd-io/etcd/issues/17234#issuecomment-220123458",
				ProvenanceClass: "external_feed",
				ContentSHA256:   "deadbeef08",
				Assertion:       "no regression observed",
			},
			{
				SourceType:      "maintainer_comment",
				SourceURL:       "https://github.com/etcd-io/etcd/issues/17234#issuecomment-220123459",
				ProvenanceClass: "external_feed",
				ContentSHA256:   "deadbeef09",
				Assertion:       "security review completed",
			},
		},
	}

	err := belief.Process(ctx, shared, sc, b)
	if err != nil {
		t.Fatalf("Process returned error: %v", err)
	}

	var status string
	_ = shared.QueryRowContext(ctx,
		`SELECT status FROM belief WHERE scenario_id=$1::UUID`, sc).Scan(&status)

	if status != "promoted" {
		t.Errorf("expected status 'promoted', got %q", status)
	}
}

func TestProcess_PromotionBlocked(t *testing.T) {
	ctx := context.Background()
	sc := scenario(4)

	b := derive.DerivedBelief{
		Claim:          "etcd v3.5.x is vulnerable to CVE-2024-24790",
		Classification: "derived",
		SupportingEvidence: []normalize.NormalizedEvidence{
			{
				SourceType:      "kev_entry",
				SourceURL:       "https://nvd.nist.gov/vuln/detail/CVE-2024-24790",
				ProvenanceClass: "external_feed",
				ContentSHA256:   "deadbeef10",
			},
		},
	}

	err := belief.Process(ctx, shared, sc, b)
	if err != nil {
		t.Fatalf("Process returned error: %v", err)
	}

	var status string
	_ = shared.QueryRowContext(ctx,
		`SELECT status FROM belief WHERE scenario_id=$1::UUID`, sc).Scan(&status)

	if status != "entered" {
		t.Errorf("expected status 'entered' (debt not fully retired), got %q", status)
	}
}

func TestProcess_Contradiction(t *testing.T) {
	ctx := context.Background()
	sc := scenario(5)

	b := derive.DerivedBelief{
		Claim:          "prior belief is contradicted",
		Classification: "derived",
		Contradicts: []normalize.NormalizedEvidence{
			{
				SourceType: "maintainer_comment",
				SourceURL:  "https://github.com/etcd-io/etcd/issues/17234#issuecomment-220123460",
			},
		},
	}

	err := belief.Process(ctx, shared, sc, b)
	if err != nil {
		t.Fatalf("Process returned error: %v", err)
	}

	var count int
	_ = shared.QueryRowContext(ctx,
		`SELECT count(*) FROM belief WHERE scenario_id=$1::UUID`, sc).Scan(&count)

	if count != 0 {
		t.Errorf("expected 0 beliefs (contradiction does not enter), got %d", count)
	}
}

func TestProcess_MalformedBelief(t *testing.T) {
	ctx := context.Background()
	sc := scenario(6)

	b := derive.DerivedBelief{
		Claim:          "",
		Classification: "derived",
	}

	err := belief.Process(ctx, shared, sc, b)
	if err == nil {
		t.Error("expected error for empty claim, got nil")
	}
}

func TestProcess_Determinism(t *testing.T) {
	ctx := context.Background()
	sc := scenario(7)

	b := derive.DerivedBelief{
		Claim:          "etcd v3.5.x is vulnerable to CVE-2024-24790",
		Classification: "derived",
		SupportingEvidence: []normalize.NormalizedEvidence{
			{
				SourceType:      "kev_entry",
				SourceURL:       "https://nvd.nist.gov/vuln/detail/CVE-2024-24790",
				ProvenanceClass: "external_feed",
				ContentSHA256:   "deadbeef11",
			},
		},
	}

	for i := 0; i < 100; i++ {
		if err := belief.Process(ctx, shared, sc, b); err != nil {
			t.Fatalf("iteration %d: %v", i, err)
		}
	}

	var count int
	_ = shared.QueryRowContext(ctx,
		`SELECT count(*) FROM belief WHERE scenario_id=$1::UUID AND claim=$2`, sc, b.Claim).Scan(&count)

	if count != 1 {
		t.Errorf("expected 1 belief after 100 iterations (idempotent), got %d", count)
	}
}

func TestProcess_PromoteHardFailure(t *testing.T) {
	ctx := context.Background()
	sc := scenario(8)

	// Create a separate pool and close it immediately.
	// Process will fail with a connection error (not ErrPromotionBlocked),
	// proving that non-ErrPromotionBlocked errors propagate to the caller.
	badDB, err := testdb.Open(dsn)
	if err != nil {
		t.Fatalf("open bad db: %v", err)
	}
	_ = badDB.Close()

	b := derive.DerivedBelief{
		Claim:          "belief that will fail on promote",
		Classification: "derived",
		SupportingEvidence: []normalize.NormalizedEvidence{
			{
				SourceType:      "kev_entry",
				SourceURL:       "https://nvd.nist.gov/vuln/detail/CVE-2024-24790",
				ProvenanceClass: "external_feed",
				ContentSHA256:   "deadbeef12",
			},
		},
	}

	err = belief.Process(ctx, badDB, sc, b)
	if err == nil {
		t.Fatal("expected error from Process with closed pool, got nil")
	}
	if errors.Is(err, kernel.ErrPromotionBlocked) {
		t.Errorf("error should NOT be ErrPromotionBlocked, got: %v", err)
	}
}

func TestProcess_ContradictionLogsWarning(t *testing.T) {
	ctx := context.Background()
	sc := scenario(9)

	// Capture slog output.
	var buf bytes.Buffer
	orig := slog.Default()
	slog.SetDefault(slog.New(slog.NewJSONHandler(&buf, nil)))
	defer slog.SetDefault(orig)

	b := derive.DerivedBelief{
		Claim:          "prior belief is contradicted",
		Classification: "derived",
		Contradicts: []normalize.NormalizedEvidence{
			{
				SourceType: "maintainer_comment",
				SourceURL:  "https://github.com/etcd-io/etcd/issues/17234#issuecomment-220123460",
			},
		},
	}

	err := belief.Process(ctx, shared, sc, b)
	if err != nil {
		t.Fatalf("Process returned error: %v", err)
	}

	output := buf.String()
	if output == "" {
		t.Fatal("expected slog warning output, got empty buffer")
	}
	if !bytes.Contains(buf.Bytes(), []byte("contradiction received")) {
		t.Errorf("expected warning about contradiction, got: %s", output)
	}
	if !bytes.Contains(buf.Bytes(), []byte("https://github.com/etcd-io/etcd/issues/17234#issuecomment-220123460")) {
		t.Errorf("expected source_url in log output, got: %s", output)
	}
}

func TestDebtItemsForEvidence_UnionRules(t *testing.T) {
	// Realistic fixture body from maintainer_no_regression.json:
	// "Confirmed no regression. All existing tests pass and the fix resolves the vulnerability."
	// Matches R2 (tested|confirmed) AND R3 (no regression).
	// Must return the UNION: needToyCheck, needNullModel, needObstruction.
	body := "Confirmed no regression. All existing tests pass and the fix resolves the vulnerability."
	items := belief.DebtItemsForEvidence("maintainer_comment", body)

	expected := map[string]bool{
		"needToyCheck":    false,
		"needNullModel":   false,
		"needObstruction": false,
	}
	for _, item := range items {
		if _, ok := expected[item]; !ok {
			t.Errorf("unexpected debt item: %s", item)
		} else {
			expected[item] = true
		}
	}
	for item, found := range expected {
		if !found {
			t.Errorf("missing expected debt item: %s", item)
		}
	}
	// Verify no duplicates.
	if len(items) != 3 {
		t.Errorf("expected 3 unique items, got %d: %v", len(items), items)
	}
}

func TestDebtItemsForEvidence_SingleRuleMatch(t *testing.T) {
	items := belief.DebtItemsForEvidence("kev_entry", "etcd is vulnerable to CVE-2024-24790")
	if len(items) != 1 || items[0] != "needMap" {
		t.Errorf("expected [needMap], got %v", items)
	}
}

func TestDebtItemsForEvidence_NoMatch(t *testing.T) {
	items := belief.DebtItemsForEvidence("maintainer_comment", "lgtm")
	if items != nil {
		t.Errorf("expected nil, got %v", items)
	}
}

func TestDebtItemsForEvidence_UnknownSourceType(t *testing.T) {
	items := belief.DebtItemsForEvidence("unknown_source", "anything")
	if items != nil {
		t.Errorf("expected nil, got %v", items)
	}
}
