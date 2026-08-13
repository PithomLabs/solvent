// Command corpus-ingest captures the etcd issue corpus from GitHub and loads it
// into CockroachDB as unattached external evidence.
//
// The corpus is memory, not conclusions: no belief is created, referenced, or
// implied by anything this command does. Rows land in corpus_issue, which exists
// precisely because evidence.belief_id is NOT NULL and a real-world issue exists
// long before anyone forms a claim about it.
//
// Three modes, selected by flag presence:
//
//	# capture the canonical snapshot (reads GitHub exactly once)
//	GITHUB_TOKEN=$(gh auth token) corpus-ingest --fetch corpus-data/etcd-issues.ndjson
//
//	# load that snapshot into a database (idempotent; safe to repeat)
//	corpus-ingest --dsn "$DSN" --ingest corpus-data/etcd-issues.ndjson
//
//	# read-only gate counts
//	corpus-ingest --dsn "$DSN" --verify
//
// The NDJSON written by --fetch, together with its .meta.json sidecar, IS the
// corpus. --ingest re-verifies the file against the digest recorded in the
// sidecar before writing a single row, so the same artifact can be proven to have
// loaded both the local and the cloud database.
package main

import (
	"context"
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
	dsn := flag.String("dsn", "", "CockroachDB connection string (env: FABLE_DSN)")
	scenario := flag.String("scenario", defaultScenario, "scenario UUID that owns the corpus")
	repo := flag.String("repo", defaultRepo, "GitHub repository, owner/name")
	batch := flag.Int("batch", 500, "rows per INSERT statement (valid only while embeddings are NULL)")
	flag.Parse()

	modes := 0
	for _, on := range []bool{*fetchTo != "", *ingestFrom != "", *verify} {
		if on {
			modes++
		}
	}
	if modes != 1 {
		fail("choose exactly one mode: --fetch, --ingest, or --verify")
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

	if err := runIngest(ctx, db, *repo, *scenario, *ingestFrom, *batch); err != nil {
		fail(fmt.Sprintf("ingest: %v", err))
	}
}

func fail(msg string) {
	fmt.Fprintln(os.Stderr, msg)
	os.Exit(1)
}
