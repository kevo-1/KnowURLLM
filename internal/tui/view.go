package tui

import (
	"fmt"
	"strings"

	"github.com/KnowURLLM/internal/models"
	"github.com/charmbracelet/lipgloss"
)

// View is the Bubble Tea view function.
func (m model) View() string {
	if !m.ready {
		return "\n  Initializing..."
	}

	var sections []string

	// Header
	sections = append(sections, m.renderHeader())

	// Search header
	sections = append(sections, m.searchHeader())

	// Table
	sections = append(sections, m.renderStyledTable())

	// Detail panel (if there are results)
	if len(m.filteredResults) > 0 {
		cursor := m.table.Cursor()
		if cursor >= 0 && cursor < len(m.filteredResults) {
			result := m.filteredResults[cursor]
			detailContent := renderDetailPanel(result, m.windowWidth)
			m.detailView.SetContent(detailContent)
			sections = append(sections, m.detailView.View())
		}
	}

	// Help bar
	sections = append(sections, m.renderHelp())

	return strings.Join(sections, "\n")
}

// renderHeader renders the top bar with title and result count.
func (m model) renderHeader() string {
	title := titleStyle.Render("KnowURLLM")
	count := mutedTextStyle.Render(fmt.Sprintf("(%d models)", len(m.filteredResults)))

	base := lipgloss.JoinHorizontal(lipgloss.Center, title, " ", count)

	// Show prominent filter status indicator
	filterIndicator := m.renderFilterStatus()
	if filterIndicator != "" {
		base = base + "  " + filterIndicator
	}

	return base
}

// renderFilterStatus shows the current active filter state prominently.
func (m model) renderFilterStatus() string {
	if !m.filters.VRAMOnly && m.filters.SearchQuery == "" {
		return mutedTextStyle.Render("[no filters]")
	}

	var parts []string

	if m.filters.VRAMOnly {
		parts = append(parts, filterTagStyle.Render("VRAM"))
	}
	if m.filters.SearchQuery != "" {
		parts = append(parts, filterTagStyle.Render("🔍 "+m.filters.SearchQuery))
	}

	return strings.Join(parts, " ")
}

// renderStyledTable renders the table with lipgloss styling.
func (m model) renderStyledTable() string {
	if len(m.filteredResults) == 0 {
		return mutedTextStyle.Render("  No models match your filters")
	}

	cursor := m.table.Cursor()
	tableHeight := m.table.Height()

	// Calculate visible row range (only render what fits in the table viewport)
	start := 0
	if cursor >= tableHeight {
		start = cursor - tableHeight + 1
	}
	end := start + tableHeight
	if end > len(m.filteredResults) {
		end = len(m.filteredResults)
	}

	// Build styled rows — only render visible rows
	var styledRows []string

	// Column header row
	colHeaders := []string{"Rank", "Model", "Size", "TPS", "Score", "Fit"}
	headerRow := headerStyle.Render(formatTableRow(colHeaders))
	styledRows = append(styledRows, headerRow)

	// Separator
	styledRows = append(styledRows, strings.Repeat("─", m.windowWidth-4))

	// Data rows — only visible range
	for i := start; i < end; i++ {
		r := m.filteredResults[i]
		isSelected := i == cursor
		row := buildStyledRow(r, isSelected)
		styledRows = append(styledRows, row)
	}

	// Scroll indicator
	if len(m.filteredResults) > tableHeight {
		indicator := mutedTextStyle.Render(fmt.Sprintf("  %d/%d (scroll with ↑↓)", cursor+1, len(m.filteredResults)))
		styledRows = append(styledRows, indicator)
	}

	// Apply table border styling
	tableContent := strings.Join(styledRows, "\n")
	return lipgloss.NewStyle().
		Padding(0, 1).
		Render(tableContent)
}

// buildStyledRow creates a styled table row string.
func buildStyledRow(r models.RankResult, isSelected bool) string {
	rank := formatRank(r.Rank)
	modelName := truncate(r.Model.DisplayName, colModelWidth)
	size := formatBytes(r.Model.ModelSizeBytes)
	tps := formatTPSInt(r.Score.EstimatedTPS)
	score := formatScore(r.Score.TotalScore)
	fit := formatFitBadge(r.Score)

	cols := []string{rank, modelName, size, tps, score, fit}

	row := formatTableRow(cols)
	if isSelected {
		row = selectedRowStyle.Render(row)
	}

	// Highlight score for top 3
	if !isSelected && r.Rank <= 3 {
		goldScore := goldStyle.Render(score)
		cols[4] = goldScore
		row = formatTableRow(cols)
	}

	return row
}

// formatTableRow formats a row of columns with consistent spacing.
func formatTableRow(cols []string, opts ...lipgloss.Style) string {
	widths := []int{colRankWidth, colModelWidth, colSizeWidth, colTPSWidth, colScoreWidth, colFitWidth}

	var parts []string
	for i, col := range cols {
		w := widths[i]
		cell := lipgloss.NewStyle().Width(w).Render(col)
		parts = append(parts, cell)
	}

	row := strings.Join(parts, " ")
	if len(opts) > 0 {
		row = opts[0].Render(row)
	}
	return row
}

// renderHelp renders the bottom help bar.
func (m model) renderHelp() string {
	if m.showHelp {
		return m.help.View(m.keys)
	}
	return helpStyle.Render("  ↑↓/jk: navigate  /: search  v: VRAM  enter: select  q: quit  ?: help")
}
