package tui

import (
	"github.com/kevo-1/KnowURLLM/internal/domain"
	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/table"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
)

// keyMap defines all key bindings for the TUI.
type keyMap struct {
	Up       key.Binding
	Down     key.Binding
	Search   key.Binding
	Escape   key.Binding
	VRAMOnly key.Binding
	Select   key.Binding
	Quit     key.Binding
	Help     key.Binding
	Expand   key.Binding
}

// keys defines the default key bindings.
var keys = keyMap{
	Up: key.NewBinding(
		key.WithKeys("up", "k"),
		key.WithHelp("↑/k", "up"),
	),
	Down: key.NewBinding(
		key.WithKeys("down", "j"),
		key.WithHelp("↓/j", "down"),
	),
	Search: key.NewBinding(
		key.WithKeys("/"),
		key.WithHelp("/", "search"),
	),
	Escape: key.NewBinding(
		key.WithKeys("esc"),
		key.WithHelp("esc", "clear search"),
	),
	VRAMOnly: key.NewBinding(
		key.WithKeys("v"),
		key.WithHelp("v", "VRAM only"),
	),
	Select: key.NewBinding(
		key.WithKeys("enter"),
		key.WithHelp("enter", "select"),
	),
	Quit: key.NewBinding(
		key.WithKeys("q", "ctrl+c"),
		key.WithHelp("q", "quit"),
	),
	Help: key.NewBinding(
		key.WithKeys("?"),
		key.WithHelp("?", "toggle help"),
	),
	Expand: key.NewBinding(
		key.WithKeys("tab"),
		key.WithHelp("tab", "expand detail"),
	),
}

// ShortHelp returns keybindings for the help bar.
func (k keyMap) ShortHelp() []key.Binding {
	return []key.Binding{k.Up, k.Down, k.Search, k.Expand, k.VRAMOnly, k.Select, k.Quit, k.Help}
}

// FullHelp returns the full help (same as short for this TUI).
func (k keyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{k.ShortHelp()}
}

// model is the Bubble Tea application state.
type model struct {
	// Data
	allResults      []domain.RankedModel
	filteredResults []domain.RankedModel

	// Components
	table       table.Model
	searchInput textinput.Model
	help        help.Model
	detailView  viewport.Model

	// State
	searching      bool
	filters        domain.FilterOptions
	selected       *domain.ModelEntry
	ready          bool
	windowWidth    int
	windowHeight   int
	showHelp       bool
	detailExpanded bool // Whether detail panel shows all fields or condensed view

	// Cached layout calculations
	cachedTableWidth  int
	cachedTableHeight int

	// Key bindings
	keys keyMap
}

// initialModel creates and initializes the model from the given results.
func initialModel(results []domain.RankedModel) model {
	m := model{
		allResults:      results,
		filteredResults: results,
		searchInput:     newSearchInput(),
		help:            help.New(),
		keys:            keys,
	}

	// Initialize the table
	h := len(results)
	if h > 10 {
		h = 10
	}
	m.table = table.New(
		table.WithColumns(tableColumns()),
		table.WithRows(buildTableRows(results)),
		table.WithFocused(true),
		table.WithHeight(h),
	)

	// Initialize the detail detail view viewport
	m.detailView = viewport.New(0, 5) // Will be resized on first WindowSizeMsg
	m.detailView.SetContent("Use ↑↓ to navigate models")

	return m
}

// Init implements tea.Model.
func (m model) Init() tea.Cmd {
	return nil
}
