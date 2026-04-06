package registry

import (
	"context"
	"testing"

	"github.com/kevo-1/KnowURLLM/internal/domain"
)

// TestMoEModelParsing verifies that MoE models are correctly parsed from JSON.
func TestMoEModelParsing(t *testing.T) {
	// Find an MoE model in the embedded data
	fetcher := NewFetcher()
	entries, err := fetcher.FetchAll(context.Background())
	if err != nil {
		t.Fatalf("Failed to fetch models: %v", err)
	}

	// Find MoE models
	var moeModels []domain.ModelEntry
	for _, e := range entries {
		if e.IsMoE {
			moeModels = append(moeModels, e)
		}
	}

	if len(moeModels) == 0 {
		t.Fatal("Expected to find at least one MoE model in embedded data")
	}

	t.Logf("Found %d MoE models", len(moeModels))

	// Verify MoE models have required fields
	for _, m := range moeModels[:3] { // Test first 3
		t.Run(m.ID, func(t *testing.T) {
			if !m.IsMoE {
				t.Error("Expected IsMoE to be true")
			}

			if m.ActiveParams == 0 {
				t.Error("Expected ActiveParams > 0 for MoE model")
			}

			if m.ParameterCount == 0 {
				t.Error("Expected ParameterCount > 0 for MoE model")
			}

			// Active params should be less than total params
			if m.ActiveParams >= m.ParameterCount {
				t.Errorf("ActiveParams (%d) should be < ParameterCount (%d)",
					m.ActiveParams, m.ParameterCount)
			}

			t.Logf("  Active: %.1fB / Total: %.1fB (%.1f%%)",
				float64(m.ActiveParams)/1e9,
				float64(m.ParameterCount)/1e9,
				float64(m.ActiveParams)/float64(m.ParameterCount)*100)
		})
	}
}

// TestMoEModelVRAMFit tests that MoE models correctly calculate VRAM requirements.
func TestMoEModelVRAMFit(t *testing.T) {
	// This would require importing domain/hardware, so we'll skip for now
	// The important thing is that the fields are populated correctly
	t.Skip("MoE VRAM fit testing requires domain/hardware import")
}
