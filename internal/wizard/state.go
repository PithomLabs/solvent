package wizard

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/PithomLabs/solvent/internal/corpus"
	"github.com/PithomLabs/solvent/kernel"
)

// Screen names, as the contract's stepper spells them.
const (
	ScreenAsk       = "ASK"
	ScreenDischarge = "DISCHARGE"
	ScreenFalsify   = "FALSIFY"
)

// Check is one row of the six-item debt list.
//
// Item is the debt string the database issued and the only thing a discharge request
// may name. Name is what the judge reads, and since Phase 5 replaced the debt
// vocabulary they are the same string -- the field is kept so a future divergence has
// somewhere to go and so the frontend never guesses which one to post.
// ReceiptIssue and ReceiptDistance carry the citation behind a retrieval receipt, so a
// caller that needs the values rather than the rendered string does not have to parse
// Receipt back apart. Screen 3's closing sentence needs exactly that, and reconstructing
// it from S.citations is what made that sentence name the wrong issue: "the first
// considered citation" is the provenance check's, not the sweep's.
type Check struct {
	Item            string  `json:"item"`
	Name            string  `json:"name"`
	Prompt          string  `json:"prompt"`
	Done            bool    `json:"done"`
	Receipt         string  `json:"receipt,omitempty"`
	ReceiptIssue    int     `json:"receipt_issue,omitempty"`
	ReceiptDistance float64 `json:"receipt_distance,omitempty"`
	Retrieval       bool    `json:"retrieval"`
}

// Citation is a selected corpus row on the record.
type Citation struct {
	BeliefID    string  `json:"belief_id"`
	CorpusID    string  `json:"corpus_id"`
	IssueNumber int     `json:"issue_number"`
	Title       string  `json:"title"`
	URL         string  `json:"url"`
	Distance    float64 `json:"distance"`
	QueryText   string  `json:"query_text"`
	Relation    string  `json:"relation"`
}

// Refusal is one logged refusal.
type Refusal struct {
	Statement  string `json:"statement"`
	SQLState   string `json:"sqlstate"`
	Constraint string `json:"constraint,omitempty"`
	Detail     string `json:"detail,omitempty"`
	LoggedAt   string `json:"logged_at"`
}

// State is the whole of what the frontend knows. There is no other source.
type State struct {
	Screen         string     `json:"screen"`
	Seeded         bool       `json:"seeded"`
	ScenarioID     string     `json:"scenario_id"`
	Claim          string     `json:"claim"`
	BeliefID       string     `json:"belief_id"`
	Status         string     `json:"status"`
	Debt           int        `json:"debt"`
	Ancestor       string     `json:"ancestor_claim"`
	AncestorStatus string     `json:"ancestor_status"`
	Checks         []Check    `json:"checks"`
	Citations      []Citation `json:"citations"`
	Refusals       []Refusal  `json:"refusals"`
	Intent         string     `json:"intent,omitempty"`
	IntentState    string     `json:"intent_state,omitempty"`
	Audit          int        `json:"live_on_nonpromoted"`
	CorpusRows     int        `json:"corpus_rows"`
}

// checkPrompts is the contract's copy deck, in the contract's order. The order is
// kernel.FullDebt's, so the six rows on screen always match the six the database
// defaulted the belief to.
var checkPrompts = map[string]string{
	"needProvenanceCheck":    "cite a retrieved issue",
	"needContradictionSweep": "cite a retrieved issue",
	"needBlastRadius":        "name affected components",
	"needRollbackPlan":       "link a runbook",
	"needVersionPin":         "state a version range",
	"needOperatorSignoff":    "record a handle",
}

// retrievalChecks are the two checks a judge cannot discharge by typing.
//
// Before Phase 5 this was a translation map: the contract's vocabulary and the
// schema's disagreed, so the UI displayed both names. db/004_debt_vocabulary.sql
// replaced the debt strings for real, so the two vocabularies are now one and the
// alias layer is gone -- a set is all that is left of it.
var retrievalChecks = map[string]bool{
	"needProvenanceCheck":    true,
	"needContradictionSweep": true,
}

