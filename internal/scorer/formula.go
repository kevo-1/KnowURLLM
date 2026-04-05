package scorer

import (
	"fmt"
	"math"
	"strings"

	"github.com/kevo-1/KnowURLLM/internal/hardware"
	"github.com/kevo-1/KnowURLLM/internal/models"
)

const (
	// GB in bytes
	GB = uint64(1_073_741_824)

	// Throughput normalization constant.
	// log2(TPS + 1) * THROUGHPUT_SCALE maps TPS to 0-100 range.
	THROUGHPUT_SCALE = 12.5

	// GPU efficiency factor for bandwidth-based speed estimation.
	GPU_EFFICIENCY_FACTOR = 0.85

	// Parameter-count-based fallback constants (K values per backend).
	K_CUDA   = 30.0
	K_METAL  = 20.0
	K_ROCM   = 18.0
	K_CPU_X86 = 8.0
	K_CPU_ARM = 10.0

	// Mode multipliers for parameter-count fallback.
	MODE_CPUGPU = 0.5
	MODE_CPU    = 0.3
	MODE_MOE    = 0.8
)

// ──────────────────────────────────────────────
// Run mode detection
// ──────────────────────────────────────────────

type RunMode int

const (
	RunModeGPU RunMode = iota // model fits in VRAM
	RunModeMoE                // MoE: active experts in VRAM, inactive in RAM
	RunModeCPUGPU             // spills from VRAM into system RAM
	RunModeCPU                // no GPU, system RAM only
)

// detectRunMode determines how the model will run based on available memory.
// For MoE models, uses ActiveParams (not total params) for VRAM fit check.
func detectRunMode(modelSizeBytes uint64, vramBytes uint64, totalRAM uint64, isMoE bool, activeParams uint64) RunMode {
	totalMem := vramBytes + totalRAM

	// MoE check: if MoE and active params fit in VRAM
	if isMoE && activeParams > 0 {
		// Estimate active expert size as proportion of total model size
		// activeSizeBytes = modelSizeBytes * (activeParams / totalParams)
		// We approximate total params from model size at Q4 baseline
		totalParamsEstimate := uint64(float64(modelSizeBytes) / 0.563)
		if totalParamsEstimate > 0 {
			activeSizeBytes := modelSizeBytes * activeParams / totalParamsEstimate
			if activeSizeBytes <= vramBytes && modelSizeBytes <= totalMem {
				return RunModeMoE
			}
		}
	}

	// Check if model fits in VRAM alone
	if modelSizeBytes <= vramBytes {
		return RunModeGPU
	}

	// Check if model fits in combined VRAM + RAM
	if modelSizeBytes <= totalMem {
		if vramBytes > 0 {
			return RunModeCPUGPU
		}
		return RunModeCPU
	}

	// Doesn't fit at all
	return RunModeCPU
}

// ──────────────────────────────────────────────
// Fit tier classification
// ──────────────────────────────────────────────

type FitTier int

const (
	FitPerfect FitTier = iota // GPU, recommended memory met with headroom
	FitGood                   // fits but tight, or best possible for MoE/CPU+GPU
	FitMarginal               // CPU-only, or very tight
	FitTooTight               // excluded from results
)

// fitTierAndScore returns the fit tier and its discrete score value.
// Perfect=100, Good=75, Marginal=40, TooTight=0.
func fitTierAndScore(mode RunMode, modelSizeBytes uint64, vramBytes uint64, totalRAM uint64) (FitTier, float64) {
	totalMem := vramBytes + totalRAM

	// Doesn't fit at all — excluded
	if modelSizeBytes > totalMem {
		return FitTooTight, 0
	}

	switch mode {
	case RunModeGPU:
		// Check headroom: ≥10% headroom = Perfect, otherwise Good
		headroom := float64(vramBytes-modelSizeBytes) / float64(modelSizeBytes)
		if headroom >= 0.10 {
			return FitPerfect, 100
		}
		return FitGood, 75

	case RunModeMoE:
		return FitGood, 75

	case RunModeCPUGPU:
		return FitGood, 75

	case RunModeCPU:
		// CPU always caps at Marginal regardless of fit
		return FitMarginal, 40

	default:
		return FitTooTight, 0
	}
}

