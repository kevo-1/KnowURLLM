package hardware

import "strings"

// gpuBandwidth maps GPU model substrings to their memory bandwidth in GB/s.
// Used for physics-based speed estimation.
var gpuBandwidth = map[string]float64{
	// NVIDIA Ada Lovelace
	"RTX 4090":  1008,
	"RTX 4080":  717,
	"RTX 4080 SUPER": 717,
	"RTX 4070":  504,
	"RTX 4060":  276,

	// NVIDIA Ampere
	"RTX 3090":  936,
	"RTX 3090 Ti": 1008,
	"RTX 3080":  760,
	"RTX 3070":  448,
	"RTX 3060":  360,

	// NVIDIA Data Center
	"H100":      3350,
	"H100 SXM":  3350,
	"A100":      2000,
	"A100 80GB": 2000,
	"A6000":     768,
	"A40":       696,

	// Apple Silicon (Unified Memory)
	"M3 Max": 400,
	"M3 Pro": 150,
	"M2 Max": 400,
	"M2 Pro": 200,
	"M1 Max": 400,
	"M1 Pro": 200,

	// AMD RDNA 3
	"RX 7900 XTX": 960,
	"RX 7900 XT":  800,
	"RX 7800 XT":  624,
}

// LookupBandwidth returns the memory bandwidth in GB/s for a GPU model.
// It performs a case-insensitive substring match against known GPUs.
// Returns (bandwidth_GB_s, found).
func LookupBandwidth(gpuModel string) (float64, bool) {
	upper := strings.ToUpper(gpuModel)
	for key, bw := range gpuBandwidth {
		if strings.Contains(upper, strings.ToUpper(key)) {
			return bw, true
		}
	}
	return 0, false
}
