package tui

import "github.com/charmbracelet/bubbles/textinput"

// newSearchInput creates and initializes the search text input component.
func newSearchInput() textinput.Model {
	ti := textinput.New()
	ti.Placeholder = "Search models..."
	ti.CharLimit = 64
	ti.Prompt = "🔍 "
	return ti
}

// searchHeader renders the search bar and active filters.
func (m model) searchHeader() string {
	var header string

	if m.searching {
		header = searchStyle.Width(m.windowWidth - 4).Render(m.searchInput.View())
	} else if m.filters.SearchQuery != "" {
		header = searchStyle.Width(m.windowWidth - 4).Render(m.filters.SearchQuery)
	} else {
		header = mutedTextStyle.Render("Press / to search")
	}

	return header
}

// toggleSearch activates the search input.
// Returns a focus command when called from update.go context.
func (m *model) toggleSearch() {
	m.searching = true
	m.searchInput.SetValue(m.filters.SearchQuery)
	m.searchInput.Focus()
	m.searchInput.CursorEnd()
}

// clearSearch clears the search query and resets filtering.
func (m *model) clearSearch() {
	m.searching = false
	m.searchInput.SetValue("")
	m.searchInput.Blur()
	m.filters.SearchQuery = ""
}
