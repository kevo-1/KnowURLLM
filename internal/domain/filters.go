// Package domain provides filtering logic for ranked models.
package domain

import (
	"strings"
)

// ApplyFilters applies filter options to a list of ranked models.
func ApplyFilters(models []RankedModel, filters FilterOptions) []RankedModel {
	if filters == (FilterOptions{}) {
		return models
	}

	filtered := make([]RankedModel, 0, len(models))
	for _, m := range models {
		if matchesFilters(m, filters) {
			filtered = append(filtered, m)
		}
	}

	// Re-assign ranks after filtering
	for i := range filtered {
		filtered[i].Rank = i + 1
	}

	return filtered
}

// matchesFilters checks if a model matches all filter criteria.
func matchesFilters(m RankedModel, filters FilterOptions) bool {
	// MinTier filter
	if filters.MinTier != "" {
		if tierValue(m.Quality.Tier) < tierValue(filters.MinTier) {
			return false
		}
	}

	// MinQuality filter (legacy, still supported)
	if filters.MinQuality > 0 && m.Quality.Overall < filters.MinQuality {
		return false
	}

	// VRAMOnly filter
	if filters.VRAMOnly && !m.Hardware.FitsInVRAM() {
		return false
	}

	// Source filter
	if filters.Source != "" {
		sourceLower := strings.ToLower(m.Model.Source)
		filterLower := strings.ToLower(filters.Source)
		if !strings.Contains(sourceLower, filterLower) && !strings.Contains(filterLower, sourceLower) {
			return false
		}
	}

	// Quantization filter
	if filters.Quantization != "" {
		if !strings.EqualFold(m.Hardware.BestQuant, filters.Quantization) {
			return false
		}
	}

	// Search query filter
	if filters.SearchQuery != "" {
		queryLower := strings.ToLower(filters.SearchQuery)
		matchName := strings.Contains(strings.ToLower(m.Model.DisplayName), queryLower)
		matchTag := false
		for _, tag := range m.Model.Tags {
			if strings.Contains(strings.ToLower(tag), queryLower) {
				matchTag = true
				break
			}
		}
		if !matchName && !matchTag {
			return false
		}
	}

	// MinTPS filter
	if filters.MinTPS > 0 && m.Hardware.EstimatedTPS < filters.MinTPS {
		return false
	}

	// MinCategory filter
	if filters.MinCategory != "" && filters.MinCategoryScore > 0 {
		catScore := m.Quality.CategoryScores[filters.MinCategory]
		if catScore < filters.MinCategoryScore {
			return false
		}
	}

	return true
}

// FitsInVRAM returns true if the model fits entirely in VRAM.
func (h ModelHardware) FitsInVRAM() bool {
	return h.Mode == RunModeGPU || h.Mode == RunModeMoE
}

// tierValue returns a numeric value for a quality tier (for comparison).
func tierValue(tier QualityTier) int {
	switch tier {
	case TierS:
		return 5
	case TierA:
		return 4
	case TierB:
		return 3
	case TierC:
		return 2
	case TierD:
		return 1
	default:
		return 0
	}
}
