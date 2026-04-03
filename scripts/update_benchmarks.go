//go:build ignore

// Script to update internal/registry/data/benchmarks.json with the latest
// known model benchmark data from public leaderboards.
//
// Usage:
//   go run scripts/update_benchmarks.go              # Curated + trending discovery
//   go run scripts/update_benchmarks.go --no-discover # Curated list only (fast)
//   go run scripts/update_benchmarks.go --limit 50    # Limit trending models
//
// Data sources:
//   - HuggingFace API: auto-discovers trending models by download count
//   - LMSYS Chatbot Arena: arena_hard_auto_leaderboard CSV
//   - Open LLM Leaderboard: open-llm-leaderboard/contents dataset
//
// The script merges fresh benchmark data with existing entries, preserving
// manually-curated scores while adding new models from discovery.
package main

import (
	"encoding/csv"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
)

const benchmarksPath = "internal/registry/data/benchmarks.json"

// ---------- Configuration ----------

// curatedModels is the seed list of important models to always include.
// These are fetched individually from the HuggingFace API.
var curatedModels = []string{
	// Meta Llama family
	"meta-llama/Llama-3.1-8B-Instruct",
	"meta-llama/Llama-3.1-70B-Instruct",
	"meta-llama/Llama-3.1-405B-Instruct",
	"meta-llama/Llama-3-8B-Instruct",
	"meta-llama/Llama-3-70B-Instruct",
	"meta-llama/Llama-3.2-1B-Instruct",
	"meta-llama/Llama-3.2-3B-Instruct",
	"meta-llama/Llama-3.3-70B-Instruct",

	// Mistral
	"mistralai/Mistral-7B-Instruct-v0.3",
	"mistralai/Mixtral-8x7B-Instruct-v0.1",
	"mistralai/Mixtral-8x22B-Instruct-v0.1",
	"mistralai/Mistral-Large-Instruct-2407",
	"mistralai/Mistral-Small-24B-Instruct-2501",
	"mistralai/Mistral-Nemo-Instruct-2407",
	"mistralai/Codestral-22B-v0.1",

	// Qwen
	"Qwen/Qwen2.5-7B-Instruct",
	"Qwen/Qwen2.5-14B-Instruct",
	"Qwen/Qwen2.5-32B-Instruct",
	"Qwen/Qwen2.5-72B-Instruct",
	"Qwen/Qwen2.5-0.5B-Instruct",
	"Qwen/Qwen2.5-1.5B-Instruct",
	"Qwen/Qwen2.5-3B-Instruct",
	"Qwen/Qwen2-7B-Instruct",
	"Qwen/Qwen2-72B-Instruct",
	"Qwen/Qwen2.5-Coder-7B-Instruct",
	"Qwen/Qwen2.5-Coder-32B-Instruct",

	// Google Gemma
	"google/gemma-2-2b-it",
	"google/gemma-2-9b-it",
	"google/gemma-2-27b-it",
	"google/gemma-7b-it",

	// Microsoft Phi
	"microsoft/Phi-3-mini-4k-instruct",
	"microsoft/Phi-3-mini-128k-instruct",
	"microsoft/Phi-3-small-8k-instruct",
	"microsoft/Phi-3-medium-4k-instruct",
	"microsoft/Phi-3.5-mini-instruct",
	"microsoft/Phi-3.5-moe-instruct",
	"microsoft/Phi-4-mini-instruct",

	// DeepSeek
	"deepseek-ai/DeepSeek-Coder-6.7b-instruct",
	"deepseek-ai/DeepSeek-Coder-33b-instruct",
	"deepseek-ai/DeepSeek-V2-Lite",
	"deepseek-ai/DeepSeek-V2.5",
	"deepseek-ai/DeepSeek-R1-Distill-Qwen-1.5B",
	"deepseek-ai/DeepSeek-R1-Distill-Qwen-7B",
	"deepseek-ai/DeepSeek-R1-Distill-Qwen-14B",
	"deepseek-ai/DeepSeek-R1-Distill-Qwen-32B",
	"deepseek-ai/DeepSeek-R1-Distill-Llama-8B",
	"deepseek-ai/DeepSeek-R1-Distill-Llama-70B",
	"deepseek-ai/DeepSeek-R1",
	"deepseek-ai/DeepSeek-V3",

	// Multimodal
	"llava-hf/llava-1.5-7b-hf",
	"llava-hf/llava-v1.6-mistral-7b-hf",

	// Community
	"TinyLlama/TinyLlama-1.1B-Chat-v1.0",
}

