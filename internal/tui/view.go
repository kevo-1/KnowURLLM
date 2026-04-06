package tui

import (
	"fmt"
	"strings"

	"github.com/kevo-1/KnowURLLM/internal/domain"
	"github.com/charmbracelet/lipgloss"
)

// View is the Bubble Tea view function.
func (m model) View() string {
	if !m.ready {
		return "\n  Initializing..."
	}

	mode := m.LayoutMode()

	switch mode {
	case LayoutSmall:
		return m.renderSmallLayout()
	case LayoutWide:
		return m.renderWideLayout()
	default:
		return m.renderNormalLayout()
	}
}

// renderSmallLayout renders only the table for very small terminals.
func (m model) renderSmallLayout() string {
	var sections []string
	sections = append(sections, m.renderCompactHeader())
	sections = append(sections, m.renderStyledTable())
	sections = append(sections, m.renderCompactHelp())
	return strings.Join(sections, "\n")
}

// renderNormalLayout renders the default vertical stack layout.
func (m model) renderNormalLayout() string {
	var sections []string

	// Header
	sections = append(sections, m.renderHeader())

	// Search header
	sections = append(sections, m.searchHeader())

	// Table
	sections = append(sections, m.renderStyledTable())

	// Detail panel (if there are results)
	if len(m.filteredResults) > 0 && m.detailView.Height >= 0 {
		cursor := m.table.Cursor()
		if cursor >= 0 && cursor < len(m.filteredResults) {
			result := m.filteredResults[cursor]
			detailContent := renderDetailPanel(result, m.detailView.Width, m.detailExpanded)
			m.detailView.SetContent(detailContent)
			// Ensure minimum height for visibility
			if m.detailView.Height < 5 {
				m.detailView.Height = 5
			}
			sections = append(sections, m.detailView.View())
		}
	}

	// Help bar
	sections = append(sections, m.renderHelp())

	return strings.Join(sections, "\n")
}

// renderWideLayout renders table and detail side-by-side.
func (m model) renderWideLayout() string {
	var sections []string

	// Header (full width)
	sections = append(sections, m.renderHeader())

	// Search header (full width)
	sections = append(sections, m.searchHeader())

	// Side-by-side table and detail
	if len(m.filteredResults) > 0 {
		tableSection := m.renderStyledTable()

		cursor := m.table.Cursor()
		var detailSection string
		if cursor >= 0 && cursor < len(m.filteredResults) && m.detailView.Height >= 0 {
			result := m.filteredResults[cursor]
			detailContent := renderDetailPanel(result, m.detailView.Width, m.detailExpanded)
			m.detailView.SetContent(detailContent)
			// Ensure minimum height for visibility
			if m.detailView.Height < 5 {
				m.detailView.Height = 5
			}
			detailSection = m.detailView.View()
		}

		// Join side-by-side
		sideBySide := lipgloss.JoinHorizontal(lipgloss.Top, tableSection, detailSection)
		sections = append(sections, sideBySide)
	} else {
		sections = append(sections, m.renderStyledTable())
	}

	// Help bar
	sections = append(sections, m.renderHelp())

	return strings.Join(sections, "\n")
}

// renderCompactHeader renders a minimal header for small screens.
func (m model) renderCompactHeader() string {
	title := titleStyle.Render("KnowURLLM")
	count := mutedTextStyle.Render(fmt.Sprintf("(%d)", len(m.filteredResults)))
	return lipgloss.JoinHorizontal(lipgloss.Center, title, " ", count)
}

// renderCompactHelp renders a single-line help for small screens.
func (m model) renderCompactHelp() string {
	return helpStyle.Render("↑↓:nav /:search v:VRAM enter:select q:quit")
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
		return ""
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
	colHeaders := []string{"Rank", "Tier", "Model", "Size", "TPS", "Quality", "Fit"}
	headerRow := headerStyle.Render(formatTableRow(colHeaders, m.cachedTableWidth))
	styledRows = append(styledRows, headerRow)

	// Separator
	sepWidth := m.cachedTableWidth
	if sepWidth < 40 {
		sepWidth = m.windowWidth - 4
	}
	styledRows = append(styledRows, strings.Repeat("─", sepWidth))

	// Data rows — only visible range
	for i := start; i < end; i++ {
		r := m.filteredResults[i]
		isSelected := i == cursor
		row := buildStyledRow(r, isSelected, m.cachedTableWidth)
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
func buildStyledRow(r domain.RankedModel, isSelected bool, availableWidth int) string {
	rank := formatRank(r.Rank)
	tier := formatTier(r.Quality.Tier)
	modelName := truncate(r.Model.DisplayName, colModelWidth)
	size := formatBytes(r.Model.ModelSizeBytes)
	tps := formatTPSInt(r.Hardware.EstimatedTPS)
	quality := formatQualityScore(r.Quality)
	fit := formatFitBadge(r.Hardware)

	cols := []string{rank, tier, modelName, size, tps, quality, fit}

	row := formatTableRow(cols, availableWidth)
	if isSelected {
		row = selectedRowStyle.Render(row)
	}

	// Highlight tier for top 3
	if !isSelected && r.Rank <= 3 {
		goldTier := goldStyle.Render(tier)
		cols[1] = goldTier
		row = formatTableRow(cols, availableWidth)
	}

	return row
}

// formatTableRow formats a row of columns with dynamic spacing.
func formatTableRow(cols []string, availableWidth int) string {
	// Calculate column widths: fixed minimums + remainder to Model column
	minWidths := []int{
		colRankWidth,  // Rank: 5
		colTierWidth,  // Tier: 5
		colModelWidth, // Model: gets remainder
		colSizeWidth,  // Size: 8
		colTPSWidth,   // TPS: 7
		colQualWidth,  // Quality: 10
		colFitWidth,   // Fit: 10
	}

	// Calculate total minimum width needed
	totalMinWidth := 0
	for _, w := range minWidths {
		totalMinWidth += w
	}

	// Add spacing between columns (5 spaces)
	spacing := 5
	totalMinWidth += spacing * (len(cols) - 1)

	// If available width is larger, distribute remainder to Model column
	modelWidth := minWidths[1]
	if availableWidth > totalMinWidth {
		modelWidth = minWidths[1] + (availableWidth - totalMinWidth)
	}

	// Build cells with appropriate widths
	widths := minWidths
	widths[1] = modelWidth // Model column gets remainder

	var parts []string
	for i, col := range cols {
		w := widths[i]
		cell := lipgloss.NewStyle().Width(w).Render(col)
		parts = append(parts, cell)
	}

	return strings.Join(parts, " ")
}

// renderHelp renders the bottom help bar.
func (m model) renderHelp() string {
	if m.showHelp {
		return m.help.View(m.keys)
	}
	return helpStyle.Render("  ↑↓/jk: navigate  /: search  tab: expand detail  v: VRAM  enter: select  q: quit  ?: help")
}
