// Package quality implements Bayesian confidence calculations.
package quality

import (
	"math"
)

// BayesianFusion performs Bayesian fusion of multiple benchmark signals.
// Returns (fused_score, overall_confidence)
func BayesianFusion(signals []BenchmarkSignal) (float64, float64) {
	if len(signals) == 0 {
		// No data: return neutral prior (50) with zero confidence
		return 50.0, 0.0
	}

	// Weighted average with confidence weighting
	// score = Σ(value × weight × confidence) / Σ(weight × confidence)
	var numerator float64
	var denominator float64

	for _, sig := range signals {
		w := sig.Weight * sig.Confidence
		numerator += sig.Value * w
		denominator += w
	}

	if denominator == 0 {
		return 50.0, 0.0
	}

	fusedScore := numerator / denominator

	// Overall confidence is based on:
	// 1. Number of signals (more = higher confidence)
	// 2. Average confidence of signals
	// 3. Weight coverage (how much of total weight is covered)
	
	avgConfidence := 0.0
	for _, sig := range signals {
		avgConfidence += sig.Confidence
	}
	avgConfidence /= float64(len(signals))

	// Weight coverage: what fraction of total possible weight is present
	totalPossibleWeight := WeightArenaELO + WeightMMLUPro + WeightIFEval + WeightGSM8K + WeightARC
	weightCoverage := 0.0
	for _, sig := range signals {
		weightCoverage += sig.Weight
	}
	weightCoverage /= totalPossibleWeight

	// Combined confidence: average of signal confidence and weight coverage
	// This penalizes both missing signals AND missing weight
	combinedConfidence := (avgConfidence + weightCoverage) / 2.0

	// Apply a small penalty for having very few signals (encourages caution)
	if len(signals) == 1 {
		combinedConfidence *= 0.7 // 30% penalty for single signal
	} else if len(signals) == 2 {
		combinedConfidence *= 0.85 // 15% penalty for two signals
	}

	return fusedScore, math.Max(0, math.Min(1, combinedConfidence))
}

// CalculateConfidenceInterval returns the confidence interval for a score.
// Returns (lower_bound, upper_bound)
func CalculateConfidenceInterval(score float64, confidence float64) (float64, float64) {
	if confidence <= 0 {
		return 0, 100 // Maximum uncertainty
	}

	// Higher confidence = narrower interval
	// Base interval width: ±20 points at confidence=0, ±2 points at confidence=1
	intervalWidth := 20.0 * (1.0 - confidence) + 2.0
	
	lower := math.Max(0, score-intervalWidth)
	upper := math.Min(100, score+intervalWidth)
	
	return lower, upper
}
