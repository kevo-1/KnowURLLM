//go:build linux || windows

package hardware

import (
	"context"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/kevo-1/KnowURLLM/internal/domain"
)

// detectNvidiaGPUs detects NVIDIA GPUs using nvidia-smi.
// On Linux, falls back to NVML (see gpu_nvidia_nvml.go).
// On Windows, nvidia-smi is the only method.
func detectNvidiaGPUs() ([]domain.GPUInfo, error) {
	// Primary: nvidia-smi
	gpus, err := detectNvidiaSMI()
	if err == nil && len(gpus) > 0 {
		return gpus, nil
	}

	// Fallback: NVML (Linux only, defined in gpu_nvidia_nvml.go)
	return detectNVMLFallback()
}

// detectNvidiaSMI runs nvidia-smi and parses the output.
func detectNvidiaSMI() ([]domain.GPUInfo, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "nvidia-smi",
		"--query-gpu=name,memory.total",
		"--format=csv,noheader,nounits")
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("nvidia-smi: %w", err)
	}

	return parseNvidiaSMIOutput(string(out)), nil
}

// parseNvidiaSMIOutput parses the CSV output of nvidia-smi.
// Expected format per line: "NVIDIA GeForce RTX 4090, 24576"
// Handles GPU names with commas by taking the last field as VRAM.
func parseNvidiaSMIOutput(raw string) []domain.GPUInfo {
	var gpus []domain.GPUInfo

	lines := strings.Split(strings.TrimSpace(raw), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		// Split from the right to handle GPU names with commas
		lastComma := strings.LastIndex(line, ",")
		if lastComma == -1 {
			continue
		}

		model := strings.TrimSpace(line[:lastComma])
		vramStr := strings.TrimSpace(line[lastComma+1:])

		vramMiB, err := strconv.ParseUint(vramStr, 10, 64)
		if err != nil {
			continue
		}

		// Convert MiB to bytes
		vramBytes := vramMiB * 1024 * 1024

		gpus = append(gpus, domain.GPUInfo{
			Vendor: normalizeGPUVendor(model),
			Model:  model,
			VRAM:   vramBytes,
		})
	}

	return gpus
}

// detectNVMLFallback is a stub for Windows — NVML requires C headers not available on Windows.
// On Linux, the real implementation is in gpu_nvidia_nvml.go.
func detectNVMLFallback() ([]domain.GPUInfo, error) {
	return nil, fmt.Errorf("nvml not available on this platform")
}