// ContradictionCheck is the debt item that requires a citation and produces the third
// refusal when none exists.
const ContradictionCheck = "needContradictionSweep"

// ProjectedState is what screen 1 looks like before anything has been written.
//
// It exists so that reading the wizard is not the same as using it. A cookie-less GET
// used to mint and seed a scenario, which meant a crawler, an uptime monitor or a link
// preview wrote 2 beliefs, 1 edge and 1 evidence row into the ledger -- observed live,
// twice, on great-goat. Now nothing is written until the judge acts.
//
// The projection is honest rather than decorative: every value on screen 1 is a
// deterministic constant of Seed, so this returns exactly what Seed would have produced
// -- the same claims, the same status, the same six debt items in the same order, and a
// real corpus count read from the database. The page a judge sees before their first
// click is therefore identical to the page after it, and W30 asserts that equality
// rather than trusting this comment.
//
// The one thing it cannot project is an id, because no row exists to have one. BeliefID
// and ScenarioID stay empty, Seeded is false, and every mutating endpoint routes through
// resolve, which creates for real before it writes.
func (s *Server) ProjectedState(ctx context.Context) (State, error) {
	st := State{
		Seeded:         false,
		Claim:          DescendantClaim,
		Status:         "entered",
		Debt:           len(kernel.FullDebt),
		Ancestor:       AncestorClaim,
		AncestorStatus: "promoted",
		Citations:      []Citation{},
		Refusals:       []Refusal{},
	}
	for _, item := range kernel.FullDebt {
		st.Checks = append(st.Checks, Check{
			Item:      item,
			Name:      item,
			Prompt:    checkPrompts[item],
			Retrieval: retrievalChecks[item],
		})
	}
	// Read-only, and the only query on this path.
	if err := s.db.QueryRowContext(ctx,
		`SELECT count(*) FROM corpus_issue WHERE scenario_id = $1::UUID`, s.corpus).Scan(&st.CorpusRows); err != nil {
		return st, fmt.Errorf("wizard: count corpus: %w", err)
	}
	st.Screen = deriveScreen(st)
	return st, nil
}

