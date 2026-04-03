// Package models defines all shared data structures used across the KnowURLLM modules.
// This is the contract layer — every other module imports models/ and nothing else
// imports models/ from a sibling module.
package models

// HardwareProfile contains detected system hardware information.
type HardwareProfile struct {
	// CPU brand and model string, e.g. "Apple M2 Max", "AMD Ryzen 9 7950X"
	CPUModel string

	// Total system RAM in bytes
	TotalRAM uint64

	// Number of logical CPU cores
	CPUCores int

	// Detected GPUs
	GPUs []GPUInfo

	// Platform identifier: "linux", "darwin", "windows"
	Platform string

	// True if the system is Apple Silicon (M1/M2/M3 family)
	IsAppleSilicon bool
}

// GPUInfo contains information about a detected GPU.
type GPUInfo struct {
	// Vendor: "nvidia", "amd", "apple", "intel", "unknown"
	Vendor string

	// GPU model name, e.g. "NVIDIA GeForce RTX 4090"
	Model string

	// Available VRAM in bytes (0 if no discrete GPU)
	VRAM uint64
}

// ModelEntry represents an LLM model from a registry (Hugging Face or Ollama).
type ModelEntry struct {
	// Unique identifier, e.g. "meta-llama/Llama-3.1-8B-Instruct"
	ID string

	// Human-readable display name
	DisplayName string

	// Model size in bytes (parameters × avg bytes per param)
	ModelSizeBytes uint64

	// Quantization label, e.g. "Q4_K_M", "Q8_0", "FP16", "" if unknown
	Quantization string

	// Context length the model was trained/fine-tuned for
	ContextLength int

	// Source registry: "huggingface", "ollama", or "huggingface+ollama"
	Source string

	// MMLU benchmark score (0-100), 0 if unavailable
	MMLUScore float64

	// Chatbot Arena ELO rating, 0 if unavailable
	ArenaELO float64

	// Hugging Face download count (popularity signal)
	Downloads int

	// Raw URL to the model card / page
	URL string

	// Tags from the registry, e.g. ["text-generation", "conversational"]
	Tags []string
}

// ModelScore contains the computed scoring metrics for a model on specific hardware.
type ModelScore struct {
	// Overall composite score (higher = better)
	TotalScore float64

	// Sub-score: hardware fit (0-100)
	HardwareFitScore float64

	// Sub-score: estimated throughput (0-100)
	ThroughputScore float64

	// Sub-score: model quality from benchmarks (0-100)
	QualityScore float64

	// Estimated tokens/sec the model will achieve on this hardware
	EstimatedTPS float64

	// True if the model fits in VRAM alone (no RAM offload needed)
	FitsInVRAM bool

	// True if the model fits in combined VRAM + system RAM
	FitsInMemory bool

	// Human-readable explanation of the fit decision
	FitReason string
}

// RankResult represents a scored and ranked model entry.
type RankResult struct {
	Model ModelEntry
	Score ModelScore
	Rank  int // 1-based position after sorting
}

// FilterOptions provides filtering options for the scorer and TUI.
type FilterOptions struct {
	// Minimum acceptable quality score (0-100)
	MinQuality float64

	// Only show models that fit in VRAM
	VRAMOnly bool

	// Filter by source: "", "huggingface", or "ollama"
	Source string

	// Free-text search applied to DisplayName and Tags
	SearchQuery string

	// Filter by quantization preset, e.g. "Q4_K_M"
	Quantization string
}
