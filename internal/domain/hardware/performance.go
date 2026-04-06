// Package hardware provides performance estimation for models on hardware.
package hardware

import (
	"math"
	"strings"

	"github.com/kevo-1/KnowURLLM/internal/domain"
)

const (
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

// EstimatePerformance estimates the performance of a model on given hardware.
// Returns estimated TPS and performance score (0-100).
func EstimatePerformance(
	hw domain.HardwareProfile,
	modelSizeBytes uint64,
	mode domain.RunMode,
	quant string,
) (float64, float64) {
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
		paramsBillions := float64(modelSizeBytes) / 0.563 / 1e9
		if paramsBillions <= 0 {
			paramsBillions = 1
		}
		estimatedTPS = k / paramsBillions

		// Quant speed multiplier
		estimatedTPS *= quantSpeedMultiplier(quant)

		// Mode penalty
		switch mode {
		case domain.RunModeCPUGPU:
			estimatedTPS *= MODE_CPUGPU
		case domain.RunModeCPU:
			estimatedTPS *= MODE_CPU
		case domain.RunModeMoE:
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
func detectGPUBandwidth(hw domain.HardwareProfile) (float64, bool) {
	if len(hw.GPUs) == 0 {
		return 0, false
	}

	var bestBW float64
	var found bool
	for _, gpu := range hw.GPUs {
		if bw, ok := LookupBandwidth(gpu.Model); ok {
			if !found || bw > bestBW {
				bestBW = bw
				found = true
			}
		}
	}
	return bestBW, found
}

// kForBackend returns the K constant for the parameter-count fallback.
func kForBackend(hw domain.HardwareProfile) float64 {
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
			return K_CUDA
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
