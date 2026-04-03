package tui

import (
	"github.com/KnowURLLM/internal/models"
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
}

// ShortHelp returns keybindings for the help bar.
func (k keyMap) ShortHelp() []key.Binding {
	return []key.Binding{k.Up, k.Down, k.Search, k.VRAMOnly, k.Select, k.Quit, k.Help}
}

// FullHelp returns the full help (same as short for this TUI).
func (k keyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{k.ShortHelp()}
}

// model is the Bubble Tea application state.
type model struct {
	// Data
	allResults      []models.RankResult
	filteredResults []models.RankResult

	// Components
	table       table.Model
	searchInput textinput.Model
	help        help.Model
	detailView  viewport.Model

	// State
	searching    bool
	filters      models.FilterOptions
	selected     *models.ModelEntry
	ready        bool
	windowWidth  int
	windowHeight int
	showHelp     bool

	// Key bindings
	keys keyMap
}

// initialModel creates and initializes the model from the given results.
func initialModel(results []models.RankResult) model {
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

	return m
}

// Init implements tea.Model.
func (m model) Init() tea.Cmd {
	return nil
}