// fitTierLabel returns a human-readable explanation of the fit decision.
func fitTierLabel(tier FitTier, mode RunMode, modelSizeBytes uint64, vramBytes uint64) string {
	modelGB := float64(modelSizeBytes) / float64(GB)
	vramGB := float64(vramBytes) / float64(GB)

	switch tier {
	case FitPerfect:
		headroom := (vramGB - modelGB) / modelGB * 100
		return fmt.Sprintf("Fits in VRAM (%.1f GB) with %.0f%% headroom", modelGB, headroom)
	case FitGood:
		switch mode {
		case RunModeMoE:
			return "MoE: active experts fit in VRAM"
		case RunModeCPUGPU:
			return "Spills from VRAM into system RAM"
		default:
			headroom := (vramGB - modelGB) / modelGB * 100
			return fmt.Sprintf("Fits in VRAM with %.0f%% headroom (tight)", headroom)
		}
	case FitMarginal:
		return "CPU-only inference"
	case FitTooTight:
		return "Too Tight — excluded"
	default:
		return "Unknown"
	}
}

// ──────────────────────────────────────────────
// Dynamic quantization selection
// ──────────────────────────────────────────────

// quantLevels lists quantization presets from best quality to most compressed.
// Each entry has a name and bytes-per-parameter multiplier.
var quantLevels = []struct {
	Name         string
	BytesPerParam float64
}{
	{"Q8_0", 1.000},
	{"Q6_K", 0.750},
	{"Q5_K_M", 0.688},
	{"Q4_K_M", 0.563},
	{"Q3_K_M", 0.438},
	{"Q2_K", 0.313},
}

// selectBestQuant walks from best quality to most compressed and picks the
// highest quantization that fits in available memory.
// Returns empty string if nothing fits.
func selectBestQuant(modelSizeBytes uint64, availableMem uint64) string {
	// Estimate param count from model size at Q4 baseline (0.563 bytes/param)
	// params ≈ modelSizeBytes / 0.563
	// For other quants: size_at_quant = params * bytes_per_param
	paramsEstimate := float64(modelSizeBytes) / 0.563

	for _, q := range quantLevels {
		sizeAtQuant := uint64(paramsEstimate * q.BytesPerParam)
		if sizeAtQuant <= availableMem {
			return q.Name
		}
	}
	return ""
}

// sizeForQuant returns the estimated model size at a given quantization level.
func sizeForQuant(baseModelSizeBytes uint64, quant string) uint64 {
	// Find the multiplier for the given quant relative to Q4_K_M baseline
	var quantMult float64
	for _, q := range quantLevels {
		if strings.EqualFold(q.Name, quant) {
			quantMult = q.BytesPerParam / 0.563 // normalize to Q4 baseline
			return uint64(float64(baseModelSizeBytes) * quantMult)
		}
	}
	// Unknown quant — return as-is
	return baseModelSizeBytes
}

// ──────────────────────────────────────────────
// Speed estimation
// ──────────────────────────────────────────────

// speedScore returns (score 0-100, estimated TPS).
// Uses bandwidth-based estimation when GPU is known, falls back to param-count method.
func speedScore(hw models.HardwareProfile, modelSizeBytes uint64, mode RunMode, quant string) (float64, float64) {
	if modelSizeBytes == 0 {
		return 0, 0
	}

	modelSizeGB := float64(modelSizeBytes) / float64(GB)
	var estimatedTPS float64

	// Primary path: bandwidth-based (when GPU is known)
	bandwidth, hasGPU := detectGPUBandwidth(hw)
	if hasGPU && bandwidth > 0 {
		estimatedTPS = (bandwidth / modelSizeGB) * GPU_EFFICIENCY_FACTOR
	} else {
		// Fallback: parameter-count-based
		k := kForBackend(hw)
		// Estimate params in billions from model size at Q4 baseline
		paramsBillions := float64(modelSizeBytes) / 0.563 / 1e9
		if paramsBillions <= 0 {
			paramsBillions = 1 // avoid division by zero
		}
		estimatedTPS = k / paramsBillions

		// Quant speed multiplier
		estimatedTPS *= quantSpeedMultiplier(quant)

		// Mode penalty
		switch mode {
		case RunModeCPUGPU:
			estimatedTPS *= MODE_CPUGPU
		case RunModeCPU:
			estimatedTPS *= MODE_CPU
		case RunModeMoE:
			estimatedTPS *= MODE_MOE
		}
	}

	if estimatedTPS <= 0 {
		return 0, 0
	}

	// Normalize to 0-100 using log scale
	score := math.Min(100, math.Log2(estimatedTPS+1)*THROUGHPUT_SCALE)
	return score, estimatedTPS
}

