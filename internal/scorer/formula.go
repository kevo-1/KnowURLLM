package scorer

import (
	"fmt"
	"math"
	"strings"

	"github.com/KnowURLLM/internal/models"
)

const (
	// GPU_TPS_PER_GB is the baseline tokens/sec per GB of VRAM for Q4 models.
	GPU_TPS_PER_GB = 80.0

	// CPU_TPS_PER_CORE is the baseline tokens/sec per core for Q4 models.
	CPU_TPS_PER_CORE = 3.0

	// GB in bytes
	GB = uint64(1_073_741_824)

	// Throughput normalization constant.
	// log2(TPS + 1) * THROUGHPUT_SCALE maps TPS to 0-100 range.
	THROUGHPUT_SCALE = 12.5
)

// quantFactor returns the quantization multiplier for throughput estimation.
func quantFactor(quant string) float64 {
	switch strings.ToUpper(quant) {
	case "Q2_K":
		return 0.6
	case "Q3_K", "Q3_K_M", "Q3_K_S", "Q3_K_L":
		return 0.7
	case "Q4_K_M", "Q4_K", "Q4_K_S":
		return 1.0
	case "Q5_K_M", "Q5_K", "Q5_K_S":
		return 0.85
	case "Q8_0":
		return 0.65
	case "FP16", "F16":
		return 0.4
	case "FP32", "F32":
		return 0.2
	default:
		// Unknown quantization → assume Q4 baseline
		return 1.0
	}
}

// totalVRAM sums VRAM across all GPUs in the hardware profile.
func totalVRAM(hw models.HardwareProfile) uint64 {
	var vram uint64
	for _, gpu := range hw.GPUs {
		vram += gpu.VRAM
	}
	return vram
}

// hardwareFitScore calculates the hardware fit sub-score (0-100).
// Returns (score, fitsInVRAM, fitsInMemory, reason, excluded).
// If excluded is true, the model should be dropped from results.
func hardwareFitScore(modelSizeBytes uint64, totalVRAMBytes uint64, totalRAM uint64) (float64, bool, bool, string, bool) {
	// Handle zero-size models — exclude them
	if modelSizeBytes == 0 {
		return 0, false, false, "Does not fit — excluded", true
	}

	availableMem := float64(totalVRAMBytes) + float64(totalRAM)*0.5
	fitRatio := availableMem / float64(modelSizeBytes)

	fitsInVRAM := modelSizeBytes <= totalVRAMBytes
	fitsInMemory := float64(modelSizeBytes) <= availableMem

	var score float64
	var reason string
	excluded := false

	if fitRatio < 0.7 {
		excluded = true
		score = 0
		reason = "Does not fit — excluded"
	} else if fitRatio >= 1.3 {
		score = 100
		if fitsInVRAM {
			reason = fmt.Sprintf("Fits in VRAM with %.0f%% headroom", (fitRatio-1)*100)
		} else {
			reason = fmt.Sprintf("Fits in VRAM+RAM (%.1f GB RAM overflow)", (float64(modelSizeBytes)-float64(totalVRAMBytes))/float64(GB))
		}
	} else if fitRatio >= 1.0 {
		score = 80 + (fitRatio-1.0)*100
		if fitsInVRAM {
			reason = fmt.Sprintf("Fits in VRAM with %.0f%% headroom", (fitRatio-1)*100)
		} else {
			overflowGB := (float64(modelSizeBytes) - float64(totalVRAMBytes)) / float64(GB)
			reason = fmt.Sprintf("Fits in VRAM+RAM (%.1f GB RAM overflow)", overflowGB)
		}
	} else {
		// fitRatio >= 0.7 && fitRatio < 1.0
		score = 50 + fitRatio*30
		overflowGB := (float64(modelSizeBytes) - float64(totalVRAMBytes)) / float64(GB)
		reason = fmt.Sprintf("Fits in VRAM+RAM (%.1f GB RAM overflow)", overflowGB)
	}

	return score, fitsInVRAM, fitsInMemory, reason, excluded
}

