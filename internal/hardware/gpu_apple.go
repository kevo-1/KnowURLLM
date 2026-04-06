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

	"github.com/kevo-1/KnowURLLM/internal/domain"
)

// detectAppleGPU detects GPU information on macOS.
// Handles both Apple Silicon (M1/M2/M3) and Intel Macs with integrated GPUs.
func detectAppleGPU() ([]domain.GPUInfo, error) {
	// Check if this is Apple Silicon or Intel
	if isAppleSiliconMac() {
		return detectAppleSiliconGPU()
	}
	return detectIntelMacGPU()
}

// isAppleSiliconMac checks if this is an Apple Silicon Mac
func isAppleSiliconMac() bool {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	
	out, err := exec.CommandContext(ctx, "uname", "-m").Output()
	if err != nil {
		return false
	}
	return strings.TrimSpace(string(out)) == "arm64"
}

// detectAppleSiliconGPU detects GPU on Apple Silicon Macs
func detectAppleSiliconGPU() ([]domain.GPUInfo, error) {
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

	// On Apple Silicon, VRAM is unified with system RAM.
	// We report the full unified memory as "VRAM" because the GPU can access
	// the entire memory pool. The calculateAvailableVRAM() function will
	// apply the ~25% OS reservation factor when computing usable memory.
	return []domain.GPUInfo{
		{
			Vendor: normalizeGPUVendor(gpuModel),
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

// detectIntelMacGPU detects GPU on Intel-based Macs
func detectIntelMacGPU() ([]domain.GPUInfo, error) {
	// Use system_profiler to get GPU info on Intel Macs
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	out, err := exec.CommandContext(ctx, "system_profiler", "SPDisplaysDataType", "-json").Output()
	if err != nil {
		return nil, fmt.Errorf("system_profiler: %w", err)
	}

	return parseIntelMacGPUOutput(string(out))
}

// parseIntelMacGPUOutput parses system_profiler output for Intel Mac GPUs
func parseIntelMacGPUOutput(raw string) ([]domain.GPUInfo, error) {
	var result map[string]interface{}
	if err := json.Unmarshal([]byte(raw), &result); err != nil {
		return nil, fmt.Errorf("parsing system_profiler JSON: %w", err)
	}

	var gpus []domain.GPUInfo

	// Navigate to GPU data
	data, ok := result["SPDisplaysDataType"].([]interface{})
	if !ok || len(data) == 0 {
		return nil, fmt.Errorf("no SPDisplaysDataType data")
	}

	for _, item := range data {
		if arr, ok := item.(map[string]interface{}); ok {
			if devices, ok := arr["sppci_devices"].([]interface{}); ok {
				for _, dev := range devices {
					if device, ok := dev.(map[string]interface{}); ok {
						var gpuName string
						if name, ok := device["spdevice_name"].(string); ok {
							gpuName = name
						} else if name, ok := device["sppci_model"].(string); ok {
							gpuName = name
						} else {
							continue
						}

						// Intel integrated GPUs share system memory
						// Estimate VRAM as 512MB-2GB (typical for integrated GPUs)
						// We use a conservative 1GB estimate for scoring purposes
						vramBytes := uint64(1024 * 1024 * 1024) // 1GB

						gpus = append(gpus, domain.GPUInfo{
							Vendor: normalizeGPUVendor(gpuName),
							Model:  gpuName,
							VRAM:   vramBytes,
						})
					}
				}
			}
		}
	}

	if len(gpus) == 0 {
		return nil, fmt.Errorf("no Intel GPUs found")
	}

	return gpus, nil
}
