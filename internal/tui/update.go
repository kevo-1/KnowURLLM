package tui

import (
	"strings"

	"github.com/kevo-1/KnowURLLM/internal/models"
	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
)

// LayoutMode represents the current layout configuration.
type LayoutMode int

const (
	LayoutSmall LayoutMode = iota // Very small terminal, table only
	LayoutNormal                  // Default vertical stack
	LayoutWide                    // Side-by-side table and detail
)

// Update is the Bubble Tea update function.
func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.windowWidth = msg.Width
		m.windowHeight = msg.Height
		if !m.ready {
			m.ready = true
		}
		m.updateTableSize()
		m.updateDetailViewSize()
		return m, nil

	case tea.KeyMsg:
		return m.handleKey(msg)
	}

	// Update sub-components
	m.table, cmd = m.table.Update(msg)
	cmds = append(cmds, cmd)

	m.searchInput, cmd = m.searchInput.Update(msg)
	cmds = append(cmds, cmd)

	m.detailView, cmd = m.detailView.Update(msg)
	cmds = append(cmds, cmd)

	return m, tea.Batch(cmds...)
}

// layoutMode returns the current layout mode based on window dimensions.
func (m model) LayoutMode() LayoutMode {
	if m.windowWidth < 60 || m.windowHeight < 15 {
		return LayoutSmall
	}
	if m.windowWidth >= 120 {
		return LayoutWide
	}
	return LayoutNormal
}

// calcChromeHeights calculates the total height of all non-table elements.
// Returns heights for: header, searchHeader, detailPanel, help, padding/margins.
func (m model) calcChromeHeights() (header int, searchHeader int, detail int, help int, padding int) {
	// Header: typically 1 line, but can be 2 with filter tags
	header = 1
	if m.filters.VRAMOnly || m.filters.SearchQuery != "" {
		header = 2
	}

	// Search header: 1 line when inactive placeholder, 2 lines when active/showing query
	if m.searching || m.filters.SearchQuery != "" {
		searchHeader = 2
	} else {
		searchHeader = 1
	}

	// Detail panel height: varies by layout mode and expanded state
	mode := m.LayoutMode()
	switch mode {
	case LayoutSmall:
		detail = 0 // Hidden on small screens
	case LayoutNormal:
		if m.detailExpanded {
			detail = 10
		} else {
			detail = 5
		}
	case LayoutWide:
		// In wide mode, detail panel matches table height (calculated separately)
		detail = 0 // Will be set equal to table height
	}

	// Help bar: 1 line normally, 3+ when full help shown
	if m.showHelp {
		help = 3
	} else {
		help = 1
	}

	// Padding/margins between sections
	padding = 4

	return
}

// handleKey processes all keyboard input.
func (m model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// If search input is focused, handle text input first
	if m.searching {
		switch msg.Type {
		case tea.KeyEnter:
			// Accept search query
			m.filters.SearchQuery = m.searchInput.Value()
			m.searching = false
			m.searchInput.Blur()
			m.applyFilters()
			return m, nil
		case tea.KeyEscape:
			m.clearSearch()
			m.applyFilters()
			return m, nil
		default:
			// Let the textinput handle the key
			var cmd tea.Cmd
			m.searchInput, cmd = m.searchInput.Update(msg)
			// Update search query live — preserve cursor during typing
			m.filters.SearchQuery = m.searchInput.Value()
			m.applyFiltersPreserveCursor()
			return m, cmd
		}
	}

	// Normal mode key handling
	switch {
	case key.Matches(msg, m.keys.Up):
		m.table.MoveUp(1)
		return m, nil

	case key.Matches(msg, m.keys.Down):
		m.table.MoveDown(1)
		return m, nil

	case key.Matches(msg, m.keys.Search):
		m.toggleSearch()
		return m, nil

	case key.Matches(msg, m.keys.Escape):
		if m.filters.SearchQuery != "" {
			m.clearSearch()
			m.applyFilters()
		}
		return m, nil

	case key.Matches(msg, m.keys.VRAMOnly):
		m.filters.VRAMOnly = !m.filters.VRAMOnly
		m.applyFilters()
		return m, nil

	case key.Matches(msg, m.keys.Expand):
		m.detailExpanded = !m.detailExpanded
		m.updateDetailViewSize()
		m.updateTableSize()
		return m, nil

	case key.Matches(msg, m.keys.Select):
		m.selectCurrentModel()
		return m, tea.Quit

	case key.Matches(msg, m.keys.Quit):
		return m, tea.Quit

	case key.Matches(msg, m.keys.Help):
		m.showHelp = !m.showHelp
		m.updateTableSize()
		m.updateDetailViewSize()
		return m, nil
	}

	return m, nil
}

