// Command corpus-ingest captures the etcd issue corpus from GitHub and loads it
// into CockroachDB as unattached external evidence.
//
// The corpus is memory, not conclusions: no belief is created, referenced, or
// implied by anything this command does. Rows land in corpus_issue, which exists
// precisely because evidence.belief_id is NOT NULL and a real-world issue exists
// long before anyone forms a claim about it.
//
// Five modes, selected by flag presence, in the order the corpus moves through
// them:
//
//	# capture the canonical snapshot (reads GitHub exactly once)
//	GITHUB_TOKEN=$(gh auth token) corpus-ingest --fetch corpus-data/etcd-issues.ndjson
//
//	# load that snapshot into a database (idempotent; safe to repeat)
//	corpus-ingest --dsn "$DSN" --ingest corpus-data/etcd-issues.ndjson
//
//	# generate Bedrock Titan vectors and reconcile them into every target
//	corpus-ingest --dsn "$DSN" [--also-dsn "$CLOUD_DSN"] --embed
//
//	# ANN retrieval, optionally proving the vector index is really used
//	corpus-ingest --dsn "$DSN" --search "is etcd v3.5.x safe to deploy?" --explain
//
//	# read-only gate counts
//	corpus-ingest --dsn "$DSN" --verify
//
// Two artifacts carry the guarantees. The NDJSON written by --fetch, with its
// .meta.json sidecar, IS the corpus: --ingest re-verifies the file against the
// recorded digest before writing a single row, so one artifact can be proven to
// have loaded both databases. The checkpoint written by --embed is the same idea
// one level up: it holds every generated vector with its digest, so both databases
// can be shown to carry identical vectors from a single set of Bedrock calls.
//
// --search deliberately writes nothing. Retrieval produces candidates, not
// evidence and not beliefs; a citation is recorded only when --cite names the
// belief the retrieval was actually performed for.
package main

import (
	"context"
	"database/sql"
	"flag"
	"fmt"
	"os"

	_ "github.com/jackc/pgx/v5/stdlib" // D-019: pgx/v5 is the sole approved driver

	"github.com/PithomLabs/solvent/internal/testdb"
)

// defaultScenario is the judge-facing Track 2 scenario — the same one the web app
// renders. The corpus shares it with the ledger without touching it: corpus rows
// live in their own table and are attached to no belief.
const defaultScenario = "00000000-0000-0000-0000-000000000002"

const defaultRepo = "etcd-io/etcd"

func main() {
	fetchTo := flag.String("fetch", "", "capture the corpus snapshot from GitHub to this NDJSON path")
	ingestFrom := flag.String("ingest", "", "load a captured NDJSON snapshot into the database")
	verify := flag.Bool("verify", false, "print the gate counts (read-only)")
	embed := flag.Bool("embed", false, "generate Bedrock Titan embeddings for unembedded corpus rows")
	search := flag.String("search", "", "ANN-search the corpus with this question (read-only)")
	topK := flag.Int("k", 8, "number of retrieval results for --search")
	explain := flag.Bool("explain", false, "with --search, EXPLAIN the production statement and assert index use")
	cite := flag.String("cite", "", "with --search, record retrieval provenance against this existing belief UUID")
	citeTop := flag.Int("cite-top", 1, "with --cite, how many of the top hits to record")
	alsoDSN := flag.String("also-dsn", "", "second database to receive the same vectors (env: FABLE_DSN_2)")
	checkpoint := flag.String("checkpoint", "corpus-data/etcd-embeddings.ndjson", "embedding checkpoint path")
	region := flag.String("region", "", "AWS region for Bedrock (default us-west-2)")
	model := flag.String("model", "", "Bedrock embedding model id (default amazon.titan-embed-text-v2:0)")
	limit := flag.Int("limit", 0, "embed at most N rows (0 = all); use for the cost/safety sample")
	concurrency := flag.Int("concurrency", 8, "parallel Bedrock calls and single-row vector writes")
	dsn := flag.String("dsn", "", "CockroachDB connection string (env: FABLE_DSN)")
	scenario := flag.String("scenario", defaultScenario, "scenario UUID that owns the corpus")
	repo := flag.String("repo", defaultRepo, "GitHub repository, owner/name")
	batch := flag.Int("batch", 500, "rows per INSERT statement (valid only while embeddings are NULL)")
	flag.Parse()

	modes := 0
	for _, on := range []bool{*fetchTo != "", *ingestFrom != "", *verify, *embed, *search != ""} {
		if on {
			modes++
		}
	}
	if modes != 1 {
		fail("choose exactly one mode: --fetch, --ingest, --embed, --search, or --verify")
	}
	// --cite is a modifier on retrieval, not a mode. Accepting it alone would imply
	// a citation can exist without the retrieval that justifies it.
	if *cite != "" && *search == "" {
		fail("--cite records the provenance of a retrieval; it requires --search")
	}

	ctx := context.Background()

	// --fetch touches no database at all.
	if *fetchTo != "" {
		if err := runFetch(ctx, *repo, *fetchTo); err != nil {
			fail(fmt.Sprintf("fetch: %v", err))
		}
		return
	}

	if *dsn == "" {
		*dsn = os.Getenv("FABLE_DSN")
	}
	if *dsn == "" {
		fail("no DSN: pass --dsn or set FABLE_DSN")
	}

	db, err := testdb.Open(*dsn)
	if err != nil {
		fail(fmt.Sprintf("open: %v", err))
	}
	defer func() { _ = db.Close() }()
	if err := db.PingContext(ctx); err != nil {
		fail(fmt.Sprintf("ping: %v", err))
	}

	if *verify {
		if err := runVerify(ctx, db, *scenario); err != nil {
			fail(fmt.Sprintf("verify: %v", err))
		}
		return
	}

	if *search != "" {
		if err := runSearch(ctx, db, *scenario, *search, *region, *model, *topK, *explain, *cite, *citeTop); err != nil {
			fail(fmt.Sprintf("search: %v", err))
		}
		return
	}

	if *embed {
		targets := map[string]*sql.DB{"local": db}
		if *alsoDSN == "" {
			*alsoDSN = os.Getenv("FABLE_DSN_2")
		}
		if *alsoDSN != "" {
			db2, err := testdb.Open(*alsoDSN)
			if err != nil {
				fail(fmt.Sprintf("open --also-dsn: %v", err))
			}
			defer func() { _ = db2.Close() }()
			if err := db2.PingContext(ctx); err != nil {
				fail(fmt.Sprintf("ping --also-dsn: %v", err))
			}
			targets["cloud"] = db2
		}
		if err := runEmbed(ctx, targets, *scenario, *checkpoint, *region, *model, *limit, *concurrency); err != nil {
			fail(fmt.Sprintf("embed: %v", err))
		}
		return
	}

	if err := runIngest(ctx, db, *repo, *scenario, *ingestFrom, *batch); err != nil {
		fail(fmt.Sprintf("ingest: %v", err))
	}
}

func fail(msg string) {
	fmt.Fprintln(os.Stderr, msg)
	os.Exit(1)
}
