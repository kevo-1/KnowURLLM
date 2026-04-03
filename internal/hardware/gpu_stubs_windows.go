//go:build windows

package hardware

import (
	"fmt"

	"github.com/kevo-1/KnowURLLM/internal/models"
)

// detectAppleGPU is not available on Windows — this is a stub.
func detectAppleGPU() ([]models.GPUInfo, error) {
	return nil, fmt.Errorf("apple GPU detection not available on windows")
}

// detectAMDGPUs is not available on Windows — this is a stub.
func detectAMDGPUs() ([]models.GPUInfo, error) {
	return nil, fmt.Errorf("AMD GPU detection not available on windows")
}