// State computes the screen from committed state. Nothing here reads a cookie, a
// session, or anything the browser sent beyond the scenario id.
//
// An unseeded scenario is not an error: it returns ProjectedState. A cookie naming a
// scenario that was destroyed out from under it therefore degrades to screen 1 rather
// than to a 500, and the judge's next action repairs it under a fresh id.
func (s *Server) State(ctx context.Context, scenarioID string) (State, error) {
	var st State
	st.ScenarioID = scenarioID
	st.Seeded = true

	// The belief under examination is the CHILD of the scenario's single edge.
	//
	// Resolving it by claim text would work today and break the moment a claim is
	// reworded; resolving it as "the belief" is ambiguous, because the scenario holds
	// two and one of them is seeded promoted. The edge is the structural answer: the
	// ancestor is the standing assumption, the descendant is what the judge drives.
	err := s.db.QueryRowContext(ctx, `
		SELECT c.id::STRING, c.claim, c.status, coalesce(array_length(c.debt, 1), 0), p.claim, p.status
		  FROM belief_edge e
		  JOIN belief p ON p.id = e.parent_id
		  JOIN belief c ON c.id = e.child_id
		 WHERE p.scenario_id = $1::UUID AND c.scenario_id = $1::UUID
		 ORDER BY e.filed_at
		 LIMIT 1`, scenarioID).Scan(&st.BeliefID, &st.Claim, &st.Status, &st.Debt, &st.Ancestor, &st.AncestorStatus)
	if err == sql.ErrNoRows {
		// Unseeded, or seeded and since destroyed. Either way the judge gets screen 1,
		// and the scenario id is deliberately dropped: whatever that cookie names is not
		// usable, so the next POST mints a fresh one rather than writing into a ghost.
		return s.ProjectedState(ctx)
	}
	if err != nil {
		return st, fmt.Errorf("wizard: read belief: %w", err)
	}

	// Scenario-wide, not belief-scoped: the contradiction introduced on screen 3
	// attaches to the ANCESTOR, and the screen has to be able to show it.
	if st.Citations, err = s.citations(ctx, scenarioID); err != nil {
		return st, err
	}
	if st.Refusals, err = s.refusals(ctx, scenarioID); err != nil {
		return st, err
	}
	if st.Checks, err = s.checks(ctx, st.BeliefID, st.Citations); err != nil {
		return st, err
	}

	// The live intent, if the judge has authorised one.
	var action, state sql.NullString
	if err := s.db.QueryRowContext(ctx, `
		SELECT action, state FROM action_intent
		 WHERE scenario_id = $1::UUID AND belief_id = $2::UUID
		 ORDER BY state = 'live' DESC
		 LIMIT 1`, scenarioID, st.BeliefID).Scan(&action, &state); err != nil && err != sql.ErrNoRows {
		return st, fmt.Errorf("wizard: read intent: %w", err)
	}
	st.Intent, st.IntentState = action.String, state.String

	if st.Audit, err = s.kern.AuditLiveOnNonPromoted(ctx, scenarioID); err != nil {
		return st, fmt.Errorf("wizard: audit: %w", err)
	}
	if err := s.db.QueryRowContext(ctx,
		`SELECT count(*) FROM corpus_issue WHERE scenario_id = $1::UUID`, s.corpus).Scan(&st.CorpusRows); err != nil {
		return st, fmt.Errorf("wizard: count corpus: %w", err)
	}

	// Empty lists must serialise as [] and never as null.
	//
	// A nil slice marshals to `null`, and the browser does `S.refusals.length` on it —
	// which throws a TypeError and blanks the screen on exactly the state a judge sees
	// first, a freshly seeded scenario with nothing refused and nothing cited. Caught by
	// driving the real endpoint rather than by a Go test using maps.
	if st.Citations == nil {
		st.Citations = []Citation{}
	}
	if st.Refusals == nil {
		st.Refusals = []Refusal{}
	}
	if st.Checks == nil {
		st.Checks = []Check{}
	}

	st.Screen = deriveScreen(st)
	return st, nil
}

// deriveScreen is the contract's table, first match wins.
//
// Kept as a pure function of State so it can be reasoned about and tested without a
// database, and so the ordering is visible in one place rather than spread across
// queries.
func deriveScreen(st State) string {
	hasContradiction := false
	for _, c := range st.Citations {
		if c.Relation == "contradicts" {
			hasContradiction = true
			break
		}
	}
	switch {
	case hasContradiction || st.Status == "retracted":
		return ScreenFalsify
	case st.Status == "promoted" && st.IntentState == "live":
		return ScreenFalsify
	case st.Status == "entered" && len(st.Refusals) > 0:
		return ScreenDischarge
	case st.Status == "entered":
		return ScreenAsk
	default:
		// Promoted with no live intent: the judge promoted but has not authorised yet.
		// That is still DISCHARGE's job — the Authorize button lives there.
		return ScreenDischarge
	}
}

