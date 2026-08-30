// Package wizard is the judge-facing three-screen demo: ASK, DISCHARGE,
// FALSIFY & AUDIT.
//
// # What this package is allowed to decide
//
// Nothing epistemic. Every promotion, authorisation, debt retirement and retraction
// goes through kernel.Store, unchanged and unwrapped, so the database remains the
// only authority on whether an action is permitted. This package chooses which
// statement to attempt and how to render what came back. It never inspects debt to
// decide whether promotion should be allowed, and it never disables a button to
// prevent a refusal — a refusal the judge can see is the entire product.
//
// The one rule that follows from that, and the one most likely to be broken by a
// later edit: there is NO handler-side precondition. If a handler ever grows an
// `if len(debt) > 0 { return }` before calling Promote, the demo stops demonstrating
// anything, because the refusal it shows would be the application's opinion rather
// than the schema's guarantee.
//
// # Why it lives in internal/ rather than demo/cloud/web
//
// demo/cloud/web keeps its connection pool and its templates in package-level
// globals and parses templates through a working-directory-relative glob. Neither is
// testable. This package takes its dependencies explicitly and embeds its templates,
// so every handler can be driven by httptest from any directory, and demo/cloud/web
// is left as the thin process that mounts it.
//
// It is also outside the reach of scripts/check_i7.sh (which scans kernel only) and
// of the Taskfile's MCP/view write-boundary grep. That is not a loophole: this
// package performs no kernel writes of its own. Its only direct writes are to
// refusal_log and belief_corpus_citation, neither of which is a frozen ledger table
// and neither of which asserts anything about truth, debt or authority. The same
// argument internal/corpus and internal/demoseed already make.
//
// # State is derived, never stored
//
// There is no currentStep, no session record of progress, and no way for the browser
// to claim a screen it has not earned. GET /api/state recomputes the screen from
// committed ledger rows and logged refusals on every call, so a reload lands exactly
// where the judge was and advancing by editing the frontend is unrepresentable.
package wizard

import (
	"context"
	"database/sql"
	"fmt"
	"sync"

	"github.com/PithomLabs/solvent/kernel"
)

// DefaultCorpusScenario is the scenario that owns the embedded etcd corpus.
//
// It is deliberately NOT the wizard's own scenario. Each judge gets a fresh ledger
// scenario, but corpus_issue's vector index is prefixed on scenario_id
// (db/002_corpus.sql:60) and all 7,239 embedded rows live under this one id — so a
// search issued against a fresh scenario would traverse an empty index and return
// nothing. Retrieval reads here; the ledger writes there.
const DefaultCorpusScenario = "00000000-0000-0000-0000-000000000002"

// QueryEmbedder turns a judge's question into a query vector.
//
// An interface only so tests can supply deterministic vectors without calling
// Bedrock; *corpus.Embedder satisfies it as written. There is exactly one production
// implementation and this package does not contain a second one — the test double
// lives in a _test.go file, which is what makes it structurally impossible to ship.
type QueryEmbedder interface {
	Embed(ctx context.Context, text string) ([]float32, int, error)
}

// Options configures a Server. Everything has a working default except db.
type Options struct {
	// CorpusScenario overrides the corpus partition searched. Tests point this at
	// their own planted rows.
	CorpusScenario string

	// Embedder, when non-nil, is used instead of building a Bedrock client. Tests set
	// it; production leaves it nil and gets lazy initialisation.
	Embedder QueryEmbedder

	// Region and Model override the Bedrock defaults in internal/corpus.
	Region, Model string

	// TopK is how many hits a search returns.
	TopK int
}

// Server holds the wizard's dependencies and serves its routes.
type Server struct {
	db     *sql.DB
	kern   *kernel.Store
	corpus string
	topK   int

	region, model string

	// The embedder is built once, on first search, and reused.
	//
	// Lazily, because the process must boot and serve the read-only ledger pages even
	// when no AWS credentials are present — and because NewEmbedder resolves the whole
	// credential chain, which is not something to do per request. bedrockruntime's
	// client is safe for concurrent use.
	embedOnce sync.Once
	embedder  QueryEmbedder
	embedErr  error

	// queries caches genuine Bedrock vectors by exact query text.
	//
	// Only real vectors ever enter it, it is dropped on restart, and it never touches
	// persisted state. It exists so a demo that asks the same two questions repeatedly
	// does not re-bill and re-wait for identical work. It is not a fallback: a cache
	// miss with no credentials is still a hard, explicit failure.
	queries *queryCache
}

// New builds a Server. db must be non-nil; everything else may be zero.
func New(db *sql.DB, opts Options) (*Server, error) {
	if db == nil {
		return nil, fmt.Errorf("wizard: nil database")
	}
	s := &Server{
		db:      db,
		kern:    kernel.New(db),
		corpus:  opts.CorpusScenario,
		topK:    opts.TopK,
		region:  opts.Region,
		model:   opts.Model,
		queries: newQueryCache(queryCacheSize),
	}
	if s.corpus == "" {
		s.corpus = DefaultCorpusScenario
	}
	if s.topK <= 0 {
		s.topK = defaultTopK
	}
	if opts.Embedder != nil {
		// Pre-satisfy the once so the injected embedder is never replaced.
		s.embedder = opts.Embedder
		s.embedOnce.Do(func() {})
	}
	return s, nil
}

const (
	defaultTopK    = 6
	queryCacheSize = 16
)
