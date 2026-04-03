package models

import (
	"encoding/json"
	"testing"
)

func TestZeroValueStructs(t *testing.T) {
	// Verify that zero-value instantiation doesn't panic
	// and that slices are nil (not initialized)
	var hw HardwareProfile
	if hw.CPUModel != "" {
		t.Errorf("expected empty CPUModel, got %q", hw.CPUModel)
	}
	if hw.TotalRAM != 0 {
		t.Errorf("expected 0 TotalRAM, got %d", hw.TotalRAM)
	}
	if hw.CPUCores != 0 {
		t.Errorf("expected 0 CPUCores, got %d", hw.CPUCores)
	}
	if hw.GPUs != nil {
		t.Errorf("expected nil GPUs, got %v", hw.GPUs)
	}
	if hw.Platform != "" {
		t.Errorf("expected empty Platform, got %q", hw.Platform)
	}
	if hw.IsAppleSilicon {
		t.Error("expected false IsAppleSilicon")
	}

	var gpu GPUInfo
	if gpu.Vendor != "" {
		t.Errorf("expected empty Vendor, got %q", gpu.Vendor)
	}
	if gpu.Model != "" {
		t.Errorf("expected empty Model, got %q", gpu.Model)
	}
	if gpu.VRAM != 0 {
		t.Errorf("expected 0 VRAM, got %d", gpu.VRAM)
	}

	var entry ModelEntry
	if entry.ID != "" {
		t.Errorf("expected empty ID, got %q", entry.ID)
	}
	if entry.DisplayName != "" {
		t.Errorf("expected empty DisplayName, got %q", entry.DisplayName)
	}
	if entry.ModelSizeBytes != 0 {
		t.Errorf("expected 0 ModelSizeBytes, got %d", entry.ModelSizeBytes)
	}
	if entry.Quantization != "" {
		t.Errorf("expected empty Quantization, got %q", entry.Quantization)
	}
	if entry.ContextLength != 0 {
		t.Errorf("expected 0 ContextLength, got %d", entry.ContextLength)
	}
	if entry.Source != "" {
		t.Errorf("expected empty Source, got %q", entry.Source)
	}
	if entry.MMLUScore != 0 {
		t.Errorf("expected 0 MMLUScore, got %f", entry.MMLUScore)
	}
	if entry.ArenaELO != 0 {
		t.Errorf("expected 0 ArenaELO, got %f", entry.ArenaELO)
	}
	if entry.Downloads != 0 {
		t.Errorf("expected 0 Downloads, got %d", entry.Downloads)
	}
	if entry.URL != "" {
		t.Errorf("expected empty URL, got %q", entry.URL)
	}
	if entry.Tags != nil {
		t.Errorf("expected nil Tags, got %v", entry.Tags)
	}

	var score ModelScore
	if score.TotalScore != 0 {
		t.Errorf("expected 0 TotalScore, got %f", score.TotalScore)
	}
	if score.HardwareFitScore != 0 {
		t.Errorf("expected 0 HardwareFitScore, got %f", score.HardwareFitScore)
	}
	if score.ThroughputScore != 0 {
		t.Errorf("expected 0 ThroughputScore, got %f", score.ThroughputScore)
	}
	if score.QualityScore != 0 {
		t.Errorf("expected 0 QualityScore, got %f", score.QualityScore)
	}
	if score.EstimatedTPS != 0 {
		t.Errorf("expected 0 EstimatedTPS, got %f", score.EstimatedTPS)
	}
	if score.FitsInVRAM {
		t.Error("expected false FitsInVRAM")
	}
	if score.FitsInMemory {
		t.Error("expected false FitsInMemory")
	}
	if score.FitReason != "" {
		t.Errorf("expected empty FitReason, got %q", score.FitReason)
	}

	var result RankResult
	if result.Model.ID != "" {
		t.Errorf("expected empty Model.ID, got %q", result.Model.ID)
	}
	if result.Rank != 0 {
		t.Errorf("expected 0 Rank, got %d", result.Rank)
	}

	var filters FilterOptions
	if filters.MinQuality != 0 {
		t.Errorf("expected 0 MinQuality, got %f", filters.MinQuality)
	}
	if filters.VRAMOnly {
		t.Error("expected false VRAMOnly")
	}
	if filters.Source != "" {
		t.Errorf("expected empty Source, got %q", filters.Source)
	}
	if filters.SearchQuery != "" {
		t.Errorf("expected empty SearchQuery, got %q", filters.SearchQuery)
	}
	if filters.Quantization != "" {
		t.Errorf("expected empty Quantization, got %q", filters.Quantization)
	}
}

