package hardware

import (
	"runtime"

	"github.com/KnowURLLM/internal/models"
)

// gpu detects all GPUs on the system and returns them.
// This is the coordinator that dispatches to platform-specific implementations.
func gpu() ([]models.GPUInfo, error) {
	switch runtime.GOOS {
	case "darwin":
		return detectAppleGPU()
	case "linux":
		// Merge NVIDIA and AMD results
		nvidiaGPUs, nvidiaErr := detectNvidiaGPUs()
		amdGPUs, amdErr := detectAMDGPUs()

		gpus := append(nvidiaGPUs, amdGPUs...)

		// If both failed, return error with empty slice
		if nvidiaErr != nil && amdErr != nil {
			return gpus, amdErr // return one of the errors
		}
		// If one failed, return the error but still return results from the other
		if nvidiaErr != nil {
			return gpus, nvidiaErr
		}
		if amdErr != nil {
			return gpus, amdErr
		}
		return gpus, nil
	case "windows":
		// NVIDIA only on Windows; AMD Windows detection is out of scope
		return detectNvidiaGPUs()
	default:
		return nil, nil
	}
}
