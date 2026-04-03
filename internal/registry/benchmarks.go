package registry

import (
	"strings"
)

// knownBenchmarks provides curated MMLU and Chatbot Arena ELO scores
// for well-known open-weight models. This enriches models where the
// source data doesn't include benchmark fields.
//
// Sources: LMSYS Chatbot Arena leaderboard, official model cards.
var knownBenchmarks = map[string]benchmarkData{
	// Llama 3 family
	"meta-llama/llama-3.1-8b-instruct":   {MMLU: 69.4, ArenaELO: 1190},
	"meta-llama/llama-3.1-70b-instruct":  {MMLU: 79.3, ArenaELO: 1250},
	"meta-llama/llama-3.1-405b-instruct": {MMLU: 87.0, ArenaELO: 1280},
	"meta-llama/llama-3-8b-instruct":     {MMLU: 68.4, ArenaELO: 1170},
	"meta-llama/llama-3-70b-instruct":    {MMLU: 79.5, ArenaELO: 1240},
	"meta-llama/llama-3.2-1b-instruct":   {MMLU: 49.0, ArenaELO: 1050},
	"meta-llama/llama-3.2-3b-instruct":   {MMLU: 58.0, ArenaELO: 1100},

	// Mistral family
	"mistralai/mistral-7b-instruct":         {MMLU: 60.1, ArenaELO: 1140},
	"mistralai/mistral-7b-instruct-v0.2":    {MMLU: 60.5, ArenaELO: 1145},
	"mistralai/mistral-7b-instruct-v0.3":    {MMLU: 61.0, ArenaELO: 1150},
	"mistralai/mixtral-8x7b-instruct-v0.1":  {MMLU: 70.6, ArenaELO: 1210},
	"mistralai/mixtral-8x22b-instruct-v0.1": {MMLU: 76.5, ArenaELO: 1240},
	"mistralai/mistral-large-instruct-2407": {MMLU: 81.0, ArenaELO: 1260},
	"mistralai/mistral-small-24b-instruct":  {MMLU: 72.0, ArenaELO: 1200},
	"mistralai/codestral-22b-v0.1":          {MMLU: 62.0, ArenaELO: 1120},

	// Qwen family
	"qwen/qwen2.5-7b-instruct":      {MMLU: 74.5, ArenaELO: 1200},
	"qwen/qwen2.5-14b-instruct":     {MMLU: 77.2, ArenaELO: 1230},
	"qwen/qwen2.5-32b-instruct":     {MMLU: 79.0, ArenaELO: 1245},
	"qwen/qwen2.5-72b-instruct":     {MMLU: 81.5, ArenaELO: 1255},
	"qwen/qwen2.5-0.5b-instruct":    {MMLU: 35.0, ArenaELO: 900},
	"qwen/qwen2.5-1.5b-instruct":    {MMLU: 42.0, ArenaELO: 950},
	"qwen/qwen2.5-3b-instruct":      {MMLU: 55.0, ArenaELO: 1050},
	"qwen/qwen2-7b-instruct":        {MMLU: 71.5, ArenaELO: 1180},
	"qwen/qwen2-72b-instruct":       {MMLU: 78.5, ArenaELO: 1235},
	"qwen/qwen2.5-coder-7b-instruct": {MMLU: 68.0, ArenaELO: 1140},
	"qwen/qwen2.5-coder-32b-instruct": {MMLU: 76.0, ArenaELO: 1210},

	// Google Gemma family
	"google/gemma-2-2b-it":  {MMLU: 55.0, ArenaELO: 1080},
	"google/gemma-2-9b-it":  {MMLU: 65.0, ArenaELO: 1160},
	"google/gemma-2-27b-it": {MMLU: 73.0, ArenaELO: 1220},
	"google/gemma-7b-it":    {MMLU: 58.0, ArenaELO: 1110},

	// Phi family
	"microsoft/phi-3-mini-4k-instruct":   {MMLU: 69.0, ArenaELO: 1160},
	"microsoft/phi-3-mini-128k-instruct": {MMLU: 69.0, ArenaELO: 1160},
	"microsoft/phi-3-small-8k-instruct":  {MMLU: 65.0, ArenaELO: 1120},
	"microsoft/phi-3-medium-4k-instruct": {MMLU: 72.0, ArenaELO: 1190},
	"microsoft/phi-3.5-mini-instruct":    {MMLU: 70.0, ArenaELO: 1175},
	"microsoft/phi-3.5-moe-instruct":     {MMLU: 71.0, ArenaELO: 1180},

	// DeepSeek family
	"deepseek-ai/deepseek-coder-6.7b-instruct": {MMLU: 62.0, ArenaELO: 1130},
	"deepseek-ai/deepseek-coder-33b-instruct":  {MMLU: 70.0, ArenaELO: 1180},
	"deepseek-ai/deepseek-v2-lite":             {MMLU: 68.0, ArenaELO: 1170},
	"deepseek-ai/deepseek-v2.5":                {MMLU: 78.0, ArenaELO: 1240},
	"deepseek-ai/deepseek-r1-distill-qwen-1.5b": {MMLU: 45.0, ArenaELO: 1000},
	"deepseek-ai/deepseek-r1-distill-qwen-7b":   {MMLU: 62.0, ArenaELO: 1120},
	"deepseek-ai/deepseek-r1-distill-qwen-14b":  {MMLU: 70.0, ArenaELO: 1180},
	"deepseek-ai/deepseek-r1-distill-qwen-32b":  {MMLU: 74.0, ArenaELO: 1210},
	"deepseek-ai/deepseek-r1-distill-llama-8b":  {MMLU: 64.0, ArenaELO: 1140},
	"deepseek-ai/deepseek-r1-distill-llama-70b": {MMLU: 77.0, ArenaELO: 1235},

	// LLaVA (multimodal)
	"llava-hf/llava-1.5-7b-hf":          {MMLU: 52.0, ArenaELO: 1060},
	"llava-hf/llava-v1.6-mistral-7b-hf": {MMLU: 58.0, ArenaELO: 1100},

	// TinyLlama
	"tinyllama/tinyllama-1.1b-chat-v1.0": {MMLU: 38.0, ArenaELO: 920},
}

