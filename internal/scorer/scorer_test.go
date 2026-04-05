package scorer

import (
	"math"
	"testing"

	"github.com/kevo-1/KnowURLLM/internal/models"
)

// ──────────────────────────────────────────────
// fitTierAndScore tests
// ──────────────────────────────────────────────

func TestFitTierAndScore(t *testing.T) {
	tests := []struct {
		name           string
		modelSizeBytes uint64
		totalVRAM      uint64
		totalRAM       uint64
		mode           RunMode
		wantTier       FitTier
		wantScore      float64
	}{
		{
			name:           "GPU perfect — 4GB model in 8GB VRAM",
			modelSizeBytes: 4 * GB,
			totalVRAM:      8 * GB,
			totalRAM:       32 * GB,
			mode:           RunModeGPU,
			wantTier:       FitPerfect,
			wantScore:      100,
		},
		{
			name:           "GPU tight — 7.5GB model in 8GB VRAM (~14% headroom)",
			modelSizeBytes: 7_500_000_000,
			totalVRAM:      8 * GB,
			totalRAM:       32 * GB,
			mode:           RunModeGPU,
			wantTier:       FitPerfect, // 14% headroom >= 10% threshold
			wantScore:      100,
		},
		{
			name:           "GPU very tight — 7.9GB model in 8GB VRAM",
			modelSizeBytes: 7_900_000_000,
			totalVRAM:      8 * GB,
			totalRAM:       32 * GB,
			mode:           RunModeGPU,
			wantTier:       FitGood, // ~8.7% headroom < 10% threshold
			wantScore:      75,
		},
		{
			name:           "MoE mode",
			modelSizeBytes: 30 * GB,
			totalVRAM:      8 * GB,
			totalRAM:       64 * GB,
			mode:           RunModeMoE,
			wantTier:       FitGood,
			wantScore:      75,
		},
		{
			name:           "CPU+GPU spill",
			modelSizeBytes: 16 * GB,
			totalVRAM:      8 * GB,
			totalRAM:       32 * GB,
			mode:           RunModeCPUGPU,
			wantTier:       FitGood,
			wantScore:      75,
		},
		{
			name:           "CPU only",
			modelSizeBytes: 8 * GB,
			totalVRAM:      0,
			totalRAM:       32 * GB,
			mode:           RunModeCPU,
			wantTier:       FitMarginal,
			wantScore:      40,
		},
		{
			name:           "Too tight — doesn't fit",
			modelSizeBytes: 200 * GB,
			totalVRAM:      8 * GB,
			totalRAM:       32 * GB,
			mode:           RunModeCPU,
			wantTier:       FitTooTight,
			wantScore:      0,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tier, score := fitTierAndScore(tc.mode, tc.modelSizeBytes, tc.totalVRAM, tc.totalRAM)
			if tier != tc.wantTier {
				t.Errorf("tier = %v, want %v", tier, tc.wantTier)
			}
			if math.Abs(score-tc.wantScore) > 0.01 {
				t.Errorf("score = %f, want %f", score, tc.wantScore)
			}
		})
	}
}

// ──────────────────────────────────────────────
// detectRunMode tests
// ──────────────────────────────────────────────

func TestDetectRunMode(t *testing.T) {
	tests := []struct {
		name            string
		modelSizeBytes  uint64
		vramBytes       uint64
		totalRAM        uint64
		isMoE           bool
		activeParams    uint64
		expectedMode    RunMode
	}{
		{
			name:           "Fits in VRAM",
			modelSizeBytes: 4 * GB,
			vramBytes:      8 * GB,
			totalRAM:       32 * GB,
			isMoE:          false,
			expectedMode:   RunModeGPU,
		},
		{
			name:           "Spills to RAM with GPU",
			modelSizeBytes: 16 * GB,
			vramBytes:      8 * GB,
			totalRAM:       32 * GB,
			isMoE:          false,
			expectedMode:   RunModeCPUGPU,
		},
		{
			name:           "CPU only",
			modelSizeBytes: 8 * GB,
			vramBytes:      0,
			totalRAM:       32 * GB,
			isMoE:          false,
			expectedMode:   RunModeCPU,
		},
		{
			name:           "Doesn't fit at all",
			modelSizeBytes: 200 * GB,
			vramBytes:      8 * GB,
			totalRAM:       32 * GB,
			isMoE:          false,
			expectedMode:   RunModeCPU,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			mode := detectRunMode(tc.modelSizeBytes, tc.vramBytes, tc.totalRAM, tc.isMoE, tc.activeParams)
			if mode != tc.expectedMode {
				t.Errorf("mode = %v, want %v", mode, tc.expectedMode)
			}
		})
	}
}

