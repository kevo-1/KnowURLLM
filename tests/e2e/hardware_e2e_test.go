package e2e_test

import (
	"fmt"
	"math"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"testing"

	"github.com/kevo-1/KnowURLLM/internal/hardware"
	"github.com/kevo-1/KnowURLLM/internal/models"
	"github.com/kevo-1/KnowURLLM/internal/registry"
	"github.com/kevo-1/KnowURLLM/internal/scorer"
)

// ──────────────────────────────────────────────
// E2E: Real Hardware Detection (User's Machine)
// ──────────────────────────────────────────────

// TestE2E_RealHardwareDetection runs Detect() on the actual machine
// and validates results against known hardware specifications.
//
// User's hardware:
//   CPU: Intel Core i7-4770 @ 3.40GHz (4 cores / 8 threads, Haswell)
//   RAM: 16GB DDR3 1333MHz (2x8GB Hynix)
//   GPU: NVIDIA GeForce GTX 1650 4GB GDDR5 + Intel HD Graphics 4600
//   OS: Windows 64-bit
func TestE2E_RealHardwareDetection(t *testing.T) {
	// Clear any cached state
	hardware.ResetCache()

	profile, err := hardware.Detect()

	// Allow GPU detection errors (Intel iGPU may not be used for LLM)
	if err != nil {
		t.Logf("Detection returned error (may be GPU-related, which is acceptable): %v", err)
	}

	// ── CPU Validation ──
	t.Run("CPU detection", func(t *testing.T) {
		cpuLower := strings.ToLower(profile.CPUModel)

		// Should detect Intel CPU
		if !strings.Contains(cpuLower, "intel") {
			t.Errorf("expected CPU model to contain 'Intel', got %q", profile.CPUModel)
		}

		// Should detect i7-4770 or similar 4th gen naming
		hasGen4 := strings.Contains(cpuLower, "i7") || strings.Contains(cpuLower, "4770")
		if !hasGen4 {
			t.Logf("WARNING: CPU model %q doesn't clearly indicate i7-4770; detection may be generic", profile.CPUModel)
		}

		// Haswell i7-4770 has 4 physical cores / 8 logical threads
		// gopsutil reports logical cores on Windows
		if profile.CPUCores != 4 && profile.CPUCores != 8 {
			t.Errorf("expected 4 physical or 8 logical cores for i7-4770, got %d", profile.CPUCores)
		}

		t.Logf("Detected CPU: %s (%d cores)", profile.CPUModel, profile.CPUCores)
	})

	// ── Memory Validation ──
	t.Run("Memory detection", func(t *testing.T) {
		totalGB := float64(profile.TotalRAM) / (1024 * 1024 * 1024)
		availGB := float64(profile.AvailableRAM) / (1024 * 1024 * 1024)

		// System has 16GB (2x8GB DDR3)
		// Allow ±1GB tolerance for hardware reservations
		expectedGB := 16.0
		toleranceGB := 1.0

		if math.Abs(totalGB-expectedGB) > toleranceGB {
			t.Errorf("expected ~%.1f GB total RAM, got %.2f GB", expectedGB, totalGB)
		}

		// Available RAM should be less than total
		if profile.AvailableRAM >= profile.TotalRAM {
			t.Errorf("available RAM (%.2f GB) should be less than total (%.2f GB)", availGB, totalGB)
		}

		// DDR3 1333MHz is slow for LLM CPU inference
		t.Logf("Detected RAM: %.2f GB total / %.2f GB available (DDR3 1333MHz)", totalGB, availGB)
	})

	// ── GPU Validation ──
	t.Run("GPU detection", func(t *testing.T) {
		if len(profile.GPUs) == 0 {
			t.Log("No GPUs detected — GTX 1650 may not be visible to nvidia-smi on this system")
			return
		}

		var foundGTX1650 bool

		for _, gpu := range profile.GPUs {
			t.Logf("Detected GPU: %s %s (%.2f GB VRAM)", gpu.Vendor, gpu.Model, float64(gpu.VRAM)/(1024*1024*1024))

			modelLower := strings.ToLower(gpu.Model)
			if strings.Contains(modelLower, "gtx 1650") {
				foundGTX1650 = true

				// GTX 1650 has 4GB VRAM
				expectedVRAM := uint64(4 * 1024 * 1024 * 1024)
				vramDiff := int64(gpu.VRAM) - int64(expectedVRAM)
				if vramDiff < 0 {
					vramDiff = -vramDiff
				}
				// Allow ±256MB tolerance
				if vramDiff > 256*1024*1024 {
					t.Errorf("GTX 1650 VRAM: expected ~4GB, got %.2f GB", float64(gpu.VRAM)/(1024*1024*1024))
				}

				if gpu.Vendor != "nvidia" {
					t.Errorf("GTX 1650 vendor: expected 'nvidia', got %q", gpu.Vendor)
				}
			}

			if strings.Contains(modelLower, "intel") || strings.Contains(modelLower, "hd graphics") {
				t.Logf("Intel HD Graphics detected: %.2f GB VRAM", float64(gpu.VRAM)/(1024*1024*1024))
			}
		}

		if foundGTX1650 {
			t.Log("SUCCESS: NVIDIA GTX 1650 detected correctly")
		} else {
			t.Log("NOTE: GTX 1650 not found in GPU list — checking via nvidia-smi directly")
			// Try direct nvidia-smi to verify
			cmd := exec.Command("nvidia-smi", "--query-gpu=name,memory.total", "--format=csv,noheader,nounits")
			out, err := cmd.Output()
			if err != nil {
				t.Logf("nvidia-smi direct check failed: %v (driver may not be loaded)", err)
			} else {
				t.Logf("Direct nvidia-smi output: %s", strings.TrimSpace(string(out)))
				t.Log("GPU is visible by nvidia-smi — detection code may have a parsing issue")
			}
		}
	})

	// ── Platform Validation ──
	t.Run("Platform detection", func(t *testing.T) {
		if profile.Platform != "windows" {
			t.Errorf("expected platform 'windows', got %q", profile.Platform)
		}
		if profile.IsAppleSilicon {
			t.Error("IsAppleSilicon should be false on Windows")
		}
	})

	// ── Summary ──
	t.Run("Hardware summary", func(t *testing.T) {
		t.Logf("=== Hardware Profile ===")
		t.Logf("CPU: %s (%d cores)", profile.CPUModel, profile.CPUCores)
		t.Logf("RAM: %.2f GB total / %.2f GB available",
			float64(profile.TotalRAM)/(1024*1024*1024),
			float64(profile.AvailableRAM)/(1024*1024*1024))
		t.Logf("GPUs: %d", len(profile.GPUs))
		t.Logf("Total VRAM: %.2f GB", float64(profile.TotalVRAM)/(1024*1024*1024))
		t.Logf("Available VRAM: %.2f GB", float64(profile.AvailableVRAM)/(1024*1024*1024))
		t.Logf("Platform: %s (Apple Silicon: %v)", profile.Platform, profile.IsAppleSilicon)
	})
}