// citations reads a scenario's citations in the order the judge selected them.
//
// The ordering is load-bearing, not cosmetic. checks() hands one citation to each
// retired retrieval check by position, so "which citation discharged which debt" is
// answered by selection order — the first thing you chose retires the first retrieval
// debt. retrieved_at is what records that, and corpus.Cite refreshes it when a
// selection is toggled, so re-picking evidence re-orders it honestly. Ordering by
// distance instead, as this did before, made the nearest hit the answer for every
// retrieval check at once.
func (s *Server) citations(ctx context.Context, scenarioID string) ([]Citation, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT c.belief_id::STRING, c.corpus_id::STRING, i.issue_number, i.title, i.url,
		       c.distance, c.query_text, c.relation
		  FROM belief_corpus_citation c
		  JOIN corpus_issue i ON i.id = c.corpus_id
		  JOIN belief b       ON b.id = c.belief_id
		 WHERE b.scenario_id = $1::UUID
		 ORDER BY c.relation, c.retrieved_at, c.distance, c.corpus_id`, scenarioID)
	if err != nil {
		return nil, fmt.Errorf("wizard: read citations: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var out []Citation
	for rows.Next() {
		var c Citation
		if err := rows.Scan(&c.BeliefID, &c.CorpusID, &c.IssueNumber, &c.Title, &c.URL, &c.Distance, &c.QueryText, &c.Relation); err != nil {
			return nil, fmt.Errorf("wizard: scan citation: %w", err)
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

func (s *Server) checks(ctx context.Context, beliefID string, cites []Citation) ([]Check, error) {
	var debt []sql.NullString
	rows, err := s.db.QueryContext(ctx,
		`SELECT unnest(debt) FROM belief WHERE id = $1::UUID`, beliefID)
	if err != nil {
		return nil, fmt.Errorf("wizard: read debt: %w", err)
	}
	defer func() { _ = rows.Close() }()
	for rows.Next() {
		var d sql.NullString
		if err := rows.Scan(&d); err != nil {
			return nil, fmt.Errorf("wizard: scan debt: %w", err)
		}
		debt = append(debt, d)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	outstanding := map[string]bool{}
	for _, d := range debt {
		if d.Valid {
			outstanding[d.String] = true
		}
	}

	// Receipts for retired retrieval checks come from the citations themselves, one
	// citation per check.
	//
	// This used to keep a single "first considered citation" and print it against both
	// retrieval checks, which made two independent debts look as though one issue had
	// discharged both. It had not: dischargeFromCitation now requires an unconsumed
	// citation per check, and this walks the same citations in the same order so the
	// receipt on screen names the row that actually paid the debt.
	considered := make([]Citation, 0, len(cites))
	for _, c := range cites {
		if c.BeliefID != beliefID {
			continue // the ancestor's contradiction is not this belief's receipt
		}
		if c.Relation == corpus.RelationConsidered {
			considered = append(considered, c)
		}
	}

	next := 0
	out := make([]Check, 0, len(kernel.FullDebt))
	for _, item := range kernel.FullDebt {
		c := Check{Item: item, Name: item, Prompt: checkPrompts[item], Done: !outstanding[item]}
		if retrievalChecks[item] {
			c.Retrieval = true
			if c.Done && next < len(considered) {
				cit := considered[next]
				next++
				// The stored distance, as measured when the judge selected it. Never
				// recomputed here -- a receipt that drifts from the row it cites is
				// worse than no receipt.
				c.Receipt = fmt.Sprintf("#%d · %.6f", cit.IssueNumber, cit.Distance)
				c.ReceiptIssue, c.ReceiptDistance = cit.IssueNumber, cit.Distance
			}
		} else if c.Done {
			if r, err := s.operatorReceipt(ctx, beliefID, item); err == nil && r != "" {
				c.Receipt = r
			}
		}
		out = append(out, c)
	}
	return out, nil
}

// operatorReceipt recovers the artifact a judge typed for an operator check.
//
// evidence has no column for "which debt item this discharged", so the item name is
// carried in source_url behind the solvent: scheme — an internal reference, never a
// fetchable URL. That keeps the mapping inspectable in SQL without a schema change.
func (s *Server) operatorReceipt(ctx context.Context, beliefID, item string) (string, error) {
	var ref string
	err := s.db.QueryRowContext(ctx, `
		SELECT source_url FROM evidence
		 WHERE belief_id = $1::UUID AND provenance_class = 'operator_asserted'
		   AND source_url LIKE $2
		 ORDER BY ingested_at DESC LIMIT 1`, beliefID, "solvent:check/"+item+"/%").Scan(&ref)
	if err != nil {
		return "", err
	}
	return artifactFromRef(ref), nil
}
