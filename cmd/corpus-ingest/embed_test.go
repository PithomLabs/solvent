package main

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	_ "github.com/jackc/pgx/v5/stdlib" // D-019: pgx/v5 is the sole approved driver

	"github.com/PithomLabs/solvent/internal/corpus"
	"github.com/PithomLabs/solvent/internal/testdb"
)

// The corpus layer references belief(id), so the frozen DDL is applied first.
var embedSchemaPaths = []string{"../../db/001_schema.sql", "../../db/002_corpus.sql", "../../db/003_wizard.sql", "../../db/004_debt_vocabulary.sql"}

// Scenario namespace for this suite. 8888/9999/aaaa belong to internal/corpus;
// bbbb is claimed here so the two suites cannot see each other's rows.
const embedScenario = "bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb"

var embedDB *sql.DB

func TestMain(m *testing.M) {
	ctx := context.Background()
	dsn := testdb.DSN()

	name, _ := testdb.DBNameFromDSN(dsn)
	testdb.AcquireResetLock(name)

	if err := testdb.Reset(ctx, dsn, embedSchemaPaths...); err != nil {
		fmt.Fprintf(os.Stderr, "corpus-ingest suite cannot start: %v\n", err)
		testdb.ReleaseResetLock(name)
		os.Exit(1)
	}

	var err error
	embedDB, err = testdb.Open(dsn)
	if err != nil {
		fmt.Fprintf(os.Stderr, "corpus-ingest suite cannot start: open pool: %v\n", err)
		testdb.ReleaseResetLock(name)
		os.Exit(1)
	}
	if err := embedDB.PingContext(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "corpus-ingest suite cannot start: ping: %v\n", err)
		testdb.ReleaseResetLock(name)
		os.Exit(1)
	}

	code := m.Run()
	_ = embedDB.Close()
	testdb.ReleaseResetLock(name)
	os.Exit(code)
}

// --- fixtures -----------------------------------------------------------------

// fixtureVec builds a deterministic unit-ish vector of the production width.
//
// corpus.HashEmbed would be the natural fixture, but it lives in a _test.go file
// inside package corpus, which is exactly what makes it uncallable from production
// — and equally uncallable from here. A local generator is the price of that
// boundary, and a cheap one.
func fixtureVec(seed int) []float32 {
	v := make([]float32, corpus.Dim)
	for i := range v {
		v[i] = float32((i*31+seed*17)%1000) / 1000.0
	}
	return v
}

func fixtureRecord(t *testing.T, issue, seed int) checkpointRecord {
	t.Helper()
	vec := fixtureVec(seed)
	return checkpointRecord{
		IssueNumber: issue,
		Digest:      vectorDigest(vec),
		Tokens:      10 + seed,
		Vector:      vec,
	}
}

func marshalLine(t *testing.T, rec checkpointRecord) []byte {
	t.Helper()
	b, err := json.Marshal(rec)
	if err != nil {
		t.Fatalf("marshal fixture: %v", err)
	}
	return append(b, '\n')
}

// writeCheckpoint writes complete records, then appends raw trailing bytes verbatim
// so a test can reproduce an exact on-disk corruption shape.
func writeCheckpoint(t *testing.T, recs []checkpointRecord, tail []byte) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "etcd-embeddings.ndjson")
	var buf []byte
	for _, r := range recs {
		buf = append(buf, marshalLine(t, r)...)
	}
	buf = append(buf, tail...)
	if err := os.WriteFile(path, buf, 0o644); err != nil {
		t.Fatalf("write checkpoint: %v", err)
	}
	return path
}

// --- E-01: the corruption that was actually found on disk ----------------------

// A process killed mid-append left 475 complete records followed by 520 NUL bytes,
// and the reader refused the entire file — stranding every vector already paid for.
// The NUL run carries no record, so nothing is dropped; it is simply not data.
func TestE01_LoadCheckpointToleratesNULPaddedTail(t *testing.T) {
	recs := []checkpointRecord{
		fixtureRecord(t, 48, 1),
		fixtureRecord(t, 96, 2),
		fixtureRecord(t, 1366, 3),
	}
	path := writeCheckpoint(t, recs, make([]byte, 520))

	got, dropped, err := loadCheckpoint(path)
	if err != nil {
		t.Fatalf("a NUL-padded tail must not fail the load: %v", err)
	}
	if len(got) != 3 {
		t.Fatalf("records loaded = %d, want 3 (the tail is padding, not a record)", len(got))
	}
	if dropped != 0 {
		t.Errorf("dropped = %d, want 0: NUL padding holds no record to drop", dropped)
	}
	for _, r := range recs {
		if _, ok := got[r.IssueNumber]; !ok {
			t.Errorf("issue %d missing from the loaded checkpoint", r.IssueNumber)
		}
	}
}

// --- E-02: a genuinely half-written final line ---------------------------------