// Orgs to skip during auto-discovery (repackers, test fixtures, etc.)
var skipOrgs = map[string]bool{
	"TheBloke":       true, // GGUF repacks
	"unsloth":        true, // Training framework repacks
	"mlx-community":  true, // MLX conversions
	"bartowski":      true, // GGUF repacks
	"mradermacher":   true, // GGUF repacks
	"trl-internal-testing": true, // Test fixtures
}

// ---------- Types ----------

// HFModelInfo represents metadata from the HuggingFace API.
type HFModelInfo struct {
	ID          string   `json:"id"`
	Downloads   int      `json:"downloads"`
	Likes       int      `json:"likes"`
	PipelineTag string   `json:"pipeline_tag"`
	Tags        []string `json:"tags"`
	CreatedAt   string   `json:"createdAt"`
	Safetensors struct {
		Total      int                `json:"total"`
		Parameters map[string]int     `json:"parameters"`
	} `json:"safetensors"`
}

// ArenaHardRow represents a row from the Arena Hard leaderboard CSV.
type ArenaHardRow struct {
	Model string
	Score float64
}

// HFContentRow represents a row from the Open LLM Leaderboard contents dataset.
type HFContentRow struct {
	Fullname string
	MMLUPRO  float64
	MMLU     float64
}

type modelEntry struct {
	MMLU     float64 `json:"mmlu"`
	ArenaELO float64 `json:"arena_elo"`
}

type benchmarksManifest struct {
	Version string                `json:"version"`
	Sources []string              `json:"sources"`
	Models  map[string]modelEntry `json:"models"`
}

// ---------- Flags ----------

var (
	flagNoDiscover bool
	flagLimit      int
	flagMinDLs     int
)

func init() {
	flag.BoolVar(&flagNoDiscover, "no-discover", false, "Skip auto-discovery, use curated list only")
	flag.IntVar(&flagLimit, "limit", 30, "Max trending models to discover")
	flag.IntVar(&flagMinDLs, "min-downloads", 10000, "Minimum download count for discovered models")
}

// ---------- Main ----------

