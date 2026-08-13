package corpus_test

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"strings"
	"testing"

	_ "github.com/jackc/pgx/v5/stdlib" // D-019: pgx/v5 is the sole approved driver

	"github.com/PithomLabs/solvent/internal/corpus"
	"github.com/PithomLabs/solvent/internal/testdb"
	"github.com/PithomLabs/solvent/kernel"
)

// 002 carries the corpus layer and references belief(id), so 001 is applied first.
var schemaPaths = []string{"../../db/001_schema.sql", "../../db/002_corpus.sql"}

// Scenario namespace for this package. 1111..6666 belong to the kernel, belief,
// intent and pipeline suites; 7777 is the kernel's lifecycle example.
const (
	scenarioA = "88888888-8888-8888-8888-888888888888" // schema-level cases
	scenarioB = "99999999-9999-9999-9999-999999999999" // isolation control
	scenarioC = "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa" // retrieval corpus, kept clean
)

var shared *sql.DB

func TestMain(m *testing.M) {
	ctx := context.Background()
	dsn := testdb.DSN()

	name, _ := testdb.DBNameFromDSN(dsn)
	testdb.AcquireResetLock(name)

	if err := testdb.Reset(ctx, dsn, schemaPaths...); err != nil {
		fmt.Fprintf(os.Stderr, "corpus suite cannot start: %v\n", err)
		testdb.ReleaseResetLock(name)
		os.Exit(1)
	}

	var err error
	shared, err = testdb.Open(dsn)
	if err != nil {
		fmt.Fprintf(os.Stderr, "corpus suite cannot start: open pool: %v\n", err)
		testdb.ReleaseResetLock(name)
		os.Exit(1)
	}
	if err := shared.PingContext(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "corpus suite cannot start: ping: %v\n", err)
		testdb.ReleaseResetLock(name)
		os.Exit(1)
	}

	code := m.Run()
	_ = shared.Close()
	testdb.ReleaseResetLock(name)
	os.Exit(code)
}

// --- C-01: the reason this layer exists ---------------------------------------

// A corpus row attaches to no belief. evidence.belief_id is NOT NULL, so the
// ledger structurally cannot hold a document nobody has formed a claim about yet.
func TestC01_CorpusAcceptsUnattachedEvidence(t *testing.T) {
	ctx := context.Background()

	id, inserted, err := corpus.Insert(ctx, shared, corpus.Issue{
		ScenarioID:    scenarioA,
		IssueNumber:   1,
		Title:         "etcd server panics on restart after snapshot",
		Body:          "observed during rolling upgrade",
		State:         "closed",
		URL:           "https://github.com/etcd-io/etcd/issues/1",
		ContentSHA256: "c01",
	})
	if err != nil || !inserted || id == "" {
		t.Fatalf("insert unattached corpus row: id=%q inserted=%v err=%v", id, inserted, err)
	}

	// It is genuinely unattached: no belief exists in this scenario at all.
	var beliefs int
	if err := shared.QueryRowContext(ctx,
		`SELECT count(*) FROM belief WHERE scenario_id = $1::UUID`, scenarioA).Scan(&beliefs); err != nil {
		t.Fatal(err)
	}
	if beliefs != 0 {
		t.Fatalf("expected 0 beliefs in the scenario, got %d", beliefs)
	}

	// And the control: the same document cannot be stored as evidence.
	_, err = shared.ExecContext(ctx,
		`INSERT INTO evidence (scenario_id, belief_id, provenance_class, content_sha256)
		 VALUES ($1::UUID, NULL, 'external_feed', 'c01')`, scenarioA)
	if err == nil {
		t.Fatal("evidence accepted a NULL belief_id; the premise of the corpus layer is void")
	}
	t.Logf("evidence correctly refused an unattached row: %v", err)
}

// --- C-02: the VECTOR column -------------------------------------------------