// ──────────────────────────────────────────────
// selectBestQuant tests
// ──────────────────────────────────────────────

func TestSelectBestQuant(t *testing.T) {
	tests := []struct {
		name           string
		modelSizeBytes uint64
		availableMem   uint64
		wantQuant      string
	}{
		{
			name:           "Plenty of memory — Q8_0",
			modelSizeBytes: 4 * GB,
			availableMem:   40 * GB,
			wantQuant:      "Q8_0",
		},
		{
			name:           "Moderate memory — Q5_K_M (4GB model, 5GB available)",
			modelSizeBytes: 4 * GB,
			availableMem:   5 * GB,
			wantQuant:      "Q5_K_M",
		},
		{
			name:           "Tight memory — Q2_K",
			modelSizeBytes: 4 * GB,
			availableMem:   3 * GB,
			wantQuant:      "Q2_K",
		},
		{
			name:           "Very tight — nothing fits",
			modelSizeBytes: 70 * GB,
			availableMem:   8 * GB,
			wantQuant:      "",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := selectBestQuant(tc.modelSizeBytes, tc.availableMem)
			if got != tc.wantQuant {
				t.Errorf("selectBestQuant(%d, %d) = %q, want %q",
					tc.modelSizeBytes, tc.availableMem, got, tc.wantQuant)
			}
		})
	}
}

// ──────────────────────────────────────────────
// sizeForQuant tests
// ──────────────────────────────────────────────

func TestSizeForQuant(t *testing.T) {
	// Q4_K_M baseline: 4GB model
	base := uint64(4 * GB)

	// Q8_0 should be larger (1.000/0.563 ≈ 1.776x)
	q8Size := sizeForQuant(base, "Q8_0")
	if q8Size <= base {
		t.Errorf("Q8_0 size %d should be > base size %d", q8Size, base)
	}

	// Q2_K should be smaller (0.313/0.563 ≈ 0.556x)
	q2Size := sizeForQuant(base, "Q2_K")
	if q2Size >= base {
		t.Errorf("Q2_K size %d should be < base size %d", q2Size, base)
	}

	// Unknown quant returns as-is
	unknownSize := sizeForQuant(base, "UNKNOWN")
	if unknownSize != base {
		t.Errorf("unknown quant: got %d, want %d", unknownSize, base)
	}
}

// ──────────────────────────────────────────────
// speedScore tests
// ──────────────────────────────────────────────