func main() {
	flag.Parse()

	fmt.Println("🔄 Fetching latest benchmark data...")

	// Read existing benchmarks
	existing, err := readExisting()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error reading existing benchmarks: %v\n", err)
		os.Exit(1)
	}

	// Build the set of models to score: curated + discovered
	modelSet := make(map[string]bool)
	for _, id := range curatedModels {
		modelSet[strings.ToLower(id)] = true
	}

	// Auto-discover trending models from HuggingFace
	if !flagNoDiscover {
		fmt.Printf("\n🔍 Discovering trending models (limit=%d, min_downloads=%d)...\n", flagLimit, flagMinDLs)
		discovered := discoverTrendingModels(flagLimit, flagMinDLs)
		for _, info := range discovered {
			modelSet[strings.ToLower(info.ID)] = true
		}
		fmt.Printf("✅ Discovered %d new models\n", len(discovered))
	}

	fmt.Printf("\n📋 Total models to score: %d\n", len(modelSet))

	// Fetch benchmark data from external sources
	arenaData, arenaErr := fetchArenaHard()
	if arenaErr != nil {
		fmt.Fprintf(os.Stderr, "⚠️  Warning: failed to fetch LMSYS Arena data: %v\n", arenaErr)
	} else {
		fmt.Printf("✅ LMSYS Arena Hard: %d models fetched\n", len(arenaData))
	}

	hfData, hfErr := fetchHFContents()
	if hfErr != nil {
		fmt.Fprintf(os.Stderr, "⚠️  Warning: failed to fetch HF Open LLM Leaderboard data: %v\n", hfErr)
	} else {
		fmt.Printf("✅ HF Open LLM Leaderboard: %d models fetched\n", len(hfData))
	}

	// Build lookup maps from benchmark sources
	arenaScores := make(map[string]float64)
	for _, row := range arenaData {
		modelID := normalizeModelID(row.Model)
		arenaScores[modelID] = row.Score
	}

	mmluScores := make(map[string]float64)
	for _, row := range hfData {
		modelID := normalizeModelID(row.Fullname)
		score := row.MMLUPRO
		if score <= 0 {
			score = row.MMLU
		}
		if score > 0 {
			mmluScores[modelID] = score
		}
	}

	// Merge: start with existing, add new models from discovery
	merged := make(map[string]modelEntry)
	for id, entry := range existing.Models {
		merged[strings.ToLower(id)] = entry
	}

	// Apply benchmark scores to all discovered models
	appliedMMLU, appliedELO := 0, 0
	for modelID := range modelSet {
		entry := merged[modelID]

		// Apply MMLU if not already set
		if entry.MMLU <= 0 {
			if score, ok := mmluScores[modelID]; ok {
				entry.MMLU = roundTo1(score)
				appliedMMLU++
			}
		}

		// Apply Arena ELO if not already set
		if entry.ArenaELO <= 0 {
			if score, ok := arenaScores[modelID]; ok {
				entry.ArenaELO = roundTo1(1000 + score*3)
				appliedELO++
			}
		}

		merged[modelID] = entry
	}

	fmt.Printf("   Applied MMLU scores to %d models\n", appliedMMLU)
	fmt.Printf("   Applied ELO scores to %d models\n", appliedELO)

	// Build output manifest
	sources := []string{
		"LMSYS Chatbot Arena leaderboard (https://lmarena.ai/leaderboard)",
		"Official model cards",
		"Open LLM Leaderboard (https://huggingface.co/spaces/open-llm-leaderboard/open_llm_leaderboard)",
		"HuggingFace trending models (auto-discovered by download count)",
	}

	out := benchmarksManifest{
		Version: time.Now().Format("2006-01-02"),
		Sources: sources,
		Models:  make(map[string]modelEntry),
	}
	for id, entry := range merged {
		if entry.MMLU > 0 || entry.ArenaELO > 0 {
			out.Models[id] = entry
		}
	}

	// Write
	if err := writeSorted(out); err != nil {
		fmt.Fprintf(os.Stderr, "Error writing benchmarks: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("\n✅ Updated %d model entries in %s\n", len(out.Models), benchmarksPath)
	fmt.Printf("   Version updated to %s\n", out.Version)
}

// ---------- I/O ----------

func readExisting() (*benchmarksManifest, error) {
	data, err := os.ReadFile(benchmarksPath)
	if err != nil {
		return nil, err
	}
	var manifest benchmarksManifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		return nil, err
	}
	return &manifest, nil
}

