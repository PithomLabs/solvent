package wizard

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"fmt"
	"net/url"
	"strings"

	"github.com/PithomLabs/solvent/kernel"
)

// The refusal text for a contradiction sweep with nothing cited.
//
// The settled contract wrote "no belief_evidence row for needContradictionSweep".
// That names a table which has never existed in this schema — it belongs to a
// superseded design where corpus rows were to live in `evidence`. A user-facing string
// must not assert a table that a judge could then fail to find, so the wording says
// what is missing and what to do about it instead.
const NoCitationDetail = "no citation for needContradictionSweep — select evidence in Search first"

// SpentCitationDetail is the refusal for a retrieval check that has citations to look
// at but none left to claim.
//
// The two retrieval debts are independent obligations. Letting one citation retire both
// would mean the provenance check and the contradiction sweep could be answered by the
// same issue, which is not a sweep -- and the receipts on screen would then name a row
// that did not pay that debt. So each retrieval check consumes its own citation, and
// running out is a refusal rather than a silent reuse.
func SpentCitationDetail(item string) string {
	return "no citation left for " + item +
		" — every selected result is already cited by another check; select another result in Search"
}

// Discharge retires one debt item, recording the artifact that justifies it.
//
// Two kinds of check, and the visible difference between them is a feature:
//
//   - The retrieval-backed pair can only be discharged by something the judge actually
//     retrieved. There is no text box; there is a citation or there is a refusal.
//   - The other four accept a typed artifact and store it as operator-asserted evidence,
//     which is what human judgement looks like on the record.
//
// Idempotent. evidence carries no uniqueness constraint — contrary to two planning
// documents that claim UNIQUE(belief_id, content_sha256) — so re-discharging an
// already-retired item would otherwise silently add a second row. The outstanding-debt
// guard, not the schema, is what prevents that, which is why it is checked first and
// why a test pins it.
func (s *Server) Discharge(ctx context.Context, scenarioID, beliefID, item, artifact string) Verdict {
	if _, known := checkPrompts[item]; !known {
		return Verdict{Statement: StmtDischarge, Detail: fmt.Sprintf("unknown check %q", item)}
	}

	outstanding, err := s.debtOutstanding(ctx, beliefID, item)
	if err != nil {
		return s.refuse(ctx, scenarioID, StmtDischarge, err, "")
	}
	if !outstanding {
		// Already done. Not an error and not a refusal — the end state the caller asked
		// for is the end state that holds.
		return Verdict{OK: true, Statement: StmtDischarge, Detail: item + " already discharged"}
	}

	if _, isRetrieval := retrievalChecks[item]; isRetrieval {
		return s.dischargeFromCitation(ctx, scenarioID, beliefID, item)
	}
	return s.dischargeFromArtifact(ctx, scenarioID, beliefID, item, artifact)
}

// dischargeFromCitation requires a citation the judge selected in ASK.
//
// This is the third refusal, and the one that proves retrieval is wired to authority
// rather than decorating it. The other two refusals would still fire if retrieval were
// a static list of fixtures; this one cannot.
func (s *Server) dischargeFromCitation(ctx context.Context, scenarioID, beliefID, item string) Verdict {
	var n int
	if err := s.db.QueryRowContext(ctx, `
		SELECT count(*) FROM belief_corpus_citation
		 WHERE belief_id = $1::UUID AND relation = 'considered'`, beliefID).Scan(&n); err != nil {
		return s.refuse(ctx, scenarioID, StmtDischarge, err, "")
	}
	if n == 0 {
		// A refusal the application raises, not the database — and it says so. The
		// SQLSTATE is the check-violation code because that is what the condition is,
		// but the verdict carries no constraint name precisely because no constraint
		// produced it. Claiming one would be inventing engine output.
		v := Verdict{Statement: StmtDischarge, SQLState: "23514", Detail: NoCitationDetail}
		if err := s.logRefusal(ctx, scenarioID, StmtDischarge, v); err != nil {
			v.Detail = fmt.Sprintf("%s (refusal_log write failed: %v)", v.Detail, err)
		}
		return v
	}

	// One citation per retrieval debt. The caller has already established that `item`
	// is still outstanding, so every retrieval check already retired has spoken for a
	// citation, and this one needs a citation beyond those.
	spent, err := s.retrievalDebtsRetired(ctx, beliefID)
	if err != nil {
		return s.refuse(ctx, scenarioID, StmtDischarge, err, "")
	}
	if n <= spent {
		v := Verdict{Statement: StmtDischarge, SQLState: "23514", Detail: SpentCitationDetail(item)}
		if err := s.logRefusal(ctx, scenarioID, StmtDischarge, v); err != nil {
			v.Detail = fmt.Sprintf("%s (refusal_log write failed: %v)", v.Detail, err)
		}
		return v
	}

	if err := s.kern.RetireDebt(ctx, beliefID, item); err != nil {
		return s.refuse(ctx, scenarioID, StmtDischarge, err, "")
	}
	return Verdict{OK: true, Statement: StmtDischarge, Detail: item + " discharged from citation"}
}