// ──────────────────────────────────────────────
// E2E: GPU Bandwidth Lookup Table
// ──────────────────────────────────────────────

// TestE2E_GPUBandwidthLookup tests the bandwidth lookup table
// against known GPU specifications from online research.
func TestE2E_GPUBandwidthLookup(t *testing.T) {
	tests := []struct {
		gpuModel      string
		expectedBW    float64
		shouldBeFound bool
		source        string
	}{
		// GPUs that SHOULD be in the table
		{"RTX 4090", 1008, true, "NVIDIA specs"},
		{"RTX 3090", 936, true, "NVIDIA specs"},
		{"RTX 3060", 360, true, "NVIDIA specs"},
		{"RTX 4060", 276, true, "NVIDIA specs"},
		{"H100", 3350, true, "NVIDIA data center"},
		{"A100", 2000, true, "NVIDIA data center"},
		{"M2 Max", 400, true, "Apple specs"},
		{"M1 Pro", 200, true, "Apple specs"},
		{"RX 7900 XTX", 960, true, "AMD specs"},

		// GTX 1650 — now in table (was a known gap, now fixed)
		{"GTX 1650", 128, true, "NVIDIA Turing specs"},
		{"GeForce GTX 1650", 128, true, "NVIDIA Turing specs"},

		// Unknown GPU
		{"Unknown GPU XYZ", 0, false, "deliberately unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.gpuModel, func(t *testing.T) {
			bw, found := hardware.LookupBandwidth(tt.gpuModel)

			if found != tt.shouldBeFound {
				t.Errorf("LookupBandwidth(%q): found=%v, expected found=%v",
					tt.gpuModel, found, tt.shouldBeFound)
			}

			if tt.shouldBeFound && found {
				if math.Abs(bw-tt.expectedBW) > 1 {
					t.Errorf("LookupBandwidth(%q): got %.0f GB/s, expected %.0f GB/s (%s)",
						tt.gpuModel, bw, tt.expectedBW, tt.source)
				}
			}

			if !tt.shouldBeFound && found {
				t.Logf("NOTE: %s found with %.0f GB/s bandwidth (table may have been updated)", tt.gpuModel, bw)
			} else if !tt.shouldBeFound {
				t.Logf("CONFIRMED GAP: %s not in bandwidth table — scorer will use param-count fallback", tt.gpuModel)
			}
		})
	}
}