func TestSpeedScore(t *testing.T) {
	tests := []struct {
		name         string
		modelSizeGB  float64
		hw           models.HardwareProfile
		mode         RunMode
		quant        string
		wantTPSMin   float64
		wantScoreMin float64
	}{
		{
			name:        "GPU inference with known bandwidth (RTX 4090)",
			modelSizeGB: 4.0,
			hw: models.HardwareProfile{
				GPUs: []models.GPUInfo{{Vendor: "nvidia", Model: "NVIDIA GeForce RTX 4090", VRAM: 24 * GB}},
			},
			mode:         RunModeGPU,
			quant:        "Q4_K_M",
			wantTPSMin:   100,
			wantScoreMin: 70,
		},
		{
			name:        "CPU inference — no GPU",
			modelSizeGB: 4.0,
			hw: models.HardwareProfile{
				CPUModel: "AMD Ryzen 9 7950X",
				GPUs:     []models.GPUInfo{},
			},
			mode:         RunModeCPU,
			quant:        "Q4_K_M",
			wantTPSMin:   0.1,
			wantScoreMin: 0,
		},
		{
			name:        "zero size",
			modelSizeGB: 0,
			hw:          models.HardwareProfile{},
			mode:        RunModeCPU,
			quant:        "Q4_K_M",
			wantTPSMin:   0,
			wantScoreMin: 0,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			modelSizeBytes := uint64(tc.modelSizeGB * float64(GB))
			score, estimatedTPS := speedScore(tc.hw, modelSizeBytes, tc.mode, tc.quant)

			if tc.modelSizeGB == 0 {
				if estimatedTPS != 0 || score != 0 {
					t.Errorf("expected 0 for zero-size model, got score=%f, tps=%f", score, estimatedTPS)
				}
				return
			}

			if estimatedTPS < tc.wantTPSMin {
				t.Errorf("TPS = %f, want >= %f", estimatedTPS, tc.wantTPSMin)
			}

			if score < tc.wantScoreMin {
				t.Errorf("speedScore = %f, want >= %f", score, tc.wantScoreMin)
			}
		})
	}
}

// ──────────────────────────────────────────────
// contextScore tests
// ──────────────────────────────────────────────

func TestContextScore(t *testing.T) {
	tests := []struct {
		name          string
		contextLength int
		wantScore     float64
	}{
		{"128k+", 128000, 100},
		{"128k+", 200000, 100},
		{"64k-128k", 64000, 85},
		{"32k-64k", 32000, 70},
		{"16k-32k", 16000, 55},
		{"8k-16k", 8000, 40},
		{"<8k", 4000, 20},
		{"<8k", 0, 20},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := contextScore(tc.contextLength)
			if got != tc.wantScore {
				t.Errorf("contextScore(%d) = %f, want %f", tc.contextLength, got, tc.wantScore)
			}
		})
	}
}

// ──────────────────────────────────────────────
// qualityScore tests
// ──────────────────────────────────────────────

func TestQualityScore(t *testing.T) {
	tests := []struct {
		name      string
		mmluScore float64
		arenaELO  float64
		wantScore float64
	}{
		{
			name:      "both scores present",
			mmluScore: 68.4,
			arenaELO:  1150.5,
			wantScore: 0.6*68.4 + 0.4*((1150.5-800)/(1300-800)*100),
		},
		{
			name:      "MMLU only",
			mmluScore: 72.0,
			arenaELO:  0,
			wantScore: 72.0,
		},
		{
			name:      "ELO only",
			mmluScore: 0,
			arenaELO:  1100.0,
			wantScore: (1100.0 - 800) / (1300 - 800) * 100,
		},
		{
			name:      "neither — default 50",
			mmluScore: 0,
			arenaELO:  0,
			wantScore: 50,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := qualityScore(tc.mmluScore, tc.arenaELO)
			if math.Abs(got-tc.wantScore) > 0.1 {
				t.Errorf("qualityScore(%f, %f) = %f, want ~%f",
					tc.mmluScore, tc.arenaELO, got, tc.wantScore)
			}
		})
	}
}

// ──────────────────────────────────────────────
// weightsForUseCase tests
// ──────────────────────────────────────────────

