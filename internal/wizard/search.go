package wizard

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/PithomLabs/solvent/internal/corpus"
)

// DefaultQuestion is the prefilled, editable query on screen 1.
const DefaultQuestion = "Is etcd v3.5.x safe to deploy?"

// ErrEmbedderUnavailable is returned when no query vector can be produced.
//
// It names the missing permission rather than the missing feature, because the failure
// this describes is almost always one IAM statement on a deployed service, and a judge
// or an operator should not have to guess that from "search failed".
var ErrEmbedderUnavailable = errors.New(
	"Bedrock query embedding unavailable: the runtime has no credentials permitting " +
		"bedrock:InvokeModel on amazon.titan-embed-text-v2:0 in us-west-2. " +
		"On App Runner this means the service needs an instance role")

// Hit is one retrieval result, already carrying whether it is currently selected.
type Hit struct {
	CorpusID    string  `json:"corpus_id"`
	IssueNumber int     `json:"issue_number"`
	Title       string  `json:"title"`
	URL         string  `json:"url"`
	State       string  `json:"state"`
	Distance    float64 `json:"distance"`
	Selected    bool    `json:"selected"`
}

// SearchResult is what screen 1 renders.
type SearchResult struct {
	Query      string `json:"query"`
	Model      string `json:"model"`
	Tokens     int    `json:"query_tokens"`
	Cached     bool   `json:"cached"`
	CorpusRows int    `json:"corpus_rows"`
	Hits       []Hit  `json:"hits"`
}

// embedder returns the query embedder, building it once.
//
// Built lazily so the process starts and serves the read-only ledger even with no AWS
// credentials at all; the failure surfaces on the first search, where it can be
// explained, rather than at boot, where it would take the whole service down.
func (s *Server) embed(ctx context.Context) (QueryEmbedder, error) {
	s.embedOnce.Do(func() {
		e, err := corpus.NewEmbedder(ctx, s.region, s.model)
		if err != nil {
			s.embedErr = err
			return
		}
		s.embedder = e
	})
	if s.embedder == nil {
		return nil, fmt.Errorf("%w: %v", ErrEmbedderUnavailable, s.embedErr)
	}
	return s.embedder, nil
}

// Search embeds the question with Bedrock Titan and runs the production ANN query.
//
// The browser never sends a vector and never sees one. The chain is:
//
//	judge's question -> Bedrock Titan -> 1024-d vector -> CockroachDB ANN -> real issues
//
// corpus.SearchSQL() is used unmodified, so what runs here is the same statement
// TestC12 EXPLAINs and proves uses corpus_issue_embedding_idx with scenario-bounded
// prefix spans. The distance operator is `<=>`; the index is vector_cosine_ops and
// `<->` would silently mean something else and drop the index.
func (s *Server) Search(ctx context.Context, beliefID, question string) (SearchResult, error) {
	question = strings.TrimSpace(question)
	if question == "" {
		return SearchResult{}, fmt.Errorf("enter a question first")
	}

	res := SearchResult{Query: question, Model: corpus.DefaultEmbedModel}
	if s.model != "" {
		res.Model = s.model
	}

	vec, cached := s.queries.get(question)
	if cached {
		res.Cached = true
	} else {
		emb, err := s.embed(ctx)
		if err != nil {
			return res, err
		}
		v, tokens, err := emb.Embed(ctx, question)
		if err != nil {
			return res, fmt.Errorf("embed query: %w", err)
		}
		// Only a genuine Titan vector ever reaches the cache.
		vec, res.Tokens = v, tokens
		s.queries.put(question, v)
	}

	hits, err := corpus.Search(ctx, s.db, s.corpus, vec, s.topK)
	if err != nil {
		return res, fmt.Errorf("ann search: %w", err)
	}

	selected, err := s.selectedIDs(ctx, beliefID)
	if err != nil {
		return res, err
	}
	for _, h := range hits {
		res.Hits = append(res.Hits, Hit{
			CorpusID:    h.ID,
			IssueNumber: h.IssueNumber,
			Title:       h.Title,
			URL:         h.URL,
			State:       h.State,
			Distance:    h.Distance,
			Selected:    selected[h.ID],
		})
	}
	if err := s.db.QueryRowContext(ctx,
		`SELECT count(*) FROM corpus_issue WHERE scenario_id = $1::UUID`, s.corpus).Scan(&res.CorpusRows); err != nil {
		return res, fmt.Errorf("count corpus: %w", err)
	}
	if res.Hits == nil {
		res.Hits = []Hit{} // [] not null, for the same reason State normalises its slices
	}
	return res, nil
}

// Select records or removes a selection.
//
// Persisted the moment the judge ticks the box, not when they leave the screen. Three
// reasons, all of which bit the earlier design: the belief already exists from the
// seed so there is nothing to defer it behind; the ASK→DISCHARGE transition is driven
// by a logged refusal rather than by a Continue click, so by the time a refusal exists
// the citations must already be on the record; and selections accumulated in browser
// memory across two searches would vanish on the reload this phase is required to
// survive.
//
// Distance and query text are the real measured values from the search that surfaced
// the row — that is the whole point of the citation.
func (s *Server) Select(ctx context.Context, beliefID, corpusID, queryText string, distance float64, on bool) error {
	if !on {
		return corpus.Uncite(ctx, s.db, beliefID, corpusID)
	}
	return corpus.Cite(ctx, s.db, beliefID, corpusID, distance, queryText, corpus.RelationConsidered)
}

func (s *Server) selectedIDs(ctx context.Context, beliefID string) (map[string]bool, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT corpus_id::STRING FROM belief_corpus_citation WHERE belief_id = $1::UUID`, beliefID)
	if err != nil {
		return nil, fmt.Errorf("wizard: read selections: %w", err)
	}
	defer func() { _ = rows.Close() }()
	out := map[string]bool{}
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		out[id] = true
	}
	return out, rows.Err()
}