// throughputScore calculates the throughput sub-score (0-100) and estimated TPS.
// Returns (throughputScore, estimatedTPS).
func throughputScore(modelSizeBytes uint64, fitsInVRAM bool, totalVRAMBytes uint64, cpuCores int) (float64, float64) {
	// Handle zero-size models
	if modelSizeBytes == 0 {
		return 0, 0
	}

	modelSizeGB := float64(modelSizeBytes) / float64(GB)
	qf := 1.0 // baseline quant factor (Q4) — caller should override via quantization

	var baseTPS float64

	if fitsInVRAM && totalVRAMBytes > 0 {
		// GPU inference
		vramGB := float64(totalVRAMBytes) / float64(GB)
		baseTPS = (vramGB * GPU_TPS_PER_GB) / (modelSizeGB * qf)
	} else {
		// CPU inference
		if cpuCores > 0 {
			baseTPS = (float64(cpuCores) * CPU_TPS_PER_CORE) / (modelSizeGB * qf)
		} else {
			baseTPS = 0
		}
	}

	// Normalize with log scale
	throughputScore := math.Min(100, math.Log2(baseTPS+1)*THROUGHPUT_SCALE)

	return throughputScore, baseTPS
}

// throughputScoreWithQuant calculates throughput with the actual quantization factor.
func throughputScoreWithQuant(modelSizeBytes uint64, fitsInVRAM bool, totalVRAMBytes uint64, cpuCores int, quant string) (float64, float64) {
	if modelSizeBytes == 0 {
		return 0, 0
	}

	modelSizeGB := float64(modelSizeBytes) / float64(GB)
	qf := quantFactor(quant)

	var baseTPS float64

	if fitsInVRAM && totalVRAMBytes > 0 {
		// GPU inference
		vramGB := float64(totalVRAMBytes) / float64(GB)
		baseTPS = (vramGB * GPU_TPS_PER_GB) / (modelSizeGB * qf)
	} else {
		// CPU inference
		if cpuCores > 0 {
			baseTPS = (float64(cpuCores) * CPU_TPS_PER_CORE) / (modelSizeGB * qf)
		} else {
			baseTPS = 0
		}
	}

	// Normalize with log scale
	throughputScore := math.Min(100, math.Log2(baseTPS+1)*THROUGHPUT_SCALE)

	return throughputScore, baseTPS
}

// qualityScore calculates the quality sub-score (0-100).
func qualityScore(mmluScore float64, arenaELO float64) float64 {
	if mmluScore > 0 && arenaELO > 0 {
		normalizedELO := (arenaELO - 800) / (1300 - 800) * 100
		return 0.6*mmluScore + 0.4*normalizedELO
	} else if mmluScore > 0 {
		return mmluScore
	} else if arenaELO > 0 {
		normalizedELO := (arenaELO - 800) / (1300 - 800) * 100
		return normalizedELO
	} else {
		return 50 // neutral default
	}
}

// scoreModel computes the full ModelScore for a single ModelEntry.
// Returns (modelScore, excluded).
func scoreModel(hw models.HardwareProfile, entry models.ModelEntry) (models.ModelScore, bool) {
	vram := totalVRAM(hw)

	// Calculate hardware fit
	hwFitScore, fitsVRAM, fitsMem, reason, excluded := hardwareFitScore(
		entry.ModelSizeBytes,
		vram,
		hw.TotalRAM,
	)

	if excluded {
		return models.ModelScore{
			HardwareFitScore: hwFitScore,
			FitsInVRAM:       fitsVRAM,
			FitsInMemory:     fitsMem,
			FitReason:        reason,
		}, true
	}

	// Calculate throughput with actual quantization
	tpsScore, estimatedTPS := throughputScoreWithQuant(
		entry.ModelSizeBytes,
		fitsVRAM,
		vram,
		hw.CPUCores,
		entry.Quantization,
	)

	// Calculate quality
	quality := qualityScore(entry.MMLUScore, entry.ArenaELO)

	// Compute weighted total
	s := &Scorer{
		HardwareFitWeight: 0.50,
		ThroughputWeight:  0.30,
		QualityWeight:     0.20,
	}

	totalScore := s.HardwareFitWeight*hwFitScore +
		s.ThroughputWeight*tpsScore +
		s.QualityWeight*quality

	return models.ModelScore{
		TotalScore:       totalScore,
		HardwareFitScore: hwFitScore,
		ThroughputScore:  tpsScore,
		QualityScore:     quality,
		EstimatedTPS:     estimatedTPS,
		FitsInVRAM:       fitsVRAM,
		FitsInMemory:     fitsMem,
		FitReason:        reason,
	}, false
}
