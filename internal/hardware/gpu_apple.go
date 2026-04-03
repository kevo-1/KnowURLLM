//go:build darwin

package hardware

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/KnowURLLM/internal/models"
)

// detectAppleGPU detects GPU information on macOS Apple Silicon.
func detectAppleGPU() ([]models.GPUInfo, error) {
	// Get total unified memory via sysctl
	totalMem, err := getUnifiedMemoryBytes()
	if err != nil {
		return nil, fmt.Errorf("getting unified memory: %w", err)
	}

	// Get GPU model name via system_profiler
	gpuModel, err := getGPUModelName()
	if err != nil {
		gpuModel = "Apple GPU" // fallback
	}

	// VRAM Decision:
	// On Apple Silicon, VRAM is unified with system RAM. The GPU shares the same
	// memory pool as the CPU. We report the full unified memory amount as "VRAM"
	// because the scorer needs the total available memory pool for model loading.
	// The scoring formula will account for this by considering the full memory pool.
	// We do NOT apply an OS-reserve deduction here — that's a scoring concern, not
	// a detection concern. The scorer knows that macOS needs ~25% for the OS and
	// will adjust accordingly via its availableMemory calculation.
	return []models.GPUInfo{
		{
			Vendor: "apple",
			Model:  gpuModel,
			VRAM:   totalMem,
		},
	}, nil
}

// getUnifiedMemoryBytes returns the total unified memory size in bytes.
func getUnifiedMemoryBytes() (uint64, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	out, err := exec.CommandContext(ctx, "sysctl", "-n", "hw.memsize").Output()
	if err != nil {
		return 0, fmt.Errorf("sysctl hw.memsize: %w", err)
	}

	val, err := strconv.ParseUint(strings.TrimSpace(string(out)), 10, 64)
	if err != nil {
		return 0, fmt.Errorf("parsing hw.memsize: %w", err)
	}

	return val, nil
}

// getGPUModelName returns the GPU model name from system_profiler.
func getGPUModelName() (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	out, err := exec.CommandContext(ctx, "system_profiler", "SPDisplaysDataType", "-json").Output()
	if err != nil {
		return "", fmt.Errorf("system_profiler: %w", err)
	}

	// Parse JSON response
	// Expected structure:
	// {
	//   "SPDisplaysDataType": [
	//     {
	//       "sppci_devices": [
	//         {
	//           "spdevice_name": "Apple M2 Max",
	//           ...
	//         }
	//       ]
	//     }
	//   ]
	// }
	var result map[string]interface{}
	if err := json.Unmarshal(out, &result); err != nil {
		return "", fmt.Errorf("parsing system_profiler JSON: %w", err)
	}

	// Navigate to GPU name
	data, ok := result["SPDisplaysDataType"].([]interface{})
	if !ok || len(data) == 0 {
		return "", fmt.Errorf("no SPDisplaysDataType data")
	}

	if arr, ok := data[0].(map[string]interface{}); ok {
		if devices, ok := arr["sppci_devices"].([]interface{}); ok && len(devices) > 0 {
			if device, ok := devices[0].(map[string]interface{}); ok {
				if name, ok := device["spdevice_name"].(string); ok {
					return name, nil
				}
				if name, ok := device["sppci_model"].(string); ok {
					return name, nil
				}
			}
		}
	}

	return "", fmt.Errorf("could not find GPU model name in system_profiler output")
}
