package registry

import (
	"embed"
	"encoding/json"
	"regexp"
	"strings"
)

//go:embed data/benchmarks.json
var benchmarksFS embed.FS

// benchmarksManifest is the top-level structure of benchmarks.json.
type benchmarksManifest struct {
	Version string                      `json:"version"`
	Sources []string                    `json:"sources"`
	Models  map[string]benchmarkEntry   `json:"models"`
}

// benchmarkEntry holds a single model's benchmark scores.
type benchmarkEntry struct {
	MMLU     float64 `json:"mmlu"`
	ArenaELO float64 `json:"arena_elo"`
}

// benchmarkData holds known benchmark scores for a model.
type benchmarkData struct {
	MMLU     float64
	ArenaELO float64
}

// loadBenchmarks reads the embedded benchmarks.json and returns a lookup map.
func loadBenchmarks() map[string]benchmarkData {
	data, err := benchmarksFS.ReadFile("data/benchmarks.json")
	if err != nil {
		// Fallback to empty — models won't be enriched but won't crash
		return map[string]benchmarkData{}
	}

	var manifest benchmarksManifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		return map[string]benchmarkData{}
	}

	result := make(map[string]benchmarkData, len(manifest.Models))
	for id, entry := range manifest.Models {
		result[strings.ToLower(id)] = benchmarkData{
			MMLU:     entry.MMLU,
			ArenaELO: entry.ArenaELO,
		}
	}
	return result
}

// knownBenchmarks is populated at init from the embedded JSON.
var knownBenchmarks = loadBenchmarks()

// lookupBenchmarks finds MMLU and Arena ELO scores for a model by name.
// Returns (mmlu, elo, found).
func lookupBenchmarks(modelID string) (float64, float64, bool) {
	normalized := strings.ToLower(strings.TrimSpace(modelID))

	// Direct lookup
	if data, ok := knownBenchmarks[normalized]; ok {
		return data.MMLU, data.ArenaELO, true
	}

	// Strip common suffixes
	stripped := normalized
	for _, suffix := range []string{"-gguf", ".gguf", "-hf", "-awq", "-gptq", "-bnb"} {
		stripped = strings.TrimSuffix(stripped, suffix)
	}
	if stripped != normalized {
		if data, ok := knownBenchmarks[stripped]; ok {
			return data.MMLU, data.ArenaELO, true
		}
	}

	// Fuzzy match — try each known key
	for key, data := range knownBenchmarks {
		if namesMatch(normalized, key) {
			return data.MMLU, data.ArenaELO, true
		}
	}

	return 0, 0, false
}

// namesMatch checks if two model names are similar enough to be the same model.
func namesMatch(a, b string) bool {
	a = normalizeForMatch(a)
	b = normalizeForMatch(b)
	return a == b
}

// normalizeForMatch produces a normalized string for fuzzy model name comparison.
// Longer size tokens are replaced first to prevent shorter tokens from corrupting
// them (e.g., "7b" must not run before "70b").
// Each size gets a unique placeholder so fuzzy matching only succeeds when the
// *same* parameter size is referenced — a 3B model must never match an 8B entry.
func normalizeForMatch(s string) string {
	s = strings.ToLower(s)
	// Remove version suffixes like v0.2, v0.3, v1, etc. using regex
	// Handles: "-v0.2", "_v1", "v0.3", etc.
	versionPattern := regexp.MustCompile(`[-_]?v\d+(\.\d+)*$`)
	s = versionPattern.ReplaceAllString(s, "")
	// Replace size tokens longest-first with unique placeholders.
	// Order matters: longer numeric prefixes must be replaced before shorter
	// ones that would otherwise consume part of the string.
	replacements := []struct{ old, new string }{
		{"405b", "s405"},
		{"70b", "s70"},
		{"72b", "s72"},
		{"32b", "s32"},
		{"33b", "s33"},
		{"27b", "s27"},
		{"22b", "s22"},
		{"14b", "s14"},
		{"6.7b", "s6.7"},
		{"1.5b", "s1.5"},
		{"0.5b", "s0.5"},
		{"8b", "s8"},
		{"7b", "s7"},
		{"9b", "s9"},
		{"3b", "s3"},
		{"2b", "s2"},
		{"1b", "s1"},
	}
	for _, r := range replacements {
		s = strings.ReplaceAll(s, r.old, r.new)
	}
	return s
}