func TestC02_VectorRoundTripAndWidthEnforced(t *testing.T) {
	ctx := context.Background()

	const title = "vector round trip probe"
	vec := corpus.HashEmbed(title)
	id, _, err := corpus.Insert(ctx, shared, corpus.Issue{
		ScenarioID: scenarioA, IssueNumber: 2, Title: title,
		State: "closed", URL: "https://x/2", ContentSHA256: "c02", Embedding: vec,
	})
	if err != nil {
		t.Fatal(err)
	}

	var lit string
	if err := shared.QueryRowContext(ctx,
		`SELECT embedding::STRING FROM corpus_issue WHERE id = $1::UUID`, id).Scan(&lit); err != nil {
		t.Fatal(err)
	}
	got, err := corpus.Decode(lit)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != corpus.Dim {
		t.Fatalf("round-tripped %d dimensions, want %d", len(got), corpus.Dim)
	}

	// The database enforces the width, not just Go.
	_, err = shared.ExecContext(ctx,
		`UPDATE corpus_issue SET embedding = '[1,2,3]'::VECTOR(3) WHERE id = $1::UUID`, id)
	if err == nil {
		t.Fatal("a 3-dimension vector was accepted into a VECTOR(1024) column")
	}
	if !strings.Contains(err.Error(), "1024") {
		t.Fatalf("expected a dimension-mismatch refusal, got: %v", err)
	}
	t.Logf("width enforced by the database: %v", err)
}

// --- C-03: deterministic corpus identity -------------------------------------

// Re-ingesting the same issue is a no-op. This is what makes Phase 3's repeated
// ingestion safe to run without producing duplicates.
func TestC03_ReingestionIsIdempotent(t *testing.T) {
	ctx := context.Background()
	iss := corpus.Issue{
		ScenarioID: scenarioA, IssueNumber: 3, Title: "duplicate probe",
		State: "open", URL: "https://x/3", ContentSHA256: "c03",
	}

	id1, ins1, err := corpus.Insert(ctx, shared, iss)
	if err != nil || !ins1 {
		t.Fatalf("first insert: id=%q inserted=%v err=%v", id1, ins1, err)
	}
	_, ins2, err := corpus.Insert(ctx, shared, iss)
	if err != nil {
		t.Fatalf("re-insert returned an error instead of a no-op: %v", err)
	}
	if ins2 {
		t.Fatal("re-insert reported a new row; ingestion is not idempotent")
	}

	var n int
	if err := shared.QueryRowContext(ctx,
		`SELECT count(*) FROM corpus_issue WHERE scenario_id = $1::UUID AND issue_number = 3`,
		scenarioA).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Fatalf("issue 3 present %d times, want exactly 1", n)
	}
}

// --- C-04 / C-05: ANN retrieval, ordering, and index usage --------------------

// seedCorpus fills a scenario with bulk rows plus planted documents.
//
// Rows are inserted one at a time. CockroachDB's documentation is explicit that
// large batch inserts of VECTOR values degrade badly and should be avoided; this
// is also the shape Phase 3 will ingest 7,238 rows with.
func seedCorpus(t *testing.T, scenario string, n int) {
	t.Helper()
	ctx := context.Background()
	for i := 100; i < 100+n; i++ {
		title := fmt.Sprintf("routine maintenance chore number %d gamma delta", i)
		if _, _, err := corpus.Insert(ctx, shared, corpus.Issue{
			ScenarioID: scenario, IssueNumber: i, Title: title, State: "closed",
			URL:           fmt.Sprintf("https://github.com/etcd-io/etcd/issues/%d", i),
			ContentSHA256: fmt.Sprintf("bulk-%d", i),
			Embedding:     corpus.HashEmbed(title),
		}); err != nil {
			t.Fatalf("seed %d: %v", i, err)
		}
	}
}

const plantedQuery = "raft leader election timeout during network partition"

var planted = []struct {
	num   int
	title string
}{
	{9001, "raft leader election timeout during network partition"},
	{9002, "raft leader election flapping under partition"},
	{9003, "leader election timeout misconfigured in raft config"},
}