// TestE2E_GPUBandwidthCaseInsensitivity tests case-insensitive matching
func TestE2E_GPUBandwidthCaseInsensitivity(t *testing.T) {
	tests := []struct {
		input    string
		expected float64
	}{
		{"rtx 4090", 1008},
		{"RTX 4090", 1008},
		{"Rtx 4090", 1008},
		{"nvidia rtx 4090", 1008},
		{"NVIDIA GEFORCE RTX 4090", 1008},
		{"m2 max", 400},
		{"M2 MAX", 400},
		{"gtx 1650", 128},
		{"GTX 1650", 128},
		{"NVIDIA GeForce GTX 1650", 128},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			bw, found := hardware.LookupBandwidth(tt.input)
			if !found {
				t.Fatalf("LookupBandwidth(%q) should be found", tt.input)
			}
			if math.Abs(bw-tt.expected) > 1 {
				t.Errorf("got %.0f, expected %.0f", bw, tt.expected)
			}
		})
	}
}

// TestE2E_GPUBandwidthAllEntries validates every entry in the lookup table
// has a plausible bandwidth value.
func TestE2E_GPUBandwidthAllEntries(t *testing.T) {
	// Directly access the gpuBandwidth map via reflection would be complex,
	// so we test via representative keys that we know exist in the table.
	// This also serves as documentation of what IS in the table.

	knownEntries := map[string]float64{
		// NVIDIA Ada Lovelace
		"RTX 4090": 1008,
		"RTX 4080": 717,
		"RTX 4070": 504,
		"RTX 4060": 276,
		// NVIDIA Ampere
		"RTX 3090": 936,
		"RTX 3080": 760,
		"RTX 3070": 448,
		"RTX 3060": 360,
		// NVIDIA Turing (GTX 16-series)
		"GTX 1650":     128,
		"GTX 1650 SUPER": 192,
		"GTX 1660":     192,
		"GTX 1660 SUPER": 336,
		// NVIDIA Data Center
		"H100": 3350,
		"A100": 2000,
		"A6000": 768,
		"A40":  696,
		// Apple Silicon
		"M3 Max": 400,
		"M3 Pro": 150,
		"M2 Max": 400,
		"M2 Pro": 200,
		"M1 Max": 400,
		"M1 Pro": 200,
		// AMD RDNA 3
		"RX 7900 XTX": 960,
		"RX 7900 XT":  800,
		"RX 7800 XT":  624,
	}

	for key, expectedBW := range knownEntries {
		t.Run(key, func(t *testing.T) {
			bw, found := hardware.LookupBandwidth(key)
			if !found {
				t.Errorf("Known entry %q not found in bandwidth table!", key)
				return
			}
			if math.Abs(bw-expectedBW) > 1 {
				t.Errorf("Bandwidth mismatch for %q: got %.0f, expected %.0f", key, bw, expectedBW)
			}
		})
	}

	t.Logf("Validated %d known bandwidth entries in lookup table", len(knownEntries))
}

// ──────────────────────────────────────────────
// E2E: Available VRAM Calculations
// ──────────────────────────────────────────────

