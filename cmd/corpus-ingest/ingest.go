package main

import (
	"bufio"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"

	"github.com/PithomLabs/solvent/internal/corpus"
)

// runIngest loads a captured snapshot into corpus_issue.
//
// It is safe to run repeatedly: identity is (scenario_id, issue_number) and the
// insert is ON CONFLICT DO NOTHING, so a second run inserts nothing. That is the
// property the Phase 3 gate checks, and it is what makes the same artifact usable
// against both the local and the cloud database.
func runIngest(ctx context.Context, db *sql.DB, repo, scenarioID, path string, batchSize int) error {
	// The digest check is what makes "the same snapshot loaded both databases" a
	// verifiable claim rather than an assertion. A mismatch means the file was
	// edited or re-fetched between ingests, which would silently break that claim.
	meta, err := readMeta(path)
	if err != nil {
		return fmt.Errorf("read snapshot metadata: %w", err)
	}
	actual, err := fileSHA256(path)
	if err != nil {
		return err
	}
	if actual != meta.NDJSONSHA256 {
		return fmt.Errorf("snapshot digest mismatch\n  recorded: %s\n  actual:   %s\n"+
			"the NDJSON changed after capture; re-fetch or restore it", meta.NDJSONSHA256, actual)
	}
	fmt.Printf("snapshot:     %s\n", path)
	fmt.Printf("fetched_at:   %s\n", meta.FetchedAt)
	fmt.Printf("digest:       %s (verified)\n", actual)
	fmt.Printf("issues_kept:  %d\n", meta.IssuesKept)

	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer func() { _ = f.Close() }()

	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 1<<20), 16<<20) // some etcd issue bodies are large

	var (
		pending  []corpus.Issue
		lines    int
		inserted int
		open     int
		closed   int
		seen     = map[int]bool{}
		dupes    int
	)

	flush := func() error {
		if len(pending) == 0 {
			return nil
		}
		n, err := corpus.InsertBatch(ctx, db, pending)
		if err != nil {
			return err
		}
		inserted += n
		pending = pending[:0]
		return nil
	}

	for sc.Scan() {
		line := sc.Bytes()
		if len(line) == 0 {
			continue
		}
		lines++

		var iss SnapshotIssue
		if err := json.Unmarshal(line, &iss); err != nil {
			return fmt.Errorf("line %d: %w", lines, err)
		}
		if seen[iss.Number] {
			dupes++
			continue
		}
		seen[iss.Number] = true

		switch iss.State {
		case "open":
			open++
		case "closed":
			closed++
		default:
			// corpus_issue's CHECK accepts only open/closed. Fail loudly rather
			// than coercing an unexpected upstream value into something legal.
			return fmt.Errorf("line %d: issue %d has unexpected state %q", lines, iss.Number, iss.State)
		}

		pending = append(pending, corpus.Issue{
			ScenarioID:    scenarioID,
			IssueNumber:   iss.Number,
			Title:         iss.Title,
			Body:          iss.Body,
			State:         iss.State,
			URL:           iss.URL,
			ClosedAt:      iss.ClosedAt,
			ContentSHA256: contentHash(repo, iss),
			// Embedding stays nil. Phase 4 backfills; a corpus row with no
			// embedding is a documented intermediate state, not an error.
		})

		if len(pending) >= batchSize {
			if err := flush(); err != nil {
				return err
			}
		}
	}
	if err := sc.Err(); err != nil {
		return err
	}
	if err := flush(); err != nil {
		return err
	}

	fmt.Printf("LINES_READ=%d\n", lines)
	fmt.Printf("DUPLICATE_ISSUE_NUMBERS=%d\n", dupes)
	fmt.Printf("OPEN=%d\n", open)
	fmt.Printf("CLOSED=%d\n", closed)
	fmt.Printf("ROWS_INSERTED=%d\n", inserted)

	if err := runVerify(ctx, db, scenarioID); err != nil {
		return err
	}

	if dupes != 0 {
		fmt.Printf("VERDICT: FAIL (%d duplicate issue numbers in the snapshot)\n", dupes)
		return fmt.Errorf("snapshot contains duplicates")
	}
	if open+closed != lines {
		fmt.Printf("VERDICT: FAIL (open+closed=%d != lines=%d)\n", open+closed, lines)
		return fmt.Errorf("snapshot is internally inconsistent")
	}
	fmt.Println("VERDICT: PASS")
	return nil
}

// runVerify prints the gate counts. Read-only.
func runVerify(ctx context.Context, db *sql.DB, scenarioID string) error {
	var rows, open, closed, distinct, embedded int
	err := db.QueryRowContext(ctx, `
		SELECT
		  count(*),
		  count(*) FILTER (WHERE state='open'),
		  count(*) FILTER (WHERE state='closed'),
		  count(DISTINCT issue_number),
		  count(*) FILTER (WHERE embedding IS NOT NULL)
		FROM corpus_issue WHERE scenario_id = $1::UUID`, scenarioID).
		Scan(&rows, &open, &closed, &distinct, &embedded)
	if err != nil {
		return err
	}

	// The ledger must be exactly where Beat 7 left it. The corpus is evidence
	// nobody has formed a belief about; if ingestion moved any of these, the
	// separation the whole design rests on has been violated.
	var beliefs, evidence, retracted, cancelled, audit int
	err = db.QueryRowContext(ctx, `
		SELECT
		  (SELECT count(*) FROM belief WHERE scenario_id=$1::UUID),
		  (SELECT count(*) FROM evidence WHERE scenario_id=$1::UUID),
		  (SELECT count(*) FROM belief WHERE scenario_id=$1::UUID AND status='retracted'),
		  (SELECT count(*) FROM action_intent WHERE scenario_id=$1::UUID AND state='cancelled'),
		  (SELECT count(*) FROM action_intent a JOIN belief b ON b.id=a.belief_id
		     WHERE a.state='live' AND b.status<>'promoted' AND a.scenario_id=$1::UUID)`, scenarioID).
		Scan(&beliefs, &evidence, &retracted, &cancelled, &audit)
	if err != nil {
		return err
	}

	var citations int
	if err := db.QueryRowContext(ctx,
		`SELECT count(*) FROM belief_corpus_citation`).Scan(&citations); err != nil {
		return err
	}

	fmt.Printf("CORPUS_ROWS=%d\n", rows)
	fmt.Printf("CORPUS_OPEN=%d\n", open)
	fmt.Printf("CORPUS_CLOSED=%d\n", closed)
	fmt.Printf("CORPUS_DISTINCT_ISSUE_NUMBERS=%d\n", distinct)
	fmt.Printf("CORPUS_WITH_EMBEDDING=%d\n", embedded)
	fmt.Printf("LEDGER_BELIEFS=%d\n", beliefs)
	fmt.Printf("LEDGER_EVIDENCE=%d\n", evidence)
	fmt.Printf("LEDGER_RETRACTED=%d\n", retracted)
	fmt.Printf("LEDGER_CANCELLED_INTENTS=%d\n", cancelled)
	fmt.Printf("LEDGER_AUDIT_LIVE_ON_NONPROMOTED=%d\n", audit)
	fmt.Printf("BELIEF_CORPUS_CITATIONS=%d\n", citations)

	if rows != distinct {
		return fmt.Errorf("corpus holds %d rows but only %d distinct issue numbers", rows, distinct)
	}
	if open+closed != rows {
		return fmt.Errorf("open+closed=%d does not equal corpus rows=%d", open+closed, rows)
	}
	if audit != 0 {
		return fmt.Errorf("audit is %d; live intents rest on non-promoted beliefs", audit)
	}
	return nil
}
