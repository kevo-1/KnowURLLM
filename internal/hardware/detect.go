// Package hardware detects CPU, memory, and GPU information for the current system.
package hardware

import (
	"fmt"
	"runtime"

	"github.com/kevo-1/KnowURLLM/internal/domain"
)

// Detect returns the hardware profile of the current machine.
// On partial failure (e.g. GPU detection fails), returns a populated profile
// with zeroed GPU fields and a wrapped error — does NOT return an empty profile.
func Detect() (domain.HardwareProfile, error) {
	var profile domain.HardwareProfile

	// Detect CPU
	cpuModel, cpuCores, err := detectCPU()
	if err != nil {
		return profile, fmt.Errorf("cpu detection failed: %w", err)
	}
	profile.CPUModel = cpuModel
	profile.CPUCores = cpuCores

	// Detect memory
	totalRAM, availableRAM, err := memory()
	if err != nil {
		return profile, fmt.Errorf("memory detection failed: %w", err)
	}
	profile.TotalRAM = totalRAM
	profile.AvailableRAM = availableRAM

	// Detect GPUs (non-critical failure)
	gpus, gpuErr := gpu()
	if gpuErr != nil {
		// Log GPU detection failure but still return the rest of the profile
		profile.GPUs = []domain.GPUInfo{} // zeroed, not nil
		profile.TotalVRAM = 0
		profile.AvailableVRAM = 0
		// Return profile with wrapped GPU error
		return profile, fmt.Errorf("gpu detection failed (continuing with CPU-only mode): %w", gpuErr)
	}
	profile.GPUs = gpus

	// Calculate total and available VRAM from detected GPUs
	var totalVRAM uint64
	var availableVRAM uint64
	for _, gpu := range gpus {
		totalVRAM += gpu.VRAM
		availableVRAM += calculateAvailableVRAM(gpu)
	}
	profile.TotalVRAM = totalVRAM
	profile.AvailableVRAM = availableVRAM

	// Set platform
	profile.Platform = runtime.GOOS

	// Detect Apple Silicon using runtime.GOARCH (more reliable than string matching)
	profile.IsAppleSilicon = runtime.GOOS == "darwin" && runtime.GOARCH == "arm64"

	return profile, nil
}