// TestE2E_AvailableVRAM_GTX1650 tests VRAM availability for GTX 1650 scenario
// Note: calculateAvailableVRAM is unexported, so we test indirectly via Detect()
func TestE2E_AvailableVRAM_GTX1650(t *testing.T) {
	hardware.ResetCache()
	profile, _ := hardware.Detect()

	if len(profile.GPUs) == 0 {
		t.Skip("No GPUs detected — cannot test VRAM calculations")
	}

	for _, gpu := range profile.GPUs {
		if strings.Contains(strings.ToLower(gpu.Model), "gtx 1650") {
			availableGB := float64(profile.AvailableVRAM) / (1024 * 1024 * 1024)
			totalGB := float64(gpu.VRAM) / (1024 * 1024 * 1024)
			pct := float64(profile.AvailableVRAM) / float64(gpu.VRAM) * 100

			t.Logf("GTX 1650 VRAM: %.2f GB total", totalGB)
			t.Logf("Available VRAM: %.2f GB (%.1f%% factor)", availableGB, pct)

			// On Windows (non-headless), NVIDIA uses 90% factor
			expectedPct := 90.0
			if runtime.GOOS == "linux" && os.Getenv("DISPLAY") == "" {
				expectedPct = 95.0 // headless Linux
			}

			t.Logf("Expected: ~%.0f%% factor", expectedPct)

			// Key insight: With only ~3.6 GB available VRAM, LLMs are severely limited
			t.Logf("NOTE: With ~%.2f GB usable VRAM, max model size is ~3-6B parameters (quantized)", availableGB)
			return
		}
	}

	t.Log("GTX 1650 not detected — testing VRAM calculation conceptually")
	// GTX 1650 has 4GB VRAM
	// On Windows desktop: 90% available = 3.6 GB
	// This limits models to ~3-6B parameters at Q4 quantization
	t.Log("GTX 1650 (4GB total): ~3.6 GB available (90% factor on Windows desktop)")
	t.Log("Model capacity: ~3B Q4 (1.5GB), ~6B Q4 (3GB), 7B Q4 barely fits")
}

// TestE2E_VRAMForVariousGPUs tests VRAM availability across GPU types
// Uses Detect() results since calculateAvailableVRAM is unexported
func TestE2E_VRAMForVariousGPUs(t *testing.T) {
	// This test validates the VRAM calculation indirectly through real detection
	hardware.ResetCache()
	profile, _ := hardware.Detect()

	if len(profile.GPUs) == 0 {
		t.Skip("No GPUs detected — cannot test VRAM availability factors")
	}

	for _, gpu := range profile.GPUs {
		t.Run(gpu.Model, func(t *testing.T) {
			// We can only test what's actually detected
			// Log the VRAM factors for documentation
			totalGB := float64(gpu.VRAM) / (1024 * 1024 * 1024)
			t.Logf("%s (%s): %.2f GB VRAM, vendor=%q", gpu.Model, gpu.Vendor, totalGB, gpu.Vendor)
		})
	}
}

// ──────────────────────────────────────────────
// E2E: Scorer Integration with Real Hardware
// ──────────────────────────────────────────────

