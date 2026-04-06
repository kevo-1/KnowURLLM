// Package hardware provides GPU bandwidth lookup for performance estimation.
package hardware

import (
	"sort"
	"strings"
	"sync"
)

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

	// NVIDIA Turing (GTX 16-series)
	"GTX 1660 SUPER": 336,
	"GTX 1660":     192,
	"GTX 1650 SUPER": 192,
	"GTX 1650":     128,

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

// sortedBandwidthKeys holds GPU model keys sorted by length descending,
// so that longer (more specific) matches are checked first.
var sortedBandwidthKeys []string
var bandwidthKeysOnce sync.Once

// LookupBandwidth returns the memory bandwidth in GB/s for a GPU model.
// It performs a case-insensitive substring match against known GPUs,
// preferring longer (more specific) matches over shorter ones.
// Returns (bandwidth_GB_s, found).
func LookupBandwidth(gpuModel string) (float64, bool) {
	upper := strings.ToUpper(gpuModel)

	// Initialize sorted keys once (thread-safe)
	bandwidthKeysOnce.Do(func() {
		keys := make([]string, 0, len(gpuBandwidth))
		for k := range gpuBandwidth {
			keys = append(keys, k)
		}
		// Sort by length descending so longer matches are checked first
		sort.Slice(keys, func(i, j int) bool {
			return len(keys[i]) > len(keys[j])
		})
		sortedBandwidthKeys = keys
	})

	for _, key := range sortedBandwidthKeys {
		if strings.Contains(upper, strings.ToUpper(key)) {
			return gpuBandwidth[key], true
		}
	}
	return 0, false
}