func TestWeightsForUseCase(t *testing.T) {
	tests := []struct {
		name    string
		useCase string
		want    UseCaseWeights
	}{
		{
			name:    "coding",
			useCase: "coding",
			want:    UseCaseWeights{0.45, 0.25, 0.20, 0.10},
		},
		{
			name:    "reasoning",
			useCase: "reasoning",
			want:    UseCaseWeights{0.55, 0.15, 0.20, 0.10},
		},
		{
			name:    "chat",
			useCase: "chat",
			want:    UseCaseWeights{0.25, 0.35, 0.25, 0.15},
		},
		{
			name:    "multimodal",
			useCase: "multimodal",
			want:    UseCaseWeights{0.40, 0.20, 0.30, 0.10},
		},
		{
			name:    "embedding",
			useCase: "embedding",
			want:    UseCaseWeights{0.50, 0.30, 0.20, 0.00},
		},
		{
			name:    "general (default)",
			useCase: "",
			want:    UseCaseWeights{0.35, 0.25, 0.25, 0.15},
		},
		{
			name:    "general (explicit)",
			useCase: "general",
			want:    UseCaseWeights{0.35, 0.25, 0.25, 0.15},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := weightsForUseCase(tc.useCase)
			if got != tc.want {
				t.Errorf("weightsForUseCase(%q) = %+v, want %+v", tc.useCase, got, tc.want)
			}
			// Verify weights sum to 1.0
			sum := got.Quality + got.Speed + got.Fit + got.Context
			if math.Abs(sum-1.0) > 0.001 {
				t.Errorf("weights sum to %f, want 1.0", sum)
			}
		})
	}
}

// ──────────────────────────────────────────────
// quantSpeedMultiplier tests
// ──────────────────────────────────────────────

func TestQuantSpeedMultiplier(t *testing.T) {
	tests := []struct {
		quant    string
		expected float64
	}{
		{"Q2_K", 1.6},
		{"Q3_K", 1.4},
		{"Q3_K_M", 1.4},
		{"Q4_K_M", 1.0},
		{"Q5_K_M", 0.85},
		{"Q6_K", 0.75},
		{"Q8_0", 0.65},
		{"FP16", 0.5},
		{"FP32", 0.3},
		{"", 1.0},
		{"UNKNOWN", 1.0},
	}

	for _, tc := range tests {
		t.Run(tc.quant, func(t *testing.T) {
			got := quantSpeedMultiplier(tc.quant)
			if got != tc.expected {
				t.Errorf("quantSpeedMultiplier(%q) = %f, want %f", tc.quant, got, tc.expected)
			}
		})
	}
}

// ──────────────────────────────────────────────
// totalVRAM tests
// ──────────────────────────────────────────────

func TestTotalVRAM(t *testing.T) {
	tests := []struct {
		name string
		hw   models.HardwareProfile
		want uint64
	}{
		{
			name: "single GPU",
			hw: models.HardwareProfile{
				GPUs: []models.GPUInfo{{Vendor: "nvidia", VRAM: 8 * GB}},
			},
			want: 8 * GB,
		},
		{
			name: "multiple GPUs",
			hw: models.HardwareProfile{
				GPUs: []models.GPUInfo{
					{Vendor: "nvidia", VRAM: 8 * GB},
					{Vendor: "nvidia", VRAM: 16 * GB},
				},
			},
			want: 24 * GB,
		},
		{
			name: "no GPUs",
			hw:   models.HardwareProfile{GPUs: []models.GPUInfo{}},
			want: 0,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := totalVRAM(tc.hw)
			if got != tc.want {
				t.Errorf("totalVRAM = %d, want %d", got, tc.want)
			}
		})
	}
}

// ──────────────────────────────────────────────
// Rank() integration tests
// ──────────────────────────────────────────────

func TestRank_BasicExclusion(t *testing.T) {
	hw := models.HardwareProfile{
		TotalRAM: 32 * 1024 * 1024 * 1024,
		CPUCores: 16,
		GPUs: []models.GPUInfo{
			{Vendor: "nvidia", VRAM: 8 * 1024 * 1024 * 1024},
		},
	}

	entries := []models.ModelEntry{
		{ID: "small", ModelSizeBytes: 4_000_000_000, Quantization: "Q4_K_M"},
		{ID: "medium", ModelSizeBytes: 16_000_000_000, Quantization: "Q4_K_M"},
		{ID: "large", ModelSizeBytes: 200_000_000_000, Quantization: "FP16"},
	}

	scorer := NewScorer()
	results, err := scorer.Rank(hw, entries)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(results) != 2 {
		t.Fatalf("expected 2 results (large excluded), got %d", len(results))
	}

	// Small model should be ranked first (higher TPS due to smaller size)
	if results[0].Model.ID != "small" {
		t.Errorf("expected rank 1 = 'small', got %q", results[0].Model.ID)
	}

	if !results[0].Score.FitsInVRAM {
		t.Error("expected small model to fit in VRAM")
	}

	if results[1].Score.FitsInVRAM {
		t.Error("expected medium model to NOT fit in VRAM")
	}

	if !results[1].Score.FitsInMemory {
		t.Error("expected medium model to fit in memory")
	}

	// Check ranks are sequential
	for i, r := range results {
		if r.Rank != i+1 {
			t.Errorf("result[%d] has rank %d, expected %d", i, r.Rank, i+1)
		}
	}
}

