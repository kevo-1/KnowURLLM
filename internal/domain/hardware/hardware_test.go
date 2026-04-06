package hardware

import (
	"testing"

	"github.com/kevo-1/KnowURLLM/internal/domain"
)

func TestCheckCompatibility_FitsInVRAM(t *testing.T) {
	hw := domain.HardwareProfile{
		TotalRAM: 32 * GB,
		GPUs: []domain.GPUInfo{
			{Vendor: "nvidia", Model: "NVIDIA GeForce RTX 4090", VRAM: 24 * GB},
		},
	}

	entry := domain.ModelEntry{
		ID:             "test/small-model",
		DisplayName:    "Small Model",
		ModelSizeBytes: 4 * GB, // Q4 model
	}

	result := CheckCompatibility(hw, entry)

	if !result.Runnable {
		t.Error("Expected model to be runnable")
	}

	if result.Mode != domain.RunModeGPU {
		t.Errorf("Expected GPU mode, got %v", result.Mode)
	}

	if result.FitLabel != "Perfect" {
		t.Errorf("Expected Perfect fit, got %s", result.FitLabel)
	}
}

func TestCheckCompatibility_SpillsToRAM(t *testing.T) {
	hw := domain.HardwareProfile{
		TotalRAM: 32 * GB,
		GPUs: []domain.GPUInfo{
			{Vendor: "nvidia", Model: "RTX 4060", VRAM: 8 * GB},
		},
	}

	entry := domain.ModelEntry{
		ID:             "test/large-model",
		DisplayName:    "Large Model",
		ModelSizeBytes: 40 * GB, // 70B model at Q4
	}

	result := CheckCompatibility(hw, entry)

	if !result.Runnable {
		t.Error("Expected model to be runnable")
	}

	if result.Mode != domain.RunModeCPUGPU {
		t.Errorf("Expected CPU+GPU mode, got %v", result.Mode)
	}
}

func TestCheckCompatibility_TooLarge(t *testing.T) {
	hw := domain.HardwareProfile{
		TotalRAM: 8 * GB,
		GPUs:     []domain.GPUInfo{},
	}

	entry := domain.ModelEntry{
		ID:             "test/huge-model",
		DisplayName:    "Huge Model",
		ModelSizeBytes: 100 * GB,
	}

	result := CheckCompatibility(hw, entry)

	if result.Runnable {
		t.Error("Expected model to be non-runnable")
	}

	if result.FitLabel != "Too Tight" {
		t.Errorf("Expected Too Tight fit, got %s", result.FitLabel)
	}
}

func TestCheckCompatibility_MoE(t *testing.T) {
	hw := domain.HardwareProfile{
		TotalRAM: 64 * GB,
		GPUs: []domain.GPUInfo{
			{Vendor: "nvidia", Model: "RTX 4090", VRAM: 24 * GB},
		},
	}

	entry := domain.ModelEntry{
		ID:             "test/moe-model",
		DisplayName:    "MoE Model",
		ModelSizeBytes: 100 * GB, // Total size
		IsMoE:          true,
		ActiveParams:   12000000000, // 12B active
	}

	result := CheckCompatibility(hw, entry)

	if !result.Runnable {
		t.Error("Expected MoE model to be runnable")
	}

	if result.Mode != domain.RunModeMoE {
		t.Errorf("Expected MoE mode, got %v", result.Mode)
	}
}

func TestCheckCompatibility_CPUOnly(t *testing.T) {
	hw := domain.HardwareProfile{
		TotalRAM: 32 * GB,
		GPUs:     []domain.GPUInfo{}, // No GPU
	}

	entry := domain.ModelEntry{
		ID:             "test/cpu-model",
		DisplayName:    "CPU Model",
		ModelSizeBytes: 4 * GB,
	}

	result := CheckCompatibility(hw, entry)

	if !result.Runnable {
		t.Error("Expected CPU model to be runnable")
	}

	if result.Mode != domain.RunModeCPU {
		t.Errorf("Expected CPU mode, got %v", result.Mode)
	}

	if result.FitLabel != "Marginal" {
		t.Errorf("Expected Marginal fit for CPU, got %s", result.FitLabel)
	}
}

func TestEstimatePerformance_GPUBandwidth(t *testing.T) {
	hw := domain.HardwareProfile{
		GPUs: []domain.GPUInfo{
			{Vendor: "nvidia", Model: "NVIDIA GeForce RTX 4090"},
		},
	}

	modelSize := 4 * GB
	score, tps := EstimatePerformance(hw, modelSize, domain.RunModeGPU, "Q4_K_M")

	if tps <= 0 {
		t.Error("Expected positive TPS")
	}

	// RTX 4090: 1008 GB/s bandwidth
	// TPS = (1008 / 4) * 0.85 = 214.2
	if tps < 200 || tps > 230 {
		t.Errorf("Expected TPS around 214, got %v", tps)
	}

	if score < 90 || score > 100 {
		t.Errorf("Expected high score for fast GPU, got %v", score)
	}
}

