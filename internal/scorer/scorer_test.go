package scorer

import (
	"math"
	"testing"

	"github.com/kevo-1/KnowURLLM/internal/models"
)

// ──────────────────────────────────────────────
// hardwareFitScore tests
// ──────────────────────────────────────────────

func TestHardwareFitScore(t *testing.T) {
	tests := []struct {
		name            string
		modelSizeBytes  uint64
		totalVRAM       uint64
		totalRAM        uint64
		wantScore       float64
		wantFitsVRAM    bool
		wantFitsMemory  bool
		wantExcluded    bool
		wantReasonMatch string // substring check
	}{
		{
			name:           "fitRatio 1.5 — fits in VRAM with headroom",
			modelSizeBytes: 4 * GB,
			totalVRAM:      8 * GB,
			totalRAM:       32 * GB,
			wantScore:      100,
			wantFitsVRAM:   true,
			wantFitsMemory: true,
			wantExcluded:   false,
		},
		{
			name:           "fitRatio ~1.07 — fits exactly with small headroom",
			modelSizeBytes: 22 * GB,
			totalVRAM:      8 * GB,
			totalRAM:       32 * GB,
			// availableMem = 8 + 32*0.5 = 24 GB, fitRatio = 24/22 ≈ 1.09
			wantScore:      80 + (24.0/22.0-1.0)*100,
			wantFitsVRAM:   false,
			wantFitsMemory: true,
			wantExcluded:   false,
		},
		{
			name:           "fitRatio ~0.86 — tight fit, does not fit in memory",
			modelSizeBytes: 28 * GB,
			totalVRAM:      8 * GB,
			totalRAM:       32 * GB,
			// availableMem = 8 + 32*0.5 = 24 GB, fitRatio = 24/28 ≈ 0.857
			// 28 GB > 24 GB available, so FitsInMemory = false
			wantScore:      50 + (24.0/28.0)*30,
			wantFitsVRAM:   false,
			wantFitsMemory: false,
			wantExcluded:   false,
		},
		{
			name:           "fitRatio 0.65 — excluded",
			modelSizeBytes: 200 * GB,
			totalVRAM:      8 * GB,
			totalRAM:       32 * GB,
			wantScore:      0,
			wantFitsVRAM:   false,
			wantFitsMemory: false,
			wantExcluded:   true,
		},
		{
			name:           "zero model size — excluded",
			modelSizeBytes: 0,
			totalVRAM:      8 * GB,
			totalRAM:       32 * GB,
			wantScore:      0,
			wantFitsVRAM:   false,
			wantFitsMemory: false,
			wantExcluded:   true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			score, fitsVRAM, fitsMem, reason, excluded := hardwareFitScore(
				tc.modelSizeBytes,
				tc.totalVRAM,
				tc.totalRAM,
			)

			if excluded != tc.wantExcluded {
				t.Errorf("excluded = %v, want %v", excluded, tc.wantExcluded)
			}

			if !excluded {
				if math.Abs(score-tc.wantScore) > 0.1 {
					t.Errorf("score = %f, want ~%f", score, tc.wantScore)
				}
			}

			if fitsVRAM != tc.wantFitsVRAM {
				t.Errorf("fitsVRAM = %v, want %v", fitsVRAM, tc.wantFitsVRAM)
			}

			if fitsMem != tc.wantFitsMemory {
				t.Errorf("fitsMem = %v, want %v", fitsMem, tc.wantFitsMemory)
			}

			if tc.wantExcluded && reason == "" {
				t.Error("expected non-empty reason for excluded model")
			}
		})
	}
}

// ──────────────────────────────────────────────
// throughputScore tests
// ──────────────────────────────────────────────

