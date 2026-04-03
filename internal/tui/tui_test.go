package tui

import (
	"testing"

	"github.com/KnowURLLM/internal/models"
	tea "github.com/charmbracelet/bubbletea"
)

// makeTestResults creates a slice of RankResult for testing.
func makeTestResults(n int) []models.RankResult {
	results := make([]models.RankResult, n)
	for i := 0; i < n; i++ {
		results[i] = models.RankResult{
			Model: models.ModelEntry{
				ID:             "model-" + string(rune('0'+i)),
				DisplayName:    "Model " + string(rune('0'+i)),
				ModelSizeBytes: uint64(4 * 1024 * 1024 * 1024),
				Quantization:   "Q4_K_M",
				ContextLength:  4096,
				Source:         "huggingface",
				MMLUScore:      70.0 + float64(i)*2,
				ArenaELO:       1100 + float64(i)*20,
				Downloads:      1000 * (i + 1),
				URL:            "https://example.com/model-" + string(rune('0'+i)),
				Tags:           []string{"text-generation", "conversational"},
			},
			Score: models.ModelScore{
				TotalScore:       90.0 - float64(i)*3,
				HardwareFitScore: 95.0,
				ThroughputScore:  80.0,
				QualityScore:     70.0 + float64(i)*2,
				EstimatedTPS:     40.0 + float64(i)*5,
				FitsInVRAM:       i < 2,
				FitsInMemory:     true,
				FitReason:        "Fits in VRAM with 40% headroom",
			},
			Rank: i + 1,
		}
	}
	return results
}

// upd calls Update and returns the concrete model.
func upd(t *testing.T, m model, msg tea.Msg) model {
	t.Helper()
	tm, cmd := m.Update(msg)
	cm, ok := tm.(model)
	if !ok {
		t.Fatalf("expected model type, got %T", tm)
	}
	_ = cmd
	return cm
}

// TestNavigation verifies that navigation keys move the table cursor.
func TestNavigation(t *testing.T) {
	results := makeTestResults(5)
	m := initialModel(results)

	m = upd(t, m, keyMsg(tea.KeyDown))
	m = upd(t, m, keyMsg(tea.KeyDown))

	if cursor := m.table.Cursor(); cursor != 2 {
		t.Errorf("expected cursor at 2, got %d", cursor)
	}

	m = upd(t, m, keyMsg(tea.KeyUp))
	if cursor := m.table.Cursor(); cursor != 1 {
		t.Errorf("expected cursor at 1, got %d", cursor)
	}
}

// TestVimNavigation verifies j/k navigation.
func TestVimNavigation(t *testing.T) {
	results := makeTestResults(5)
	m := initialModel(results)

	m = upd(t, m, keyMsg(tea.KeyRunes, 'j'))
	m = upd(t, m, keyMsg(tea.KeyRunes, 'j'))

	if cursor := m.table.Cursor(); cursor != 2 {
		t.Errorf("expected cursor at 2 after jj, got %d", cursor)
	}

	m = upd(t, m, keyMsg(tea.KeyRunes, 'k'))
	if cursor := m.table.Cursor(); cursor != 1 {
		t.Errorf("expected cursor at 1 after k, got %d", cursor)
	}
}

// TestSearch verifies that search filters results.
func TestSearch(t *testing.T) {
	results := makeTestResults(5)
	results[1].Model.DisplayName = "unique-llama-model"

	m := initialModel(results)

	m = upd(t, m, keyMsg(tea.KeyRunes, '/'))
	for _, r := range []rune("unique") {
		m = upd(t, m, keyMsg(tea.KeyRunes, r))
	}

	if len(m.filteredResults) != 1 {
		t.Errorf("expected 1 filtered result, got %d", len(m.filteredResults))
	}
	if m.filteredResults[0].Model.DisplayName != "unique-llama-model" {
		t.Errorf("expected unique-llama-model, got %s", m.filteredResults[0].Model.DisplayName)
	}
}

// TestSelect verifies that Enter sets the selected model.
func TestSelect(t *testing.T) {
	results := makeTestResults(3)
	m := initialModel(results)

	m = upd(t, m, keyMsg(tea.KeyEnter))

	if m.selected == nil {
		t.Fatal("expected selected model, got nil")
	}
	if m.selected.ID != results[0].Model.ID {
		t.Errorf("expected %s, got %s", results[0].Model.ID, m.selected.ID)
	}
}

// TestSelectAfterNavigation verifies selecting a non-first row.
func TestSelectAfterNavigation(t *testing.T) {
	results := makeTestResults(3)
	m := initialModel(results)

	m = upd(t, m, keyMsg(tea.KeyDown))
	m = upd(t, m, keyMsg(tea.KeyDown))
	m = upd(t, m, keyMsg(tea.KeyEnter))

	if m.selected == nil {
		t.Fatal("expected selected model, got nil")
	}
	if m.selected.ID != results[2].Model.ID {
		t.Errorf("expected %s, got %s", results[2].Model.ID, m.selected.ID)
	}
}

