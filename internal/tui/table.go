package tui

import (
	"fmt"
	"strconv"

	"github.com/kevo-1/KnowURLLM/internal/domain"
	"github.com/charmbracelet/bubbles/table"
)

// Column widths for the table (minimum widths).
// The Model column will expand to fill remaining width in formatTableRow().
const (
	colRankWidth  = 5  // Rank: fixed
	colTierWidth  = 5  // Tier: fixed
	colModelWidth = 20 // Model: minimum, expands with available width
	colSizeWidth  = 8  // Size: fixed
	colTPSWidth   = 7  // TPS: fixed
	colQualWidth  = 10 // Quality: fixed
	colFitWidth   = 10 // Fit: fixed
)

// tableColumns returns the column definitions for the results table.
func tableColumns() []table.Column {
	return []table.Column{
		{Title: "Rank", Width: colRankWidth},
		{Title: "Tier", Width: colTierWidth},
		{Title: "Model", Width: colModelWidth},
		{Title: "Size", Width: colSizeWidth},
		{Title: "TPS", Width: colTPSWidth},
		{Title: "Quality", Width: colQualWidth},
		{Title: "Fit", Width: colFitWidth},
	}
}

// buildTableRows converts filtered RankedModels into table rows.
func buildTableRows(results []domain.RankedModel) []table.Row {
	rows := make([]table.Row, 0, len(results))
	for _, r := range results {
		rows = append(rows, table.Row{
			formatRank(r.Rank),
			formatTier(r.Quality.Tier),
			truncate(r.Model.DisplayName, colModelWidth),
			formatBytes(r.Model.ModelSizeBytes),
			formatTPSInt(r.Hardware.EstimatedTPS),
			formatQualityScore(r.Quality),
			formatFitBadge(r.Hardware),
		})
	}
	return rows
}

// formatRank formats the rank number, highlighting top 3.
func formatRank(rank int) string {
	return strconv.Itoa(rank)
}

// formatTier formats the quality tier with tier badge styling.
func formatTier(tier domain.QualityTier) string {
	style := tierBadgeStyle.Copy().Foreground(GetTierColor(string(tier)))
	return style.Render(string(tier))
}

// formatQualityScore formats the quality score with confidence.
func formatQualityScore(q domain.ModelQuality) string {
	if q.Confidence < 0.5 {
		// Low confidence: show score with question mark
		return fmt.Sprintf("%.0f?", q.Overall)
	}
	return fmt.Sprintf("%.0f", q.Overall)
}

// formatScore formats the total score to 1 decimal place.
// DEPRECATED: Use formatQualityScore instead.
func formatScore(score float64) string {
	return fmt.Sprintf("%.1f", score)
}

// formatTPSInt formats estimated TPS as an integer string.
func formatTPSInt(tps float64) string {
	return strconv.Itoa(int(tps))
}

// formatFitBadge returns the fit badge string with proper styling.
func formatFitBadge(hw domain.ModelHardware) string {
	if hw.Mode == domain.RunModeGPU {
		return vramBadgeStyle.Render("VRAM ✓")
	}
	if hw.Mode == domain.RunModeMoE {
		return moeBadgeStyle.Render("MoE ✓")
	}
	if hw.Mode == domain.RunModeCPUGPU {
		return ramBadgeStyle.Render("RAM")
	}
	return mutedTextStyle.Render("CPU")
}

// truncate truncates a string to maxLen, adding ellipsis if needed.
func truncate(s string, maxLen int) string {
	runes := []rune(s)
	if len(runes) <= maxLen {
		return s
	}
	if maxLen <= 3 {
		return string(runes[:maxLen])
	}
	return string(runes[:maxLen-3]) + "…"
}
