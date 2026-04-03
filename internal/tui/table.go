package tui

import (
	"fmt"
	"strconv"

	"github.com/kevo-1/KnowURLLM/internal/models"
	"github.com/charmbracelet/bubbles/table"
)

// Column widths for the table.
const (
	colRankWidth  = 5
	colModelWidth = 30
	colSizeWidth  = 8
	colTPSWidth   = 7
	colScoreWidth = 9
	colFitWidth   = 10
)

// tableColumns returns the column definitions for the results table.
func tableColumns() []table.Column {
	return []table.Column{
		{Title: "Rank", Width: colRankWidth},
		{Title: "Model", Width: colModelWidth},
		{Title: "Size", Width: colSizeWidth},
		{Title: "TPS", Width: colTPSWidth},
		{Title: "Score", Width: colScoreWidth},
		{Title: "Fit", Width: colFitWidth},
	}
}

// buildTableRows converts filtered RankResults into table rows.
func buildTableRows(results []models.RankResult) []table.Row {
	rows := make([]table.Row, 0, len(results))
	for _, r := range results {
		rows = append(rows, table.Row{
			formatRank(r.Rank),
			truncate(r.Model.DisplayName, colModelWidth),
			formatBytes(r.Model.ModelSizeBytes),
			formatTPSInt(r.Score.EstimatedTPS),
			formatScore(r.Score.TotalScore),
			formatFitBadge(r.Score),
		})
	}
	return rows
}

// formatRank formats the rank number, highlighting top 3.
func formatRank(rank int) string {
	return strconv.Itoa(rank)
}

// formatScore formats the total score to 1 decimal place.
func formatScore(score float64) string {
	return fmt.Sprintf("%.1f", score)
}

// formatTPSInt formats estimated TPS as an integer string.
func formatTPSInt(tps float64) string {
	return strconv.Itoa(int(tps))
}

// formatFitBadge returns the fit badge string with proper styling.
func formatFitBadge(score models.ModelScore) string {
	if score.FitsInVRAM {
		return vramBadgeStyle.Render("VRAM ✓")
	}
	if score.FitsInMemory {
		return ramBadgeStyle.Render("RAM ✓")
	}
	return mutedTextStyle.Render("—")
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