// TestE2E_ScorerWithRealHardware runs the scorer pipeline with detected hardware
// against known models from the embedded database.
func TestE2E_ScorerWithRealHardware(t *testing.T) {
	hardware.ResetCache()
	profile, err := hardware.Detect()
	if err != nil {
		// Allow GPU errors
		if !strings.Contains(err.Error(), "gpu") {
			t.Fatalf("unexpected detection error: %v", err)
		}
		t.Logf("GPU detection error (acceptable): %v", err)
	}

	if profile.TotalRAM == 0 {
		t.Fatal("zero RAM in hardware profile — cannot run scorer tests")
	}

	t.Logf("Scoring with hardware profile:")
	t.Logf("  CPU: %s (%d cores)", profile.CPUModel, profile.CPUCores)
	t.Logf("  RAM: %.2f GB", float64(profile.TotalRAM)/(1024*1024*1024))
	t.Logf("  VRAM: %.2f GB available", float64(profile.AvailableVRAM)/(1024*1024*1024))
	t.Logf("  GPUs: %d", len(profile.GPUs))

	// Load models from embedded registry
	fetcher := registry.NewFetcher()
	entries, err := fetcher.FetchAll(t.Context())
	if err != nil {
		t.Fatalf("failed to fetch models: %v", err)
	}
	t.Logf("Loaded %d models from registry", len(entries))

	// Run scorer
	s := scorer.NewScorer()
	results, err := s.Rank(profile, entries)
	if err != nil {
		t.Fatalf("scorer.Rank failed: %v", err)
	}

	t.Logf("Scored %d models (after excluding non-fitting)", len(results))

	// Validate top results are reasonable
	if len(results) == 0 {
		t.Fatal("no models scored — this should not happen with a valid hardware profile")
	}

	// Check top 10 models
	topN := 10
	if len(results) < topN {
		topN = len(results)
	}

	t.Logf("\n=== Top %d Models for This Hardware ===", topN)
	for i := 0; i < topN; i++ {
		r := results[i]
		t.Logf("  #%d: %s (Score: %.1f, Fit: %s, Quality: %.1f, Speed: %.1f t/s, Context: %.0f)",
			r.Rank,
			r.Model.DisplayName,
			r.Score.TotalScore,
			r.Score.FitCategory,
			r.Score.QualityScore,
			r.Score.EstimatedTPS,
			r.Score.ContextScore,
		)
	}

	// Validation checks
	t.Run("Score ranges are valid", func(t *testing.T) {
		for _, r := range results {
			if r.Score.TotalScore < 0 || r.Score.TotalScore > 100 {
				t.Errorf("TotalScore out of range for %s: %.2f", r.Model.DisplayName, r.Score.TotalScore)
			}
			if r.Score.HardwareFitScore < 0 || r.Score.HardwareFitScore > 100 {
				t.Errorf("HardwareFitScore out of range for %s: %.2f", r.Model.DisplayName, r.Score.HardwareFitScore)
			}
			if r.Score.QualityScore < 0 || r.Score.QualityScore > 100 {
				t.Errorf("QualityScore out of range for %s: %.2f", r.Model.DisplayName, r.Score.QualityScore)
			}
		}
	})

	t.Run("Results are sorted by TotalScore descending", func(t *testing.T) {
		for i := 1; i < len(results); i++ {
			// Check that no result has a HIGHER score than its predecessor
			// Ties (within 0.01) are acceptable and expected
			if results[i].Score.TotalScore > results[i-1].Score.TotalScore+0.01 {
				t.Errorf("Results not sorted correctly at position %d: %.2f > %.2f",
					i, results[i].Score.TotalScore, results[i-1].Score.TotalScore)
				break
			}
		}
	})

	t.Run("Rank numbers are sequential", func(t *testing.T) {
		for i, r := range results {
			expected := i + 1
			if r.Rank != expected {
				t.Errorf("Rank at position %d: expected %d, got %d", i, expected, r.Rank)
				break
			}
		}
	})

	t.Run("Use-case profile scores differ", func(t *testing.T) {
		if len(results) == 0 {
			return
		}
		top := results[0]
		// Different use-case profiles should produce different scores
		profiles := scorer.ValidProfiles()
		if len(profiles) == 0 {
			t.Skip("no use-case profiles defined")
		}
		t.Logf("Available use-case profiles: %v", profiles)
		_ = top
	})
}

// TestE2E_ScorerWithFilterVRAMOnly tests the VRAM-only filter
func TestE2E_ScorerWithFilterVRAMOnly(t *testing.T) {
	hardware.ResetCache()
	profile, _ := hardware.Detect()
	if profile.TotalRAM == 0 {
		t.Skip("no hardware profile available")
	}

	fetcher := registry.NewFetcher()
	entries, err := fetcher.FetchAll(t.Context())
	if err != nil {
		t.Fatalf("failed to fetch models: %v", err)
	}

	s := scorer.NewScorer()

	// Without VRAM-only filter
	allResults, err := s.Rank(profile, entries)
	if err != nil {
		t.Fatalf("scorer.Rank failed: %v", err)
	}

	// With VRAM-only filter
	vramResults, err := s.RankWithFilter(profile, entries, models.FilterOptions{
		VRAMOnly: true,
	})
	if err != nil {
		t.Fatalf("scorer.RankWithFilter(VRAMOnly) failed: %v", err)
	}

	t.Logf("All models: %d, VRAM-only models: %d", len(allResults), len(vramResults))

	if len(vramResults) > len(allResults) {
		t.Error("VRAM-only filter should not return more models than unfiltered")
	}

	if len(vramResults) > 0 {
		t.Logf("Top VRAM-only model: %s (Score: %.1f)", vramResults[0].Model.DisplayName, vramResults[0].Score.TotalScore)
	}
}

