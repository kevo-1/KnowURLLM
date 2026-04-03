// Package scorer calculates how well each LLM runs on a given hardware profile.
package scorer

import (
	"errors"

	"github.com/KnowURLLM/internal/models"
)

// Scorer calculates how well each model runs on the given hardware.
type Scorer struct {
	// Weights must sum to 1.0.
	HardwareFitWeight float64 // default 0.50
	ThroughputWeight  float64 // default 0.30
	QualityWeight     float64 // default 0.20
}

// NewScorer returns a Scorer with default weights.
func NewScorer() *Scorer {
	return &Scorer{
		HardwareFitWeight: 0.50,
		ThroughputWeight:  0.30,
		QualityWeight:     0.20,
	}
}

// validateInput checks that the inputs are valid.
func validateInput(hw models.HardwareProfile, entries []models.ModelEntry) error {
	if entries == nil {
		return errors.New("entries must not be nil")
	}
	if hw.TotalRAM == 0 {
		return errors.New("hardware profile has zero RAM (uninitialized profile)")
	}
	return nil
}