// applyFilters filters the results based on current filter options and rebuilds the table.
func (m *model) applyFilters() {
	m.filteredResults = filterResults(m.allResults, m.filters)
	m.table.SetRows(buildTableRows(m.filteredResults))
	// Reset cursor to first row only when results change significantly
	m.table.SetCursor(0)
}

// applyFiltersPreserveCursor filters the results but preserves the cursor position
// when possible. Used during live search typing to avoid jarring jumps.
func (m *model) applyFiltersPreserveCursor() {
	m.filteredResults = filterResults(m.allResults, m.filters)
	m.table.SetRows(buildTableRows(m.filteredResults))
	// Clamp cursor to valid range
	cursor := m.table.Cursor()
	if cursor >= len(m.filteredResults) && len(m.filteredResults) > 0 {
		m.table.SetCursor(len(m.filteredResults) - 1)
	}
}

// selectCurrentModel sets the selected model from the currently focused row.
func (m *model) selectCurrentModel() {
	cursor := m.table.Cursor()
	if cursor >= 0 && cursor < len(m.filteredResults) {
		entry := m.filteredResults[cursor].Model
		m.selected = &entry
	}
}

// updateTableSize adjusts the table dimensions based on the current window size.
func (m *model) updateTableSize() {
	if !m.ready {
		return
	}

	_, _, detailH, _, _ := m.calcChromeHeights()

	// Calculate available height for the table
	totalChromeHeight := m.calcTotalChromeHeight()
	tableHeight := m.windowHeight - totalChromeHeight

	// Enforce minimum table height
	if tableHeight < 3 {
		tableHeight = 3
	}

	// In wide mode, cache table height for detail panel sizing
	mode := m.LayoutMode()
	if mode == LayoutWide && detailH == 0 {
		m.cachedTableHeight = tableHeight
	}

	// Calculate dynamic table width (account for padding)
	tableWidth := m.windowWidth - 4
	if mode == LayoutWide {
		// Table takes ~65% of width
		tableWidth = (m.windowWidth * 2 / 3) - 4
	}
	if tableWidth < 40 {
		tableWidth = 40 // Minimum usable width
	}
	m.cachedTableWidth = tableWidth

	m.table.SetWidth(tableWidth)
	m.table.SetHeight(tableHeight)
}

// calcTotalChromeHeight returns the sum of all non-table element heights.
func (m model) calcTotalChromeHeight() int {
	headerH, searchH, detailH, helpH, padding := m.calcChromeHeights()
	return headerH + searchH + detailH + helpH + padding
}

// updateDetailViewSize adjusts the detail viewport size.
func (m *model) updateDetailViewSize() {
	if !m.ready {
		return
	}

	mode := m.LayoutMode()

	switch mode {
	case LayoutSmall:
		// Hidden on small screens
		m.detailView.Width = 0
		m.detailView.Height = 0
	case LayoutWide:
		// In wide mode, detail panel takes right side, same height as table
		m.detailView.Width = (m.windowWidth / 3) - 4 // ~33% of width
		if m.cachedTableHeight > 0 {
			m.detailView.Height = m.cachedTableHeight
		} else {
			m.detailView.Height = 10 // Fallback
		}
	case LayoutNormal:
		// Normal vertical stack: detail panel below table
		m.detailView.Width = m.windowWidth - 8
		if m.detailExpanded {
			m.detailView.Height = 10
		} else {
			m.detailView.Height = 5
		}
	}
}

// filterResults applies the filter options to the given results.
func filterResults(all []models.RankResult, opts models.FilterOptions) []models.RankResult {
	if opts.SearchQuery == "" && !opts.VRAMOnly && opts.Quantization == "" && opts.MinQuality == 0 {
		return all
	}

	var filtered []models.RankResult
	for _, r := range all {
		// Search query filter (case-insensitive substring on DisplayName and Tags)
		if opts.SearchQuery != "" {
			query := strings.ToLower(opts.SearchQuery)
			nameMatch := strings.Contains(strings.ToLower(r.Model.DisplayName), query)
			tagMatch := false
			for _, tag := range r.Model.Tags {
				if strings.Contains(strings.ToLower(tag), query) {
					tagMatch = true
					break
				}
			}
			if !nameMatch && !tagMatch {
				continue
			}
		}

		// VRAM only filter
		if opts.VRAMOnly && !r.Score.FitsInVRAM {
			continue
		}

		// Quantization filter
		if opts.Quantization != "" && r.Model.Quantization != opts.Quantization {
			continue
		}

		// Minimum quality filter — compare against QualityScore (benchmark-derived),
		// not TotalScore (which includes hardware fit and throughput).
		if opts.MinQuality > 0 && r.Score.QualityScore < opts.MinQuality {
			continue
		}

		filtered = append(filtered, r)
	}
	return filtered
}