func writeSorted(manifest benchmarksManifest) error {
	// Sort model keys
	keys := make([]string, 0, len(manifest.Models))
	for k := range manifest.Models {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	ordered := make(map[string]interface{})
	for _, k := range keys {
		entry := manifest.Models[k]
		e := map[string]float64{}
		if entry.MMLU > 0 {
			e["mmlu"] = entry.MMLU
		}
		if entry.ArenaELO > 0 {
			e["arena_elo"] = entry.ArenaELO
		}
		ordered[k] = e
	}

	out := map[string]interface{}{
		"version": manifest.Version,
		"sources": manifest.Sources,
		"models":  ordered,
	}

	data, err := json.MarshalIndent(out, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal: %w", err)
	}
	data = append(data, '\n')

	absPath, _ := filepath.Abs(benchmarksPath)
	if err := os.WriteFile(absPath, data, 0644); err != nil {
		return fmt.Errorf("write: %w", err)
	}
	return nil
}

// ---------- Discovery ----------

// discoverTrendingModels queries the HuggingFace API for top text-generation
// models sorted by download count. This replaces the need for a manually
// maintained list — new popular models are automatically picked up.
func discoverTrendingModels(limit int, minDownloads int) []HFModelInfo {
	pipelines := []string{"text-generation", "text2text-generation", "image-text-to-text"}
	curated := make(map[string]bool)
	for _, id := range curatedModels {
		curated[strings.ToLower(id)] = true
	}

	var results []HFModelInfo
	seen := make(map[string]bool)

	for _, pipeline := range pipelines {
		if len(results) >= limit {
			break
		}

		fetchLimit := min(limit*8, 10000)
		url := fmt.Sprintf(
			"https://huggingface.co/api/models?pipeline_tag=%s&sort=downloads&direction=-1&limit=%d&expand[]=safetensors",
			pipeline, fetchLimit,
		)

		resp, err := http.Get(url)
		if err != nil {
			fmt.Fprintf(os.Stderr, "  ⚠ Failed to fetch trending %s: %v\n", pipeline, err)
			continue
		}

		var models []HFModelInfo
		if err := json.NewDecoder(resp.Body).Decode(&models); err != nil {
			resp.Body.Close()
			fmt.Fprintf(os.Stderr, "  ⚠ Failed to parse trending %s: %v\n", pipeline, err)
			continue
		}
		resp.Body.Close()

		for _, m := range models {
			if len(results) >= limit {
				break
			}
			if m.ID == "" || !strings.Contains(m.ID, "/") {
				continue
			}

			lower := strings.ToLower(m.ID)
			if curated[lower] || seen[lower] {
				continue
			}

			// Skip repack orgs
			org := strings.Split(m.ID, "/")[0]
			if skipOrgs[org] {
				continue
			}

			if m.Downloads < minDownloads {
				continue
			}

			// Skip GGUF-only repos, adapters, merges
			tags := make(map[string]bool)
			for _, t := range m.Tags {
				tags[t] = true
			}
			if tags["gguf"] || tags["adapter"] || tags["merge"] || tags["lora"] || tags["qlora"] {
				continue
			}

			// Must have parameter count
			totalParams := m.Safetensors.Total
			if totalParams == 0 && len(m.Safetensors.Parameters) > 0 {
				for _, v := range m.Safetensors.Parameters {
					if v > totalParams {
						totalParams = v
					}
				}
			}
			if totalParams == 0 {
				continue
			}

			seen[lower] = true
			results = append(results, m)
			fmt.Printf("  📥 %s (%d downloads, %d params)\n", m.ID, m.Downloads, totalParams)
		}
	}

	return results
}

// ---------- Fetchers ----------

// fetchArenaHard downloads the latest Arena Hard leaderboard CSV from the
// lmarena-ai/arena-leaderboard HuggingFace space.
// Arena Hard uses a 0-100 score representing win rate vs a reference model.
func fetchArenaHard() ([]ArenaHardRow, error) {
	files := []string{
		"https://huggingface.co/spaces/lmarena-ai/arena-leaderboard/resolve/main/arena_hard_auto_leaderboard_v1.csv",
		"https://huggingface.co/spaces/lmarena-ai/arena-leaderboard/resolve/main/arena_hard_auto_leaderboard_v0.1.csv",
	}

	var lastErr error
	for _, url := range files {
		rows, err := fetchArenaHardCSV(url)
		if err == nil && len(rows) > 0 {
			return rows, nil
		}
		lastErr = err
	}

	// Try dynamic file listing
	files, err := listArenaHardFiles()
	if err != nil {
		return nil, fmt.Errorf("arena hard fetch failed: %w (last: %v)", err, lastErr)
	}

	for _, f := range files {
		rows, err := fetchArenaHardCSV(f)
		if err == nil && len(rows) > 0 {
			return rows, nil
		}
	}

	return nil, fmt.Errorf("no arena hard data found")
}

func listArenaHardFiles() ([]string, error) {
	resp, err := http.Get("https://huggingface.co/api/spaces/lmarena-ai/arena-leaderboard/tree/main")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("list files: status %d", resp.StatusCode)
	}

	var files []struct {
		Path string `json:"path"`
		Type string `json:"type"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&files); err != nil {
		return nil, err
	}

	var result []string
	for _, f := range files {
		if strings.HasPrefix(f.Path, "arena_hard_auto_leaderboard_v") && strings.HasSuffix(f.Path, ".csv") {
			result = append(result, "https://huggingface.co/spaces/lmarena-ai/arena-leaderboard/resolve/main/"+f.Path)
		}
	}
	sort.Strings(result)
	return result, nil
}

func fetchArenaHardCSV(url string) ([]ArenaHardRow, error) {
	resp, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("fetch %s: status %d", url, resp.StatusCode)
	}

	return parseArenaHardCSV(resp.Body)
}

func parseArenaHardCSV(r io.Reader) ([]ArenaHardRow, error) {
	reader := csv.NewReader(r)
	records, err := reader.ReadAll()
	if err != nil {
		return nil, err
	}

	if len(records) < 2 {
		return nil, fmt.Errorf("CSV has no data rows")
	}

	header := records[0]
	colIndex := make(map[string]int)
	for i, h := range header {
		colIndex[strings.ToLower(strings.TrimSpace(h))] = i
	}

	modelCol, ok1 := colIndex["model"]
	if !ok1 {
		for _, alt := range []string{"model_name", "fullname", "name"} {
			if idx, ok := colIndex[alt]; ok {
				modelCol = idx
				ok1 = true
				break
			}
		}
	}
	if !ok1 {
		return nil, fmt.Errorf("CSV missing model column, headers: %v", header)
	}

	scoreCol, ok2 := colIndex["score"]
	if !ok2 {
		for _, alt := range []string{"rating", "elo", "arena_elo"} {
			if idx, ok := colIndex[alt]; ok {
				scoreCol = idx
				ok2 = true
				break
			}
		}
	}
	if !ok2 {
		return nil, fmt.Errorf("CSV missing score column, headers: %v", header)
	}

	var rows []ArenaHardRow
	for _, rec := range records[1:] {
		if len(rec) <= modelCol || len(rec) <= scoreCol {
			continue
		}
		model := strings.TrimSpace(rec[modelCol])
		if model == "" {
			continue
		}
		score, err := strconv.ParseFloat(strings.TrimSpace(rec[scoreCol]), 64)
		if err != nil {
			continue
		}
		rows = append(rows, ArenaHardRow{Model: model, Score: score})
	}
	return rows, nil
}

// fetchHFContents downloads the Open LLM Leaderboard contents dataset via the
// HuggingFace dataset server API.
func fetchHFContents() ([]HFContentRow, error) {
	url := "https://datasets-server.huggingface.co/rows?dataset=open-llm-leaderboard%2Fcontents&config=default&split=train&offset=0&limit=10000"

	resp, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("fetch HF contents: status %d", resp.StatusCode)
	}

	var result struct {
		Rows []struct {
			Row json.RawMessage `json:"row"`
		} `json:"rows"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	var rows []HFContentRow
	for _, r := range result.Rows {
		var entry struct {
			Fullname string  `json:"fullname"`
			MMLUPRO  float64 `json:"MMLU-PRO"`
			MMLU     float64 `json:"MMLU"`
		}
		if err := json.Unmarshal(r.Row, &entry); err != nil {
			continue
		}
		if entry.Fullname == "" {
			continue
		}

		row := HFContentRow{
			Fullname: entry.Fullname,
			MMLUPRO:  entry.MMLUPRO,
			MMLU:     entry.MMLU,
		}

		if row.MMLUPRO > 0 || row.MMLU > 0 {
			rows = append(rows, row)
		}
	}
	return rows, nil
}

// ---------- Helpers ----------

// normalizeModelID converts a raw model name from external sources into a
// normalized key that matches our benchmarks.json convention (lowercase,
// with org/model path structure).
func normalizeModelID(raw string) string {
	s := strings.ToLower(strings.TrimSpace(raw))
	s = strings.TrimPrefix(s, "@")

	// If it already looks like org/model, return as-is
	if strings.Contains(s, "/") {
		return s
	}

	// Map shorthand names to full HF repo IDs
	mappings := map[string]string{
		"llama-3.1-8b-instruct":    "meta-llama/llama-3.1-8b-instruct",
		"llama-3.1-70b-instruct":   "meta-llama/llama-3.1-70b-instruct",
		"llama-3.1-405b-instruct":  "meta-llama/llama-3.1-405b-instruct",
		"llama-3-8b-instruct":      "meta-llama/llama-3-8b-instruct",
		"llama-3-70b-instruct":     "meta-llama/llama-3-70b-instruct",
		"llama-3.2-1b-instruct":    "meta-llama/llama-3.2-1b-instruct",
		"llama-3.2-3b-instruct":    "meta-llama/llama-3.2-3b-instruct",
		"llama-3.3-70b-instruct":   "meta-llama/llama-3.3-70b-instruct",
		"qwen2.5-7b-instruct":      "qwen/qwen2.5-7b-instruct",
		"qwen2.5-14b-instruct":     "qwen/qwen2.5-14b-instruct",
		"qwen2.5-32b-instruct":     "qwen/qwen2.5-32b-instruct",
		"qwen2.5-72b-instruct":     "qwen/qwen2.5-72b-instruct",
		"qwen2.5-0.5b-instruct":    "qwen/qwen2.5-0.5b-instruct",
		"qwen2.5-1.5b-instruct":    "qwen/qwen2.5-1.5b-instruct",
		"qwen2.5-3b-instruct":      "qwen/qwen2.5-3b-instruct",
		"qwen2-7b-instruct":        "qwen/qwen2-7b-instruct",
		"qwen2-72b-instruct":       "qwen/qwen2-72b-instruct",
		"qwen2.5-coder-7b-instruct": "qwen/qwen2.5-coder-7b-instruct",
		"qwen2.5-coder-32b-instruct": "qwen/qwen2.5-coder-32b-instruct",
		"gemma-2-2b-it":            "google/gemma-2-2b-it",
		"gemma-2-9b-it":            "google/gemma-2-9b-it",
		"gemma-2-27b-it":           "google/gemma-2-27b-it",
		"gemma-7b-it":              "google/gemma-7b-it",
		"phi-3-mini-4k-instruct":   "microsoft/phi-3-mini-4k-instruct",
		"phi-3-mini-128k-instruct": "microsoft/phi-3-mini-128k-instruct",
		"phi-3-small-8k-instruct":  "microsoft/phi-3-small-8k-instruct",
		"phi-3-medium-4k-instruct": "microsoft/phi-3-medium-4k-instruct",
		"phi-3.5-mini-instruct":    "microsoft/phi-3.5-mini-instruct",
		"phi-3.5-moe-instruct":     "microsoft/phi-3.5-moe-instruct",
		"phi-4-mini-instruct":      "microsoft/phi-4-mini-instruct",
		"deepseek-coder-6.7b-instruct": "deepseek-ai/deepseek-coder-6.7b-instruct",
		"deepseek-coder-33b-instruct":  "deepseek-ai/deepseek-coder-33b-instruct",
		"deepseek-v2-lite":         "deepseek-ai/deepseek-v2-lite",
		"deepseek-v2.5":            "deepseek-ai/deepseek-v2.5",
		"deepseek-r1-distill-qwen-1.5b":  "deepseek-ai/deepseek-r1-distill-qwen-1.5b",
		"deepseek-r1-distill-qwen-7b":    "deepseek-ai/deepseek-r1-distill-qwen-7b",
		"deepseek-r1-distill-qwen-14b":   "deepseek-ai/deepseek-r1-distill-qwen-14b",
		"deepseek-r1-distill-qwen-32b":   "deepseek-ai/deepseek-r1-distill-qwen-32b",
		"deepseek-r1-distill-llama-8b":   "deepseek-ai/deepseek-r1-distill-llama-8b",
		"deepseek-r1-distill-llama-70b":  "deepseek-ai/deepseek-r1-distill-llama-70b",
		"deepseek-r1":              "deepseek-ai/deepseek-r1",
		"deepseek-v3":              "deepseek-ai/deepseek-v3",
		"mistral-7b-instruct-v0.3": "mistralai/mistral-7b-instruct-v0.3",
		"mixtral-8x7b-instruct-v0.1": "mistralai/mixtral-8x7b-instruct-v0.1",
		"mixtral-8x22b-instruct-v0.1": "mistralai/mixtral-8x22b-instruct-v0.1",
		"mistral-large-instruct-2407": "mistralai/mistral-large-instruct-2407",
		"mistral-small-24b-instruct-2501": "mistralai/mistral-small-24b-instruct-2501",
		"mistral-nemo-instruct-2407": "mistralai/mistral-nemo-instruct-2407",
		"codestral-22b-v0.1":       "mistralai/codestral-22b-v0.1",
		"llava-1.5-7b-hf":          "llava-hf/llava-1.5-7b-hf",
		"llava-v1.6-mistral-7b-hf": "llava-hf/llava-v1.6-mistral-7b-hf",
		"tinyllama-1.1b-chat-v1.0": "tinyllama/tinyllama-1.1b-chat-v1.0",
	}

	if mapped, ok := mappings[s]; ok {
		return mapped
	}

	return s
}

func roundTo1(f float64) float64 {
	return float64(int(f*10+0.5)) / 10
}