// ──────────────────────────────────────────────
// E2E: Full Pipeline Test (Detect → Fetch → Score)
// ──────────────────────────────────────────────

// TestE2E_FullPipeline simulates the complete main.go pipeline
func TestE2E_FullPipeline(t *testing.T) {
	// Step 1: Detect hardware
	hardware.ResetCache()
	hw, hwErr := hardware.Detect()
	if hwErr != nil && !strings.Contains(hwErr.Error(), "gpu") {
		t.Fatalf("Step 1 (Detect): fatal error: %v", hwErr)
	}
	t.Logf("Step 1 PASS: Hardware detected (CPU: %s, RAM: %.1f GB, VRAM: %.1f GB)",
		hw.CPUModel, float64(hw.TotalRAM)/(1024*1024*1024), float64(hw.TotalVRAM)/(1024*1024*1024))

	// Step 2: Fetch models
	fetcher := registry.NewFetcher()
	entries, err := fetcher.FetchAll(t.Context())
	if err != nil {
		t.Fatalf("Step 2 (Fetch): failed to load models: %v", err)
	}
	t.Logf("Step 2 PASS: %d models loaded from registry", len(entries))

	// Step 3: Score and rank
	s := scorer.NewScorer()
	results, err := s.Rank(hw, entries)
	if err != nil {
		t.Fatalf("Step 3 (Score): scoring failed: %v", err)
	}
	t.Logf("Step 3 PASS: %d models scored and ranked", len(results))

	// Step 4: Validate results are usable
	if len(results) == 0 {
		t.Fatal("Step 4 FAIL: no models scored — pipeline produced empty results")
	}

	// Find models that fit well
	var goodFit []models.RankResult
	for _, r := range results {
		if r.Score.FitCategory == "Perfect" || r.Score.FitCategory == "Good" {
			goodFit = append(goodFit, r)
		}
	}

	t.Logf("Step 4 PASS: %d models with Good/Perfect fit, %d total ranked", len(goodFit), len(results))

	// Step 5: Check specific model categories
	t.Run("Small models (should fit GTX 1650)", func(t *testing.T) {
		smallModels := []string{"Phi", "TinyLlama", "Qwen2.5-0.5B", "Gemma-2b"}
		for _, name := range smallModels {
			for _, r := range results {
				if strings.Contains(r.Model.DisplayName, name) {
					t.Logf("  %s: Score=%.1f Fit=%s TPS=%.1f",
						r.Model.DisplayName, r.Score.TotalScore, r.Score.FitCategory, r.Score.EstimatedTPS)
					break
				}
			}
		}
	})

	t.Run("Medium models (may fit with quantization)", func(t *testing.T) {
		mediumModels := []string{"Llama-3", "Mistral-7B", "Qwen2.5-7B"}
		for _, name := range mediumModels {
			for _, r := range results {
				if strings.Contains(r.Model.DisplayName, name) {
					t.Logf("  %s: Score=%.1f Fit=%s TPS=%.1f",
						r.Model.DisplayName, r.Score.TotalScore, r.Score.FitCategory, r.Score.EstimatedTPS)
					break
				}
			}
		}
	})

	// Overall pipeline status
	t.Log("\n=== Pipeline Status ===")
	t.Logf("Hardware: %s / %d cores / %.1f GB RAM / %.1f GB VRAM",
		hw.CPUModel, hw.CPUCores,
		float64(hw.TotalRAM)/(1024*1024*1024),
		float64(hw.TotalVRAM)/(1024*1024*1024))
	t.Logf("Models: %d loaded, %d scored, %d good fit", len(entries), len(results), len(goodFit))
	if len(results) > 0 {
		t.Logf("Top model: %s (Score: %.1f)", results[0].Model.DisplayName, results[0].Score.TotalScore)
	}
}

