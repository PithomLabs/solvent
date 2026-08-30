package wizard

import (
	"context"
	"database/sql"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/PithomLabs/solvent/internal/corpus"
	"github.com/PithomLabs/solvent/internal/demoseed"
)

// issueURLRe is the server-side allowlist for the one input on screen 3.
//
// Anchored, etcd-only, digits only. A judge types a URL and the server must not treat
// that as licence to fetch anything: the issue is looked up in the corpus we already
// ingested, so the allowlist is about refusing to *interpret* arbitrary input, not
// about sanitising a request we were going to make. There is no HTTP client anywhere in
// this path.
var issueURLRe = regexp.MustCompile(`^https://github\.com/etcd-io/etcd/issues/(\d+)$`)

// ErrNotAnIssueURL is the inline message the contract specifies.
var ErrNotAnIssueURL = fmt.Errorf("Enter an etcd issue URL")

// Retract modes.
const (
	// ModeUnsafe is the naive thing a normal application does: retract the belief.
	ModeUnsafe = "unsafe"
	// ModeSafe is cancel-then-retract, which is what the kernel offers.
	ModeSafe = "safe"
)

// Introduction is what screen 3 renders after a judge introduces an issue.
type Introduction struct {
	IssueNumber  int     `json:"issue_number"`
	Title        string  `json:"title"`
	URL          string  `json:"url"`
	State        string  `json:"state"`
	ClosedAt     string  `json:"closed_at,omitempty"`
	Distance     float64 `json:"distance"`
	Relation     string  `json:"relation"`
	AgainstClaim string  `json:"against_claim"`
}

// CascadeRefusal reports whether v is the Tier 1 refusal specifically.
//
// # Why both fields have to match
//
// 23514 is "check constraint violated" — it does not say *which*. This schema has two
// that a judge can reach: promoted_is_debt_free, which is screen 1's refusal, and
// live_requires_promoted, which is screen 3's. Classifying on the code alone would let
// a debt-free violation be narrated as the cascade refusal, which is the demo's climax
// attributed to the wrong cause. The constraint name is the discriminator, and pgx
// carries it as a struct field rather than in the error text.
func (v Verdict) CascadeRefusal() bool {
	return !v.OK && v.SQLState == "23514" && v.Constraint == "live_requires_promoted"
}

// Introduce resolves an etcd issue URL against the corpus and records it as a
// contradiction of the belief.
//
// Deliberately explicit. Phase 4 §7 measured that the deployment question does not
// surface the integrity incident, and this belief's own claim is worse: #13766 sits
// 573rd of 7,239 against "etcd v3.5.x has no known data-integrity defects". Retrieval
// was never going to hand the judge this issue, so the judge introduces it — and the
// distance is shown honestly, large, rather than hidden or reframed.
func (s *Server) Introduce(ctx context.Context, scenarioID, beliefID, issueURL string) (Introduction, error) {
	var out Introduction

	m := issueURLRe.FindStringSubmatch(strings.TrimSpace(issueURL))
	if m == nil {
		return out, ErrNotAnIssueURL
	}
	number, err := strconv.Atoi(m[1])
	if err != nil {
		return out, ErrNotAnIssueURL
	}

	// Resolved from the corpus partition, never fetched.
	var corpusID, closedAt sql.NullString
	err = s.db.QueryRowContext(ctx, `
		SELECT id::STRING, issue_number, title, url, state, closed_at::DATE::STRING
		  FROM corpus_issue
		 WHERE scenario_id = $1::UUID AND issue_number = $2`,
		s.corpus, number).Scan(&corpusID, &out.IssueNumber, &out.Title, &out.URL, &out.State, &closedAt)
	if err == sql.ErrNoRows {
		return out, fmt.Errorf("issue #%d is not in the ingested corpus", number)
	}
	if err != nil {
		return out, fmt.Errorf("wizard: resolve issue #%d: %w", number, err)
	}
	out.ClosedAt = closedAt.String

	// The contradiction attaches to the ANCESTOR, not to the belief the judge drove.
	//
	// #13766 refutes "etcd v3.5.x has no known data-integrity defects" on its face. It
	// says nothing directly about whether upgrading is safe — that claim is only wrong
	// *because* the assumption under it is. Citing the descendant would put the evidence
	// one hop from what it actually contradicts, and would measure the distance against
	// the wrong sentence.
	//
	// It is also what makes the next step Tier 1 rather than Tier 2: the belief being
	// retracted is the ancestor, and the refusal comes from the traversal reaching the
	// descendant's live intent.
	ancestorID, err := s.ancestorOf(ctx, scenarioID)
	if err != nil {
		return out, err
	}
	var claim string
	if err := s.db.QueryRowContext(ctx,
		`SELECT claim FROM belief WHERE id = $1::UUID`, ancestorID).Scan(&claim); err != nil {
		return out, fmt.Errorf("wizard: read ancestor claim: %w", err)
	}
	out.AgainstClaim = claim

	dist, err := s.distanceToClaim(ctx, claim, corpusID.String)
	if err != nil {
		return out, err
	}
	out.Distance = dist
	out.Relation = corpus.RelationContradicts

	if err := corpus.Cite(ctx, s.db, ancestorID, corpusID.String, dist, claim, corpus.RelationContradicts); err != nil {
		return out, err
	}
	return out, nil
}

