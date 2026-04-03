// Package registry loads pre-curated LLM model data from a local JSON file.
package registry

import (
	"context"
	"embed"
	"encoding/json"
	"fmt"
	"log"
	"sort"
	"strings"

	"github.com/KnowURLLM/internal/models"
)

//go:embed data
var dataFS embed.FS

// Fetcher loads model data from the embedded models JSON file.
type Fetcher struct {
	// MaxModels limits results (0 = no limit).
	MaxModels int
}

// Option is a functional option for configuring a Fetcher.
type Option func(*Fetcher)

// WithMaxModels sets the maximum number of models to return.
func WithMaxModels(n int) Option {
	return func(f *Fetcher) {
		f.MaxModels = n
	}
}

// NewFetcher creates a Fetcher with sensible defaults.
func NewFetcher(opts ...Option) *Fetcher {
	f := &Fetcher{
		MaxModels: 0, // no limit — use all models from the data file
	}

	for _, opt := range opts {
		opt(f)
	}

	return f
}

// FetchAll loads all models from the embedded JSON data file.
// Returns models sorted by downloads descending.
func (f *Fetcher) FetchAll(ctx context.Context) ([]models.ModelEntry, error) {
	_ = ctx // no network calls needed

	// Read embedded data
	data, err := dataFS.ReadFile("data/hf_models.json")
	if err != nil {
		return nil, fmt.Errorf("reading embedded models: %w", err)
	}

	// Parse JSON
	var rawModels []hfModel
	if err := json.Unmarshal(data, &rawModels); err != nil {
		return nil, fmt.Errorf("parsing embedded models JSON: %w", err)
	}

	// Convert to ModelEntry
	entries := make([]models.ModelEntry, 0, len(rawModels))
	for _, raw := range rawModels {
		entry := hfModelToEntry(raw)
		entries = append(entries, entry)
	}

	// Sort by downloads descending
	sort.SliceStable(entries, func(i, j int) bool {
		return entries[i].Downloads > entries[j].Downloads
	})

	// Apply limit if set
	if f.MaxModels > 0 && len(entries) > f.MaxModels {
		entries = entries[:f.MaxModels]
	}

	log.Printf("Loaded %d models from embedded data file", len(entries))
	return entries, nil
}

// FetchHuggingFace is a compatibility alias — loads all models from the data file.
func (f *Fetcher) FetchHuggingFace(ctx context.Context) ([]models.ModelEntry, error) {
	return f.FetchAll(ctx)
}

// FetchOllama returns empty — Ollama is not included in the data file.
func (f *Fetcher) FetchOllama(ctx context.Context) ([]models.ModelEntry, error) {
	return []models.ModelEntry{}, nil
}

// hfModel represents a single model entry from the hf_models.json file.
type hfModel struct {
	Name           string   `json:"name"`
	Provider       string   `json:"provider"`
	ParamCount     string   `json:"parameter_count"`
	ParamsRaw      uint64   `json:"parameters_raw"`
	MinVRAMGB      float64  `json:"min_vram_gb"`
	RecommendedRAM float64  `json:"recommended_ram_gb"`
	Quantization   string   `json:"quantization"`
	Format         string   `json:"format"`
	ContextLength  int      `json:"context_length"`
	UseCase        string   `json:"use_case"`
	Capabilities   []string `json:"capabilities"`
	PipelineTag    string   `json:"pipeline_tag"`
	Architecture   string   `json:"architecture"`
	HFDownloads    int      `json:"hf_downloads"`
	HFLikes        int      `json:"hf_likes"`
	IsMoE          bool     `json:"is_moe"`
	NumExperts     int      `json:"num_experts"`
	ActiveExperts  int      `json:"active_experts"`
	ActiveParams   uint64   `json:"active_parameters"`
	GGUFSources    []struct {
		Repo     string `json:"repo"`
		Provider string `json:"provider"`
	} `json:"gguf_sources"`
}

// hfModelToEntry converts a raw JSON model entry to the internal ModelEntry.
func hfModelToEntry(raw hfModel) models.ModelEntry {
	// Calculate model size in bytes from parameters_raw and quantization
	modelSizeBytes := calcModelSize(raw.ParamsRaw, raw.Quantization)

	// Build tags from capabilities, architecture, and use_case
	tags := buildTags(raw)

	// Build URL
	url := "https://huggingface.co/" + raw.Name

	// Extract display name without provider prefix
	displayName := raw.Name
	if idx := strings.LastIndex(raw.Name, "/"); idx != -1 && idx+1 < len(raw.Name) {
		displayName = raw.Name[idx+1:]
	}

	// Try to enrich with known benchmark data
	var mmluScore, arenaELO float64
	if mmlu, elo, found := lookupBenchmarks(raw.Name); found {
		mmluScore = mmlu
		arenaELO = elo
	}

	return models.ModelEntry{
		ID:             raw.Name,
		DisplayName:    displayName,
		ModelSizeBytes: modelSizeBytes,
		Quantization:   raw.Quantization,
		ContextLength:  raw.ContextLength,
		Source:         "huggingface",
		MMLUScore:      mmluScore,
		ArenaELO:       arenaELO,
		Downloads:      raw.HFDownloads,
		URL:            url,
		Tags:           tags,
	}
}

// calcModelSize estimates model file size from parameter count and quantization.
// parameters_raw × bytes_per_param gives the GGUF file size.
func calcModelSize(paramsRaw uint64, quant string) uint64 {
	if paramsRaw == 0 {
		return 0
	}

	bpp := bytesPerParam(quant)
	return uint64(float64(paramsRaw) * bpp)
}

// buildTags creates a tag list from model metadata.
func buildTags(raw hfModel) []string {
	var tags []string

	// Add architecture
	if raw.Architecture != "" && raw.Architecture != "unknown" {
		tags = append(tags, raw.Architecture)
	}

	// Add use case
	if raw.UseCase != "" {
		tags = append(tags, raw.UseCase)
	}

	// Add capabilities
	for _, cap := range raw.Capabilities {
		if cap != "" {
			tags = append(tags, cap)
		}
	}

	// Add format
	if raw.Format != "" {
		tags = append(tags, raw.Format)
	}

	// Add MoE info
	if raw.IsMoE {
		tags = append(tags, "moe")
	}

	return tags
}