// TestQuit verifies that 'q' returns zero-value selection.
func TestQuit(t *testing.T) {
	results := makeTestResults(3)
	m := initialModel(results)

	m = upd(t, m, keyMsg(tea.KeyRunes, 'q'))

	if m.selected != nil {
		t.Errorf("expected nil selected, got %+v", m.selected)
	}
}

// TestVRAMOnlyFilter verifies the VRAM-only toggle.
func TestVRAMOnlyFilter(t *testing.T) {
	results := makeTestResults(5)
	results[0].Score.FitsInVRAM = true
	results[1].Score.FitsInVRAM = true
	results[2].Score.FitsInVRAM = false
	results[3].Score.FitsInVRAM = false
	results[4].Score.FitsInVRAM = false

	m := initialModel(results)

	m = upd(t, m, keyMsg(tea.KeyRunes, 'v'))

	if !m.filters.VRAMOnly {
		t.Error("expected VRAMOnly to be true")
	}
	if len(m.filteredResults) != 2 {
		t.Errorf("expected 2 VRAM-only results, got %d", len(m.filteredResults))
	}
}

// TestWindowSizeMsg verifies that resize doesn't panic.
func TestWindowSizeMsg(t *testing.T) {
	results := makeTestResults(5)
	m := initialModel(results)

	m = upd(t, m, windowSizeMsg(120, 40))

	if !m.ready {
		t.Error("expected ready to be true")
	}
}

// TestClearSearch verifies that Escape clears the search.
func TestClearSearch(t *testing.T) {
	results := makeTestResults(5)
	m := initialModel(results)

	m.filters.SearchQuery = "model-0"
	m.applyFilters()

	m = upd(t, m, keyMsg(tea.KeyEscape))

	if m.filters.SearchQuery != "" {
		t.Errorf("expected search query to be empty, got %s", m.filters.SearchQuery)
	}
	if len(m.filteredResults) != 5 {
		t.Errorf("expected 5 results after clearing search, got %d", len(m.filteredResults))
	}
}

// TestHelpToggle verifies that ? toggles the help panel.
func TestHelpToggle(t *testing.T) {
	results := makeTestResults(3)
	m := initialModel(results)

	if m.showHelp {
		t.Error("expected showHelp to be false initially")
	}

	m = upd(t, m, keyMsg(tea.KeyRunes, '?'))
	if !m.showHelp {
		t.Error("expected showHelp to be true after ?")
	}

	m = upd(t, m, keyMsg(tea.KeyRunes, '?'))
	if m.showHelp {
		t.Error("expected showHelp to be false after second ?")
	}
}

// TestFormatBytes verifies the byte formatting function.
func TestFormatBytes(t *testing.T) {
	tests := []struct {
		input    uint64
		expected string
	}{
		{500, "500 B"},
		{1024, "1 KB"},
		{1536, "2 KB"},
		{1048576, "1 MB"},
		{524288000, "500 MB"},
		{4294967296, "4.0 GB"},
		{5368709120, "5.0 GB"},
	}

	for _, tc := range tests {
		got := formatBytes(tc.input)
		if got != tc.expected {
			t.Errorf("formatBytes(%d) = %s, expected %s", tc.input, got, tc.expected)
		}
	}
}

// TestFormatContext verifies context length formatting.
func TestFormatContext(t *testing.T) {
	tests := []struct {
		input    int
		expected string
	}{
		{512, "512"},
		{2048, "2k"},
		{4096, "4k"},
		{128000, "128k"},
	}

	for _, tc := range tests {
		got := formatContext(tc.input)
		if got != tc.expected {
			t.Errorf("formatContext(%d) = %s, expected %s", tc.input, got, tc.expected)
		}
	}
}

// TestTruncate verifies string truncation.
func TestTruncate(t *testing.T) {
	tests := []struct {
		input    string
		maxLen   int
		expected string
	}{
		{"short", 10, "short"},
		{"hello world", 8, "hello…"},
		{"abc", 3, "abc"},
		{"abcdef", 3, "abc"},
	}

	for _, tc := range tests {
		got := truncate(tc.input, tc.maxLen)
		if got != tc.expected {
			t.Errorf("truncate(%q, %d) = %q, expected %q", tc.input, tc.maxLen, got, tc.expected)
		}
	}
}

// TestFilterResults verifies the filterResults function.
func TestFilterResults(t *testing.T) {
	results := makeTestResults(5)

	got := filterResults(results, models.FilterOptions{})
	if len(got) != 5 {
		t.Errorf("expected 5 results with no filters, got %d", len(got))
	}

	// Search by display name (case-insensitive)
	got = filterResults(results, models.FilterOptions{SearchQuery: "model 0"})
	if len(got) != 1 {
		t.Errorf("expected 1 result for search, got %d", len(got))
	}

	got = filterResults(results, models.FilterOptions{VRAMOnly: true})
	if len(got) != 2 {
		t.Errorf("expected 2 VRAM results, got %d", len(got))
	}
}

// Key message helpers
func keyMsg(kt tea.KeyType, runes ...rune) tea.KeyMsg {
	k := tea.KeyMsg{Type: kt}
	if len(runes) > 0 {
		k.Runes = runes
	}
	return k
}

func windowSizeMsg(w, h int) tea.WindowSizeMsg {
	return tea.WindowSizeMsg{Width: w, Height: h}
}
