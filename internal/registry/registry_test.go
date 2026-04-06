package registry

import (
	"context"
	"testing"
	"time"
)

// TestFetchAll verifies that FetchAll loads models from the embedded JSON.
func TestFetchAll(t *testing.T) {
	f := NewFetcher()

	entries, err := f.FetchAll(context.Background())
	if err != nil {
		t.Fatalf("FetchAll failed: %v", err)
	}

	if len(entries) == 0 {
		t.Fatal("expected at least one entry from embedded data")
	}

	// Verify first entry (should be highest downloads)
	first := entries[0]
	if first.Downloads == 0 {
		t.Error("expected non-zero downloads on first entry")
	}
	if first.Source != "huggingface" {
		t.Errorf("expected source 'huggingface', got %s", first.Source)
	}
	if first.ID == "" {
		t.Error("expected non-empty ID")
	}
	if first.DisplayName == "" {
		t.Error("expected non-empty DisplayName")
	}
	if first.ModelSizeBytes == 0 {
		t.Error("expected non-zero ModelSizeBytes")
	}
}

// TestFetchAllWithLimit verifies that MaxModels limits results.
func TestFetchAllWithLimit(t *testing.T) {
	f := NewFetcher(WithMaxModels(5))

	entries, err := f.FetchAll(context.Background())
	if err != nil {
		t.Fatalf("FetchAll failed: %v", err)
	}

	if len(entries) != 5 {
		t.Errorf("expected 5 entries with limit=5, got %d", len(entries))
	}
}

// TestFetchAllSortedByDownloads verifies results are sorted by downloads descending.
func TestFetchAllSortedByDownloads(t *testing.T) {
	f := NewFetcher()

	entries, err := f.FetchAll(context.Background())
	if err != nil {
		t.Fatalf("FetchAll failed: %v", err)
	}

	for i := 1; i < len(entries); i++ {
		if entries[i].Downloads > entries[i-1].Downloads {
			t.Errorf("entries not sorted by downloads: [%d]=%d > [%d]=%d",
				i, entries[i].Downloads, i-1, entries[i-1].Downloads)
			break
		}
	}
}

// TestFetchHuggingFaceCompatibility verifies the compatibility alias.
func TestFetchHuggingFaceCompatibility(t *testing.T) {
	f := NewFetcher(WithMaxModels(10))

	entries, err := f.FetchHuggingFace(context.Background())
	if err != nil {
		t.Fatalf("FetchHuggingFace failed: %v", err)
	}

	if len(entries) != 10 {
		t.Errorf("expected 10 entries, got %d", len(entries))
	}
}

// TestFetchOllamaIntegration verifies Ollama fetch works (may be skipped offline).
func TestFetchOllamaIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping Ollama integration test in short mode")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	f := NewFetcher()
	entries, err := f.FetchOllama(ctx)
	if err != nil {
		t.Fatalf("FetchOllama failed: %v", err)
	}

	// We expect at least some models from the Ollama library
	if len(entries) == 0 {
		t.Log("warning: no Ollama models returned — may be network issue or API change")
	}

	// Verify entries have proper fields populated
	for _, e := range entries {
		if e.ID == "" {
			t.Error("expected non-empty ID")
		}
		if e.Source != "ollama" {
			t.Errorf("expected source 'ollama', got %s", e.Source)
		}
		if e.URL == "" {
			t.Errorf("expected non-empty URL for %s", e.ID)
		}
	}
}

// TestParseParameterSize tests the parameter size parser.
func TestParseParameterSize(t *testing.T) {
	tests := []struct {
		input    string
		expected uint64
	}{
		{"8B", 8_000_000_000},
		{"70B", 70_000_000_000},
		{"0.5B", 500_000_000},
		{"1.5B", 1_500_000_000},
		{"3.8B", 3_800_000_000},
		{"", 0},
		{"invalid", 0},
		{"7b", 7_000_000_000}, // lowercase
		{" 8B ", 8_000_000_000}, // whitespace
	}

	for _, tc := range tests {
		t.Run(tc.input, func(t *testing.T) {
			got := parseParameterSize(tc.input)
			if got != tc.expected {
				t.Errorf("parseParameterSize(%q) = %d, expected %d", tc.input, got, tc.expected)
			}
		})
	}
}

