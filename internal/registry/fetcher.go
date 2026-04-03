// Package registry loads pre-curated LLM model data from a local JSON file.
package registry

import (
	"context"
	"embed"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/kevo-1/KnowURLLM/internal/models"
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

// FetchOllama queries the Ollama library API for available models.
// https://ollama.com/search provides a public API at /api/library.
func (f *Fetcher) FetchOllama(ctx context.Context) ([]models.ModelEntry, error) {
	// Ollama's library API returns model entries with metadata.
	// We fetch the top models and convert them to our internal format.
	entries, err := fetchOllamaLibrary(ctx)
	if err != nil {
		log.Printf("warning: Ollama fetch failed, returning empty: %v", err)
		return []models.ModelEntry{}, nil
	}

	// Sort by popularity (downloads descending)
	sort.SliceStable(entries, func(i, j int) bool {
		return entries[i].Downloads > entries[j].Downloads
	})

	// Apply limit if set
	if f.MaxModels > 0 && len(entries) > f.MaxModels {
		entries = entries[:f.MaxModels]
	}

	log.Printf("Loaded %d models from Ollama library", len(entries))
	return entries, nil
}

// ollamaLibraryResponse is the JSON response from Ollama's library search API.
type ollamaLibraryResponse struct {
	Models []ollamaModelResult `json:"models"`
}

// ollamaModelResult represents a single model in the library response.
type ollamaModelResult struct {
	Name        string  `json:"name"`
	DisplayName string  `json:"display_name"`
	Description string  `json:"description"`
	PullCount   int     `json:"pull_count"`
	LikeCount   int     `json:"like_count"`
	Verified    bool    `json:"verified"`
}

// ollamaTagsResponse is the JSON response from the model tags API.
type ollamaTagsResponse struct {
	Tags []ollamaTag `json:"tags"`
}

// ollamaTag represents a single tag/variant of a model.
type ollamaTag struct {
	Name       string `json:"name"`
	Digest     string `json:"digest"`
	Size       uint64 `json:"size"`
	Parameters string `json:"param_size"`
	Quant      string `json:"quantization_level"`
}

// fetchOllamaLibrary queries the Ollama API for available models.
func fetchOllamaLibrary(ctx context.Context) ([]models.ModelEntry, error) {
	client := &http.Client{Timeout: 30 * time.Second}

	// Ollama doesn't have a single "list all models" API that returns everything.
	// The best public approach: use the search API with common model families.
	// See: https://github.com/ollama/ollama/blob/main/docs/api.md
	// We query for popular model families to build a comprehensive list.

	searchQueries := []string{
		"llama", "qwen", "mistral", "gemma", "phi",
		"deepseek", "mixtral", "llava", "codellama",
	}

	seen := make(map[string]bool)
	var allEntries []models.ModelEntry

	for _, query := range searchQueries {
		if ctx.Err() != nil {
			break
		}

		url := fmt.Sprintf("https://ollama.com/api/library/search?q=%s&limit=25", query)
		req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
		if err != nil {
			continue
		}
		req.Header.Set("Accept", "application/json")
		req.Header.Set("User-Agent", "KnowURLLM/1.0")

		resp, err := client.Do(req)
		if err != nil {
			continue
		}

		body, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			continue
		}

		if resp.StatusCode != http.StatusOK {
			continue
		}

		var libResp struct {
			Models []struct {
				Name         string `json:"name"`
				DisplayName  string `json:"display_name"`
				Description  string `json:"description"`
				PullCount    int    `json:"pull_count"`
				LikeCount    int    `json:"like_count"`
				ShortDesc    string `json:"short_description"`
				LastPushedAt string `json:"last_pushed_at"`
			} `json:"models"`
		}

		if err := json.Unmarshal(body, &libResp); err != nil {
			continue
		}

		for _, m := range libResp.Models {
			if seen[m.Name] {
				continue
			}
			seen[m.Name] = true

			entry := models.ModelEntry{
				ID:          m.Name,
				DisplayName: m.DisplayName,
				Source:      "ollama",
				Downloads:   m.PullCount,
				URL:         "https://ollama.com/library/" + m.Name,
				Tags:        []string{"ollama"},
			}

			// Try to get more details from the tags endpoint
			tagURL := fmt.Sprintf("https://ollama.com/v2/library/%s/tags", m.Name)
			if tags := fetchOllamaTags(ctx, client, tagURL); tags != nil {
				entry.ModelSizeBytes = tags.Size
				entry.Quantization = tags.Quantization
				entry.ContextLength = tags.ContextLength
				if tags.Quantization != "" {
					entry.Tags = append(entry.Tags, tags.Quantization)
				}
			}

			// Enrich with benchmarks if available
			if mmlu, elo, found := lookupBenchmarks(m.Name); found {
				entry.MMLUScore = mmlu
				entry.ArenaELO = elo
			}

			allEntries = append(allEntries, entry)
		}
	}

	return allEntries, nil
}

// ollamaTagInfo holds parsed tag information.
type ollamaTagInfo struct {
	Size          uint64
	Quantization  string
	ContextLength int
}

// fetchOllamaTags queries the tags endpoint for a specific model.
// Returns the largest/default variant's info.
func fetchOllamaTags(ctx context.Context, client *http.Client, url string) *ollamaTagInfo {
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "KnowURLLM/1.0")

	resp, err := client.Do(req)
	if err != nil {
		return nil
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil
	}

	var tagResp ollamaTagsResponse
	if err := json.Unmarshal(body, &tagResp); err != nil {
		return nil
	}

	if len(tagResp.Tags) == 0 {
		return nil
	}

	// Pick the "latest" tag (usually first) or the largest
	best := &tagResp.Tags[0]
	for i := range tagResp.Tags {
		if tagResp.Tags[i].Size > best.Size {
			best = &tagResp.Tags[i]
		}
	}

	return &ollamaTagInfo{
		Size:         best.Size,
		Quantization: best.Quant,
	}
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
