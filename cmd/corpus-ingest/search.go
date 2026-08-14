package main

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"github.com/PithomLabs/solvent/internal/corpus"
)

// runSearch answers a natural-language question against the corpus and shows what
// the vector index returned, with distances.
//
// This is the minimum reusable retrieval path Phase 5's agent will call. Retrieval
// itself is read-only and creates nothing: a result is a candidate, not evidence,
// and certainly not a belief.
//
// citeBeliefID is the one exception, and it is opt-in for a reason. A citation is
// not a byproduct of searching — it is the record that a named belief was examined
// against retrieved material, which is what Phase 5 needs to inherit. Recording one
// on every search would fill belief_corpus_citation with provenance nobody claimed,
// so it happens only when a caller names the belief. Even then nothing is promoted
// and no belief is created or modified: the citation is a pointer, not a judgement.
func runSearch(ctx context.Context, db *sql.DB, scenarioID, question, region, modelID string, k int, explain bool, citeBeliefID string, citeTop int) error {
	emb, err := corpus.NewEmbedder(ctx, region, modelID)
	if err != nil {
		return err
	}

	// The query is embedded with the SAME model and parameters as the corpus.
	// A query vector from a different model or dimension would still produce
	// plausible-looking distances against these vectors while meaning nothing.
	vec, tokens, err := emb.Embed(ctx, question)
	if err != nil {
		return err
	}

	fmt.Printf("question:     %s\n", question)
	fmt.Printf("model:        %s\n", emb.ModelID())
	fmt.Printf("query_tokens: %d\n", tokens)
	fmt.Printf("scenario:     %s\n", scenarioID)

	hits, err := corpus.Search(ctx, db, scenarioID, vec, k)
	if err != nil {
		return err
	}

	fmt.Printf("HITS=%d\n", len(hits))
	for i, h := range hits {
		fmt.Printf("  %2d. #%-6d dist=%.6f  [%s]  %s\n", i+1, h.IssueNumber, h.Distance, h.State, h.Title)
		fmt.Printf("      %s\n", h.URL)
	}

	if citeBeliefID != "" {
		if err := citeHits(ctx, db, citeBeliefID, question, hits, citeTop); err != nil {
			return err
		}
	}

	if !explain {
		return nil
	}

	// EXPLAIN the statement that actually ran, not a reconstruction of it.
	//
	// A hand-written lookalike is what let a real regression through in Phase 3:
	// the test stayed green while the production query silently stopped using the
	// vector index. corpus.SearchSQL() is the same constant Search executes.
	lit, err := corpus.Encode(vec)
	if err != nil {
		return err
	}
	rows, err := db.QueryContext(ctx, "EXPLAIN "+corpus.SearchSQL(), scenarioID, lit, k)
	if err != nil {
		return err
	}
	defer func() { _ = rows.Close() }()

	var plan strings.Builder
	for rows.Next() {
		var line string
		if err := rows.Scan(&line); err != nil {
			return err
		}
		plan.WriteString(line)
		plan.WriteString("\n")
	}
	p := plan.String()
	fmt.Printf("--- EXPLAIN (production statement) ---\n%s", p)

	switch {
	case !strings.Contains(p, "vector search"):
		fmt.Println("VERDICT: FAIL (no vector search in the plan)")
		return fmt.Errorf("ANN query did not use a vector search")
	case !strings.Contains(p, "corpus_issue_embedding_idx"):
		fmt.Println("VERDICT: FAIL (vector index not selected)")
		return fmt.Errorf("ANN query did not use corpus_issue_embedding_idx")
	case !strings.Contains(p, "prefix spans"):
		fmt.Println("VERDICT: FAIL (scenario prefix bound missing)")
		return fmt.Errorf("ANN query lost its scenario prefix bound")
	}
	fmt.Println("INDEX_USED=corpus_issue_embedding_idx")
	fmt.Println("VERDICT: PASS")
	return nil
}

// citeHits records that a retrieval was performed on behalf of one named belief.
//
// The belief is verified to exist first, and its claim is printed alongside the
// issues being cited. That is not decoration: a citation is only honest if a human
// can see the claim and the retrieved material together and judge whether the
// second actually bears on the first. corpus.Cite upserts on (belief_id,
// corpus_id), so re-running a demo refreshes the distance and timestamp instead of
// accumulating duplicates.
//
// Nothing here promotes, creates or edits a belief. The four frozen ledger tables
// are untouched; belief_corpus_citation is a side table that points at them.
func citeHits(ctx context.Context, db *sql.DB, beliefID, question string, hits []corpus.Hit, top int) error {
	var claim, status string
	err := db.QueryRowContext(ctx,
		`SELECT claim, status FROM belief WHERE id = $1::UUID`, beliefID).Scan(&claim, &status)
	if err == sql.ErrNoRows {
		return fmt.Errorf("--cite %s: no such belief; a citation must point at a real belief", beliefID)
	}
	if err != nil {
		return err
	}

	if top <= 0 || top > len(hits) {
		top = len(hits)
	}

	fmt.Printf("--- citing retrieval against an existing belief ---\n")
	fmt.Printf("belief_id:    %s\n", beliefID)
	fmt.Printf("belief:       %s\n", claim)
	fmt.Printf("status:       %s\n", status)

	for _, h := range hits[:top] {
		// A CLI citation records material a reviewer consulted, never a contradiction:
		// introducing a contradiction is a judgement, and this command makes none.
		if err := corpus.Cite(ctx, db, beliefID, h.ID, h.Distance, question, corpus.RelationConsidered); err != nil {
			return fmt.Errorf("cite issue #%d: %w", h.IssueNumber, err)
		}
		fmt.Printf("CITED issue=#%-6d distance=%.6f  %s\n", h.IssueNumber, h.Distance, h.Title)
	}
	fmt.Printf("CITATIONS_WRITTEN=%d\n", top)
	return nil
}
