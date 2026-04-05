package scorer

import (
	"fmt"
	"sort"
	"strings"

	"github.com/kevo-1/KnowURLLM/internal/models"
)

// Rank scores all models against the hardware profile.
// Models that cannot fit in memory are excluded.
// Results are sorted by TotalScore descending.
// Returns error only if entries is nil or hw is zero-value.
func (s *Scorer) Rank(
	hw models.HardwareProfile,
	entries []models.ModelEntry,
) ([]models.RankResult, error) {
	if err := validateInput(hw, entries); err != nil {
		return nil, fmt.Errorf("scorer: invalid input: %w", err)
	}

	results := make([]models.RankResult, 0, len(entries))

	for _, entry := range entries {
		score, excluded := scoreModel(hw, entry)
		if excluded {
			continue
		}
		results = append(results, models.RankResult{
			Model: entry,
			Score: score,
		})
	}

	// Sort by TotalScore descending, applying tiebreaker
	sort.SliceStable(results, func(i, j int) bool {
		return compareResults(results[i], results[j]) < 0
	})

	// Assign 1-based rank
	for i := range results {
		results[i].Rank = i + 1
	}

	return results, nil
}

// RankWithFilter applies filter options before ranking.
func (s *Scorer) RankWithFilter(
	hw models.HardwareProfile,
	entries []models.ModelEntry,
	filters models.FilterOptions,
) ([]models.RankResult, error) {
	if err := validateInput(hw, entries); err != nil {
		return nil, fmt.Errorf("scorer: invalid input: %w", err)
	}

	vram := totalVRAM(hw)

	// Pre-filter entries before scoring
	filtered := make([]models.ModelEntry, 0, len(entries))
	for _, entry := range entries {
		// VRAMOnly: pre-check if model would fit in VRAM
		if filters.VRAMOnly && entry.ModelSizeBytes > vram {
			continue
		}

		// Source filter
		if filters.Source != "" {
			sourceLower := strings.ToLower(entry.Source)
			filterLower := strings.ToLower(filters.Source)
			// Support exact match or contains (e.g., "huggingface+ollama" matches "huggingface")
			if !strings.Contains(sourceLower, filterLower) && !strings.Contains(filterLower, sourceLower) {
				continue
			}
		}

		// Quantization filter (case-insensitive)
		if filters.Quantization != "" {
			if !strings.EqualFold(entry.Quantization, filters.Quantization) {
				continue
			}
		}

		// Search query filter (case-insensitive, matches DisplayName or Tags)
		if filters.SearchQuery != "" {
			queryLower := strings.ToLower(filters.SearchQuery)
			matchName := strings.Contains(strings.ToLower(entry.DisplayName), queryLower)
			matchTag := false
			for _, tag := range entry.Tags {
				if strings.Contains(strings.ToLower(tag), queryLower) {
					matchTag = true
					break
				}
			}
			if !matchName && !matchTag {
				continue
			}
		}

		filtered = append(filtered, entry)
	}

	// Rank the filtered entries
	results, err := s.Rank(hw, filtered)
	if err != nil {
		return nil, err
	}

	// Post-filter by MinQuality (requires scoring first)
	if filters.MinQuality > 0 {
		filteredResults := make([]models.RankResult, 0, len(results))
		for _, r := range results {
			if r.Score.QualityScore >= filters.MinQuality {
				filteredResults = append(filteredResults, r)
			}
		}

		// Re-assign ranks after filtering
		for i := range filteredResults {
			filteredResults[i].Rank = i + 1
		}
		return filteredResults, nil
	}

	return results, nil
}

// compareResults compares two RankResults for sorting.
// Returns negative if a should come before b, positive if b should come before a, 0 if equal.
func compareResults(a, b models.RankResult) int {
	// Primary: TotalScore descending (higher first)
	diff := b.Score.TotalScore - a.Score.TotalScore
	if diff > 0.01 {
		return 1 // b wins
	}
	if diff < -0.01 {
		return -1 // a wins
	}

	// Tiebreaker 1: Higher FitCategory wins (Perfect > Good > Marginal > Too Tight)
	catOrder := map[string]int{"Perfect": 4, "Good": 3, "Marginal": 2, "Too Tight": 1}
	aCatOrder := catOrder[a.Score.FitCategory]
	bCatOrder := catOrder[b.Score.FitCategory]
	catDiff := bCatOrder - aCatOrder
	if catDiff > 0 {
		return 1
	}
	if catDiff < 0 {
		return -1
	}

	// Tiebreaker 2: Higher QualityScore wins
	qDiff := b.Score.QualityScore - a.Score.QualityScore
	if qDiff > 0 {
		return 1
	}
	if qDiff < 0 {
		return -1
	}

	// Tiebreaker 3: Higher Downloads wins
	dDiff := b.Model.Downloads - a.Model.Downloads
	if dDiff > 0 {
		return 1
	}
	if dDiff < 0 {
		return -1
	}

	// Tiebreaker 4: Alphabetical DisplayName (ascending)
	return strings.Compare(a.Model.DisplayName, b.Model.DisplayName)
}