func TestModelEntryJSONRoundTrip(t *testing.T) {
	original := ModelEntry{
		ID:             "meta-llama/Llama-3.1-8B-Instruct",
		DisplayName:    "Llama 3.1 8B Instruct",
		ModelSizeBytes: 4700000000,
		Quantization:   "Q4_K_M",
		ContextLength:  8192,
		Source:         "huggingface",
		MMLUScore:      68.4,
		ArenaELO:       1150.5,
		Downloads:      1250000,
		URL:            "https://huggingface.co/meta-llama/Llama-3.1-8B-Instruct",
		Tags:           []string{"text-generation", "conversational"},
	}

	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("failed to marshal ModelEntry: %v", err)
	}

	var decoded ModelEntry
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("failed to unmarshal ModelEntry: %v", err)
	}

	if decoded.ID != original.ID {
		t.Errorf("ID mismatch: got %q, want %q", decoded.ID, original.ID)
	}
	if decoded.DisplayName != original.DisplayName {
		t.Errorf("DisplayName mismatch: got %q, want %q", decoded.DisplayName, original.DisplayName)
	}
	if decoded.ModelSizeBytes != original.ModelSizeBytes {
		t.Errorf("ModelSizeBytes mismatch: got %d, want %d", decoded.ModelSizeBytes, original.ModelSizeBytes)
	}
	if decoded.Quantization != original.Quantization {
		t.Errorf("Quantization mismatch: got %q, want %q", decoded.Quantization, original.Quantization)
	}
	if decoded.ContextLength != original.ContextLength {
		t.Errorf("ContextLength mismatch: got %d, want %d", decoded.ContextLength, original.ContextLength)
	}
	if decoded.Source != original.Source {
		t.Errorf("Source mismatch: got %q, want %q", decoded.Source, original.Source)
	}
	if decoded.MMLUScore != original.MMLUScore {
		t.Errorf("MMLUScore mismatch: got %f, want %f", decoded.MMLUScore, original.MMLUScore)
	}
	if decoded.ArenaELO != original.ArenaELO {
		t.Errorf("ArenaELO mismatch: got %f, want %f", decoded.ArenaELO, original.ArenaELO)
	}
	if decoded.Downloads != original.Downloads {
		t.Errorf("Downloads mismatch: got %d, want %d", decoded.Downloads, original.Downloads)
	}
	if decoded.URL != original.URL {
		t.Errorf("URL mismatch: got %q, want %q", decoded.URL, original.URL)
	}
	if len(decoded.Tags) != len(original.Tags) {
		t.Errorf("Tags length mismatch: got %d, want %d", len(decoded.Tags), len(original.Tags))
	}
	for i, tag := range decoded.Tags {
		if tag != original.Tags[i] {
			t.Errorf("Tags[%d] mismatch: got %q, want %q", i, tag, original.Tags[i])
		}
	}
}

func TestModelEntryZeroValuesJSON(t *testing.T) {
	original := ModelEntry{}

	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("failed to marshal zero-value ModelEntry: %v", err)
	}

	var decoded ModelEntry
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("failed to unmarshal zero-value ModelEntry: %v", err)
	}

	if decoded.ID != "" || decoded.DisplayName != "" || decoded.ModelSizeBytes != 0 ||
		decoded.Quantization != "" || decoded.ContextLength != 0 || decoded.Source != "" ||
		decoded.MMLUScore != 0 || decoded.ArenaELO != 0 || decoded.Downloads != 0 ||
		decoded.URL != "" || len(decoded.Tags) != 0 {
		t.Errorf("expected zero-value after round-trip, got %+v", decoded)
	}
}

func TestHardwareProfileJSONRoundTrip(t *testing.T) {
	original := HardwareProfile{
		CPUModel:       "Apple M2 Max",
		TotalRAM:       34359738368, // 32 GB
		CPUCores:       12,
		GPUs:           []GPUInfo{{Vendor: "apple", Model: "Apple M2 Max", VRAM: 25769803776}},
		Platform:       "darwin",
		IsAppleSilicon: true,
	}

	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("failed to marshal HardwareProfile: %v", err)
	}

	var decoded HardwareProfile
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("failed to unmarshal HardwareProfile: %v", err)
	}

	if decoded.CPUModel != original.CPUModel {
		t.Errorf("CPUModel mismatch: got %q, want %q", decoded.CPUModel, original.CPUModel)
	}
	if decoded.TotalRAM != original.TotalRAM {
		t.Errorf("TotalRAM mismatch: got %d, want %d", decoded.TotalRAM, original.TotalRAM)
	}
	if decoded.CPUCores != original.CPUCores {
		t.Errorf("CPUCores mismatch: got %d, want %d", decoded.CPUCores, original.CPUCores)
	}
	if len(decoded.GPUs) != len(original.GPUs) {
		t.Fatalf("GPUs length mismatch: got %d, want %d", len(decoded.GPUs), len(original.GPUs))
	}
	for i, gpu := range decoded.GPUs {
		if gpu.Vendor != original.GPUs[i].Vendor {
			t.Errorf("GPUs[%d].Vendor mismatch: got %q, want %q", i, gpu.Vendor, original.GPUs[i].Vendor)
		}
		if gpu.Model != original.GPUs[i].Model {
			t.Errorf("GPUs[%d].Model mismatch: got %q, want %q", i, gpu.Model, original.GPUs[i].Model)
		}
		if gpu.VRAM != original.GPUs[i].VRAM {
			t.Errorf("GPUs[%d].VRAM mismatch: got %d, want %d", i, gpu.VRAM, original.GPUs[i].VRAM)
		}
	}
	if decoded.Platform != original.Platform {
		t.Errorf("Platform mismatch: got %q, want %q", decoded.Platform, original.Platform)
	}
	if decoded.IsAppleSilicon != original.IsAppleSilicon {
		t.Errorf("IsAppleSilicon mismatch: got %v, want %v", decoded.IsAppleSilicon, original.IsAppleSilicon)
	}
}

func TestHardwareProfileZeroValuesJSON(t *testing.T) {
	original := HardwareProfile{}

	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("failed to marshal zero-value HardwareProfile: %v", err)
	}

	var decoded HardwareProfile
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("failed to unmarshal zero-value HardwareProfile: %v", err)
	}

	if decoded.CPUModel != "" || decoded.TotalRAM != 0 || decoded.CPUCores != 0 ||
		len(decoded.GPUs) != 0 || decoded.Platform != "" || decoded.IsAppleSilicon {
		t.Errorf("expected zero-value after round-trip, got %+v", decoded)
	}
}
