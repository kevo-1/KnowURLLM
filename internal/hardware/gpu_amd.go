//go:build linux

package hardware

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/KnowURLLM/internal/models"
)

// detectAMDGPUs detects AMD GPUs using rocm-smi first, falling back to sysfs.
func detectAMDGPUs() ([]models.GPUInfo, error) {
	// Primary: rocm-smi
	gpus, err := detectROCMSMI()
	if err == nil && len(gpus) > 0 {
		return gpus, nil
	}

	// Fallback: sysfs
	return detectAMDSysfs()
}

// detectROCMSMI runs rocm-smi and parses the output.
func detectROCMSMI() ([]models.GPUInfo, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "rocm-smi", "--showmeminfo", "vram", "--csv")
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("rocm-smi: %w", err)
	}

	return parseROCMSMIOutput(string(out))
}

// parseROCMSMIOutput parses the CSV output of rocm-smi.
func parseROCMSMIOutput(raw string) ([]models.GPUInfo, error) {
	var gpus []models.GPUInfo

	lines := strings.Split(strings.TrimSpace(raw), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "device") {
			continue // skip header
		}

		// Expected format varies; look for VRAM Total
		// Common: "GPU ID, VRAM Total (MiB), ..."
		parts := strings.Split(line, ",")
		if len(parts) < 2 {
			continue
		}

		// Try to find a numeric VRAM value in the fields
		for _, part := range parts[1:] {
			part = strings.TrimSpace(part)
			vramMiB, err := strconv.ParseUint(part, 10, 64)
			if err != nil {
				continue
			}

			vramBytes := vramMiB * 1024 * 1024
			gpus = append(gpus, models.GPUInfo{
				Vendor: "amd",
				Model:  "AMD GPU",
				VRAM:   vramBytes,
			})
			break // take first numeric field after GPU ID
		}
	}

	if len(gpus) == 0 {
		return nil, fmt.Errorf("no AMD GPUs found via rocm-smi")
	}

	return gpus, nil
}

// detectAMDSysfs reads GPU info from /sys/class/drm/card*/device/.
func detectAMDSysfs() ([]models.GPUInfo, error) {
	// Find all mem_info_vram_total files
	matches, err := filepath.Glob("/sys/class/drm/card*/device/mem_info_vram_total")
	if err != nil {
		return nil, fmt.Errorf("glob sysfs: %w", err)
	}

	if len(matches) == 0 {
		return nil, fmt.Errorf("no AMD GPU sysfs entries found")
	}

	var gpus []models.GPUInfo
	for _, vramPath := range matches {
		vramBytes, err := readSysfsUint64(vramPath)
		if err != nil {
			continue
		}

		// Get GPU name from product_name
		productName := "AMD GPU"
		namePath := filepath.Join(filepath.Dir(vramPath), "product_name")
		if name, err := readSysfsString(namePath); err == nil {
			productName = strings.TrimSpace(name)
		}

		gpus = append(gpus, models.GPUInfo{
			Vendor: "amd",
			Model:  productName,
			VRAM:   vramBytes,
		})
	}

	if len(gpus) == 0 {
		return nil, fmt.Errorf("no readable AMD GPU sysfs entries")
	}

	return gpus, nil
}

// readSysfsUint64 reads a uint64 value from a sysfs file.
func readSysfsUint64(path string) (uint64, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return 0, err
	}
	val, err := strconv.ParseUint(strings.TrimSpace(string(data)), 10, 64)
	if err != nil {
		return 0, err
	}
	return val, nil
}

// readSysfsString reads a string from a sysfs file.
func readSysfsString(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return string(data), nil
}
