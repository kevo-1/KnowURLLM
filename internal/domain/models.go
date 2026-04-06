// Package domain defines all shared data structures used across the KnowURLLM modules.
// This is the contract layer — every other module imports domain/ and nothing else
// imports domain/ from a sibling module.
package domain

// HardwareProfile contains detected system hardware information.
type HardwareProfile struct {
	// CPU brand and model string, e.g. "Apple M2 Max", "AMD Ryzen 9 7950X"
	CPUModel string

	// Total system RAM in bytes
	TotalRAM uint64

	// Available system RAM in bytes (not used by other processes at detection time)
	AvailableRAM uint64

	// Number of logical CPU cores
	CPUCores int

	// Detected GPUs
	GPUs []GPUInfo

	// Total VRAM across all GPUs in bytes (sum of all GPU.VRAM)
	TotalVRAM uint64

	// Available VRAM in bytes (accounting for OS reservations, display output, etc.)
	// On Apple Silicon, this is typically ~75% of TotalVRAM due to OS reservation
	AvailableVRAM uint64

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

	// IFEval (Instruction Following) score (0-100), 0 if unavailable
	IFEvalScore float64

	// GSM8K (math reasoning) score (0-100), 0 if unavailable
	GSM8KScore float64

	// ARC-Challenge score (0-100), 0 if unavailable
	ARCScore float64

	// Hugging Face download count (popularity signal)
	Downloads int

	// Raw URL to the model card / page
	URL string

	// Tags from the registry, e.g. ["text-generation", "conversational"]
	Tags []string

	// Total parameter count (for quality scoring)
	ParameterCount uint64

	// True if this is a Mixture-of-Experts model
	IsMoE bool

	// Active parameters per token (for MoE models)
	ActiveParams uint64

	// Use case for scoring weights, e.g. "coding", "reasoning", "chat"
	UseCase string
}

// ModelQuality contains Arena-style quality scoring metrics.
type ModelQuality struct {
	// Overall composite quality score (0-100)
	Overall float64

	// Confidence in the quality score (0-1, based on benchmark data availability)
	Confidence float64

	// Percentile rank among all models (0-100)
	Percentile int

	// Quality tier (S, A, B, C, D)
	Tier QualityTier

	// Category-specific scores
	CategoryScores map[string]float64 // "general_chat", "coding", "reasoning", etc.

	// Source benchmark scores (for transparency)
	ArenaELO    float64 // Normalized LMSYS ELO (0-100)
	MMLUPro     float64 // MMLU-PRO score (0-100)
	IFEval      float64 // Instruction following (0-100)
	GSM8K       float64 // Math reasoning (0-100)
	ARC         float64 // Science QA (0-100)
}

// QualityTier represents a quality tier classification.
type QualityTier string

const (
	TierS QualityTier = "S" // Top 5% (95th+ percentile)
	TierA QualityTier = "A" // Top 15% (85-95th percentile)
	TierB QualityTier = "B" // Top 35% (65-85th percentile)
	TierC QualityTier = "C" // Top 60% (40-65th percentile)
	TierD QualityTier = "D" // Bottom 40% (<40th percentile)
)

// ModelHardware contains hardware compatibility metrics for a model.
type ModelHardware struct {
	// Can the model run on this hardware?
	Runnable bool

	// Run mode: GPU, MoE, CPU+GPU, CPU
	Mode RunMode

	// Best quantization that fits
	BestQuant string

	// Model size at selected quantization (bytes)
	SizeAtQuant uint64

	// Memory utilization
	VRAMUsed    uint64
	VRAMUtilPct float64 // 0-1 (sweet spot: 0.5-0.8)
	RAMUsed     uint64
	RAMUtilPct  float64 // 0-1

	// Performance estimates
	EstimatedTPS    float64
	LatencyFirst    float64 // Time to first token (ms)
	LatencyPerToken float64 // Per-token latency (ms)

	// Fit classification
	FitLabel  string // "Perfect", "Good", "Marginal", "Tight"
	FitReason string // Human-readable explanation
}

// RunMode represents how a model will execute on the hardware.
type RunMode int

const (
	RunModeGPU RunMode = iota // Model fits entirely in VRAM
	RunModeMoE                // MoE: active experts in VRAM, inactive in RAM
	RunModeCPUGPU             // Model spills from VRAM into system RAM
	RunModeCPU                // No GPU available, system RAM only
)

// ModelScore contains the computed scoring metrics for a model on specific hardware.
// DEPRECATED: Use ModelQuality + ModelHardware instead. Kept for backward compatibility.
type ModelScore struct {
	// Overall composite score (higher = better)
	TotalScore float64

	// Sub-score: hardware fit (0-100)
	HardwareFitScore float64

	// Sub-score: estimated throughput/speed (0-100)
	ThroughputScore float64

	// Sub-score: model quality from benchmarks (0-100)
	QualityScore float64

	// Sub-score: context window capability (0-100)
	ContextScore float64

	// Estimated tokens/sec the model will achieve on this hardware
	EstimatedTPS float64

	// True if the model fits in VRAM alone (no RAM offload needed)
	FitsInVRAM bool

	// True if the model fits in combined VRAM + system RAM
	FitsInMemory bool

	// Human-readable explanation of the fit decision
	FitReason string

	// Fit category classification
	FitCategory string // "Perfect", "Good", "Marginal", "Too Tight"

	// Selected quantization level that fits (e.g. "Q4_K_M", "Q8_0")
	SelectedQuant string
}

// RankResult represents a scored and ranked model entry.
// DEPRECATED: Use RankedModel instead. Kept for backward compatibility.
type RankResult struct {
	Model ModelEntry
	Score ModelScore
	Rank  int // 1-based position after sorting
}

// RankedModel represents a fully scored and ranked model with quality tiers.
type RankedModel struct {
	Model  ModelEntry
	Quality ModelQuality
	Hardware ModelHardware
	Rank   int // Global rank (1-based)
}

// FilterOptions provides filtering options for the scorer and TUI.
type FilterOptions struct {
	// Minimum acceptable quality score (0-100)
	MinQuality float64

	// Minimum acceptable quality tier (e.g., TierA = only S and A tier)
	MinTier QualityTier

	// Only show models that fit in VRAM
	VRAMOnly bool

	// Filter by source: "", "huggingface", or "ollama"
	Source string

	// Free-text search applied to DisplayName and Tags
	SearchQuery string

	// Filter by quantization preset, e.g. "Q4_K_M"
	Quantization string

	// Use case profile for scoring weights
	UseCaseProfile string // "General", "Coding", "Reasoning", "Chat", "Multimodal", "Embedding"

	// Inference backend override (empty for auto-detect)
	InferenceBackend string // "", "CUDA", "ROCm", "Metal", "SYCL", "CPU"

	// Minimum acceptable TPS
	MinTPS float64

	// Filter by quality category
	MinCategory string // "general_chat", "coding", "reasoning", etc.
	MinCategoryScore float64
}