// detectGPUBandwidth returns the best GPU bandwidth from the hardware profile.
func detectGPUBandwidth(hw models.HardwareProfile) (float64, bool) {
	if len(hw.GPUs) == 0 {
		return 0, false
	}

	var bestBW float64
	var found bool
	for _, gpu := range hw.GPUs {
		if bw, ok := hardware.LookupBandwidth(gpu.Model); ok {
			if !found || bw > bestBW {
				bestBW = bw
				found = true
			}
		}
	}
	return bestBW, found
}

// kForBackend returns the K constant for the parameter-count fallback.
func kForBackend(hw models.HardwareProfile) float64 {
	// Check GPU first
	for _, gpu := range hw.GPUs {
		vendor := strings.ToLower(gpu.Vendor)
		switch vendor {
		case "nvidia":
			return K_CUDA
		case "amd":
			return K_ROCM
		case "apple":
			return K_METAL
		case "intel":
			return K_CUDA // SYCL similar to CUDA
		}
	}

	// CPU fallback
	cpuModel := strings.ToLower(hw.CPUModel)
	if strings.Contains(cpuModel, "arm") || strings.Contains(cpuModel, "apple") {
		return K_CPU_ARM
	}
	return K_CPU_X86
}

// quantSpeedMultiplier returns a speed factor for the param-count fallback.
// Higher quants are slower (lower multiplier).
func quantSpeedMultiplier(quant string) float64 {
	switch strings.ToUpper(quant) {
	case "Q2_K":
		return 1.6
	case "Q3_K", "Q3_K_M", "Q3_K_S", "Q3_K_L":
		return 1.4
	case "Q4_K_M", "Q4_K", "Q4_K_S":
		return 1.0
	case "Q5_K_M", "Q5_K", "Q5_K_S":
		return 0.85
	case "Q6_K", "Q6_K_M":
		return 0.75
	case "Q8_0":
		return 0.65
	case "FP16", "F16":
		return 0.5
	case "FP32", "F32":
		return 0.3
	default:
		return 1.0
	}
}

// ──────────────────────────────────────────────
// Context score (4th dimension)
// ──────────────────────────────────────────────

func contextScore(contextLength int) float64 {
	switch {
	case contextLength >= 128000:
		return 100
	case contextLength >= 64000:
		return 85
	case contextLength >= 32000:
		return 70
	case contextLength >= 16000:
		return 55
	case contextLength >= 8000:
		return 40
	default:
		return 20
	}
}

// ──────────────────────────────────────────────
// Quality score
// ──────────────────────────────────────────────

func qualityScore(mmluScore float64, arenaELO float64) float64 {
	if mmluScore > 0 && arenaELO > 0 {
		normalizedELO := (arenaELO - 800) / (1300 - 800) * 100
		return 0.6*mmluScore + 0.4*normalizedELO
	} else if mmluScore > 0 {
		return mmluScore
	} else if arenaELO > 0 {
		normalizedELO := (arenaELO - 800) / (1300 - 800) * 100
		return normalizedELO
	}
	return 50 // neutral default
}

// ──────────────────────────────────────────────
// Use-case weights
// ──────────────────────────────────────────────

type UseCaseWeights struct {
	Quality float64
	Speed   float64
	Fit     float64
	Context float64
}