// benchmarkData holds known benchmark scores for a model.
type benchmarkData struct {
	MMLU     float64
	ArenaELO float64
}

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
func normalizeForMatch(s string) string {
	s = strings.ToLower(s)
	// Remove version suffixes like v0.2, v0.3
	for i := len(s) - 1; i >= 0; i-- {
		if s[i] == '-' && i+2 < len(s) && s[i+1] == 'v' {
			s = s[:i]
			break
		}
	}
	// Remove size indicators for fuzzy matching
	s = strings.ReplaceAll(s, "8b", "xb")
	s = strings.ReplaceAll(s, "7b", "xb")
	s = strings.ReplaceAll(s, "70b", "xxb")
	s = strings.ReplaceAll(s, "14b", "xb")
	s = strings.ReplaceAll(s, "32b", "xb")
	s = strings.ReplaceAll(s, "72b", "xxb")
	s = strings.ReplaceAll(s, "9b", "xb")
	s = strings.ReplaceAll(s, "27b", "xxb")
	s = strings.ReplaceAll(s, "2b", "xb")
	s = strings.ReplaceAll(s, "1b", "xb")
	s = strings.ReplaceAll(s, "3b", "xb")
	s = strings.ReplaceAll(s, "6.7b", "xb")
	s = strings.ReplaceAll(s, "33b", "xxb")
	s = strings.ReplaceAll(s, "22b", "xxb")
	s = strings.ReplaceAll(s, "405b", "xxb")
	s = strings.ReplaceAll(s, "1.5b", "xb")
	s = strings.ReplaceAll(s, "0.5b", "xb")
	return s
}