// The other crash shape: the process died partway through writing the JSON itself.
// One vector is lost and must be regenerated, which is what the drop count reports.
func TestE02_LoadCheckpointToleratesTruncatedFinalLine(t *testing.T) {
	recs := []checkpointRecord{fixtureRecord(t, 48, 1), fixtureRecord(t, 96, 2)}
	partial := marshalLine(t, fixtureRecord(t, 1366, 3))
	path := writeCheckpoint(t, recs, partial[:len(partial)/2])

	got, dropped, err := loadCheckpoint(path)
	if err != nil {
		t.Fatalf("a truncated final line must not fail the load: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("records loaded = %d, want 2", len(got))
	}
	if dropped != 1 {
		t.Errorf("dropped = %d, want 1: the half-written record must be reported, not hidden", dropped)
	}
	if _, ok := got[1366]; ok {
		t.Error("the truncated record was loaded; it must be regenerated instead")
	}
}

// --- E-03: interior corruption is a different fact -----------------------------

// A bad line in the middle means the file was damaged after the fact, not merely
// cut short. Skipping it would quietly under-populate every target database while
// still reporting success, so it stays a hard error.
func TestE03_LoadCheckpointRejectsInteriorCorruption(t *testing.T) {
	var buf []byte
	buf = append(buf, marshalLine(t, fixtureRecord(t, 48, 1))...)
	buf = append(buf, []byte("{not json at all}\n")...)
	buf = append(buf, marshalLine(t, fixtureRecord(t, 96, 2))...)

	path := filepath.Join(t.TempDir(), "etcd-embeddings.ndjson")
	if err := os.WriteFile(path, buf, 0o644); err != nil {
		t.Fatalf("write checkpoint: %v", err)
	}

	_, _, err := loadCheckpoint(path)
	if err == nil {
		t.Fatal("interior corruption must fail the load, not be silently skipped")
	}
	if !strings.Contains(err.Error(), "not the final line") {
		t.Errorf("error must distinguish interior corruption from a crash tail, got: %v", err)
	}
}

// --- E-04/E-05: a record must prove itself before it is trusted ----------------

// A line can be valid JSON and still hold a wrong-width vector. corpus.Encode would
// reject it at write time, but only after it had been counted as available work.
func TestE04_LoadCheckpointDropsWrongWidthVector(t *testing.T) {
	good := fixtureRecord(t, 48, 1)
	bad := fixtureRecord(t, 96, 2)
	bad.Vector = bad.Vector[:corpus.Dim-1] // digest no longer matches either

	path := writeCheckpoint(t, []checkpointRecord{good, bad}, nil)

	got, dropped, err := loadCheckpoint(path)
	if err != nil {
		t.Fatalf("loadCheckpoint: %v", err)
	}
	if _, ok := got[96]; ok {
		t.Errorf("a %d-wide vector was loaded; want it dropped", corpus.Dim-1)
	}
	if dropped != 1 {
		t.Errorf("dropped = %d, want 1", dropped)
	}
	if len(got) != 1 {
		t.Errorf("records loaded = %d, want 1", len(got))
	}
}

// The digest is the whole point of the artifact: it is what lets a target database
// be verified independently. A record whose digest does not describe its own vector
// cannot verify anything, so it is not usable evidence of a generated vector.
func TestE05_LoadCheckpointDropsDigestMismatch(t *testing.T) {
	good := fixtureRecord(t, 48, 1)
	bad := fixtureRecord(t, 96, 2)
	bad.Digest = vectorDigest(fixtureVec(999)) // a real digest, of the wrong vector

	path := writeCheckpoint(t, []checkpointRecord{good, bad}, nil)

	got, dropped, err := loadCheckpoint(path)
	if err != nil {
		t.Fatalf("loadCheckpoint: %v", err)
	}
	if _, ok := got[96]; ok {
		t.Error("a record whose digest does not match its vector was loaded")
	}
	if dropped != 1 {
		t.Errorf("dropped = %d, want 1", dropped)
	}
}

// --- E-06: the digest is comparable across the wire ----------------------------

// vectorDigest hashes the canonical text form the database stores, which is what
// makes a checkpoint digest and a digest taken from SELECT embedding::STRING
// directly comparable without float formatting wobble.
func TestE06_VectorDigestHashesTheStoredForm(t *testing.T) {
	vec := fixtureVec(7)
	lit, err := corpus.Encode(vec)
	if err != nil {
		t.Fatalf("encode: %v", err)
	}
	want := fmt.Sprintf("%x", sha256.Sum256([]byte(lit)))
	if got := vectorDigest(vec); got != want {
		t.Errorf("vectorDigest = %s, want sha256 of the encoded literal %s", got, want)
	}

	// Round-trip: what the database gives back must digest identically.
	back, err := corpus.Decode(lit)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if got := vectorDigest(back); got != want {
		t.Errorf("digest after a Decode round-trip = %s, want %s", got, want)
	}
}

// --- helpers for the database-backed cases -------------------------------------

func insertCorpusRow(t *testing.T, issue int, vec []float32) string {
	t.Helper()
	id, inserted, err := corpus.Insert(context.Background(), embedDB, corpus.Issue{
		ScenarioID:    embedScenario,
		IssueNumber:   issue,
		Title:         fmt.Sprintf("fixture issue %d", issue),
		Body:          "body",
		State:         "closed",
		URL:           fmt.Sprintf("https://example.invalid/%d", issue),
		ContentSHA256: fmt.Sprintf("%064x", issue),
		Embedding:     vec,
	})
	if err != nil {
		t.Fatalf("insert corpus row %d: %v", issue, err)
	}
	if !inserted {
		t.Fatalf("corpus row %d already existed; the scenario is not clean", issue)
	}
	return id
}

func storedDigest(t *testing.T, id string) string {
	t.Helper()
	var lit sql.NullString
	err := embedDB.QueryRowContext(context.Background(),
		`SELECT embedding::STRING FROM corpus_issue WHERE id = $1::UUID`, id).Scan(&lit)
	if err != nil {
		t.Fatalf("read back embedding: %v", err)
	}
	if !lit.Valid || lit.String == "" {
		return ""
	}
	v, err := corpus.Decode(lit.String)
	if err != nil {
		t.Fatalf("decode stored embedding: %v", err)
	}
	return vectorDigest(v)
}

// --- E-07: reconcile writes what is missing, repairs what is wrong -------------

// The three states a target row can be in, exercised together, because the
// interesting one is the third: a non-NULL vector that does not match the
// checkpoint must be overwritten rather than accepted. A NOT NULL check would call
// that row done.
func TestE07_ReconcileWritesRepairsAndVerifies(t *testing.T) {
	ctx := context.Background()

	wantMissing := fixtureVec(11)
	wantCorrect := fixtureVec(12)
	wantRepair := fixtureVec(13)

	idMissing := insertCorpusRow(t, 7001, nil)         // no vector at all
	idCorrect := insertCorpusRow(t, 7002, wantCorrect) // already right
	idRepair := insertCorpusRow(t, 7003, fixtureVec(99))

	ckpt := map[int]checkpointRecord{
		7001: {IssueNumber: 7001, Digest: vectorDigest(wantMissing), Vector: wantMissing},
		7002: {IssueNumber: 7002, Digest: vectorDigest(wantCorrect), Vector: wantCorrect},
		7003: {IssueNumber: 7003, Digest: vectorDigest(wantRepair), Vector: wantRepair},
	}

	written, verified, err := reconcileTarget(ctx, embedDB, embedScenario, ckpt, 4)
	if err != nil {
		t.Fatalf("reconcile: %v", err)
	}
	if written != 2 {
		t.Errorf("written = %d, want 2 (one missing + one wrong)", written)
	}
	if verified != 1 {
		t.Errorf("verified = %d, want 1 (the already-correct row)", verified)
	}

	for id, want := range map[string][]float32{
		idMissing: wantMissing,
		idCorrect: wantCorrect,
		idRepair:  wantRepair,
	} {
		if got := storedDigest(t, id); got != vectorDigest(want) {
			t.Errorf("row %s digest = %q, want %q", id, got, vectorDigest(want))
		}
	}

	// E-07b: a second pass is a no-op. This is the property that makes a re-run
	// after a completed embedding cost nothing at all.
	written2, verified2, err := reconcileTarget(ctx, embedDB, embedScenario, ckpt, 4)
	if err != nil {
		t.Fatalf("second reconcile: %v", err)
	}
	if written2 != 0 {
		t.Errorf("second pass written = %d, want 0: reconcile is not idempotent", written2)
	}
	if verified2 != 3 {
		t.Errorf("second pass verified = %d, want 3", verified2)
	}
}

// --- E-08: the work list is driven by the checkpoint, not by NULL --------------

// A row holding a non-NULL vector that no checkpoint record can vouch for used to
// be unreachable: phase A skipped it for not being NULL, phase B skipped it for
// having no record to compare against. It must appear as work.
func TestE08_RowsNeedingVectorsCoversUnverifiableEmbeddings(t *testing.T) {
	ctx := context.Background()

	idsByIssue := map[int]string{
		8001: insertCorpusRow(t, 8001, nil),            // NULL — obviously work
		8002: insertCorpusRow(t, 8002, fixtureVec(21)), // stored, and in the checkpoint
		8003: insertCorpusRow(t, 8003, fixtureVec(22)), // stored, but nothing vouches for it
	}
	_ = idsByIssue

	vouched := fixtureVec(21)
	ckpt := map[int]checkpointRecord{
		8002: {IssueNumber: 8002, Digest: vectorDigest(vouched), Vector: vouched},
	}

	pending, err := rowsNeedingVectors(ctx, embedDB, embedScenario, ckpt, 0)
	if err != nil {
		t.Fatalf("rowsNeedingVectors: %v", err)
	}

	got := map[int]bool{}
	for _, p := range pending {
		got[p.issueNumber] = true
	}
	if !got[8001] {
		t.Error("a NULL embedding must be queued for generation")
	}
	if !got[8003] {
		t.Error("a non-NULL embedding with no checkpoint record must be queued; " +
			"otherwise a wrong vector survives every re-run")
	}
	if got[8002] {
		t.Error("a row the checkpoint already vouches for must not be re-billed to Bedrock")
	}
}