// TestParseFloat tests the float parser with scientific notation and edge cases.
func TestParseFloat(t *testing.T) {
	tests := []struct {
		input     string
		expected  float64
		shouldErr bool
	}{
		{"3.8e9", 3.8e9, false},
		{"1.5e9", 1.5e9, false},
		{"70e9", 70e9, false},
		{"3.14159", 3.14159, false},
		{"0.5", 0.5, false},
		{"-2.5", -2.5, false},
		{"invalid", 0, true},
		{"", 0, true},
	}

	for _, tc := range tests {
		t.Run(tc.input, func(t *testing.T) {
			got, err := parseFloat(tc.input)
			if tc.shouldErr && err == nil {
				t.Errorf("parseFloat(%q) expected error, got nil", tc.input)
			}
			if !tc.shouldErr && err != nil {
				t.Errorf("parseFloat(%q) unexpected error: %v", tc.input, err)
			}
			if !tc.shouldErr && got != tc.expected {
				t.Errorf("parseFloat(%q) = %v, expected %v", tc.input, got, tc.expected)
			}
		})
	}
}

// TestParseQuantizationFromFilename tests quantization extraction from filenames.
func TestParseQuantizationFromFilename(t *testing.T) {
	tests := []struct {
		filename string
		expected string
	}{
		{"model-Q4_K_M.gguf", "Q4_K_M"},
		{"model-Q8_0.gguf", "Q8_0"},
		{"model-FP16.gguf", "FP16"},
		{"model-FP32.gguf", "FP32"},
		{"mistral-7b-instruct-v0.3-q4_k_m.gguf", "Q4_K_M"},
		{"qwen2.5-7b-instruct-q4_k_m.gguf", "Q4_K_M"},
		{"model-Q4_K_S.gguf", "Q4_K_M"}, // Normalized
		{"model-Q5_K_S.gguf", "Q5_K_M"}, // Normalized
		{"model.gguf", "unknown"},
		{"not-a-gguf.txt", "unknown"},
	}

	for _, tc := range tests {
		t.Run(tc.filename, func(t *testing.T) {
			got := parseQuantizationFromFilename(tc.filename)
			if got != tc.expected {
				t.Errorf("parseQuantizationFromFilename(%q) = %q, expected %q", tc.filename, got, tc.expected)
			}
		})
	}
}

// TestBytesPerParam tests the bytes-per-param lookup table.
func TestBytesPerParam(t *testing.T) {
	tests := []struct {
		quant    string
		expected float64
	}{
		{"Q2_K", 0.313},
		{"Q3_K", 0.438},
		{"Q4_K_M", 0.563},
		{"Q5_K_M", 0.688},
		{"Q8_0", 1.0},
		{"FP16", 2.0},
		{"FP32", 4.0},
		{"unknown", 0.563}, // Default
		{"", 0.563},        // Default
		{"q4_k_m", 0.563},  // Case insensitive
	}

	for _, tc := range tests {
		t.Run(tc.quant, func(t *testing.T) {
			got := bytesPerParam(tc.quant)
			if got != tc.expected {
				t.Errorf("bytesPerParam(%q) = %f, expected %f", tc.quant, got, tc.expected)
			}
		})
	}
}

// TestNormalizeModelName tests the model name normalization.
func TestNormalizeModelName(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"meta-llama/Llama-3.1-8B-Instruct", "llama318"},
		{"llama3.1:8b-instruct-q4_K_M", "llama318"},
		{"mistralai/Mistral-7B-Instruct-v0.3", "mistral703"},
		{"mistral:7b-instruct-v0.3-q4_K_M", "mistral703"},
	}

	for _, tc := range tests {
		t.Run(tc.input, func(t *testing.T) {
			got := normalizeModelName(tc.input)
			if got != tc.expected {
				t.Errorf("normalizeModelName(%q) = %q, expected %q", tc.input, got, tc.expected)
			}
		})
	}
}

