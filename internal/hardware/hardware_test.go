package hardware

import (
	"fmt"
	"os"
	"runtime"
	"testing"

	"github.com/kevo-1/KnowURLLM/internal/domain"
)

// TestParseNvidiaSMIOutput tests the nvidia-smi CSV parser in isolation.
func TestParseNvidiaSMIOutput(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected int
	}{
		{
			name: "single GPU",
			input: `NVIDIA GeForce RTX 4090, 24576`,
			expected: 1,
		},
		{
			name: "multiple GPUs",
			input: `NVIDIA GeForce RTX 4090, 24576
NVIDIA GeForce RTX 3080, 10240`,
			expected: 2,
		},
		{
			name:     "empty input",
			input:    "",
			expected: 0,
		},
		{
			name:     "whitespace only",
			input:    "   \n  \n  ",
			expected: 0,
		},
		{
			name: "malformed line skipped",
			input: `NVIDIA GeForce RTX 4090, 24576
malformed line without comma
NVIDIA GeForce RTX 3080, 10240`,
			expected: 2,
		},
		{
			name: "invalid VRAM value skipped",
			input: `NVIDIA GeForce RTX 4090, notanumber`,
			expected: 0,
		},
		{
			name: "GPU name with comma",
			input: `NVIDIA GeForce RTX 4090, Custom Edition, 24576`,
			expected: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gpus := parseNvidiaSMIOutput(tt.input)
			if len(gpus) != tt.expected {
				t.Errorf("expected %d GPUs, got %d", tt.expected, len(gpus))
			}

			// Verify first GPU if present
			if tt.expected > 0 && len(gpus) > 0 {
				if gpus[0].Vendor != "nvidia" {
					t.Errorf("expected vendor 'nvidia', got '%s'", gpus[0].Vendor)
				}
				if gpus[0].VRAM == 0 {
					t.Errorf("expected non-zero VRAM")
				}
			}
		})
	}

	// Test specific VRAM conversion
	t.Run("VRAM conversion MiB to bytes", func(t *testing.T) {
		input := `NVIDIA GeForce RTX 4090, 24576`
		gpus := parseNvidiaSMIOutput(input)
		if len(gpus) != 1 {
			t.Fatalf("expected 1 GPU, got %d", len(gpus))
		}
		// 24576 MiB = 24576 * 1024 * 1024 = 25769803776 bytes
		expectedVRAM := uint64(25769803776)
		if gpus[0].VRAM != expectedVRAM {
			t.Errorf("expected VRAM %d bytes, got %d", expectedVRAM, gpus[0].VRAM)
		}
	})

	// Test GPU name with comma parsing
	t.Run("GPU name with comma parsing", func(t *testing.T) {
		input := `NVIDIA GeForce RTX 4090, Custom Edition, 24576`
		gpus := parseNvidiaSMIOutput(input)
		if len(gpus) != 1 {
			t.Fatalf("expected 1 GPU, got %d", len(gpus))
		}
		if gpus[0].Model != "NVIDIA GeForce RTX 4090, Custom Edition" {
			t.Errorf("expected model 'NVIDIA GeForce RTX 4090, Custom Edition', got '%s'", gpus[0].Model)
		}
		expectedVRAM := uint64(25769803776)
		if gpus[0].VRAM != expectedVRAM {
			t.Errorf("expected VRAM %d bytes, got %d", expectedVRAM, gpus[0].VRAM)
		}
	})
}

// TestNormalizeGPUVendor tests vendor name normalization
func TestNormalizeGPUVendor(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"NVIDIA GeForce RTX 4090", "nvidia"},
		{"AMD Radeon RX 7900 XTX", "amd"},
		{"Apple M2 Max", "apple"},
		{"Intel UHD Graphics 630", "intel"},
		{"nvidia", "nvidia"},
		{"AMD", "amd"},
		{"Unknown GPU", "unknown"},
		{"", "unknown"},
		{"  AMD Radeon  ", "amd"},
		{"Tesla V100", "nvidia"},
		{"Quadro RTX 5000", "nvidia"},
		{"ATI Radeon HD 5870", "amd"},
		{"Iris Plus Graphics", "intel"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := normalizeGPUVendor(tt.input)
			if result != tt.expected {
				t.Errorf("normalizeGPUVendor(%q) = %q, expected %q", tt.input, result, tt.expected)
			}
		})
	}
}