func TestRank_NilEntries(t *testing.T) {
	scorer := NewScorer()
	hw := models.HardwareProfile{TotalRAM: 32 * GB, CPUCores: 16}

	_, err := scorer.Rank(hw, nil)
	if err == nil {
		t.Error("expected error for nil entries, got nil")
	}
}

func TestRank_ZeroRAM(t *testing.T) {
	scorer := NewScorer()
	hw := models.HardwareProfile{TotalRAM: 0, CPUCores: 16}
	entries := []models.ModelEntry{{ID: "test", ModelSizeBytes: 4 * GB}}

	_, err := scorer.Rank(hw, entries)
	if err == nil {
		t.Error("expected error for zero RAM, got nil")
	}
}

func TestRank_Tiebreaker(t *testing.T) {
	hw := models.HardwareProfile{
		TotalRAM: 32 * GB,
		CPUCores: 16,
		GPUs:     []models.GPUInfo{{Vendor: "nvidia", VRAM: 24 * GB}},
	}

	// Two identical models — tiebreaker should use downloads, then name
	entries := []models.ModelEntry{
		{
			ID:             "model-b",
			DisplayName:    "Model B",
			ModelSizeBytes: 4 * GB,
			Downloads:      100,
			MMLUScore:      65.0,
		},
		{
			ID:             "model-a",
			DisplayName:    "Model A",
			ModelSizeBytes: 4 * GB,
			Downloads:      200,
			MMLUScore:      65.0,
		},
	}

	scorer := NewScorer()
	results, err := scorer.Rank(hw, entries)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}

	// Model A should win due to higher downloads
	if results[0].Model.ID != "model-a" {
		t.Errorf("expected rank 1 = 'model-a' (higher downloads), got %q", results[0].Model.ID)
	}
}

func TestRank_Tiebreaker_Alphabetical(t *testing.T) {
	hw := models.HardwareProfile{
		TotalRAM: 32 * GB,
		CPUCores: 16,
		GPUs:     []models.GPUInfo{{Vendor: "nvidia", VRAM: 24 * GB}},
	}

	entries := []models.ModelEntry{
		{
			ID:             "model-z",
			DisplayName:    "Model Z",
			ModelSizeBytes: 4 * GB,
			Downloads:      100,
			MMLUScore:      65.0,
		},
		{
			ID:             "model-a",
			DisplayName:    "Model A",
			ModelSizeBytes: 4 * GB,
			Downloads:      100,
			MMLUScore:      65.0,
		},
	}

	scorer := NewScorer()
	results, err := scorer.Rank(hw, entries)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if results[0].Model.DisplayName != "Model A" {
		t.Errorf("expected rank 1 = 'Model A' (alphabetical), got %q", results[0].Model.DisplayName)
	}
}

// ──────────────────────────────────────────────
// RankWithFilter tests
// ──────────────────────────────────────────────

