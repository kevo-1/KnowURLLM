// Package quality implements benchmark normalization utilities.
package quality

import (
	"math"
)

// Benchmark signal weights for Bayesian fusion
const (
	WeightArenaELO  = 0.50 // LMSYS Chatbot Arena (gold standard)
	WeightMMLUPro   = 0.30 // MMLU-PRO (strong academic benchmark)
	WeightIFEval    = 0.10 // Instruction Following
	WeightGSM8K     = 0.10 // Math Reasoning
	WeightARC       = 0.10 // Science QA (used if available, redistributes weight)

	// Confidence levels for different benchmark types
	ConfidenceArenaELO = 0.95
	ConfidenceMMLUPro  = 0.85
	ConfidenceIFEval   = 0.60
	ConfidenceGSM8K    = 0.60
	ConfidenceARC      = 0.60
)

// normalizeMMLU normalizes MMLU-PRO score (already 0-100, just validate)
func normalizeMMLU(score float64) float64 {
	if score <= 0 {
		return 0
	}
	return math.Max(0, math.Min(100, score))
}

// normalizeIFEval normalizes IFEval score to 0-100
func normalizeIFEval(score float64) float64 {
	if score <= 0 {
		return 0
	}
	// IFEval is typically reported as percentage already
	return math.Max(0, math.Min(100, score))
}

// normalizeGSM8K normalizes GSM8K score to 0-100
func normalizeGSM8K(score float64) float64 {
	if score <= 0 {
		return 0
	}
	// GSM8K accuracy is typically 0-100
	return math.Max(0, math.Min(100, score))
}

// normalizeARC normalizes ARC-Challenge score to 0-100
func normalizeARC(score float64) float64 {
	if score <= 0 {
		return 0
	}
	// ARC-Challenge accuracy is typically 0-100
	return math.Max(0, math.Min(100, score))
}

// BenchmarkSignal represents a single benchmark signal for Bayesian fusion
type BenchmarkSignal struct {
	Name       string  // "arena_elo", "mmlu_pro", etc.
	Value      float64 // Normalized score (0-100)
	Weight     float64 // Relative importance (0-1)
	Confidence float64 // Reliability of this signal (0-1)
}

// GetBenchmarkSignals extracts all available benchmark signals from a model entry
func GetBenchmarkSignals(
	arenaELO float64,
	mmluPro float64,
	ifeval float64,
	gsm8k float64,
	arc float64,
) []BenchmarkSignal {
	signals := []BenchmarkSignal{}

	if arenaELO > 0 {
		signals = append(signals, BenchmarkSignal{
			Name:       "arena_elo",
			Value:      normalizeELO(arenaELO),
			Weight:     WeightArenaELO,
			Confidence: ConfidenceArenaELO,
		})
	}

	if mmluPro > 0 {
		signals = append(signals, BenchmarkSignal{
			Name:       "mmlu_pro",
			Value:      normalizeMMLU(mmluPro),
			Weight:     WeightMMLUPro,
			Confidence: ConfidenceMMLUPro,
		})
	}

	if ifeval > 0 {
		signals = append(signals, BenchmarkSignal{
			Name:       "ifeval",
			Value:      normalizeIFEval(ifeval),
			Weight:     WeightIFEval,
			Confidence: ConfidenceIFEval,
		})
	}

	if gsm8k > 0 {
		signals = append(signals, BenchmarkSignal{
			Name:       "gsm8k",
			Value:      normalizeGSM8K(gsm8k),
			Weight:     WeightGSM8K,
			Confidence: ConfidenceGSM8K,
		})
	}

	if arc > 0 {
		signals = append(signals, BenchmarkSignal{
			Name:       "arc",
			Value:      normalizeARC(arc),
			Weight:     WeightARC,
			Confidence: ConfidenceARC,
		})
	}

	return signals
}
