//go:build linux || windows

package hardware

import (
	"context"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/kevo-1/KnowURLLM/internal/models"
)

// detectNvidiaGPUs detects NVIDIA GPUs using nvidia-smi.
// On Linux, falls back to NVML (see gpu_nvidia_nvml.go).
// On Windows, nvidia-smi is the only method.
func detectNvidiaGPUs() ([]models.GPUInfo, error) {
	// Primary: nvidia-smi
	gpus, err := detectNvidiaSMI()
	if err == nil && len(gpus) > 0 {
		return gpus, nil
	}

	// Fallback: NVML (Linux only, defined in gpu_nvidia_nvml.go)
	return detectNVMLFallback()
}

// detectNvidiaSMI runs nvidia-smi and parses the output.
func detectNvidiaSMI() ([]models.GPUInfo, error) {
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
func parseNvidiaSMIOutput(raw string) []models.GPUInfo {
	var gpus []models.GPUInfo

	lines := strings.Split(strings.TrimSpace(raw), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		parts := strings.SplitN(line, ",", 2)
		if len(parts) != 2 {
			continue
		}

		model := strings.TrimSpace(parts[0])
		vramStr := strings.TrimSpace(parts[1])

		vramMiB, err := strconv.ParseUint(vramStr, 10, 64)
		if err != nil {
			continue
		}

		// Convert MiB to bytes
		vramBytes := vramMiB * 1024 * 1024

		gpus = append(gpus, models.GPUInfo{
			Vendor: "nvidia",
			Model:  model,
			VRAM:   vramBytes,
		})
	}

	return gpus
}

// detectNVMLFallback is a stub for Windows — NVML requires C headers not available on Windows.
// On Linux, the real implementation is in gpu_nvidia_nvml.go.
func detectNVMLFallback() ([]models.GPUInfo, error) {
	return nil, fmt.Errorf("nvml not available on this platform")
}
