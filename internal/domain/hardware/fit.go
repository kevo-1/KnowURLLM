// Package hardware provides hardware compatibility checking.
package hardware

import (
	"fmt"
	"strings"

	"github.com/kevo-1/KnowURLLM/internal/domain"
)

const (
	// GB in bytes
	GB = uint64(1_073_741_824)
)

// quantLevels lists quantization presets from best quality to most compressed.
// Each entry has a name and bytes-per-parameter multiplier.
var quantLevels = []struct {
	Name          string
	BytesPerParam float64
}{
	{"Q8_0", 1.000},
	{"Q6_K", 0.750},
	{"Q5_K_M", 0.688},
	{"Q4_K_M", 0.563},
	{"Q3_K_M", 0.438},
	{"Q2_K", 0.313},
}

// CheckCompatibility checks if a model can run on the given hardware.
// Returns hardware compatibility information.
func CheckCompatibility(hw domain.HardwareProfile, entry domain.ModelEntry) domain.ModelHardware {
	vram := totalVRAM(hw)
	totalMem := vram + hw.TotalRAM

	// Step 1: Use existing quantization or select best
	var bestQuant string
	var modelSizeBytes uint64

	if entry.Quantization != "" {
		// Model already has quantization - use it directly
		bestQuant = entry.Quantization
		modelSizeBytes = entry.ModelSizeBytes
	} else {
		// Select best quantization that fits available memory
		bestQuant = selectBestQuant(entry.ModelSizeBytes, totalMem)

		// Half-context fallback if nothing fits
		if bestQuant == "" {
			bestQuant = selectBestQuant(entry.ModelSizeBytes, totalMem/2)
		}

		if bestQuant == "" {
			return domain.ModelHardware{
				Runnable:  false,
				FitLabel:  "Too Tight",
				FitReason: "Model too large for available memory at any quantization",
			}
		}

		// Compute model size at selected quant
		modelSizeBytes = sizeForQuant(entry.ModelSizeBytes, bestQuant)
	}

	// Step 3: Detect run mode
	mode := detectRunMode(modelSizeBytes, vram, hw.TotalRAM, entry.IsMoE, entry.ActiveParams, bestQuant)

	// Step 4: Classify fit tier
	tier, fitLabel := fitTierAndScore(mode, modelSizeBytes, vram, hw.TotalRAM)
	if tier == fitTooTight {
		return domain.ModelHardware{
			Runnable:    false,
			BestQuant:   bestQuant,
			SizeAtQuant: modelSizeBytes,
			FitLabel:    "Too Tight",
			FitReason:   "Too Tight — excluded",
		}
	}

	// Step 5: Calculate memory utilization
	var vramUsed, ramUsed uint64
	var vramUtilPct, ramUtilPct float64

	switch mode {
	case domain.RunModeGPU:
		vramUsed = modelSizeBytes
		if vram > 0 {
			vramUtilPct = float64(vramUsed) / float64(vram)
		}
	case domain.RunModeMoE:
		// MoE: active params in VRAM, rest in RAM
		bytesPerParam := bytesPerParamForQuant(bestQuant)
		if entry.ActiveParams > 0 && bytesPerParam > 0 {
			activeSizeBytes := uint64(float64(entry.ActiveParams) * bytesPerParam)
			vramUsed = activeSizeBytes
			ramUsed = modelSizeBytes - activeSizeBytes
		}
		if vram > 0 {
			vramUtilPct = float64(vramUsed) / float64(vram)
		}
		if hw.TotalRAM > 0 {
			ramUtilPct = float64(ramUsed) / float64(hw.TotalRAM)
		}
	case domain.RunModeCPUGPU:
		// Spills from VRAM to RAM
		vramUsed = vram
		ramUsed = modelSizeBytes - vram
		if vram > 0 {
			vramUtilPct = 1.0 // Full VRAM used
		}
		if hw.TotalRAM > 0 {
			ramUtilPct = float64(ramUsed) / float64(hw.TotalRAM)
		}
	case domain.RunModeCPU:
		ramUsed = modelSizeBytes
		if hw.TotalRAM > 0 {
			ramUtilPct = float64(ramUsed) / float64(hw.TotalRAM)
		}
	}

	return domain.ModelHardware{
		Runnable:    true,
		Mode:        mode,
		BestQuant:   bestQuant,
		SizeAtQuant: modelSizeBytes,
		VRAMUsed:    vramUsed,
		VRAMUtilPct: vramUtilPct,
		RAMUsed:     ramUsed,
		RAMUtilPct:  ramUtilPct,
		FitLabel:    fitLabel,
		FitReason:   fitTierLabel(tier, mode, modelSizeBytes, vram),
	}
}

