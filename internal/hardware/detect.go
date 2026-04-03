// Package hardware detects CPU, memory, and GPU information for the current system.
package hardware

import (
	"fmt"
	"runtime"
	"strings"

	"github.com/kevo-1/KnowURLLM/internal/models"
)

// Detect returns the hardware profile of the current machine.
// On partial failure (e.g. GPU detection fails), returns a populated profile
// with zeroed GPU fields and a wrapped error — does NOT return an empty profile.
func Detect() (models.HardwareProfile, error) {
	var profile models.HardwareProfile

	// Detect CPU
	cpuModel, cpuCores, err := detectCPU()
	if err != nil {
		return profile, fmt.Errorf("cpu detection failed: %w", err)
	}
	profile.CPUModel = cpuModel
	profile.CPUCores = cpuCores

	// Detect memory
	totalRAM, err := memory()
	if err != nil {
		return profile, fmt.Errorf("memory detection failed: %w", err)
	}
	profile.TotalRAM = totalRAM

	// Detect GPUs (non-critical failure)
	gpus, gpuErr := gpu()
	if gpuErr != nil {
		// Log GPU detection failure but still return the rest of the profile
		profile.GPUs = []models.GPUInfo{} // zeroed, not nil
		// Return profile with wrapped GPU error
		return profile, fmt.Errorf("gpu detection failed (continuing with CPU-only mode): %w", gpuErr)
	}
	profile.GPUs = gpus

	// Set platform
	profile.Platform = runtime.GOOS

	// Detect Apple Silicon
	profile.IsAppleSilicon = runtime.GOOS == "darwin" && strings.Contains(strings.ToLower(cpuModel), "apple")

	return profile, nil
}