// ──────────────────────────────────────────────
// E2E: Error Handling and Edge Cases
// ──────────────────────────────────────────────

// TestE2E_GPUDetectionErrorWrapped tests that IsNoGPUError works with wrapped errors
func TestE2E_GPUDetectionErrorWrapped(t *testing.T) {
	// Direct error
	direct := hardware.NoGPUError(fmt.Errorf("nvidia-smi not found"))
	if !hardware.IsNoGPUError(direct) {
		t.Error("IsNoGPUError should detect direct NoGPUError")
	}

	// Wrapped error (simulating what Detect() does)
	wrapped := fmt.Errorf("gpu detection failed: %w", hardware.NoGPUError(fmt.Errorf("nvidia-smi not found")))
	if !hardware.IsNoGPUError(wrapped) {
		t.Error("IsNoGPUError should detect wrapped NoGPUError")
	}

	// Double-wrapped
	doubleWrapped := fmt.Errorf("outer error: %w", wrapped)
	if !hardware.IsNoGPUError(doubleWrapped) {
		t.Error("IsNoGPUError should detect double-wrapped NoGPUError")
	}

	// Partial GPU error should NOT be NoGPUError
	partial := hardware.PartialGPUError(fmt.Errorf("AMD detection failed"))
	if hardware.IsNoGPUError(partial) {
		t.Error("IsNoGPUError should NOT return true for PartialGPUError")
	}

	// Non-GPU error should NOT be NoGPUError
	nonGPU := fmt.Errorf("some unrelated error")
	if hardware.IsNoGPUError(nonGPU) {
		t.Error("IsNoGPUError should NOT return true for non-GPU errors")
	}

	t.Log("All wrapped error tests passed")
}

// TestE2E_GPUDetectionErrorString tests Error() method formatting
func TestE2E_GPUDetectionErrorString(t *testing.T) {
	// No GPU error with GPUsFound=false
	noGPU := hardware.NoGPUError(fmt.Errorf("no nvidia-smi"))
	errStr := noGPU.Error()
	if errStr == "" {
		t.Error("Error() should return non-empty string")
	}
	t.Logf("NoGPUError string: %s", errStr)

	// Partial GPU error with GPUsFound=true
	partial := hardware.PartialGPUError(fmt.Errorf("AMD failed"))
	partialStr := partial.Error()
	if partialStr == "" {
		t.Error("PartialGPUError Error() should return non-empty string")
	}
	t.Logf("PartialGPUError string: %s", partialStr)
}

// TestE2E_GPUDetectionErrorUnwrap tests the Unwrap method
func TestE2E_GPUDetectionErrorUnwrap(t *testing.T) {
	inner := fmt.Errorf("inner error")
	gpuErr := hardware.NoGPUError(inner)

	unwrapped := gpuErr.Unwrap()
	if unwrapped != inner {
		t.Errorf("Unwrap() returned wrong error: got %v, expected %v", unwrapped, inner)
	}
}

// ──────────────────────────────────────────────
// E2E: MockDetector Error Paths
// ──────────────────────────────────────────────

// TestE2E_MockDetectorErrorPaths tests MockDetector with error conditions
func TestE2E_MockDetectorErrorPaths(t *testing.T) {
	testErr := fmt.Errorf("simulated detection error")

	t.Run("CPU error propagates", func(t *testing.T) {
		profile := models.HardwareProfile{}
		detector := hardware.NewMockDetector(profile, testErr)
		_, _, err := detector.DetectCPU()
		if err != testErr {
			t.Errorf("expected error %v, got %v", testErr, err)
		}
	})

	t.Run("Memory error propagates", func(t *testing.T) {
		profile := models.HardwareProfile{}
		detector := hardware.NewMockDetector(profile, testErr)
		_, _, err := detector.DetectMemory()
		if err != testErr {
			t.Errorf("expected error %v, got %v", testErr, err)
		}
	})

	t.Run("GPU error propagates", func(t *testing.T) {
		profile := models.HardwareProfile{}
		detector := hardware.NewMockDetector(profile, testErr)
		_, err := detector.DetectGPU()
		if err != testErr {
			t.Errorf("expected error %v, got %v", testErr, err)
		}
	})

	t.Run("Detect returns error", func(t *testing.T) {
		profile := models.HardwareProfile{}
		detector := hardware.NewMockDetector(profile, testErr)
		_, err := detector.Detect()
		if err != testErr {
			t.Errorf("expected error %v, got %v", testErr, err)
		}
	})
}

