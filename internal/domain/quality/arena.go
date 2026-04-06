// Package quality implements Arena-style Bayesian fusion quality scoring.
package quality

import (
	"math"

	"github.com/kevo-1/KnowURLLM/internal/domain"
)

// Arena ELO normalization constants
const (
	// EloMin and EloMax define the expected range of Arena ELO scores
	EloMin = 800.0
	EloMax = 1350.0

	// EloMinNormalized and EloMaxNormalized define the output range
	EloMinNormalized = 0.0
	EloMaxNormalized = 100.0
)

// normalizeELO normalizes Arena ELO from [800, 1350] to [0, 100]
func normalizeELO(elo float64) float64 {
	if elo <= 0 {
		return 0 // Missing data
	}
	
	normalized := (elo - EloMin) / (EloMax - EloMin) * (EloMaxNormalized - EloMinNormalized) + EloMinNormalized
	
	// Clamp to [0, 100]
	return math.Max(0, math.Min(100, normalized))
}

// eloConfidence returns the confidence in an ELO score based on vote count
// More votes = higher confidence (narrower confidence interval)
func eloConfidence(numVotes int) float64 {
	if numVotes <= 0 {
		return 0
	}
	
	// Confidence grows logarithmically with vote count
	// At 100 votes: ~0.7 confidence
	// At 1000 votes: ~0.9 confidence
	// At 10000 votes: ~0.98 confidence
	confidence := math.Log10(float64(numVotes)+1) / math.Log10(10001)
	return math.Max(0, math.Min(1, confidence))
}

// CalculateArenaQuality computes the quality score from Arena ELO
// Returns (normalized_score, confidence)
func CalculateArenaQuality(entry domain.ModelEntry) (float64, float64) {
	normalized := normalizeELO(entry.ArenaELO)
	
	// Confidence based on whether ELO exists
	confidence := 0.0
	if entry.ArenaELO > 0 {
		confidence = 0.95 // High confidence for Arena data (gold standard)
	}
	
	return normalized, confidence
}
