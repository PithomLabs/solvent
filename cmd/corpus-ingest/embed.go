package main

import (
	"bufio"
	"bytes"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/PithomLabs/solvent/internal/corpus"
)

// checkpointRecord is one line of the embedding checkpoint.
//
// # The checkpoint is an embedding artifact, not a persistence receipt
//
// It records that a vector was GENERATED, never that it was stored. Those are
// different facts, and conflating them creates a silent hole: Bedrock succeeds,
// the local write succeeds, the cloud write fails, the process restarts, the
// checkpoint says "done", and the cloud row stays empty forever.
//
// So on resume the checkpoint is only the source of the vector. Each target
// database is independently checked for the expected digest and repaired from the
// checkpoint if it is missing or different. Phase 4 is green only when both
// databases independently verify the same digest for all rows.
type checkpointRecord struct {
	IssueNumber int       `json:"issue_number"`
	Digest      string    `json:"digest"` // sha256 of the vector, for independent verification
	Tokens      int       `json:"tokens"`
	Vector      []float32 `json:"vector"`
}

// embedMeta is the sidecar that makes "both databases hold the same vectors" a
// checkable claim rather than an assertion, the way the Phase 3 snapshot digest did
// for the corpus itself.
//
// The per-run and cumulative figures are kept as separate fields on purpose. A
// resumed run reports a small VectorsThisRun against a full VectorsInCkpt, and
// collapsing the two would make a resume look like a corpus that shrank.
type embedMeta struct {
	StartedAt  string `json:"started_at"`
	FinishedAt string `json:"finished_at"`
	Model      string `json:"model"`
	Region     string `json:"region"`
	Dimensions int    `json:"dimensions"`
	Normalize  bool   `json:"normalize"`
	MaxChars   int    `json:"max_embed_chars"`
	ScenarioID string `json:"scenario_id"`

	VectorsThisRun int `json:"vectors_generated_this_run"`
	VectorsInCkpt  int `json:"vectors_in_checkpoint"`
	TokensThisRun  int `json:"tokens_this_run"`
	TotalTokens    int `json:"total_tokens_all_runs"`
	CkptDropped    int `json:"checkpoint_records_dropped"`

	CheckpointSH string `json:"checkpoint_sha256"`
	Note         string `json:"note"`
}

// vectorDigest is the independent verification key: a hash of the vector itself,
// computed identically from the checkpoint and from what a database returns.
func vectorDigest(v []float32) string {
	h := sha256.New()
	// Encode is the same canonical text form the database stores, so a digest taken
	// here and a digest taken from a SELECT are comparable without float wobble.
	lit, err := corpus.Encode(v)
	if err != nil {
		return ""
	}
	_, _ = h.Write([]byte(lit))
	return fmt.Sprintf("%x", h.Sum(nil))
}

type pendingRow struct {
	id          string
	issueNumber int
	title       string
	body        string
}

