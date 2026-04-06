// Package quality implements category-specific quality scoring.
package quality

import (
	"github.com/kevo-1/KnowURLLM/internal/domain"
)

// Category weights for different use cases
// These determine which benchmarks matter most for each category
var categoryBenchmarkWeights = map[string]map[string]float64{
	"general_chat": {
		"arena_elo": 0.70, // Conversational quality is king
		"mmlu_pro":  0.20, // General knowledge
		"ifeval":    0.10, // Instruction following
	},
	"coding": {
		"arena_elo": 0.40, // General capability
		"mmlu_pro":  0.30, // Technical knowledge
		"gsm8k":     0.30, // Logical reasoning (important for code)
	},
	"reasoning": {
		"gsm8k":     0.40, // Math reasoning (primary signal)
		"arena_elo": 0.30, // General capability
		"mmlu_pro":  0.20, // Knowledge breadth
		"arc":       0.10, // Science reasoning
	},
	"long_context": {
		"arena_elo": 0.50, // General capability
		"mmlu_pro":  0.50, // Knowledge (proxy for context handling)
	},
	"multimodal": {
		"arena_elo": 0.60, // General capability
		"mmlu_pro":  0.40, // Knowledge (vision/language tasks)
	},
}

// CalculateCategoryScore computes the quality score for a specific category.
// Uses category-specific benchmark weights.
func CalculateCategoryScore(category string, signals []BenchmarkSignal) float64 {
	weights, exists := categoryBenchmarkWeights[category]
	if !exists {
		// Fallback to general scoring
		return calculateGeneralScore(signals)
	}

	// Filter signals to those relevant for this category
	var relevantSignals []BenchmarkSignal
	for _, sig := range signals {
		if weight, ok := weights[sig.Name]; ok {
			// Adjust signal weight to category weight
			adjusted := sig
			adjusted.Weight = weight
			relevantSignals = append(relevantSignals, adjusted)
		}
	}

	if len(relevantSignals) == 0 {
		return 50.0 // Neutral default
	}

	// Re-normalize weights to sum to 1.0
	totalWeight := 0.0
	for _, sig := range relevantSignals {
		totalWeight += sig.Weight
	}
	for i := range relevantSignals {
		relevantSignals[i].Weight /= totalWeight
	}

	// Weighted average (simplified Bayesian fusion without confidence penalty)
	var score float64
	var totalW float64
	for _, sig := range relevantSignals {
		score += sig.Value * sig.Weight * sig.Confidence
		totalW += sig.Weight * sig.Confidence
	}

	if totalW == 0 {
		return 50.0
	}

	return score / totalW
}

// calculateGeneralScore computes a general quality score from signals.
func calculateGeneralScore(signals []BenchmarkSignal) float64 {
	if len(signals) == 0 {
		return 50.0
	}

	var score float64
	var totalW float64
	for _, sig := range signals {
		score += sig.Value * sig.Weight * sig.Confidence
		totalW += sig.Weight * sig.Confidence
	}

	if totalW == 0 {
		return 50.0
	}

	return score / totalW
}

// CalculateAllCategoryScores computes scores for all categories.
func CalculateAllCategoryScores(signals []BenchmarkSignal) map[string]float64 {
	categories := []string{
		"general_chat",
		"coding",
		"reasoning",
		"long_context",
		"multimodal",
	}

	scores := make(map[string]float64)
	for _, cat := range categories {
		scores[cat] = CalculateCategoryScore(cat, signals)
	}

	return scores
}

// AssignQualityTier assigns a quality tier based on percentile and confidence.
func AssignQualityTier(percentile int, confidence float64) domain.QualityTier {
	// Base tier from percentile
	var tier domain.QualityTier
	switch {
	case percentile >= 95:
		tier = domain.TierS
	case percentile >= 85:
		tier = domain.TierA
	case percentile >= 65:
		tier = domain.TierB
	case percentile >= 40:
		tier = domain.TierC
	default:
		tier = domain.TierD
	}

	// Conservative adjustment: low confidence pushes down one tier
	if confidence < 0.5 && tier != domain.TierD {
		switch tier {
		case domain.TierS:
			tier = domain.TierA
		case domain.TierA:
			tier = domain.TierB
		case domain.TierB:
			tier = domain.TierC
		case domain.TierC:
			tier = domain.TierD
		}
	}

	return tier
}
