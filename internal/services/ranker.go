// Package services provides application-level orchestration.
package services

import (
	"sort"

	"github.com/kevo-1/KnowURLLM/internal/domain"
	domainhw "github.com/kevo-1/KnowURLLM/internal/domain/hardware"
	"github.com/kevo-1/KnowURLLM/internal/domain/quality"
)

// Ranker orchestrates the full ranking pipeline:
// 1. Quality scoring (Arena-style, hardware-agnostic)
// 2. Hardware compatibility checking
// 3. Tier-based ranking with hardware sub-sort
type Ranker struct {
	QualityScorer  *quality.Scorer
}

// NewRanker creates a new ranker with default configuration.
func NewRanker() *Ranker {
	return &Ranker{
		QualityScorer: quality.NewScorer(),
	}
}

// Rank performs quality-first tier-based ranking.
// Returns models ranked by quality tier, then sub-sorted by hardware fit.
func (r *Ranker) Rank(hw domain.HardwareProfile, entries []domain.ModelEntry) []domain.RankedModel {
	if len(entries) == 0 {
		return []domain.RankedModel{}
	}

	// Step 1: Calculate quality scores for all models (hardware-agnostic)
	qualityScores := r.QualityScorer.ScoreAll(entries)

	// Step 2: Check hardware compatibility for each model
	rankedModels := make([]domain.RankedModel, 0, len(entries))
	for i, entry := range entries {
		hwCompat := domainhw.CheckCompatibility(hw, entry)
		
		// Skip non-runnable models
		if !hwCompat.Runnable {
			continue
		}

		// Step 3: Estimate performance (TPS)
		_, estimatedTPS := domainhw.EstimatePerformance(
			hw,
			hwCompat.SizeAtQuant,
			hwCompat.Mode,
			hwCompat.BestQuant,
		)
		hwCompat.EstimatedTPS = estimatedTPS

		rankedModels = append(rankedModels, domain.RankedModel{
			Model:    entry,
			Quality:  qualityScores[i],
			Hardware: hwCompat,
		})
	}

	// Step 4: Sort by tier, then hardware sub-sort within tier
	sort.SliceStable(rankedModels, func(i, j int) bool {
		return compareRankedModels(rankedModels[i], rankedModels[j])
	})

	// Step 5: Assign ranks
	for i := range rankedModels {
		rankedModels[i].Rank = i + 1
	}

	return rankedModels
}

// RankWithFilter ranks models with filtering applied.
func (r *Ranker) RankWithFilter(hw domain.HardwareProfile, entries []domain.ModelEntry, filters domain.FilterOptions) []domain.RankedModel {
	// First rank all models
	ranked := r.Rank(hw, entries)

	// Then apply filters
	return domain.ApplyFilters(ranked, filters)
}

// compareRankedModels compares two models for sorting.
// Primary: Quality tier (S > A > B > C > D)
// Secondary: Within same tier, sort by hardware fit quality
// Tertiary: TPS (faster first)
// Quaternary: Downloads (popularity)
func compareRankedModels(a, b domain.RankedModel) bool {
	// Primary: Quality tier
	tierOrder := map[domain.QualityTier]int{
		domain.TierS: 5,
		domain.TierA: 4,
		domain.TierB: 3,
		domain.TierC: 2,
		domain.TierD: 1,
	}

	aTierVal := tierOrder[a.Quality.Tier]
	bTierVal := tierOrder[b.Quality.Tier]

	if aTierVal != bTierVal {
		return aTierVal > bTierVal // Higher tier first
	}

	// Secondary: Within same tier, sort by quality score
	if a.Quality.Overall != b.Quality.Overall {
		return a.Quality.Overall > b.Quality.Overall
	}

	// Tertiary: Hardware fit quality
	fitOrder := map[string]int{
		"Perfect":  4,
		"Good":     3,
		"Marginal": 2,
		"Tight":    1,
	}

	aFitVal := fitOrder[a.Hardware.FitLabel]
	bFitVal := fitOrder[b.Hardware.FitLabel]

	if aFitVal != bFitVal {
		return aFitVal > bFitVal // Better fit first
	}

	// Quaternary: TPS (faster first)
	if a.Hardware.EstimatedTPS != b.Hardware.EstimatedTPS {
		return a.Hardware.EstimatedTPS > b.Hardware.EstimatedTPS
	}

	// Quinary: Downloads (popularity)
	return a.Model.Downloads > b.Model.Downloads
}
