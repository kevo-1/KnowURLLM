// Package tui provides an interactive terminal interface for browsing ranked LLM models.
package tui

import (
	"github.com/kevo-1/KnowURLLM/internal/models"
	tea "github.com/charmbracelet/bubbletea"
)

// App represents the TUI application.
type App struct {
	Results []models.RankResult
}

// NewApp creates a new TUI application.
func NewApp(results []models.RankResult) *App {
	return &App{Results: results}
}

// Run starts the TUI and blocks until the user exits.
// Returns the selected model entry, or a zero-value ModelEntry if the user
// quit without selecting.
func (a *App) Run() (models.ModelEntry, error) {
	m := initialModel(a.Results)
	p := tea.NewProgram(m, tea.WithAltScreen())
	finalModel, err := p.Run()
	if err != nil {
		return models.ModelEntry{}, err
	}
	fm := finalModel.(model)
	if fm.selected != nil {
		return *fm.selected, nil
	}
	return models.ModelEntry{}, nil
}
