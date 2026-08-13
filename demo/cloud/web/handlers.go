package main

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"strings"
	"time"
)

// ledgerCounts holds the summary counts for the landing page.
type ledgerCounts struct {
	Beliefs     int
	Evidence    int
	Promoted    int
	LiveIntents int
	Retracted   int
}

func getLedgerCounts(ctx context.Context) (ledgerCounts, error) {
	var c ledgerCounts
	const sid = "00000000-0000-0000-0000-000000000002"

	err := db.QueryRowContext(ctx,
		`SELECT count(*) FROM belief WHERE scenario_id = $1::UUID`, sid).Scan(&c.Beliefs)
	if err != nil {
		return c, err
	}

	err = db.QueryRowContext(ctx,
		`SELECT count(*) FROM evidence WHERE scenario_id = $1::UUID`, sid).Scan(&c.Evidence)
	if err != nil {
		return c, err
	}

	err = db.QueryRowContext(ctx,
		`SELECT count(*) FROM belief WHERE scenario_id = $1::UUID AND status = 'promoted'`, sid).Scan(&c.Promoted)
	if err != nil {
		return c, err
	}

	err = db.QueryRowContext(ctx,
		`SELECT count(*) FROM action_intent WHERE scenario_id = $1::UUID AND state = 'live'`, sid).Scan(&c.LiveIntents)
	if err != nil {
		return c, err
	}

	err = db.QueryRowContext(ctx,
		`SELECT count(*) FROM belief WHERE scenario_id = $1::UUID AND status = 'retracted'`, sid).Scan(&c.Retracted)
	if err != nil {
		return c, err
	}

	return c, nil
}