func TestRankWithFilter_VRAMOnly(t *testing.T) {
	hw := models.HardwareProfile{
		TotalRAM: 32 * GB,
		CPUCores: 16,
		GPUs:     []models.GPUInfo{{Vendor: "nvidia", VRAM: 8 * GB}},
	}

	entries := []models.ModelEntry{
		{ID: "small", ModelSizeBytes: 4 * GB, Quantization: "Q4_K_M"},
		{ID: "medium", ModelSizeBytes: 16 * GB, Quantization: "Q4_K_M"},
	}

	scorer := NewScorer()
	results, err := scorer.RankWithFilter(hw, entries, models.FilterOptions{VRAMOnly: true})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(results) != 1 {
		t.Fatalf("expected 1 result (VRAM only), got %d", len(results))
	}

	if results[0].Model.ID != "small" {
		t.Errorf("expected 'small', got %q", results[0].Model.ID)
	}
}

func TestRankWithFilter_Source(t *testing.T) {
	hw := models.HardwareProfile{
		TotalRAM: 32 * GB,
		CPUCores: 16,
	}

	entries := []models.ModelEntry{
		{ID: "hf-model", ModelSizeBytes: 4 * GB, Source: "huggingface"},
		{ID: "ollama-model", ModelSizeBytes: 4 * GB, Source: "ollama"},
	}

	scorer := NewScorer()
	results, err := scorer.RankWithFilter(hw, entries, models.FilterOptions{Source: "huggingface"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(results) != 1 {
		t.Fatalf("expected 1 result (huggingface only), got %d", len(results))
	}

	if results[0].Model.Source != "huggingface" {
		t.Errorf("expected huggingface source, got %q", results[0].Model.Source)
	}
}

func TestRankWithFilter_Quantization(t *testing.T) {
	hw := models.HardwareProfile{
		TotalRAM: 32 * GB,
		CPUCores: 16,
	}

	entries := []models.ModelEntry{
		{ID: "q4-model", ModelSizeBytes: 4 * GB, Quantization: "Q4_K_M"},
		{ID: "q8-model", ModelSizeBytes: 4 * GB, Quantization: "Q8_0"},
	}

	scorer := NewScorer()
	results, err := scorer.RankWithFilter(hw, entries, models.FilterOptions{Quantization: "Q4_K_M"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(results) != 1 {
		t.Fatalf("expected 1 result (Q4_K_M only), got %d", len(results))
	}

	if results[0].Model.Quantization != "Q4_K_M" {
		t.Errorf("expected Q4_K_M, got %q", results[0].Model.Quantization)
	}
}

func TestRankWithFilter_SearchQuery(t *testing.T) {
	hw := models.HardwareProfile{
		TotalRAM: 32 * GB,
		CPUCores: 16,
	}

	entries := []models.ModelEntry{
		{ID: "llama-model", DisplayName: "Llama 3.1 8B", ModelSizeBytes: 4 * GB, Tags: []string{"text-generation", "conversational"}},
		{ID: "mistral-model", DisplayName: "Mistral 7B", ModelSizeBytes: 4 * GB, Tags: []string{"text-generation"}},
	}

	scorer := NewScorer()
	results, err := scorer.RankWithFilter(hw, entries, models.FilterOptions{SearchQuery: "llama"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(results) != 1 {
		t.Fatalf("expected 1 result (search 'llama'), got %d", len(results))
	}

	if results[0].Model.ID != "llama-model" {
		t.Errorf("expected 'llama-model', got %q", results[0].Model.ID)
	}
}

func TestRankWithFilter_MinQuality(t *testing.T) {
	hw := models.HardwareProfile{
		TotalRAM: 32 * GB,
		CPUCores: 16,
	}

	entries := []models.ModelEntry{
		{ID: "high-quality", ModelSizeBytes: 4 * GB, MMLUScore: 75.0},
		{ID: "low-quality", ModelSizeBytes: 4 * GB, MMLUScore: 40.0},
	}

	scorer := NewScorer()
	results, err := scorer.RankWithFilter(hw, entries, models.FilterOptions{MinQuality: 60.0})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(results) != 1 {
		t.Fatalf("expected 1 result (MinQuality >= 60), got %d", len(results))
	}

	if results[0].Model.ID != "high-quality" {
		t.Errorf("expected 'high-quality', got %q", results[0].Model.ID)
	}
}

// ──────────────────────────────────────────────
// scoreModel integration test
// ──────────────────────────────────────────────

func TestScoreModel(t *testing.T) {
	hw := models.HardwareProfile{
		TotalRAM: 32 * GB,
		CPUCores: 16,
		GPUs:     []models.GPUInfo{{Vendor: "nvidia", VRAM: 8 * GB}},
	}

	entry := models.ModelEntry{
		ID:             "test-model",
		ModelSizeBytes: 4 * GB,
		Quantization:   "Q4_K_M",
		MMLUScore:      68.4,
		ArenaELO:       1150.0,
	}

	score, excluded := scoreModel(hw, entry)

	if excluded {
		t.Fatal("expected model to not be excluded")
	}

	if score.TotalScore <= 0 {
		t.Errorf("expected positive total score, got %f", score.TotalScore)
	}

	if !score.FitsInVRAM {
		t.Error("expected model to fit in VRAM")
	}

	if score.EstimatedTPS <= 0 {
		t.Errorf("expected positive estimated TPS, got %f", score.EstimatedTPS)
	}

	if score.QualityScore <= 0 {
		t.Errorf("expected positive quality score, got %f", score.QualityScore)
	}

	if score.SelectedQuant == "" {
		t.Error("expected non-empty SelectedQuant")
	}
}

func TestScoreModel_Excluded(t *testing.T) {
	hw := models.HardwareProfile{
		TotalRAM: 8 * GB,
		CPUCores: 4,
		GPUs:     []models.GPUInfo{},
	}

	entry := models.ModelEntry{
		ID:             "huge-model",
		ModelSizeBytes: 200 * GB,
	}

	_, excluded := scoreModel(hw, entry)
	if !excluded {
		t.Error("expected huge model to be excluded")
	}
}

func TestScoreModel_SelectedQuant(t *testing.T) {
	hw := models.HardwareProfile{
		TotalRAM: 32 * GB,
		CPUCores: 16,
		GPUs:     []models.GPUInfo{{Vendor: "nvidia", VRAM: 24 * GB}},
	}

	entry := models.ModelEntry{
		ID:             "test-model",
		ModelSizeBytes: 4 * GB,
	}

	score, excluded := scoreModel(hw, entry)
	if excluded {
		t.Fatal("expected model to not be excluded")
	}

	if score.SelectedQuant == "" {
		t.Error("expected non-empty SelectedQuant")
	}

	// With 24GB VRAM + 32GB RAM, should pick Q8_0 (highest quality)
	if score.SelectedQuant != "Q8_0" {
		t.Errorf("expected Q8_0 with plenty of memory, got %q", score.SelectedQuant)
	}
}

// ──────────────────────────────────────────────
// fitTierLabel tests
// ──────────────────────────────────────────────

func TestFitTierLabel(t *testing.T) {
	tests := []struct {
		name           string
		tier           FitTier
		mode           RunMode
		modelSizeBytes uint64
		vramBytes      uint64
		wantContains   string
	}{
		{
			name:           "Perfect fit",
			tier:           FitPerfect,
			mode:           RunModeGPU,
			modelSizeBytes: 4 * GB,
			vramBytes:      8 * GB,
			wantContains:   "headroom",
		},
		{
			name:         "MoE",
			tier:         FitGood,
			mode:         RunModeMoE,
			wantContains: "MoE",
		},
		{
			name:         "CPU+GPU",
			tier:         FitGood,
			mode:         RunModeCPUGPU,
			wantContains: "RAM",
		},
		{
			name:         "CPU",
			tier:         FitMarginal,
			mode:         RunModeCPU,
			wantContains: "CPU",
		},
		{
			name:         "Too tight",
			tier:         FitTooTight,
			mode:         RunModeCPU,
			wantContains: "excluded",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			label := fitTierLabel(tc.tier, tc.mode, tc.modelSizeBytes, tc.vramBytes)
			if label == "" {
				t.Error("expected non-empty label")
			}
		})
	}
}