// TestFetcherDefaults tests that NewFetcher sets correct defaults.
func TestFetcherDefaults(t *testing.T) {
	fetcher := NewFetcher()

	if fetcher.MaxModels != 0 {
		t.Errorf("expected MaxModels=0 (no limit), got %d", fetcher.MaxModels)
	}
}

// TestFetcherWithOptions tests that functional options work correctly.
func TestFetcherWithOptions(t *testing.T) {
	fetcher := NewFetcher(
		WithMaxModels(100),
	)

	if fetcher.MaxModels != 100 {
		t.Errorf("expected MaxModels=100, got %d", fetcher.MaxModels)
	}
}

// TestModelSizeCalculation verifies that model sizes are calculated correctly.
func TestModelSizeCalculation(t *testing.T) {
	f := NewFetcher()
	entries, err := f.FetchAll(context.Background())
	if err != nil {
		t.Fatalf("FetchAll failed: %v", err)
	}

	// Find a known model and verify its size
	for _, e := range entries {
		if e.ID == "Qwen/Qwen2.5-7B-Instruct" {
			// 7.6B params × 0.563 bytes/param (Q4_K_M) ≈ 4.3 GB
			paramCount := float64(7615616512)
			expectedSize := uint64(paramCount * 0.563)
			if e.ModelSizeBytes == 0 {
				t.Error("expected non-zero model size")
			}
			// Verify it's in the right ballpark (within 10%)
			lo := expectedSize * 9 / 10
			hi := expectedSize * 11 / 10
			if e.ModelSizeBytes < lo || e.ModelSizeBytes > hi {
				t.Errorf("Qwen2.5-7B size %d not within 10%% of expected %d",
					e.ModelSizeBytes, expectedSize)
			}
			break
		}
	}
}

// TestModelTags verifies that models have meaningful tags.
func TestModelTags(t *testing.T) {
	f := NewFetcher()
	entries, err := f.FetchAll(context.Background())
	if err != nil {
		t.Fatalf("FetchAll failed: %v", err)
	}

	// Find a model with capabilities
	for _, e := range entries {
		if e.ID == "Qwen/Qwen2.5-7B-Instruct" {
			if len(e.Tags) == 0 {
				t.Error("expected Qwen2.5-7B-Instruct to have tags")
			}
			break
		}
	}
}

// TestModelContextLength verifies context length is populated.
func TestModelContextLength(t *testing.T) {
	f := NewFetcher()
	entries, err := f.FetchAll(context.Background())
	if err != nil {
		t.Fatalf("FetchAll failed: %v", err)
	}

	var withContext int
	for _, e := range entries {
		if e.ContextLength > 0 {
			withContext++
		}
	}

	if withContext == 0 {
		t.Error("expected at least some models to have context length")
	}
}

// TestBenchmarkEnrichment verifies that popular models get MMLU/ArenaELO scores.
func TestBenchmarkEnrichment(t *testing.T) {
	f := NewFetcher()
	entries, err := f.FetchAll(context.Background())
	if err != nil {
		t.Fatalf("FetchAll failed: %v", err)
	}

	// Check specific models that should have benchmark data
	benchmarks := map[string]struct {
		wantMMLU float64
		wantELO  float64
	}{
		"meta-llama/Llama-3.1-8B-Instruct":  {69.4, 1190},
		"Qwen/Qwen2.5-7B-Instruct":          {74.5, 1200},
		"microsoft/Phi-3-mini-4k-instruct":  {69.0, 1160},
		"google/gemma-2-9b-it":              {65.0, 1160},
	}

	for _, e := range entries {
		if want, ok := benchmarks[e.ID]; ok {
			if e.MMLUScore == 0 {
				t.Errorf("%s: expected MMLUScore > 0, got 0", e.ID)
			}
			if e.MMLUScore != want.wantMMLU {
				t.Errorf("%s: MMLUScore = %f, want %f", e.ID, e.MMLUScore, want.wantMMLU)
			}
			if e.ArenaELO != want.wantELO {
				t.Errorf("%s: ArenaELO = %f, want %f", e.ID, e.ArenaELO, want.wantELO)
			}
		}
	}
}
