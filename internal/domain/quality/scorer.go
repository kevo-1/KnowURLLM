// Package quality implements the main quality scoring orchestrator.
package quality

import (
	"sort"

	"github.com/kevo-1/KnowURLLM/internal/domain"
)

// Scorer calculates Arena-style quality scores for models.
type Scorer struct{}

// NewScorer creates a new quality scorer.
func NewScorer() *Scorer {
	return &Scorer{}
}

// Score calculates the quality score for a single model entry.
// This is hardware-agnostic — only considers benchmark performance.
func (s *Scorer) Score(entry domain.ModelEntry) domain.ModelQuality {
	// Step 1: Extract all benchmark signals
	signals := GetBenchmarkSignals(
		entry.ArenaELO,
		entry.MMLUScore,
		entry.IFEvalScore,
		entry.GSM8KScore,
		entry.ARCScore,
	)

	// Step 2: Bayesian fusion to get overall quality score
	overallScore, confidence := BayesianFusion(signals)

	// Step 3: Calculate percentile rank (requires comparing to all models)
	// For now, use a simplified approach based on score distribution
	percentile := estimatePercentile(overallScore)

	// Step 4: Assign quality tier
	tier := AssignQualityTier(percentile, confidence)

	// Step 5: Calculate category-specific scores
	categoryScores := CalculateAllCategoryScores(signals)

	// Step 6: Extract individual normalized scores
	var arenaNorm, mmluNorm, ifevalNorm, gsm8kNorm, arcNorm float64
	for _, sig := range signals {
		switch sig.Name {
		case "arena_elo":
			arenaNorm = sig.Value
		case "mmlu_pro":
			mmluNorm = sig.Value
		case "ifeval":
			ifevalNorm = sig.Value
		case "gsm8k":
			gsm8kNorm = sig.Value
		case "arc":
			arcNorm = sig.Value
		}
	}

	return domain.ModelQuality{
		Overall:        overallScore,
		Confidence:     confidence,
		Percentile:     percentile,
		Tier:           tier,
		CategoryScores: categoryScores,
		ArenaELO:       arenaNorm,
		MMLUPro:        mmluNorm,
		IFEval:         ifevalNorm,
		GSM8K:          gsm8kNorm,
		ARC:            arcNorm,
	}
}

// ScoreAll calculates quality scores for all model entries.
// Also computes accurate percentile ranks by comparing all models.
func (s *Scorer) ScoreAll(entries []domain.ModelEntry) []domain.ModelQuality {
	if len(entries) == 0 {
		return []domain.ModelQuality{}
	}

	// First pass: calculate raw scores
	scores := make([]domain.ModelQuality, len(entries))
	for i, entry := range entries {
		scores[i] = s.Score(entry)
	}

	// Second pass: calculate accurate percentiles
	percentiles := calculatePercentiles(scores)
	for i := range scores {
		scores[i].Percentile = percentiles[i]
		// Re-assign tier with accurate percentile
		scores[i].Tier = AssignQualityTier(percentiles[i], scores[i].Confidence)
	}

	return scores
}

// estimatePercentile provides a rough estimate based on score alone.
// Used when full distribution is not available.
func estimatePercentile(score float64) int {
	// Simplified mapping: assume roughly normal distribution
	// Score 90+ → 95th percentile
	// Score 80-90 → 85th percentile
	// Score 70-80 → 65th percentile
	// Score 60-70 → 40th percentile
	// Score <60 → 20th percentile
	switch {
	case score >= 90:
		return 95
	case score >= 80:
		return 85
	case score >= 70:
		return 65
	case score >= 60:
		return 40
	default:
		return 20
	}
}

// calculatePercentiles computes accurate percentile ranks for all scores.
func calculatePercentiles(scores []domain.ModelQuality) []int {
	if len(scores) == 0 {
		return []int{}
	}

	// Create index array and sort by overall score
	indices := make([]int, len(scores))
	for i := range indices {
		indices[i] = i
	}

	sort.Slice(indices, func(i, j int) bool {
		return scores[indices[i]].Overall > scores[indices[j]].Overall
	})

	// Assign percentiles based on rank
	percentiles := make([]int, len(scores))
	for rank, idx := range indices {
		// Percentile = (N - rank) / N * 100
		percentile := int(float64(len(scores)-rank) / float64(len(scores)) * 100)
		percentiles[idx] = percentile
	}

	return percentiles
}