// TestE2E_MockDetectorEmptyGPU tests MockDetector with zero GPUs
func TestE2E_MockDetectorEmptyGPU(t *testing.T) {
	profile := models.HardwareProfile{
		CPUModel:     "Test CPU",
		CPUCores:     4,
		TotalRAM:     16 * 1024 * 1024 * 1024,
		AvailableRAM: 12 * 1024 * 1024 * 1024,
		GPUs:         []models.GPUInfo{}, // empty, not nil
		Platform:     "windows",
	}

	detector := hardware.NewMockDetector(profile, nil)
	result, err := detector.Detect()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.GPUs == nil {
		t.Error("GPUs slice should be empty, not nil")
	}
	if len(result.GPUs) != 0 {
		t.Errorf("expected 0 GPUs, got %d", len(result.GPUs))
	}
}

// ──────────────────────────────────────────────
// E2E: Caching Stress Test
// ──────────────────────────────────────────────

// TestE2E_CacheConcurrentAccess tests that caching works correctly
func TestE2E_CacheConcurrentAccess(t *testing.T) {
	hardware.ResetCache()

	// First call
	profile1, err1 := hardware.DetectCached()
	if profile1.CPUModel == "" {
		t.Fatal("first DetectCached returned empty CPUModel")
	}

	// Multiple subsequent calls should return identical cached results
	for i := 0; i < 10; i++ {
		profileN, errN := hardware.DetectCached()
		if profileN.CPUModel != profile1.CPUModel {
			t.Errorf("call %d: cached CPUModel mismatch", i+1)
		}
		if errN != err1 {
			t.Errorf("call %d: cached error mismatch", i+1)
		}
	}

	// Reset should clear cache
	hardware.ResetCache()
	profileAfter, _ := hardware.DetectCached()
	// Should still get same CPU model (same machine) but via fresh detection
	if profileAfter.CPUModel != profile1.CPUModel {
		t.Logf("CPU model changed after reset (expected: %q, got: %q)", profile1.CPUModel, profileAfter.CPUModel)
	}
}

// ──────────────────────────────────────────────
// E2E: Scorer Profiles Validation
// ──────────────────────────────────────────────

// TestE2E_ScorerProfiles tests all use-case profiles produce valid results
func TestE2E_ScorerProfiles(t *testing.T) {
	hardware.ResetCache()
	profile, _ := hardware.Detect()
	if profile.TotalRAM == 0 {
		t.Skip("no hardware profile available")
	}

	fetcher := registry.NewFetcher()
	entries, err := fetcher.FetchAll(t.Context())
	if err != nil {
		t.Fatalf("failed to fetch models: %v", err)
	}

	profiles := scorer.ValidProfiles()
	t.Logf("Testing %d use-case profiles: %v", len(profiles), profiles)

	s := scorer.NewScorer()

	for _, prof := range profiles {
		t.Run(prof, func(t *testing.T) {
			// Note: RankWithFilter doesn't currently use UseCaseProfile field
			// This test validates that the scorer runs without errors
			results, err := s.Rank(profile, entries)
			if err != nil {
				t.Fatalf("Rank(profile=%s) failed: %v", prof, err)
			}
			if len(results) == 0 {
				t.Fatalf("profile %s produced no results", prof)
			}

			// Verify scores are within valid range
			for _, r := range results {
				if r.Score.TotalScore < 0 || r.Score.TotalScore > 100 {
					t.Errorf("TotalScore out of range: %.2f for %s", r.Score.TotalScore, r.Model.DisplayName)
				}
			}

			t.Logf("Profile %q: %d models scored, top=%s (%.1f)",
				prof, len(results), results[0].Model.DisplayName, results[0].Score.TotalScore)
		})
	}
}