func TestThroughputScore(t *testing.T) {
	tests := []struct {
		name          string
		modelSizeGB   float64
		fitsInVRAM    bool
		totalVRAMGB   float64
		cpuCores      int
		quantization  string
		wantTPSRange  [2]float64 // min, max expected TPS
		wantScoreMin  float64
	}{
		{
			name:         "GPU inference — 8GB VRAM, 4GB model, Q4_K_M",
			modelSizeGB:  4.0,
			fitsInVRAM:   true,
			totalVRAMGB:  8.0,
			cpuCores:     16,
			quantization: "Q4_K_M",
			wantTPSRange: [2]float64{150, 170}, // (8 * 80) / (4 * 1.0) = 160
			wantScoreMin: 80,
		},
		{
			name:         "CPU inference — 16 cores, 16GB model, Q4_K_M",
			modelSizeGB:  16.0,
			fitsInVRAM:   false,
			totalVRAMGB:  8.0,
			cpuCores:     16,
			quantization: "Q4_K_M",
			wantTPSRange: [2]float64{2, 4}, // (16 * 3) / (16 * 1.0) = 3
			wantScoreMin: 20,
		},
		{
			name:         "CPU inference — 16 cores, 4GB model, Q4_K_M",
			modelSizeGB:  4.0,
			fitsInVRAM:   false,
			totalVRAMGB:  8.0,
			cpuCores:     16,
			quantization: "Q4_K_M",
			wantTPSRange: [2]float64{10, 14}, // (16 * 3) / (4 * 1.0) = 12
			wantScoreMin: 40,
		},
		{
			name:         "zero size — excluded",
			modelSizeGB:  0,
			fitsInVRAM:   false,
			totalVRAMGB:  8.0,
			cpuCores:     16,
			quantization: "Q4_K_M",
			wantTPSRange: [2]float64{0, 0},
			wantScoreMin: 0,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			modelSizeBytes := uint64(tc.modelSizeGB * float64(GB))
			vramBytes := uint64(tc.totalVRAMGB * float64(GB))

			tpsScore, estimatedTPS := throughputScoreWithQuant(
				modelSizeBytes,
				tc.fitsInVRAM,
				vramBytes,
				tc.cpuCores,
				tc.quantization,
			)

			if tc.modelSizeGB == 0 {
				if estimatedTPS != 0 {
					t.Errorf("expected 0 TPS for zero-size model, got %f", estimatedTPS)
				}
				return
			}

			if estimatedTPS < tc.wantTPSRange[0] || estimatedTPS > tc.wantTPSRange[1] {
				t.Errorf("TPS = %f, want in range [%f, %f]", estimatedTPS, tc.wantTPSRange[0], tc.wantTPSRange[1])
			}

			if tpsScore < tc.wantScoreMin {
				t.Errorf("throughputScore = %f, want >= %f", tpsScore, tc.wantScoreMin)
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
			wantScore: (1100.0 - 800) / (1300 - 800) * 100, // = 60
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
				t.Errorf("qualityScore(%f, %f) = %f, want %f",
					tc.mmluScore, tc.arenaELO, got, tc.wantScore)
			}
		})
	}
}

// ──────────────────────────────────────────────
// quantFactor tests
// ──────────────────────────────────────────────

func TestQuantFactor(t *testing.T) {
	tests := []struct {
		quant    string
		expected float64
	}{
		{"Q2_K", 0.6},
		{"Q3_K", 0.7},
		{"Q3_K_M", 0.7},
		{"Q4_K_M", 1.0},
		{"Q5_K_M", 0.85},
		{"Q8_0", 0.65},
		{"FP16", 0.4},
		{"FP32", 0.2},
		{"", 1.0},         // unknown → baseline
		{"UNKNOWN", 1.0},  // unknown → baseline
		{"q4_k_m", 1.0},   // case insensitive
	}

	for _, tc := range tests {
		t.Run(tc.quant, func(t *testing.T) {
			got := quantFactor(tc.quant)
			if got != tc.expected {
				t.Errorf("quantFactor(%q) = %f, want %f", tc.quant, got, tc.expected)
			}
		})
	}
}

// ──────────────────────────────────────────────
// Rank() integration tests
// ──────────────────────────────────────────────

func TestRank_BasicExclusion(t *testing.T) {
	hw := models.HardwareProfile{
		TotalRAM: 32 * 1024 * 1024 * 1024, // 32 GB
		CPUCores: 16,
		GPUs: []models.GPUInfo{
			{Vendor: "nvidia", VRAM: 8 * 1024 * 1024 * 1024}, // 8 GB VRAM
		},
	}

	entries := []models.ModelEntry{
		{ID: "small", ModelSizeBytes: 4_000_000_000, Quantization: "Q4_K_M"},   // fits in VRAM
		{ID: "medium", ModelSizeBytes: 16_000_000_000, Quantization: "Q4_K_M"}, // fits in RAM
		{ID: "large", ModelSizeBytes: 200_000_000_000, Quantization: "FP16"},   // excluded
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
			Quantization:   "Q4_K_M",
			Downloads:      100,
			MMLUScore:      65.0,
		},
		{
			ID:             "model-a",
			DisplayName:    "Model A",
			ModelSizeBytes: 4 * GB,
			Quantization:   "Q4_K_M",
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

	// Two identical models with same scores and downloads
	entries := []models.ModelEntry{
		{
			ID:             "model-z",
			DisplayName:    "Model Z",
			ModelSizeBytes: 4 * GB,
			Quantization:   "Q4_K_M",
			Downloads:      100,
			MMLUScore:      65.0,
		},
		{
			ID:             "model-a",
			DisplayName:    "Model A",
			ModelSizeBytes: 4 * GB,
			Quantization:   "Q4_K_M",
			Downloads:      100,
			MMLUScore:      65.0,
		},
	}

	scorer := NewScorer()
	results, err := scorer.Rank(hw, entries)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Model A should win due to alphabetical order
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
		{ID: "small", ModelSizeBytes: 4 * GB, Quantization: "Q4_K_M"},  // fits in VRAM
		{ID: "medium", ModelSizeBytes: 16 * GB, Quantization: "Q4_K_M"}, // needs RAM
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
}
