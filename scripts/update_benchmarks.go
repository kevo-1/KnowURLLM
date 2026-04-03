//go:build ignore

// Script to regenerate internal/registry/data/benchmarks.json with the latest
// known model benchmark data.
//
// Usage:
//   go run scripts/update_benchmarks.go
//
// This updates the embedded benchmarks.json file with versioned data and
// adds newly released models. Data sources:
//   - LMSYS Chatbot Arena: https://lmarena.ai/leaderboard
//   - Open LLM Leaderboard: https://huggingface.co/spaces/open-llm-leaderboard/open_llm_leaderboard
//   - Official model cards
//
// The script does NOT make network calls — it validates the JSON structure
// and updates the version date. To add new models, edit the JSON directly
// or extend this script to scrape/fetch from the sources above.
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

const benchmarksPath = "internal/registry/data/benchmarks.json"

func main() {
	data, err := os.ReadFile(benchmarksPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error reading %s: %v\n", benchmarksPath, err)
		os.Exit(1)
	}

	var manifest map[string]interface{}
	if err := json.Unmarshal(data, &manifest); err != nil {
		fmt.Fprintf(os.Stderr, "Error parsing JSON: %v\n", err)
		os.Exit(1)
	}

	// Validate structure
	if _, ok := manifest["version"]; !ok {
		fmt.Fprintln(os.Stderr, "Missing 'version' field")
		os.Exit(1)
	}
	if _, ok := manifest["models"]; !ok {
		fmt.Fprintln(os.Stderr, "Missing 'models' field")
		os.Exit(1)
	}

	models, ok := manifest["models"].(map[string]interface{})
	if !ok {
		fmt.Fprintln(os.Stderr, "'models' field must be an object")
		os.Exit(1)
	}

	// Validate each model entry
	validated := 0
	for id, raw := range models {
		entry, ok := raw.(map[string]interface{})
		if !ok {
			fmt.Fprintf(os.Stderr, "Warning: invalid entry for %q, skipping\n", id)
			delete(models, id)
			continue
		}
		mmlu, hasMMLU := entry["mmlu"].(float64)
		el, hasELO := entry["arena_elo"].(float64)
		if !hasMMLU || !hasELO {
			fmt.Fprintf(os.Stderr, "Warning: %q missing mmlu or arena_elo, skipping\n", id)
			delete(models, id)
			continue
		}
		if mmlu < 0 || mmlu > 100 {
			fmt.Fprintf(os.Stderr, "Warning: %q has invalid MMLU %.1f, skipping\n", id, mmlu)
			delete(models, id)
			continue
		}
		validated++
	}

	// Update version to current date
	manifest["version"] = time.Now().Format("2006-01-02")

	// Write back with indentation
	out, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error marshaling JSON: %v\n", err)
		os.Exit(1)
	}
	out = append(out, '\n')

	absPath, _ := filepath.Abs(benchmarksPath)
	if err := os.WriteFile(absPath, out, 0644); err != nil {
		fmt.Fprintf(os.Stderr, "Error writing %s: %v\n", benchmarksPath, err)
		os.Exit(1)
	}

	fmt.Printf("✅ Validated %d model entries in %s\n", validated, absPath)
	fmt.Printf("   Version updated to %s\n", manifest["version"])
}