func TestC04_ANNReturnsPlantedRowsInDistanceOrder(t *testing.T) {
	ctx := context.Background()
	seedCorpus(t, scenarioC, 500)

	for _, p := range planted {
		if _, _, err := corpus.Insert(ctx, shared, corpus.Issue{
			ScenarioID: scenarioC, IssueNumber: p.num, Title: p.title, State: "closed",
			URL:           fmt.Sprintf("https://github.com/etcd-io/etcd/issues/%d", p.num),
			ContentSHA256: fmt.Sprintf("planted-%d", p.num),
			Embedding:     corpus.HashEmbed(p.title),
		}); err != nil {
			t.Fatalf("plant %d: %v", p.num, err)
		}
	}

	hits, err := corpus.Search(ctx, shared, scenarioC, corpus.HashEmbed(plantedQuery), 3)
	if err != nil {
		t.Fatal(err)
	}
	if len(hits) != 3 {
		t.Fatalf("got %d hits, want 3", len(hits))
	}

	// Correctness, not arity: every planted document must rank ahead of all 500
	// unrelated ones. That is the claim ANN retrieval is making; the exact order
	// among the planted three is not something this fixture guarantees.
	found := map[int]bool{}
	for i, h := range hits {
		if h.IssueNumber >= 9001 {
			found[h.IssueNumber] = true
			continue
		}
		t.Errorf("bulk issue %d ranked at position %d, ahead of a planted document", h.IssueNumber, i)
	}
	for _, p := range planted {
		if !found[p.num] {
			t.Errorf("planted issue %d (%q) missing from the top %d", p.num, p.title, len(hits))
		}
	}
	if hits[0].IssueNumber != 9001 {
		t.Errorf("nearest hit is %d, want 9001 (the exact-text match)", hits[0].IssueNumber)
	}
	if hits[0].Distance != 0 {
		t.Errorf("exact-text match has distance %v, want 0", hits[0].Distance)
	}

	// Distances must be non-decreasing.
	for i := 1; i < len(hits); i++ {
		if hits[i].Distance < hits[i-1].Distance {
			t.Errorf("distances out of order at %d: %v < %v", i, hits[i].Distance, hits[i-1].Distance)
		}
	}
	for _, h := range hits {
		t.Logf("hit #%d  distance=%.6f  %s", h.IssueNumber, h.Distance, h.Title)
	}
}

// The vector index is only used when every prefix column is pinned. This asserts
// both halves of that contract, because an ANN query that silently degrades to a
// full scan still returns correct rows and would otherwise pass unnoticed.
func TestC05_VectorIndexIsUsedOnlyWithScenarioPinned(t *testing.T) {
	ctx := context.Background()
	lit, err := corpus.Encode(corpus.HashEmbed(plantedQuery))
	if err != nil {
		t.Fatal(err)
	}

	explain := func(q string, args ...any) string {
		rows, err := shared.QueryContext(ctx, q, args...)
		if err != nil {
			t.Fatal(err)
		}
		defer func() { _ = rows.Close() }()
		var b strings.Builder
		for rows.Next() {
			var line string
			if err := rows.Scan(&line); err != nil {
				t.Fatal(err)
			}
			b.WriteString(line)
			b.WriteString("\n")
		}
		return b.String()
	}

	withScenario := explain(`EXPLAIN SELECT id FROM corpus_issue
		WHERE scenario_id = $1::UUID ORDER BY embedding <=> $2::VECTOR LIMIT 5`, scenarioC, lit)
	if !strings.Contains(withScenario, "vector search") ||
		!strings.Contains(withScenario, "corpus_issue_embedding_idx") {
		t.Errorf("scenario-pinned ANN query did not use the vector index:\n%s", withScenario)
	}
	t.Logf("scenario-pinned plan:\n%s", withScenario)

	withoutScenario := explain(`EXPLAIN SELECT id FROM corpus_issue
		ORDER BY embedding <=> $1::VECTOR LIMIT 5`, lit)
	if strings.Contains(withoutScenario, "vector search") {
		t.Errorf("unpinned query used the vector index; the prefix-column contract does not hold:\n%s", withoutScenario)
	}
	t.Logf("unpinned plan:\n%s", withoutScenario)
}

// --- C-06: scenario isolation -------------------------------------------------