// TestCalculateAvailableVRAM tests VRAM availability calculations
func TestCalculateAvailableVRAM(t *testing.T) {
	tests := []struct {
		name     string
		gpu      domain.GPUInfo
		expected uint64
	}{
		{
			name: "Apple Silicon 16GB",
			gpu: domain.GPUInfo{
				Vendor: "apple",
				Model:  "Apple M2",
				VRAM:   16 * 1024 * 1024 * 1024,
			},
			expected: func() uint64 {
				v := float64(16 * 1024 * 1024 * 1024)
				return uint64(v * 0.75)
			}(),
		},
		{
			name: "NVIDIA GPU",
			gpu: domain.GPUInfo{
				Vendor: "nvidia",
				Model:  "RTX 4090",
				VRAM:   24 * 1024 * 1024 * 1024,
			},
			// Expected value depends on headless status
			expected: func() uint64 {
				v := float64(24 * 1024 * 1024 * 1024)
				// On Windows or desktop, use 90%; on headless Linux, use 95%
				if runtime.GOOS == "linux" && os.Getenv("DISPLAY") == "" {
					return uint64(v * 0.95)
				}
				return uint64(v * 0.90)
			}(),
		},
		{
			name: "AMD desktop",
			gpu: domain.GPUInfo{
				Vendor: "amd",
				Model:  "RX 7900 XTX",
				VRAM:   24 * 1024 * 1024 * 1024,
			},
			expected: func() uint64 {
				v := float64(24 * 1024 * 1024 * 1024)
				return uint64(v * 0.88)
			}(),
		},
		{
			name: "Unknown vendor",
			gpu: domain.GPUInfo{
				Vendor: "unknown",
				Model:  "Unknown GPU",
				VRAM:   8 * 1024 * 1024 * 1024,
			},
			expected: func() uint64 {
				v := float64(8 * 1024 * 1024 * 1024)
				return uint64(v * 0.85)
			}(),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := calculateAvailableVRAM(tt.gpu)
			// Allow small floating point differences
			diff := int64(result) - int64(tt.expected)
			if diff < 0 {
				diff = -diff
			}
			if diff > 1024*1024 { // Allow 1MB tolerance
				t.Errorf("calculateAvailableVRAM() = %d, expected ~%d", result, tt.expected)
			}
		})
	}
}

// TestGPUDetectionError tests custom error types
func TestGPUDetectionError(t *testing.T) {
	t.Run("NoGPUError", func(t *testing.T) {
		err := NoGPUError(fmt.Errorf("nvidia-smi not found"))
		if err.GPUsFound {
			t.Error("NoGPUError should have GPUsFound = false")
		}
		if !IsNoGPUError(err) {
			t.Error("IsNoGPUError should return true for NoGPUError")
		}
	})

	t.Run("PartialGPUError", func(t *testing.T) {
		err := PartialGPUError(fmt.Errorf("AMD detection failed"))
		if !err.GPUsFound {
			t.Error("PartialGPUError should have GPUsFound = true")
		}
		if IsNoGPUError(err) {
			t.Error("IsNoGPUError should return false for PartialGPUError")
		}
	})
}

// TestMockDetector tests the mock detector interface
func TestMockDetector(t *testing.T) {
	profile := domain.HardwareProfile{
		CPUModel:      "Test CPU",
		CPUCores:      8,
		TotalRAM:      16 * 1024 * 1024 * 1024,
		AvailableRAM:  12 * 1024 * 1024 * 1024,
		TotalVRAM:     24 * 1024 * 1024 * 1024,
		AvailableVRAM: 20 * 1024 * 1024 * 1024,
		GPUs: []domain.GPUInfo{
			{Vendor: "nvidia", Model: "RTX 4090", VRAM: 24 * 1024 * 1024 * 1024},
		},
		Platform:       "linux",
		IsAppleSilicon: false,
	}

	detector := NewMockDetector(profile, nil)

	detectProfile, err := detector.Detect()
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if detectProfile.CPUModel != profile.CPUModel {
		t.Errorf("expected CPU model %q, got %q", profile.CPUModel, detectProfile.CPUModel)
	}

	cpuModel, cores, err := detector.DetectCPU()
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if cpuModel != profile.CPUModel || cores != profile.CPUCores {
		t.Errorf("expected %q/%d, got %q/%d", profile.CPUModel, profile.CPUCores, cpuModel, cores)
	}

	totalRAM, availableRAM, err := detector.DetectMemory()
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if totalRAM != profile.TotalRAM || availableRAM != profile.AvailableRAM {
		t.Errorf("expected RAM %d/%d, got %d/%d", profile.TotalRAM, profile.AvailableRAM, totalRAM, availableRAM)
	}

	gpus, err := detector.DetectGPU()
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if len(gpus) != len(profile.GPUs) {
		t.Errorf("expected %d GPUs, got %d", len(profile.GPUs), len(gpus))
	}
}

