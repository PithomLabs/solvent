// Package evidence implements the EvidenceService: quality projections,
// provenance tracking, and evidence scoring.
//
// Evidence quality is a projection, not a mutation. The service computes
// quality scores from existing evidence rows without modifying the ledger.
package evidence

import (
	"context"
	"database/sql"
	"fmt"
	"math"
	"time"
)

// QualityScore represents the computed quality of evidence for a belief.
type QualityScore struct {
	BeliefID          string    `json:"belief_id"`
	EvidenceCount     int       `json:"evidence_count"`
	ProvenanceDiversity int     `json:"provenance_diversity"`
	SourceURLCount    int       `json:"source_url_count"`
	HasReproducible   bool      `json:"has_reproducible"`
	HasExternalFeed   bool      `json:"has_external_feed"`
	HasLiveScan       bool      `json:"has_live_scan"`
	HasOperatorAsserted bool   `json:"has_operator_asserted"`
	Score             float64   `json:"score"`
	Grade             string    `json:"grade"`
	ComputedAt        time.Time `json:"computed_at"`
}

// ProvenanceStats aggregates evidence provenance for a scenario.
type ProvenanceStats struct {
	ScenarioID     string         `json:"scenario_id"`
	TotalEvidence  int            `json:"total_evidence"`
	ByClass        map[string]int `json:"by_class"`
	UniqueBeliefs  int            `json:"unique_beliefs"`
	ComputedAt     time.Time      `json:"computed_at"`
}

// Service computes evidence quality projections.
type Service struct {
	db *sql.DB
}

// New creates a new evidence Service.
func New(db *sql.DB) *Service {
	return &Service{db: db}
}

// ComputeQuality calculates a quality score for all evidence attached to a belief.
//
// The scoring algorithm:
//   - Base: 0.2 per evidence item (up to 1.0)
//   - Provenance diversity bonus: +0.1 per distinct provenance class (max +0.4)
//   - Reproducible artifact bonus: +0.2 if present
//   - External feed bonus: +0.1 if present
//   - Operator asserted penalty: -0.1 per operator_asserted (min 0)
//
// Grade: A (>=0.8), B (>=0.6), C (>=0.4), D (>=0.2), F (<0.2)
func (s *Service) ComputeQuality(ctx context.Context, beliefID string) (*QualityScore, error) {
	q := &QualityScore{
		BeliefID:   beliefID,
		ComputedAt: time.Now(),
	}

	rows, err := s.db.QueryContext(ctx, `
		SELECT provenance_class, source_url
		FROM evidence WHERE belief_id = $1::UUID`, beliefID)
	if err != nil {
		return nil, fmt.Errorf("query evidence for quality: %w", err)
	}
	defer rows.Close()

	provenances := make(map[string]bool)
	sourceURLs := make(map[string]bool)

	for rows.Next() {
		var provClass, sourceURL string
		if err := rows.Scan(&provClass, &sourceURL); err != nil {
			return nil, fmt.Errorf("scan evidence row: %w", err)
		}
		q.EvidenceCount++
		provenances[provClass] = true
		if sourceURL != "" {
			sourceURLs[sourceURL] = true
		}
		switch provClass {
		case "reproducible_artifact":
			q.HasReproducible = true
		case "external_feed":
			q.HasExternalFeed = true
		case "live_scan":
			q.HasLiveScan = true
		case "operator_asserted":
			q.HasOperatorAsserted = true
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	q.ProvenanceDiversity = len(provenances)
	q.SourceURLCount = len(sourceURLs)

	// Compute score.
	score := math.Min(float64(q.EvidenceCount)*0.2, 1.0)
	score += math.Min(float64(q.ProvenanceDiversity)*0.1, 0.4)
	if q.HasReproducible {
		score += 0.2
	}
	if q.HasExternalFeed {
		score += 0.1
	}
	if q.HasOperatorAsserted {
		score -= 0.1 * float64(q.EvidenceCount)
	}
	score = math.Max(0, math.Min(1.0, score))
	q.Score = math.Round(score*100) / 100

	switch {
	case q.Score >= 0.8:
		q.Grade = "A"
	case q.Score >= 0.6:
		q.Grade = "B"
	case q.Score >= 0.4:
		q.Grade = "C"
	case q.Score >= 0.2:
		q.Grade = "D"
	default:
		q.Grade = "F"
	}

	return q, nil
}

// ComputeScenarioStats aggregates evidence provenance stats for a scenario.
func (s *Service) ComputeScenarioStats(ctx context.Context, scenarioID string) (*ProvenanceStats, error) {
	stats := &ProvenanceStats{
		ScenarioID: scenarioID,
		ByClass:    make(map[string]int),
		ComputedAt: time.Now(),
	}

	rows, err := s.db.QueryContext(ctx, `
		SELECT provenance_class, count(*) as cnt
		FROM evidence WHERE scenario_id = $1::UUID
		GROUP BY provenance_class`, scenarioID)
	if err != nil {
		return nil, fmt.Errorf("query evidence stats: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var provClass string
		var count int
		if err := rows.Scan(&provClass, &count); err != nil {
			return nil, fmt.Errorf("scan evidence stat: %w", err)
		}
		stats.ByClass[provClass] = count
		stats.TotalEvidence += count
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	err = s.db.QueryRowContext(ctx, `
		SELECT count(DISTINCT belief_id)
		FROM evidence WHERE scenario_id = $1::UUID`, scenarioID).Scan(&stats.UniqueBeliefs)
	if err != nil {
		return nil, fmt.Errorf("count unique beliefs: %w", err)
	}

	return stats, nil
}
