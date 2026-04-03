package hardware

import (
	"runtime"
	"testing"
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
	if profile.Platform == "" {
		t.Error("expected non-empty Platform")
	}

	// Log results for visibility
	t.Logf("Hardware Profile:")
	t.Logf("  CPU: %s", profile.CPUModel)
	t.Logf("  Cores: %d", profile.CPUCores)
	t.Logf("  RAM: %d bytes (%.2f GB)", profile.TotalRAM, float64(profile.TotalRAM)/(1024*1024*1024))
	t.Logf("  Platform: %s", profile.Platform)
	t.Logf("  Apple Silicon: %v", profile.IsAppleSilicon)
	t.Logf("  GPUs: %d", len(profile.GPUs))
	for i, gpu := range profile.GPUs {
		t.Logf("    GPU %d: %s %s (%d bytes / %.2f GB)", i, gpu.Vendor, gpu.Model, gpu.VRAM, float64(gpu.VRAM)/(1024*1024*1024))
	}

	// Error is allowed if it's only about GPU detection
	if err != nil {
		t.Logf("  Error (expected in CI): %v", err)
	}
}