// runEmbed generates Titan embeddings for the corpus and writes them to one or
// more databases.
//
// Resumable and idempotent: rows that already carry the expected vector are
// skipped without a Bedrock call, so a completed run repeated costs nothing.
func runEmbed(ctx context.Context, targets map[string]*sql.DB, scenarioID, ckptPath, region, modelID string, limit, concurrency int) error {
	emb, err := corpus.NewEmbedder(ctx, region, modelID)
	if err != nil {
		return err
	}

	startedAt := time.Now().UTC()

	ckpt, ckptDropped, err := loadCheckpoint(ckptPath)
	if err != nil {
		return err
	}

	fmt.Printf("model:        %s\n", emb.ModelID())
	fmt.Printf("region:       %s\n", emb.Region())
	fmt.Printf("dimensions:   %d\n", corpus.Dim)
	fmt.Printf("max_chars:    %d\n", corpus.MaxEmbedChars)
	fmt.Printf("checkpoint:   %s (%d vectors already generated", ckptPath, len(ckpt))
	if ckptDropped > 0 {
		fmt.Printf(", %d unusable record(s) dropped and queued for regeneration", ckptDropped)
	}
	fmt.Printf(")\n")
	for name := range targets {
		fmt.Printf("target:       %s\n", name)
	}

	// Phase A — generate any vector that is missing from the checkpoint.
	//
	// The work list comes from the PRIMARY target's unembedded rows, but a row is
	// only skipped here if the checkpoint already holds its vector. Storage state is
	// reconciled separately in phase B.
	primary := targets["local"]
	if primary == nil {
		for _, db := range targets {
			primary = db
			break
		}
	}

	pending, err := rowsNeedingVectors(ctx, primary, scenarioID, ckpt, limit)
	if err != nil {
		return err
	}
	fmt.Printf("to_generate:  %d\n", len(pending))

	var (
		mu       sync.Mutex
		made     int
		tokens   int
		failures int
		firstErr error
	)

	if len(pending) > 0 {
		f, err := os.OpenFile(ckptPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
		if err != nil {
			return err
		}

		sem := make(chan struct{}, concurrency)
		var wg sync.WaitGroup
		start := time.Now()

		for _, row := range pending {
			wg.Add(1)
			sem <- struct{}{}
			go func(row pendingRow) {
				defer wg.Done()
				defer func() { <-sem }()

				vec, tok, err := emb.Embed(ctx, corpus.BuildEmbedText(row.title, row.body))
				mu.Lock()
				defer mu.Unlock()
				if err != nil {
					failures++
					if firstErr == nil {
						firstErr = fmt.Errorf("issue %d: %w", row.issueNumber, err)
					}
					return
				}
				rec := checkpointRecord{
					IssueNumber: row.issueNumber,
					Digest:      vectorDigest(vec),
					Tokens:      tok,
					Vector:      vec,
				}
				// Appended immediately, so a crash loses at most the calls still in
				// flight and never re-bills for a vector already produced.
				line, _ := json.Marshal(rec)
				_, _ = f.Write(append(line, '\n'))
				ckpt[row.issueNumber] = rec
				made++
				tokens += tok
				if made%500 == 0 {
					rate := float64(made) / time.Since(start).Seconds()
					fmt.Printf("  … generated %d/%d  (%.1f/s, %d tokens)\n", made, len(pending), rate, tokens)
				}
			}(row)
		}
		wg.Wait()
		_ = f.Close()

		if firstErr != nil && made == 0 {
			return firstErr
		}
	}

	// Phase B — reconcile every target against the checkpoint, independently.
	//
	// This is what makes the dual write recoverable. A target is repaired whenever
	// its stored vector is absent or its digest differs, regardless of what the
	// checkpoint says about generation.
	totals := map[string][2]int{} // name -> {written, verified}
	for name, db := range targets {
		written, verified, err := reconcileTarget(ctx, db, scenarioID, ckpt, concurrency)
		if err != nil {
			return fmt.Errorf("reconcile %s: %w", name, err)
		}
		totals[name] = [2]int{written, verified}
	}

	digest, err := fileSHA256(ckptPath)
	if err != nil {
		return err
	}
	// Tokens are summed over the whole checkpoint, not just this run. A resumed run
	// generates only the remainder, so reporting its own count as "total" would
	// under-report the corpus's real cost by however much a previous run had paid.
	var totalTokens int
	for _, rec := range ckpt {
		totalTokens += rec.Tokens
	}

	meta := embedMeta{
		StartedAt:      startedAt.Format(time.RFC3339),
		FinishedAt:     time.Now().UTC().Format(time.RFC3339),
		Model:          emb.ModelID(),
		Region:         emb.Region(),
		Dimensions:     corpus.Dim,
		Normalize:      corpus.EmbedNormalize,
		MaxChars:       corpus.MaxEmbedChars,
		ScenarioID:     scenarioID,
		VectorsThisRun: made,
		VectorsInCkpt:  len(ckpt),
		TokensThisRun:  tokens,
		TotalTokens:    totalTokens,
		CkptDropped:    ckptDropped,
		CheckpointSH:   digest,
		Note: "Checkpoint records vector GENERATION, not persistence. Each target database is " +
			"verified independently against these digests and repaired if missing or different.",
	}
	b, _ := json.MarshalIndent(meta, "", "  ")
	if err := os.WriteFile(ckptPath+".meta.json", append(b, '\n'), 0o644); err != nil {
		return err
	}

	fmt.Printf("MODEL=%s\n", emb.ModelID())
	fmt.Printf("REGION=%s\n", emb.Region())
	fmt.Printf("DIMENSIONS=%d\n", corpus.Dim)
	fmt.Printf("VECTORS_GENERATED_THIS_RUN=%d\n", made)
	fmt.Printf("VECTORS_IN_CHECKPOINT=%d\n", len(ckpt))
	fmt.Printf("CHECKPOINT_RECORDS_DROPPED=%d\n", ckptDropped)
	fmt.Printf("TOKENS_THIS_RUN=%d\n", tokens)
	fmt.Printf("TOKENS_ALL_RUNS=%d\n", totalTokens)
	fmt.Printf("BEDROCK_FAILURES=%d\n", failures)
	fmt.Printf("CHECKPOINT_SHA256=%s\n", digest)
	for name, t := range totals {
		fmt.Printf("TARGET_%s_WRITTEN=%d\n", name, t[0])
		fmt.Printf("TARGET_%s_VERIFIED=%d\n", name, t[1])
	}

	for name, db := range targets {
		fmt.Printf("--- %s ---\n", name)
		if err := runVerify(ctx, db, scenarioID); err != nil {
			return err
		}
	}

	if failures != 0 {
		fmt.Printf("VERDICT: FAIL (%d Bedrock failures; re-run to resume)\n", failures)
		return fmt.Errorf("embedding incomplete: %v", firstErr)
	}
	fmt.Println("VERDICT: PASS")
	return nil
}

// rowsNeedingVectors lists corpus rows that have no checkpoint-backed vector.
//
// # Why the work list is not "rows where embedding IS NULL"
//
// The obvious query — select the NULL embeddings — leaves a row unreachable in one
// specific state: a stored vector that is non-NULL but wrong, whose issue is absent
// from the checkpoint. Phase A would skip it because it is not NULL, and phase B
// would skip it because there is no checkpoint record to compare it against, so a
// bad vector would survive every re-run while the gate reported full coverage.
//
// The membership test that closes that hole is the checkpoint, not the column: a
// row needs work whenever the artifact of record holds no verified vector for it,
// whatever the database currently contains. The cost of this is that deleting the
// checkpoint after a successful run makes the next run re-embed the corpus rather
// than trust vectors it can no longer verify — the conservative direction, and about
// five cents.
func rowsNeedingVectors(ctx context.Context, db *sql.DB, scenarioID string, ckpt map[int]checkpointRecord, limit int) ([]pendingRow, error) {
	q := `SELECT id, issue_number, title, coalesce(body,'') FROM corpus_issue
	      WHERE scenario_id = $1::UUID ORDER BY issue_number`
	rows, err := db.QueryContext(ctx, q, scenarioID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var out []pendingRow
	for rows.Next() {
		var r pendingRow
		if err := rows.Scan(&r.id, &r.issueNumber, &r.title, &r.body); err != nil {
			return nil, err
		}
		if _, have := ckpt[r.issueNumber]; have {
			continue // vector already generated; phase B will store it
		}
		out = append(out, r)
		if limit > 0 && len(out) >= limit {
			break
		}
	}
	return out, rows.Err()
}

// reconcileTarget makes one database match the checkpoint, and reports how many
// rows it had to write versus how many already verified.
//
// Verification compares the digest of what the database actually holds against the
// checkpoint digest — not merely "is it non-NULL" — so a truncated or wrong-model
// vector is repaired rather than accepted.
func reconcileTarget(ctx context.Context, db *sql.DB, scenarioID string, ckpt map[int]checkpointRecord, concurrency int) (written, verified int, err error) {
	rows, err := db.QueryContext(ctx,
		`SELECT issue_number, id, coalesce(embedding::STRING,'') FROM corpus_issue
		 WHERE scenario_id = $1::UUID ORDER BY issue_number`, scenarioID)
	if err != nil {
		return 0, 0, err
	}

	type task struct {
		id  string
		vec []float32
	}
	var todo []task
	for rows.Next() {
		var num int
		var id, lit string
		if err := rows.Scan(&num, &id, &lit); err != nil {
			_ = rows.Close()
			return 0, 0, err
		}
		rec, have := ckpt[num]
		if !have {
			continue // no vector generated for this row yet
		}
		if lit != "" {
			if stored, e := corpus.Decode(lit); e == nil && vectorDigest(stored) == rec.Digest {
				verified++
				continue
			}
		}
		todo = append(todo, task{id: id, vec: rec.Vector})
	}
	_ = rows.Close()
	if err := rows.Err(); err != nil {
		return 0, 0, err
	}

	// One row per statement — never batched. CockroachDB documents that batching
	// VECTOR values degrades badly, and corpus.InsertBatch refuses embeddings for
	// the same reason. Concurrency here is parallel single-row statements, which
	// respects that constraint while keeping a remote write from taking 20 minutes.
	sem := make(chan struct{}, concurrency)
	var wg sync.WaitGroup
	var mu sync.Mutex
	var firstErr error

	for _, t := range todo {
		wg.Add(1)
		sem <- struct{}{}
		go func(t task) {
			defer wg.Done()
			defer func() { <-sem }()
			if e := corpus.SetEmbedding(ctx, db, t.id, t.vec); e != nil {
				mu.Lock()
				if firstErr == nil {
					firstErr = e
				}
				mu.Unlock()
				return
			}
			mu.Lock()
			written++
			mu.Unlock()
		}(t)
	}
	wg.Wait()
	return written, verified, firstErr
}

// loadCheckpoint reads the generated-vector artifact, returning the usable records
// and a count of records it refused to trust.
//
// # A truncated tail is tolerated; interior corruption is not
//
// The checkpoint is appended to as vectors arrive, so a process killed mid-write
// leaves a partial final line — and where the filesystem had already extended the
// file, a run of NUL bytes. That is what a crash looks like, and it is exactly what
// was found on disk here: 475 complete records followed by 520 NUL bytes. Refusing
// to load that file would strand every vector already paid for, which is the
// opposite of what a checkpoint is for. Tolerating it costs one regenerated vector.
//
// A malformed line in the INTERIOR is a different fact. It means the file was
// damaged after the fact rather than merely cut short, and silently skipping
// records from the middle would quietly under-populate every target database while
// still reporting success. That stays a hard error.
//
// Every surviving record is re-verified against its own digest and width. A line
// can be syntactically valid JSON and still hold a half-written vector, and an
// unverified checkpoint entry would propagate to every target — the one thing the
// digest discipline exists to prevent.
func loadCheckpoint(path string) (map[int]checkpointRecord, int, error) {
	out := map[int]checkpointRecord{}
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			if e := os.MkdirAll(filepath.Dir(path), 0o755); e != nil {
				return nil, 0, e
			}
			return out, 0, nil
		}
		return nil, 0, err
	}
	defer func() { _ = f.Close() }()

	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 1<<20), 64<<20) // a 1024-float line is ~20 KB

	var (
		dropped int
		lineNo  int
		badLine int // line that failed to parse, held until we learn whether it was last
		badErr  error
	)
	for sc.Scan() {
		lineNo++
		if badLine != 0 {
			// The unparseable line has a successor, so it was not a truncated tail.
			return nil, 0, fmt.Errorf(
				"corrupt checkpoint at line %d of %s (not the final line, so this is not a "+
					"crash tail): %w", badLine, path, badErr)
		}
		// NUL padding is how a killed process leaves an extended-but-unwritten tail.
		b := bytes.TrimSpace(bytes.Trim(sc.Bytes(), "\x00"))
		if len(b) == 0 {
			continue
		}
		var rec checkpointRecord
		if err := json.Unmarshal(b, &rec); err != nil {
			badLine, badErr = lineNo, err
			continue
		}
		if len(rec.Vector) != corpus.Dim {
			dropped++
			continue
		}
		if vectorDigest(rec.Vector) != rec.Digest {
			dropped++
			continue
		}
		out[rec.IssueNumber] = rec
	}
	if err := sc.Err(); err != nil {
		return nil, 0, err
	}
	if badLine != 0 {
		dropped++ // tolerated: the final line was cut short mid-append
	}
	return out, dropped, nil
}