// distanceToClaim measures one corpus row against one claim.
//
// The claim is embedded with the same model and parameters as the corpus, and the
// distance uses `<=>` — cosine, matching the index's vector_cosine_ops. This is a
// targeted measurement rather than a ranked search: the row is already chosen, so
// there is nothing to rank, and going through the ANN path would be answering a
// different question.
func (s *Server) distanceToClaim(ctx context.Context, claim, corpusID string) (float64, error) {
	vec, cached := s.queries.get(claim)
	if !cached {
		emb, err := s.embed(ctx)
		if err != nil {
			return 0, err
		}
		v, _, err := emb.Embed(ctx, claim)
		if err != nil {
			return 0, fmt.Errorf("embed claim: %w", err)
		}
		vec = v
		s.queries.put(claim, v)
	}
	lit, err := corpus.Encode(vec)
	if err != nil {
		return 0, err
	}
	var d sql.NullFloat64
	if err := s.db.QueryRowContext(ctx, `
		SELECT embedding <=> $2::VECTOR FROM corpus_issue WHERE id = $1::UUID`,
		corpusID, lit).Scan(&d); err != nil {
		return 0, fmt.Errorf("wizard: measure distance: %w", err)
	}
	if !d.Valid {
		return 0, fmt.Errorf("wizard: issue has no embedding")
	}
	return d.Float64, nil
}

// Retract attempts to retract the ANCESTOR, in one of two ways.
//
// # The two paths are not two spellings of one operation
//
// mode=unsafe issues demoseed.SQLRetractWithoutCancel: a single UPDATE over the
// subtree, which is what a normal application writes. The gate FK propagates the new
// status into every dependent intent row and live_requires_promoted refuses. That
// refusal is the demo's climax.
//
// mode=safe calls kernel.RetractCascade, which cancels strictly before retracting
// (I-8). It therefore CANNOT produce the refusal — it is safe by construction. Routing
// the first button through the kernel would make Tier 1 look broken when it is not,
// and that mistake has already been made once during this project.
//
// The traversal in both paths is a WITH RECURSIVE CTE in application code.
// CockroachDB does not traverse belief_edge. What the database enforces is that the
// traversal cannot finish having left a live intent behind.
func (s *Server) Retract(ctx context.Context, scenarioID, beliefID, mode string) Verdict {
	ancestorID, err := s.ancestorOf(ctx, scenarioID)
	if err != nil {
		return Verdict{Statement: StmtRetractUnsafe, Detail: err.Error()}
	}

	switch mode {
	case ModeUnsafe:
		if _, err := s.db.ExecContext(ctx, demoseed.SQLRetractWithoutCancel, ancestorID, scenarioID); err != nil {
			v := s.refuse(ctx, scenarioID, StmtRetractUnsafe, err, "")
			if !v.CascadeRefusal() {
				// A refusal, but not the one this button exists to produce. Say so rather
				// than letting the UI narrate the climax over the wrong constraint.
				v.Detail = fmt.Sprintf("unexpected refusal on the unsafe retract path: %s · %s — %s",
					v.SQLState, v.Constraint, v.Detail)
			}
			return v
		}
		// Reaching here means the database allowed a retraction that orphaned a live
		// intent. That is the enforcement failing, not the demo succeeding.
		return Verdict{
			Statement: StmtRetractUnsafe,
			Detail: "the unsafe retract COMMITTED — live_requires_promoted did not fire, " +
				"which contradicts proof/act6_tier_probe.log",
		}

	case ModeSafe:
		n, err := s.kern.RetractCascade(ctx, scenarioID, ancestorID)
		if err != nil {
			return s.refuse(ctx, scenarioID, StmtRetractUnsafe, err, "cancel-then-retract was refused")
		}
		return Verdict{
			OK:        true,
			Statement: "retract_safe",
			Detail:    fmt.Sprintf("COMMIT · %d belief(s) retracted, intent cancelled", n),
		}

	default:
		return Verdict{Statement: StmtRetractUnsafe, Detail: fmt.Sprintf("unknown retract mode %q", mode)}
	}
}

// ancestorOf returns the parent of the scenario's single edge — the belief screen 3
// retracts. Resolved structurally, like State does, not by claim text.
func (s *Server) ancestorOf(ctx context.Context, scenarioID string) (string, error) {
	var id string
	err := s.db.QueryRowContext(ctx, `
		SELECT p.id::STRING
		  FROM belief_edge e
		  JOIN belief p ON p.id = e.parent_id
		  JOIN belief c ON c.id = e.child_id
		 WHERE p.scenario_id = $1::UUID AND c.scenario_id = $1::UUID
		 ORDER BY e.filed_at
		 LIMIT 1`, scenarioID).Scan(&id)
	if err == sql.ErrNoRows {
		return "", fmt.Errorf("scenario %s has no derivation edge to retract along", scenarioID)
	}
	if err != nil {
		return "", fmt.Errorf("wizard: read ancestor: %w", err)
	}
	return id, nil
}
