package tui

import (
	"strings"

	"github.com/KnowURLLM/internal/models"
	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
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

	case key.Matches(msg, m.keys.Select):
		m.selectCurrentModel()
		return m, tea.Quit

	case key.Matches(msg, m.keys.Quit):
		return m, tea.Quit

	case key.Matches(msg, m.keys.Help):
		m.showHelp = !m.showHelp
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
	// Calculate available height for the table
	// Account for: header (1) + search header (2) + table header+separator (2) + detail panel (10) + help bar (2) + padding (4)
	totalChromeHeight := 21
	tableHeight := m.windowHeight - totalChromeHeight
	if tableHeight < 3 {
		tableHeight = 3
	}
	// Cap table height to available terminal space (let internal scrolling handle overflow)
	// Do NOT expand to show all results — that overflows the terminal
	m.table.SetWidth(m.windowWidth - 4)
	m.table.SetHeight(tableHeight)
}

// updateDetailViewSize adjusts the detail viewport size.
func (m *model) updateDetailViewSize() {
	if !m.ready {
		return
	}
	m.detailView.Width = m.windowWidth - 8
	m.detailView.Height = 8
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

		// Minimum quality filter
		if opts.MinQuality > 0 && r.Score.TotalScore < opts.MinQuality {
			continue
		}

		filtered = append(filtered, r)
	}
	return filtered
}