// dischargeFromArtifact stores a typed artifact as operator-asserted evidence.
//
// The debt item is carried in source_url behind a `solvent:` scheme. evidence has no
// column saying which check a row discharged, and adding one would be a schema change
// this phase does not have; an internal reference keeps the mapping inspectable in
// plain SQL and is honest about not being fetchable. `source_url` is nullable, so
// nothing is invented — the alternative would be a fake https URL, which the dataset
// review explicitly forbids for operator-asserted rows.
func (s *Server) dischargeFromArtifact(ctx context.Context, scenarioID, beliefID, item, artifact string) Verdict {
	artifact = strings.TrimSpace(artifact)
	if artifact == "" {
		v := Verdict{
			Statement: StmtDischarge,
			SQLState:  "23514",
			Detail:    fmt.Sprintf("%s needs an artifact — %s", item, checkPrompts[item]),
		}
		if err := s.logRefusal(ctx, scenarioID, StmtDischarge, v); err != nil {
			v.Detail = fmt.Sprintf("%s (refusal_log write failed: %v)", v.Detail, err)
		}
		return v
	}

	sum := sha256.Sum256([]byte(artifact))
	ref := "solvent:check/" + item + "/" + url.PathEscape(artifact)
	if err := s.kern.AddEvidence(ctx, scenarioID, beliefID,
		"operator_asserted", ref, hex.EncodeToString(sum[:])); err != nil {
		return s.refuse(ctx, scenarioID, StmtDischarge, err, "")
	}
	if err := s.kern.RetireDebt(ctx, beliefID, item); err != nil {
		return s.refuse(ctx, scenarioID, StmtDischarge, err, "")
	}
	return Verdict{OK: true, Statement: StmtDischarge, Detail: item + " discharged"}
}

// artifactFromRef recovers what the judge typed from the internal reference.
func artifactFromRef(ref string) string {
	const prefix = "solvent:check/"
	if !strings.HasPrefix(ref, prefix) {
		return ""
	}
	rest := ref[len(prefix):]
	slash := strings.Index(rest, "/")
	if slash < 0 {
		return ""
	}
	got, err := url.PathUnescape(rest[slash+1:])
	if err != nil {
		return ""
	}
	return got
}

// retrievalDebtsRetired counts how many of the two retrieval checks are already
// discharged, which is how many citations are already spoken for.
//
// It reads the debt array rather than the evidence table because a citation-backed
// discharge writes no evidence row -- the citation IS the record. The same unnest
// checks() uses, so the count and the receipts can never disagree.
func (s *Server) retrievalDebtsRetired(ctx context.Context, beliefID string) (int, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT unnest(debt) FROM belief WHERE id = $1::UUID`, beliefID)
	if err != nil {
		return 0, fmt.Errorf("wizard: read debt for retrieval count: %w", err)
	}
	defer func() { _ = rows.Close() }()

	outstanding := map[string]bool{}
	for rows.Next() {
		var d sql.NullString
		if err := rows.Scan(&d); err != nil {
			return 0, fmt.Errorf("wizard: scan debt for retrieval count: %w", err)
		}
		if d.Valid {
			outstanding[d.String] = true
		}
	}
	if err := rows.Err(); err != nil {
		return 0, err
	}

	n := 0
	for item := range retrievalChecks {
		if !outstanding[item] {
			n++
		}
	}
	return n, nil
}

func (s *Server) debtOutstanding(ctx context.Context, beliefID, item string) (bool, error) {
	var yes bool
	err := s.db.QueryRowContext(ctx, `
		SELECT $2::STRING = ANY(debt) FROM belief WHERE id = $1::UUID`, beliefID, item).Scan(&yes)
	if err != nil {
		return false, fmt.Errorf("wizard: read debt for %s: %w", item, err)
	}
	return yes, nil
}

// FullDebtNames is kernel.FullDebt, exposed so a template can render the six rows in
// the database's own order without importing the kernel.
func FullDebtNames() []string {
	out := make([]string, len(kernel.FullDebt))
	copy(out, kernel.FullDebt)
	return out
}