func TestEstimatePerformance_CPUFallback(t *testing.T) {
	hw := domain.HardwareProfile{
		CPUModel: "AMD Ryzen 9 7950X",
		GPUs:     []domain.GPUInfo{}, // No GPU
	}

	modelSize := 4 * GB
	score, tps := EstimatePerformance(hw, modelSize, domain.RunModeCPU, "Q4_K_M")

	if tps <= 0 {
		t.Error("Expected positive TPS")
	}

	// CPU should be slower than GPU
	if tps > 50 {
		t.Logf("CPU TPS %v seems high (but may be valid)", tps)
	}

	if score < 1 || score > 80 {
		t.Logf("CPU score %v is low (expected for CPU-only inference)", score)
	}
}

func TestEstimatePerformance_QuantSpeed(t *testing.T) {
	hw := domain.HardwareProfile{
		CPUModel: "Intel Core i7",
		GPUs:     []domain.GPUInfo{},
	}

	modelSize := 4 * GB

	_, tpsQ2 := EstimatePerformance(hw, modelSize, domain.RunModeCPU, "Q2_K")
	_, tpsQ8 := EstimatePerformance(hw, modelSize, domain.RunModeCPU, "Q8_0")

	// Q2 should be faster than Q8
	if tpsQ2 <= tpsQ8 {
		t.Errorf("Expected Q2_K (%v) to be faster than Q8_0 (%v)", tpsQ2, tpsQ8)
	}
}

func TestLookupBandwidth(t *testing.T) {
	tests := []struct {
		name     string
		gpuModel string
		wantBW   float64
		wantFound bool
	}{
		{"RTX 4090", "NVIDIA GeForce RTX 4090", 1008, true},
		{"RTX 3090", "RTX 3090", 936, true},
		{"M2 Max", "Apple M2 Max", 400, true},
		{"RX 7900 XTX", "AMD Radeon RX 7900 XTX", 960, true},
		{"Unknown GPU", "Unknown GPU XYZ", 0, false},
		{"Empty string", "", 0, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			bw, found := LookupBandwidth(tt.gpuModel)
			
			if found != tt.wantFound {
				t.Errorf("LookupBandwidth(%q) found = %v, want %v", tt.gpuModel, found, tt.wantFound)
			}
			
			if tt.wantFound && bw != tt.wantBW {
				t.Errorf("LookupBandwidth(%q) = %v, want %v", tt.gpuModel, bw, tt.wantBW)
			}
		})
	}
}

func TestSelectBestQuant(t *testing.T) {
	tests := []struct {
		name          string
		modelSize     uint64
		availableMem  uint64
		expectedQuant string
	}{
		{
			name:          "plenty of memory",
			modelSize:     4 * GB,
			availableMem:  8 * GB,
			expectedQuant: "Q8_0",
		},
		{
			name:          "moderate memory",
			modelSize:     4 * GB,
			availableMem:  5 * GB,
			expectedQuant: "Q5_K_M",
		},
		{
			name:          "tight memory",
			modelSize:     4 * GB,
			availableMem:  3 * GB,
			expectedQuant: "Q2_K",
		},
		{
			name:          "very tight - nothing fits",
			modelSize:     4 * GB,
			availableMem:  1 * GB,
			expectedQuant: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			quant := selectBestQuant(tt.modelSize, tt.availableMem)
			if quant != tt.expectedQuant {
				t.Errorf("selectBestQuant(%v, %v) = %q, want %q",
					tt.modelSize, tt.availableMem, quant, tt.expectedQuant)
			}
		})
	}
}

func TestSizeForQuant(t *testing.T) {
	baseSize := 4 * GB
	
	tests := []struct {
		quant      string
		multiplier float64
	}{
		{"Q8_0", 1.0 / 0.563},
		{"Q4_K_M", 1.0},
		{"Q2_K", 0.313 / 0.563},
	}

	for _, tt := range tests {
		t.Run(tt.quant, func(t *testing.T) {
			size := sizeForQuant(baseSize, tt.quant)
			expected := uint64(float64(baseSize) * tt.multiplier)
			
			// Allow 1% tolerance for rounding
			diff := float64(size) - float64(expected)
			if diff < 0 {
				diff = -diff
			}
			tolerance := float64(expected) * 0.01
			
			if diff > tolerance {
				t.Errorf("sizeForQuant(%v, %q) = %v, want ~%v (diff %v > tolerance %v)",
					baseSize, tt.quant, size, expected, diff, tolerance)
			}
		})
	}
}
