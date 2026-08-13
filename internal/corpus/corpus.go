// Package corpus is the external institutional-memory layer: the issue corpus and
// the citations that connect retrieved corpus rows back to ledger beliefs.
//
// # Why this is not part of the kernel
//
// The kernel decides what an agent is entitled to do. This package decides
// nothing. It stores documents that exist in the world independently of any
// claim, and it proposes candidates by similarity. A retrieval result is not
// evidence and confers no authority; it becomes evidence only when a reviewer
// attaches it to a belief through the kernel's own write paths.
//
// That separation is also mechanical. internal/kernel is governed by I-7, which
// scripts/check_i7.sh enforces by counting exactly seven crdb.ExecuteTx write
// sites and exactly one pool-level read. Corpus code in that package would break
// the count and, worse, would blur the boundary the count exists to protect.
//
// # Transactions
//
// Every operation here is a single statement. CockroachDB executes a lone
// statement as an implicit transaction and retries serialization failures on the
// gateway, so no client-side retry wrapper is warranted and none is used. This
// package deliberately does not reach for the kernel's crdb.ExecuteTx discipline:
// that discipline exists to make multi-statement epistemic transitions atomic,
// and nothing here is an epistemic transition.
package corpus

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
)

// Dim is the embedding width, fixed by db/002_corpus.sql's VECTOR(1024).
//
// It matches Amazon Titan Text Embeddings v2's default and maximum output width,
// so the placeholder embedder in embed.go and the real one that replaces it
// produce the same shape and the column never has to change.
const Dim = 1024

// Issue is one external corpus document.
//
// Embedding may be nil: ingestion lands rows first and embeds second, so a row
// with no embedding is a legitimate intermediate state, not an error.
type Issue struct {
	ScenarioID    string
	IssueNumber   int
	Title         string
	Body          string
	State         string // "open" or "closed"
	URL           string
	ClosedAt      *string
	ContentSHA256 string
	Embedding     []float32
}

// Hit is one retrieval result, carrying the distance that produced it.
type Hit struct {
	ID          string
	IssueNumber int
	Title       string
	URL         string
	State       string
	Distance    float64
}

const sqlInsertIssue = `
	INSERT INTO corpus_issue
	  (scenario_id, issue_number, title, body, state, url, closed_at, content_sha256, embedding)
	VALUES ($1::UUID, $2::INT, $3::STRING, $4::STRING, $5::STRING, $6::STRING, $7::TIMESTAMPTZ, $8::STRING, $9::VECTOR)
	ON CONFLICT (scenario_id, issue_number) DO NOTHING
	RETURNING id`

// Insert adds one corpus row and returns its id.
//
// Idempotent on (scenario_id, issue_number): re-ingesting an issue already present
// is a no-op and returns inserted=false, which is what makes repeated ingestion
// safe to run. It returns no error in that case, because "already known" is a
// normal outcome of re-reading an upstream feed, not a failure.
//
// No belief is required, referenced, or created. That is the entire point of this
// table: evidence.belief_id is NOT NULL, so the ledger cannot hold a document that
// nobody has yet formed a claim about.
func Insert(ctx context.Context, db *sql.DB, iss Issue) (id string, inserted bool, err error) {
	var emb any
	if iss.Embedding != nil {
		lit, err := Encode(iss.Embedding)
		if err != nil {
			return "", false, err
		}
		emb = lit
	}

	row := db.QueryRowContext(ctx, sqlInsertIssue,
		iss.ScenarioID, iss.IssueNumber, iss.Title, iss.Body,
		iss.State, iss.URL, iss.ClosedAt, iss.ContentSHA256, emb)

	err = row.Scan(&id)
	if err == sql.ErrNoRows {
		return "", false, nil // already present; DO NOTHING suppressed the insert
	}
	if err != nil {
		return "", false, fmt.Errorf("insert corpus issue %d: %w", iss.IssueNumber, err)
	}
	return id, true, nil
}

