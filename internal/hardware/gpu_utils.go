package hardware

import (
	"os"
	"runtime"
	"strings"

	"github.com/kevo-1/KnowURLLM/internal/models"
)

// calculateAvailableVRAM estimates the actually usable VRAM for a GPU,
// accounting for OS reservations, display output, and driver overhead.
func calculateAvailableVRAM(gpu models.GPUInfo) uint64 {
	switch gpu.Vendor {
	case "apple":
		// Apple Silicon unified memory: ~25% reserved for OS/display
		// See: https://developer.apple.com/documentation/metal/optimizing_memory_usage
		return uint64(float64(gpu.VRAM) * 0.75)
	case "nvidia":
		// NVIDIA GPUs: ~5-10% reserved for driver/display
		if isHeadless() {
			// Headless/server: minimal reservation
			return uint64(float64(gpu.VRAM) * 0.95)
		}
		// Desktop with display: higher reservation
		return uint64(float64(gpu.VRAM) * 0.90)
	case "amd":
		// AMD GPUs: ~10% reserved for driver/display
		if isHeadless() {
			return uint64(float64(gpu.VRAM) * 0.92)
		}
		return uint64(float64(gpu.VRAM) * 0.88)
	default:
		// Conservative default: 85% available
		return uint64(float64(gpu.VRAM) * 0.85)
	}
}

// isHeadless checks if the system is running without a display server.
// Used to determine VRAM reservation requirements.
func isHeadless() bool {
	// Check common headless indicators
	if runtime.GOOS == "linux" {
		// No DISPLAY variable = likely headless
		if display := getenv("DISPLAY"); display == "" {
			return true
		}
	}
	return false
}

// normalizeGPUVendor normalizes GPU vendor strings to canonical form.
func normalizeGPUVendor(vendor string) string {
	vendor = strings.ToLower(strings.TrimSpace(vendor))

	// Normalize common vendor names
	if strings.Contains(vendor, "nvidia") || strings.Contains(vendor, "geforce") || strings.Contains(vendor, "quadro") || strings.Contains(vendor, "tesla") {
		return "nvidia"
	}
	if strings.Contains(vendor, "amd") || strings.Contains(vendor, "radeon") || strings.Contains(vendor, "ati") {
		return "amd"
	}
	if strings.Contains(vendor, "apple") || strings.Contains(vendor, "metal") {
		return "apple"
	}
	if strings.Contains(vendor, "intel") || strings.Contains(vendor, "uhd") || strings.Contains(vendor, "iris") {
		return "intel"
	}

	// Return as-is if already normalized
	switch vendor {
	case "nvidia", "amd", "apple", "intel":
		return vendor
	}

	return "unknown"
}

// getenv is a helper to get environment variables safely
func getenv(key string) string {
	return os.Getenv(key)
}