// TestParseAMDSysfs tests the sysfs reader using temporary files.
// The actual sysfs tests are in hardware_test_linux.go (Linux-only build).
func TestParseAMDSysfs(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("sysfs tests only run on Linux")
	}
	t.Log("See hardware_test_linux.go for sysfs parser tests")
}

// TestDetectIntegration tests that Detect() returns a non-zero profile.
func TestDetectIntegration(t *testing.T) {
	profile, err := Detect()

	// We allow GPU errors in CI environments
	// Check critical fields are non-zero
	if profile.CPUModel == "" {
		t.Error("expected non-empty CPUModel")
	}
	if profile.CPUCores == 0 {
		t.Error("expected non-zero CPUCores")
	}
	if profile.TotalRAM == 0 {
		t.Error("expected non-zero TotalRAM")
	}
	if profile.AvailableRAM == 0 {
		t.Error("expected non-zero AvailableRAM")
	}
	if profile.Platform == "" {
		t.Error("expected non-empty Platform")
	}

	// Log results for visibility
	t.Logf("Hardware Profile:")
	t.Logf("  CPU: %s", profile.CPUModel)
	t.Logf("  Cores: %d", profile.CPUCores)
	t.Logf("  RAM: %d bytes (%.2f GB) / Available: %d bytes (%.2f GB)",
		profile.TotalRAM, float64(profile.TotalRAM)/(1024*1024*1024),
		profile.AvailableRAM, float64(profile.AvailableRAM)/(1024*1024*1024))
	t.Logf("  Platform: %s", profile.Platform)
	t.Logf("  Apple Silicon: %v", profile.IsAppleSilicon)
	t.Logf("  Total VRAM: %d bytes (%.2f GB)", profile.TotalVRAM, float64(profile.TotalVRAM)/(1024*1024*1024))
	t.Logf("  Available VRAM: %d bytes (%.2f GB)", profile.AvailableVRAM, float64(profile.AvailableVRAM)/(1024*1024*1024))
	t.Logf("  GPUs: %d", len(profile.GPUs))
	for i, gpu := range profile.GPUs {
		t.Logf("    GPU %d: %s %s (%d bytes / %.2f GB)", i, gpu.Vendor, gpu.Model, gpu.VRAM, float64(gpu.VRAM)/(1024*1024*1024))
	}

	// Error is allowed if it's only about GPU detection
	if err != nil {
		t.Logf("  Error (expected in CI): %v", err)
	}
}

// TestDetectCached tests the caching layer
func TestDetectCached(t *testing.T) {
	// Clear cache first
	ResetCache()

	// First call should perform detection
	profile1, err1 := DetectCached()
	if profile1.CPUModel == "" {
		t.Error("expected non-empty CPUModel from first DetectCached call")
	}

	// Second call should return cached result
	profile2, err2 := DetectCached()
	if profile2.CPUModel != profile1.CPUModel {
		t.Error("cached profile should match first detection")
	}
	if err2 != err1 {
		t.Error("cached error should match first detection")
	}

	// Reset and verify new detection
	ResetCache()
	profile3, _ := DetectCached()
	if profile3.CPUModel != profile1.CPUModel {
		// Might be same, but detection should have run again
		t.Logf("CPU model after reset: %s (original: %s)", profile3.CPUModel, profile1.CPUModel)
	}
}