// InsertBatch inserts many corpus rows in ONE statement and returns how many
// were new. Same identity and same conflict behaviour as Insert.
//
// # This path is valid ONLY while every row's embedding is nil
//
// CockroachDB documents that large batch inserts of VECTOR values degrade badly
// and should be avoided, which is why Insert is one row per statement and why the
// corpus tests seed that way. That hazard is about the vector payload. Phase 3
// ingests the issue corpus with embedding NULL — there is no vector in flight —
// so batching is safe here and turns a 20-30 minute cloud ingest into about a
// minute.
//
// The moment embeddings are present, this function is the wrong tool: it rejects
// any row carrying one rather than letting a later phase inherit the shortcut by
// accident. Use Insert for embedded rows.
func InsertBatch(ctx context.Context, db *sql.DB, issues []Issue) (inserted int, err error) {
	if len(issues) == 0 {
		return 0, nil
	}

	const cols = 8
	args := make([]any, 0, len(issues)*cols)
	var b strings.Builder
	b.WriteString(`INSERT INTO corpus_issue
	  (scenario_id, issue_number, title, body, state, url, closed_at, content_sha256)
	VALUES `)

	for i, iss := range issues {
		if iss.Embedding != nil {
			return 0, fmt.Errorf(
				"InsertBatch received an embedding for issue %d: batching VECTOR values is unsafe, use Insert",
				iss.IssueNumber)
		}
		if i > 0 {
			b.WriteByte(',')
		}
		n := i * cols
		fmt.Fprintf(&b, "($%d::UUID,$%d::INT,$%d::STRING,$%d::STRING,$%d::STRING,$%d::STRING,$%d::TIMESTAMPTZ,$%d::STRING)",
			n+1, n+2, n+3, n+4, n+5, n+6, n+7, n+8)
		args = append(args, iss.ScenarioID, iss.IssueNumber, iss.Title, iss.Body,
			iss.State, iss.URL, iss.ClosedAt, iss.ContentSHA256)
	}
	b.WriteString(" ON CONFLICT (scenario_id, issue_number) DO NOTHING")

	res, err := db.ExecContext(ctx, b.String(), args...)
	if err != nil {
		return 0, fmt.Errorf("insert corpus batch of %d: %w", len(issues), err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return 0, err
	}
	return int(n), nil
}

const sqlSetEmbedding = `
	UPDATE corpus_issue SET embedding = $2::VECTOR WHERE id = $1::UUID`

// SetEmbedding backfills the embedding for one corpus row.
//
// Separate from Insert because the two happen at different times and, later, from
// different sources: ingestion is a GitHub read, embedding is a model call.
func SetEmbedding(ctx context.Context, db *sql.DB, id string, vec []float32) error {
	lit, err := Encode(vec)
	if err != nil {
		return err
	}
	if _, err := db.ExecContext(ctx, sqlSetEmbedding, id, lit); err != nil {
		return fmt.Errorf("set embedding %s: %w", id, err)
	}
	return nil
}

// sqlSearch is the ANN query.
//
// scenario_id is pinned to a single value on purpose. It is the vector index's
// prefix column, and CockroachDB only uses the index when every prefix column is
// constrained to a specific value -- so this predicate is not merely a filter, it
// is what makes the query an index search instead of a full scan. Verified: the
// plan reads "vector search table: corpus_issue@corpus_issue_embedding_idx" with
// "prefix spans" bounded to the scenario.
//
// <=> is the cosine-distance operator, matching the index's vector_cosine_ops
// opclass. Using a different operator here would silently fall back to a scan.
//
// There is deliberately NO "AND embedding IS NOT NULL" predicate here, and that
// is a measured decision rather than an oversight.
//
// Ingestion lands rows before embeddings exist, so the corpus legitimately holds
// unembedded rows, and the obvious instinct is to filter them out in SQL. Doing so
// is actively harmful: adding that predicate makes CockroachDB abandon the vector
// index and fall back to a scan of corpus_issue_scenario_number_key — verified by
// EXPLAIN on both query shapes over identical data.
//
// The predicate is also unnecessary. A row with a NULL embedding is not present in
// the vector index at all, so the index-backed path can never surface one and can
// never produce a NULL distance. Verified directly: with 50 unembedded rows and 1
// embedded row in the same scenario, this query returns exactly the embedded row
// at distance 0.
//
// Robustness against the degenerate case is handled in Go instead — see the
// NullFloat64 scan in Search — so that if the optimizer ever does choose a scan,
// unembedded rows are skipped rather than crashing the caller. That costs nothing
// and keeps the index path optimal.
const sqlSearch = `
	SELECT id, issue_number, title, url, state, embedding <=> $2::VECTOR AS distance
	FROM corpus_issue
	WHERE scenario_id = $1::UUID
	ORDER BY embedding <=> $2::VECTOR
	LIMIT $3`

// Search returns the k nearest corpus rows to query within one scenario.
//
// Results are candidates, not conclusions. The distance travels with each hit so
// that whatever cites it can record how close the match actually was.
func Search(ctx context.Context, db *sql.DB, scenarioID string, query []float32, k int) ([]Hit, error) {
	lit, err := Encode(query)
	if err != nil {
		return nil, err
	}

	rows, err := db.QueryContext(ctx, sqlSearch, scenarioID, lit, k)
	if err != nil {
		return nil, fmt.Errorf("corpus search: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var out []Hit
	for rows.Next() {
		var h Hit
		// Distance is scanned as nullable purely as a backstop. On the index path
		// it is never NULL, because unembedded rows are not in the vector index.
		// If the optimizer ever falls back to a scan, unembedded rows would appear
		// with a NULL distance and sort first under ASC; skipping them here keeps
		// that degradation from becoming a crash or a page of empty results.
		var dist sql.NullFloat64
		if err := rows.Scan(&h.ID, &h.IssueNumber, &h.Title, &h.URL, &h.State, &dist); err != nil {
			return nil, fmt.Errorf("corpus search scan: %w", err)
		}
		if !dist.Valid {
			continue
		}
		h.Distance = dist.Float64
		out = append(out, h)
	}
	return out, rows.Err()
}

const sqlCite = `
	INSERT INTO belief_corpus_citation (belief_id, corpus_id, distance, query_text)
	VALUES ($1::UUID, $2::UUID, $3::FLOAT8, $4::STRING)
	ON CONFLICT (belief_id, corpus_id) DO UPDATE
	  SET distance = excluded.distance,
	      query_text = excluded.query_text,
	      retrieved_at = now()`

// Cite records that a belief was formed citing a corpus row, at a measured
// distance, in answer to a specific question.
//
// This is the auditable half of retrieval. Without it, "the agent consulted
// relevant history" is an assertion nobody can check; with it, the distance and
// the query are on the record and a reviewer can re-run the search and compare.
//
// belief_id is a real foreign key to belief(id), so a citation cannot name a
// belief that does not exist.
func Cite(ctx context.Context, db *sql.DB, beliefID, corpusID string, distance float64, queryText string) error {
	if _, err := db.ExecContext(ctx, sqlCite, beliefID, corpusID, distance, queryText); err != nil {
		return fmt.Errorf("cite %s -> %s: %w", beliefID, corpusID, err)
	}
	return nil
}