func weightsForUseCase(useCase string) UseCaseWeights {
	switch strings.ToLower(useCase) {
	case "coding":
		return UseCaseWeights{0.45, 0.25, 0.20, 0.10}
	case "reasoning":
		return UseCaseWeights{0.55, 0.15, 0.20, 0.10}
	case "chat":
		return UseCaseWeights{0.25, 0.35, 0.25, 0.15}
	case "multimodal":
		return UseCaseWeights{0.40, 0.20, 0.30, 0.10}
	case "embedding":
		return UseCaseWeights{0.50, 0.30, 0.20, 0.00}
	default: // "general"
		return UseCaseWeights{0.35, 0.25, 0.25, 0.15}
	}
}

// ──────────────────────────────────────────────
// Helper: total VRAM
// ──────────────────────────────────────────────

func totalVRAM(hw models.HardwareProfile) uint64 {
	var vram uint64
	for _, gpu := range hw.GPUs {
		vram += gpu.VRAM
	}
	return vram
}

// ──────────────────────────────────────────────
// Wire it together
// ──────────────────────────────────────────────

// scoreModel computes the full ModelScore for a single ModelEntry.
// Returns (modelScore, excluded).
func scoreModel(hw models.HardwareProfile, entry models.ModelEntry) (models.ModelScore, bool) {
	vram := totalVRAM(hw)

	// Step 1: Select best quantization that fits available memory
	availableMem := vram + hw.TotalRAM
	bestQuant := selectBestQuant(entry.ModelSizeBytes, availableMem)

	// Half-context fallback if nothing fits
	if bestQuant == "" {
		availableMem = availableMem / 2
		bestQuant = selectBestQuant(entry.ModelSizeBytes, availableMem)
	}

	if bestQuant == "" {
		return models.ModelScore{
			FitCategory:   "Too Tight",
			FitReason:     "Too Tight — excluded",
			SelectedQuant: "",
		}, true
	}

	// Step 2: Compute model size at selected quant
	modelSizeBytes := sizeForQuant(entry.ModelSizeBytes, bestQuant)

	// Step 3: Detect run mode (use ActiveParams for MoE)
	mode := detectRunMode(modelSizeBytes, vram, hw.TotalRAM, entry.IsMoE, entry.ActiveParams)

	// Step 4: Classify fit tier
	tier, fitScore := fitTierAndScore(mode, modelSizeBytes, vram, hw.TotalRAM)
	if tier == FitTooTight {
		return models.ModelScore{
			FitCategory:   "Too Tight",
			FitReason:     "Too Tight — excluded",
			SelectedQuant: bestQuant,
		}, true
	}

	// Step 5: Speed estimation
	tpsScore, estimatedTPS := speedScore(hw, modelSizeBytes, mode, bestQuant)

	// Step 6: Quality score
	quality := qualityScore(entry.MMLUScore, entry.ArenaELO)

	// Step 7: Context score
	ctx := contextScore(entry.ContextLength)

	// Step 8: Use-case weighted total
	w := weightsForUseCase(entry.UseCase)
	total := w.Quality*quality + w.Speed*tpsScore + w.Fit*fitScore + w.Context*ctx

	return models.ModelScore{
		TotalScore:       total,
		HardwareFitScore: fitScore,
		ThroughputScore:  tpsScore,
		QualityScore:     quality,
		ContextScore:     ctx,
		EstimatedTPS:     estimatedTPS,
		FitsInVRAM:       mode == RunModeGPU || mode == RunModeMoE,
		FitsInMemory:     true,
		FitCategory:      fitTierCategory(tier),
		FitReason:        fitTierLabel(tier, mode, modelSizeBytes, vram),
		SelectedQuant:    bestQuant,
	}, false
}

// fitTierCategory returns the string label for a fit tier.
func fitTierCategory(tier FitTier) string {
	switch tier {
	case FitPerfect:
		return "Perfect"
	case FitGood:
		return "Good"
	case FitMarginal:
		return "Marginal"
	case FitTooTight:
		return "Too Tight"
	default:
		return "Unknown"
	}
}
