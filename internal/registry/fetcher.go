// Package registry loads pre-curated LLM model data from a local JSON file.
package registry

import (
	"context"
	"embed"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/kevo-1/KnowURLLM/internal/domain"
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
func (f *Fetcher) FetchAll(ctx context.Context) ([]domain.ModelEntry, error) {
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
	entries := make([]domain.ModelEntry, 0, len(rawModels))
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

	slog.Info("loaded models from embedded data file", "component", "registry", "count", len(entries))
	return entries, nil
}

// FetchHuggingFace is a compatibility alias — loads all models from the data file.
func (f *Fetcher) FetchHuggingFace(ctx context.Context) ([]domain.ModelEntry, error) {
	return f.FetchAll(ctx)
}

// FetchOllama queries the Ollama library API for available domain.
// https://ollama.com/search provides a public API at /api/library.
func (f *Fetcher) FetchOllama(ctx context.Context) ([]domain.ModelEntry, error) {
	// Ollama's library API returns model entries with metadata.
	// We fetch the top models and convert them to our internal format.
	entries, err := fetchOllamaLibrary(ctx)
	if err != nil {
		slog.Warn("ollama fetch failed, returning empty", "component", "registry", "error", err)
		return []domain.ModelEntry{}, nil
	}

	// Sort by popularity (downloads descending)
	sort.SliceStable(entries, func(i, j int) bool {
		return entries[i].Downloads > entries[j].Downloads
	})

	// Apply limit if set
	if f.MaxModels > 0 && len(entries) > f.MaxModels {
		entries = entries[:f.MaxModels]
	}

	slog.Info("loaded models from ollama library", "component", "registry", "count", len(entries))
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

// fetchOllamaLibrary queries the Ollama API for available domain.
func fetchOllamaLibrary(ctx context.Context) ([]domain.ModelEntry, error) {
	client := &http.Client{Timeout: 5 * time.Second}

	// Ollama doesn't have a single "list all models" API that returns everything.
	// The best public approach: use the search API with common model families.
	// See: https://github.com/ollama/ollama/blob/main/docs/api.md
	// We query for popular model families to build a comprehensive list.

	searchQueries := []string{
		"llama", "qwen", "mistral", "gemma", "phi",
		"deepseek", "mixtral", "llava", "codellama",
	}

	// Use channels to collect results from goroutines
	type searchResult struct {
		entries []domain.ModelEntry
	}
	resultCh := make(chan searchResult, len(searchQueries))

	// Semaphore to limit concurrency to 3
	sem := make(chan struct{}, 3)

	// Fire all requests with concurrency limit
	for _, query := range searchQueries {
		go func(q string) {
			// Acquire semaphore
			sem <- struct{}{}
			defer func() { <-sem }()

			if ctx.Err() != nil {
				resultCh <- searchResult{}
				return
			}

			// Retry logic with exponential backoff
			const maxRetries = 3
			var entries []domain.ModelEntry
			var lastErr error

			for attempt := 0; attempt < maxRetries; attempt++ {
				if ctx.Err() != nil {
					resultCh <- searchResult{}
					return
				}

				url := fmt.Sprintf("https://ollama.com/api/library/search?q=%s&limit=25", q)
				req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
				if err != nil {
					lastErr = err
					break
				}
				req.Header.Set("Accept", "application/json")
				req.Header.Set("User-Agent", "KnowURLLM/1.0")

				resp, err := client.Do(req)
				if err != nil {
					lastErr = err
					// Retry on network errors
					if attempt < maxRetries-1 {
						backoff := time.Duration(1<<uint(attempt)) * time.Second
						time.Sleep(backoff)
					}
					continue
				}

				body, readErr := io.ReadAll(resp.Body)
				resp.Body.Close()
				if readErr != nil {
					lastErr = readErr
					if attempt < maxRetries-1 {
						backoff := time.Duration(1<<uint(attempt)) * time.Second
						time.Sleep(backoff)
					}
					continue
				}

				// Check for rate limiting (429) or service unavailable (503)
				if resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode == http.StatusServiceUnavailable {
					lastErr = fmt.Errorf("HTTP %d", resp.StatusCode)
					if attempt < maxRetries-1 {
						backoff := time.Duration(1<<uint(attempt)) * time.Second
						time.Sleep(backoff)
					}
					continue
				}

				if resp.StatusCode != http.StatusOK {
					lastErr = fmt.Errorf("HTTP %d", resp.StatusCode)
					break
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
					lastErr = err
					if attempt < maxRetries-1 {
						backoff := time.Duration(1<<uint(attempt)) * time.Second
						time.Sleep(backoff)
					}
					continue
				}

				// Success - process results
				for _, m := range libResp.Models {
					entry := domain.ModelEntry{
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
					if mmlu, elo, ifeval, gsm8k, arc, found := lookupBenchmarks(m.Name); found {
						entry.MMLUScore = mmlu
						entry.ArenaELO = elo
						entry.IFEvalScore = ifeval
						entry.GSM8KScore = gsm8k
						entry.ARCScore = arc
					}

					entries = append(entries, entry)
				}

				// Success - break retry loop
				lastErr = nil
				break
			}

			if lastErr != nil {
				slog.Warn("ollama search query failed after retries", "component", "registry", "query", q, "retries", maxRetries, "error", lastErr)
			}

			resultCh <- searchResult{entries: entries}
		}(query)
	}

	// Collect and deduplicate results
	seen := make(map[string]bool)
	var allEntries []domain.ModelEntry

	for i := 0; i < len(searchQueries); i++ {
		select {
		case result := <-resultCh:
			for _, entry := range result.entries {
				if seen[entry.ID] {
					continue
				}
				seen[entry.ID] = true
				allEntries = append(allEntries, entry)
			}
		case <-ctx.Done():
			break
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
func hfModelToEntry(raw hfModel) domain.ModelEntry {
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
	var mmluScore, arenaELO, ifevalScore, gsm8kScore, arcScore float64
	if mmlu, elo, ifeval, gsm8k, arc, found := lookupBenchmarks(raw.Name); found {
		mmluScore = mmlu
		arenaELO = elo
		ifevalScore = ifeval
		gsm8kScore = gsm8k
		arcScore = arc
	}

	// Validate MoE fields
	isMoE := raw.IsMoE
	activeParams := raw.ActiveParams
	if isMoE && activeParams == 0 {
		// MoE flag set but no active params - disable MoE
		isMoE = false
		slog.Warn("model has IsMoE=true but ActiveParams=0, disabling MoE", "component", "registry", "model", raw.Name)
	}
	if activeParams > raw.ParamsRaw && activeParams > 0 {
		// Active params exceed total params - clamp
		slog.Warn("model has ActiveParams > ParameterCount, clamping", "component", "registry", "model", raw.Name, "activeParams", activeParams, "parameterCount", raw.ParamsRaw)
		activeParams = raw.ParamsRaw
	}

	return domain.ModelEntry{
		ID:             raw.Name,
		DisplayName:    displayName,
		ModelSizeBytes: modelSizeBytes,
		Quantization:   raw.Quantization,
		ContextLength:  raw.ContextLength,
		Source:         "huggingface",
		MMLUScore:      mmluScore,
		ArenaELO:       arenaELO,
		IFEvalScore:    ifevalScore,
		GSM8KScore:     gsm8kScore,
		ARCScore:       arcScore,
		Downloads:      raw.HFDownloads,
		URL:            url,
		Tags:           tags,
		IsMoE:          isMoE,
		ActiveParams:   activeParams,
		ParameterCount: raw.ParamsRaw,
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