func TestC06_ScenarioIsolation(t *testing.T) {
	ctx := context.Background()

	// An exact duplicate of the nearest planted document, in a different scenario.
	if _, _, err := corpus.Insert(ctx, shared, corpus.Issue{
		ScenarioID: scenarioB, IssueNumber: 9001, Title: planted[0].title, State: "closed",
		URL: "https://x/b-9001", ContentSHA256: "iso-b", Embedding: corpus.HashEmbed(planted[0].title),
	}); err != nil {
		t.Fatal(err)
	}

	hits, err := corpus.Search(ctx, shared, scenarioB, corpus.HashEmbed(plantedQuery), 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(hits) != 1 {
		t.Fatalf("scenario B returned %d hits, want exactly its own 1", len(hits))
	}

	// The same issue_number exists in both scenarios, which is only possible
	// because identity is (scenario_id, issue_number) rather than issue_number.
	var n int
	if err := shared.QueryRowContext(ctx,
		`SELECT count(*) FROM corpus_issue WHERE issue_number = 9001`).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 2 {
		t.Fatalf("issue 9001 present %d times across scenarios, want 2", n)
	}
}

// --- C-07: citation links corpus to ledger ------------------------------------

func TestC07_CitationConnectsCorpusToBelief(t *testing.T) {
	ctx := context.Background()
	st := kernel.New(shared)

	beliefID, err := st.EnterBelief(ctx, scenarioA,
		"etcd leader election is unstable under partition", kernel.Derived)
	if err != nil {
		t.Fatal(err)
	}

	hits, err := corpus.Search(ctx, shared, scenarioC, corpus.HashEmbed(plantedQuery), 3)
	if err != nil {
		t.Fatal(err)
	}
	if len(hits) == 0 {
		t.Fatal("no hits to cite")
	}

	for _, h := range hits {
		if err := corpus.Cite(ctx, shared, beliefID, h.ID, h.Distance, plantedQuery); err != nil {
			t.Fatal(err)
		}
	}

	// The provenance is queryable: which corpus rows, how close, and to what question.
	rows, err := shared.QueryContext(ctx, `
		SELECT c.issue_number, bc.distance, bc.query_text
		FROM belief_corpus_citation bc
		JOIN corpus_issue c ON c.id = bc.corpus_id
		WHERE bc.belief_id = $1::UUID
		ORDER BY bc.distance`, beliefID)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = rows.Close() }()

	var cited int
	for rows.Next() {
		var num int
		var dist float64
		var q string
		if err := rows.Scan(&num, &dist, &q); err != nil {
			t.Fatal(err)
		}
		if q != plantedQuery {
			t.Errorf("citation recorded query %q, want %q", q, plantedQuery)
		}
		cited++
		t.Logf("belief cites corpus issue #%d at distance %.6f", num, dist)
	}
	if cited != len(hits) {
		t.Fatalf("recorded %d citations, want %d", cited, len(hits))
	}

	// Re-citing is idempotent, so replaying retrieval cannot duplicate provenance.
	if err := corpus.Cite(ctx, shared, beliefID, hits[0].ID, 0.123, plantedQuery); err != nil {
		t.Fatal(err)
	}
	var after int
	if err := shared.QueryRowContext(ctx,
		`SELECT count(*) FROM belief_corpus_citation WHERE belief_id = $1::UUID`,
		beliefID).Scan(&after); err != nil {
		t.Fatal(err)
	}
	if after != cited {
		t.Fatalf("re-citing changed the citation count from %d to %d", cited, after)
	}
}

// The FK is real: a citation cannot name a belief that does not exist.
func TestC08_CitationRequiresARealBelief(t *testing.T) {
	ctx := context.Background()

	var corpusID string
	if err := shared.QueryRowContext(ctx,
		`SELECT id FROM corpus_issue WHERE scenario_id = $1::UUID LIMIT 1`, scenarioC).Scan(&corpusID); err != nil {
		t.Fatal(err)
	}

	err := corpus.Cite(ctx, shared, "00000000-0000-0000-0000-0000000000ff", corpusID, 0.5, "ghost")
	if err == nil {
		t.Fatal("a citation naming a non-existent belief was accepted")
	}
	t.Logf("citation FK refused a phantom belief: %v", err)
}
