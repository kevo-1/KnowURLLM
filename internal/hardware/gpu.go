package hardware

import (
	"fmt"
	"runtime"

	"github.com/kevo-1/KnowURLLM/internal/domain"
)

// gpu detects all GPUs on the system and returns them.
// This is the coordinator that dispatches to platform-specific implementations.
func gpu() ([]domain.GPUInfo, error) {
	switch runtime.GOOS {
	case "darwin":
		gpus, err := detectAppleGPU()
		if err != nil {
			return []domain.GPUInfo{}, NoGPUError(err)
		}
		return gpus, nil
	case "linux":
		// Merge NVIDIA and AMD results
		nvidiaGPUs, nvidiaErr := detectNvidiaGPUs()
		amdGPUs, amdErr := detectAMDGPUs()

		gpus := append(nvidiaGPUs, amdGPUs...)

		// If both failed, return error with empty slice
		if nvidiaErr != nil && amdErr != nil {
			return gpus, NoGPUError(fmt.Errorf("nvidia: %v, amd: %v", nvidiaErr, amdErr))
		}
		// If one failed, return partial error but still return results from the other
		if nvidiaErr != nil {
			return gpus, PartialGPUError(nvidiaErr)
		}
		if amdErr != nil {
			return gpus, PartialGPUError(amdErr)
		}
		return gpus, nil
	case "windows":
		// NVIDIA only on Windows; AMD Windows detection is out of scope
		gpus, err := detectNvidiaGPUs()
		if err != nil {
			return []domain.GPUInfo{}, NoGPUError(err)
		}
		return gpus, nil
	default:
		return []domain.GPUInfo{}, NoGPUError(fmt.Errorf("unsupported platform: %s", runtime.GOOS))
	}
}