// detectRunMode determines how the model will run based on available memory.
func detectRunMode(modelSizeBytes uint64, vramBytes uint64, totalRAM uint64, isMoE bool, activeParams uint64, quant string) domain.RunMode {
	totalMem := vramBytes + totalRAM

	// MoE check: if MoE and active params fit in VRAM
	if isMoE && activeParams > 0 {
		bytesPerParam := bytesPerParamForQuant(quant)
		if bytesPerParam > 0 {
			activeSizeBytes := uint64(float64(activeParams) * bytesPerParam)
			if activeSizeBytes <= vramBytes && modelSizeBytes <= totalMem {
				return domain.RunModeMoE
			}
		}
	}

	// Check if model fits in VRAM alone
	if modelSizeBytes <= vramBytes {
		return domain.RunModeGPU
	}

	// Check if model fits in combined VRAM + RAM
	if modelSizeBytes <= totalMem {
		if vramBytes > 0 {
			return domain.RunModeCPUGPU
		}
		return domain.RunModeCPU
	}

	// Doesn't fit at all
	return domain.RunModeCPU
}

// fitTier classification
type fitTier int

const (
	fitPerfect fitTier = iota
	fitGood
	fitMarginal
	fitTooTight
)

// fitTierAndScore returns the fit tier and its discrete score value.
func fitTierAndScore(mode domain.RunMode, modelSizeBytes uint64, vramBytes uint64, totalRAM uint64) (fitTier, string) {
	totalMem := vramBytes + totalRAM

	// Doesn't fit at all — excluded
	if modelSizeBytes > totalMem {
		return fitTooTight, "Too Tight"
	}

	switch mode {
	case domain.RunModeGPU:
		headroom := float64(vramBytes-modelSizeBytes) / float64(modelSizeBytes)
		if headroom >= 0.10 {
			return fitPerfect, "Perfect"
		}
		return fitGood, "Good"

	case domain.RunModeMoE:
		return fitGood, "Good"

	case domain.RunModeCPUGPU:
		return fitGood, "Good"

	case domain.RunModeCPU:
		return fitMarginal, "Marginal"

	default:
		return fitTooTight, "Too Tight"
	}
}

// fitTierLabel returns a human-readable explanation of the fit decision.
func fitTierLabel(tier fitTier, mode domain.RunMode, modelSizeBytes uint64, vramBytes uint64) string {
	modelGB := float64(modelSizeBytes) / float64(GB)
	vramGB := float64(vramBytes) / float64(GB)

	switch tier {
	case fitPerfect:
		headroom := (vramGB - modelGB) / modelGB * 100
		return fmt.Sprintf("Fits in VRAM (%.1f GB) with %.0f%% headroom", modelGB, headroom)
	case fitGood:
		switch mode {
		case domain.RunModeMoE:
			return "MoE: active experts fit in VRAM"
		case domain.RunModeCPUGPU:
			return "Spills from VRAM into system RAM"
		default:
			headroom := (vramGB - modelGB) / modelGB * 100
			return fmt.Sprintf("Fits in VRAM with %.0f%% headroom (tight)", headroom)
		}
	case fitMarginal:
		return "CPU-only inference"
	case fitTooTight:
		return "Too Tight — excluded"
	default:
		return "Unknown"
	}
}

// bytesPerParamForQuant returns the bytes-per-parameter for a given quantization.
// Returns 0.563 (Q4_K_M) as default for unknown quantizations.
func bytesPerParamForQuant(quant string) float64 {
	quantUpper := strings.ToUpper(quant)
	for _, q := range quantLevels {
		if strings.ToUpper(q.Name) == quantUpper {
			return q.BytesPerParam
		}
	}
	// Default to Q4_K_M
	return 0.563
}

// selectBestQuant walks from best quality to most compressed and picks the
// highest quantization that fits in available memory.
func selectBestQuant(modelSizeBytes uint64, availableMem uint64) string {
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
	var quantMult float64
	for _, q := range quantLevels {
		if strings.EqualFold(q.Name, quant) {
			quantMult = q.BytesPerParam / 0.563
			return uint64(float64(baseModelSizeBytes) * quantMult)
		}
	}
	return baseModelSizeBytes
}

// totalVRAM calculates total VRAM from all GPUs.
func totalVRAM(hw domain.HardwareProfile) uint64 {
	var vram uint64
	for _, gpu := range hw.GPUs {
		vram += gpu.VRAM
	}
	return vram
}