func handleIndex(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
	defer cancel()

	counts, err := getLedgerCounts(ctx)
	if err != nil {
		log.Printf("index: query counts: %v", err)
		renderWarmingUp(w)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	templates.ExecuteTemplate(w, "index.html", counts)
}

func handleLedger(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
	defer cancel()

	counts, err := getLedgerCounts(ctx)
	if err != nil {
		log.Printf("ledger: query counts: %v", err)
		renderWarmingUp(w)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	templates.ExecuteTemplate(w, "ledger.html", counts)
}

type beliefRow struct {
	ID        string
	Claim     string
	ClaimType string
	Status    string
	Debt      string
}

func handleBeliefs(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
	defer cancel()

	rows, err := db.QueryContext(ctx,
		`SELECT id, claim, claim_type, status, debt::STRING
		 FROM belief WHERE scenario_id = '00000000-0000-0000-0000-000000000002'
		 ORDER BY claim`)
	if err != nil {
		log.Printf("beliefs: query: %v", err)
		renderWarmingUp(w)
		return
	}
	defer rows.Close()

	var beliefs []beliefRow
	for rows.Next() {
		var b beliefRow
		if err := rows.Scan(&b.ID, &b.Claim, &b.ClaimType, &b.Status, &b.Debt); err != nil {
			log.Printf("beliefs: scan: %v", err)
			renderWarmingUp(w)
			return
		}
		beliefs = append(beliefs, b)
	}
	if err := rows.Err(); err != nil {
		log.Printf("beliefs: rows: %v", err)
		renderWarmingUp(w)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	templates.ExecuteTemplate(w, "beliefs.html", beliefs)
}

func handleBeliefDetail(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimPrefix(r.URL.Path, "/belief/")
	if id == "" {
		http.NotFound(w, r)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
	defer cancel()

	type beliefDetail struct {
		ID        string
		Claim     string
		ClaimType string
		Status    string
		Debt      string
		Evidence  []evidenceRow
		Intents   []intentRow
	}

	var b beliefDetail
	err := db.QueryRowContext(ctx,
		`SELECT id, claim, claim_type, status, debt::STRING
		 FROM belief WHERE id = $1::UUID AND scenario_id = '00000000-0000-0000-0000-000000000002'`,
		id).Scan(&b.ID, &b.Claim, &b.ClaimType, &b.Status, &b.Debt)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	// Evidence for this belief.
	eRows, err := db.QueryContext(ctx,
		`SELECT belief_id, source_url, provenance_class, content_sha256
		 FROM evidence WHERE belief_id = $1::UUID AND scenario_id = '00000000-0000-0000-0000-000000000002'
		 ORDER BY ingested_at`,
		id)
	if err == nil {
		defer eRows.Close()
		for eRows.Next() {
			var e evidenceRow
			if err := eRows.Scan(&e.BeliefID, &e.SourceURL, &e.ProvenanceClass, &e.ContentSHA256); err == nil {
				b.Evidence = append(b.Evidence, e)
			}
		}
	}

	// Intents for this belief.
	iRows, err := db.QueryContext(ctx,
		`SELECT belief_id, action, state
		 FROM action_intent WHERE belief_id = $1::UUID AND scenario_id = '00000000-0000-0000-0000-000000000002'`,
		id)
	if err == nil {
		defer iRows.Close()
		for iRows.Next() {
			var i intentRow
			if err := iRows.Scan(&i.BeliefID, &i.Action, &i.State); err == nil {
				b.Intents = append(b.Intents, i)
			}
		}
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	templates.ExecuteTemplate(w, "belief_detail.html", b)
}

type evidenceRow struct {
	BeliefID        string
	SourceURL       string
	ProvenanceClass string
	ContentSHA256   string
}

func handleEvidence(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
	defer cancel()

	rows, err := db.QueryContext(ctx,
		`SELECT belief_id, source_url, provenance_class, content_sha256
		 FROM evidence WHERE scenario_id = '00000000-0000-0000-0000-000000000002'
		 ORDER BY ingested_at`)
	if err != nil {
		log.Printf("evidence: query: %v", err)
		renderWarmingUp(w)
		return
	}
	defer rows.Close()

	var evidence []evidenceRow
	for rows.Next() {
		var e evidenceRow
		if err := rows.Scan(&e.BeliefID, &e.SourceURL, &e.ProvenanceClass, &e.ContentSHA256); err != nil {
			log.Printf("evidence: scan: %v", err)
			renderWarmingUp(w)
			return
		}
		evidence = append(evidence, e)
	}
	if err := rows.Err(); err != nil {
		log.Printf("evidence: rows: %v", err)
		renderWarmingUp(w)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	templates.ExecuteTemplate(w, "evidence.html", evidence)
}

type intentRow struct {
	BeliefID string
	Action   string
	State    string
}

func handleIntents(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
	defer cancel()

	rows, err := db.QueryContext(ctx,
		`SELECT belief_id, action, state
		 FROM action_intent WHERE scenario_id = '00000000-0000-0000-0000-000000000002'
		 ORDER BY belief_id`)
	if err != nil {
		log.Printf("intents: query: %v", err)
		renderWarmingUp(w)
		return
	}
	defer rows.Close()

	var intents []intentRow
	for rows.Next() {
		var i intentRow
		if err := rows.Scan(&i.BeliefID, &i.Action, &i.State); err != nil {
			log.Printf("intents: scan: %v", err)
			renderWarmingUp(w)
			return
		}
		intents = append(intents, i)
	}
	if err := rows.Err(); err != nil {
		log.Printf("intents: rows: %v", err)
		renderWarmingUp(w)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	templates.ExecuteTemplate(w, "intents.html", intents)
}

func handleAudit(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
	defer cancel()

	var count int
	err := db.QueryRowContext(ctx,
		`SELECT count(*) FROM action_intent a
		 JOIN belief b ON b.id = a.belief_id
		 WHERE a.state = 'live' AND b.status <> 'promoted' AND a.scenario_id = '00000000-0000-0000-0000-000000000002'`,
	).Scan(&count)
	if err != nil {
		log.Printf("audit: query: %v", err)
		renderWarmingUp(w)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	templates.ExecuteTemplate(w, "audit.html", count)
}

func handleHealth(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()

	type healthResponse struct {
		Status   string `json:"status"`
		Database string `json:"database"`
	}

	err := db.PingContext(ctx)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusServiceUnavailable)
		json.NewEncoder(w).Encode(healthResponse{
			Status:   "warming_up",
			Database: "unreachable",
		})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(healthResponse{
		Status:   "ok",
		Database: "connected",
	})
}

func renderWarmingUp(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusServiceUnavailable)
	templates.ExecuteTemplate(w, "warming_up.html", nil)
}

func loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		log.Printf("%s %s %s", r.Method, r.URL.Path, time.Since(start))
	})
}
