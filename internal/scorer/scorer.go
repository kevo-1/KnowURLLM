// Package scorer calculates how well each LLM runs on a given hardware profile.
package scorer

import (
	"errors"

	"github.com/kevo-1/KnowURLLM/internal/domain"
)

// Scorer calculates how well each model runs on the given hardware.
// Weights are determined per-model based on entry.UseCase via weightsForUseCase().
type Scorer struct{}

// NewScorer returns a Scorer.
func NewScorer() *Scorer {
	return &Scorer{}
}

// validateInput checks that the inputs are valid.
func validateInput(hw domain.HardwareProfile, entries []domain.ModelEntry) error {
	if entries == nil {
		return errors.New("entries must not be nil")
	}
	if hw.TotalRAM == 0 {
		return errors.New("hardware profile has zero RAM (uninitialized profile)")
	}
	return nil
}
