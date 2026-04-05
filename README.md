# KnowURLLM

> A Go-based TUI tool that detects your hardware, fetches LLM models from Hugging Face and Ollama, ranks them by hardware compatibility and benchmark quality, and displays results in an interactive terminal interface.

## Features

- **Automatic hardware detection** — CPU cores, RAM, GPU VRAM (NVIDIA via NVML/AMD ROCm, Apple Silicon unified memory)
- **Model registry aggregation** — fetches from both Hugging Face and Ollama APIs with fuzzy model name matching
- **Intelligent scoring** — ranks models by hardware fit (50%), estimated throughput (30%), and benchmark quality (20%)
- **Embedded benchmark database** — 200+ models with MMLU and Chatbot Arena ELO scores, auto-updated from live leaderboards
- **Interactive TUI** — searchable, filterable table with live detail panels showing score breakdowns
- **Cross-platform** — Linux, macOS (Intel + Apple Silicon), Windows

## Installation

### From source

```bash
git clone https://github.com/KnowURLLM/knowurllm.git
cd knowurllm
go build -o knowurllm ./cmd/knowurllm/
```

### Using the build script

```bash
bash scripts/build.sh
# Outputs cross-compiled binaries in dist/
```

### Using go install

```bash
go install github.com/kevo-1/KnowURLLM/cmd/knowurllm@latest
```

## Usage

```bash
knowurllm
```

The tool will:

1. **Detect your hardware** — CPU cores, RAM, GPU(s) and VRAM
2. **Fetch available models** — from Hugging Face and Ollama registries
3. **Score and rank** — using hardware fit, estimated throughput, and benchmark quality
4. **Launch the TUI** — browse, search, filter, and select models

When you select a model and press **Enter**, it prints details and the `ollama run` command if applicable.

### Key Bindings

| Key            | Action                                           |
| -------------- | ------------------------------------------------ |
| `↑` / `k`      | Move up in the list                              |
| `↓` / `j`      | Move down in the list                            |
| `/`            | Open search                                      |
| `esc`          | Clear search / exit search mode                  |
| `v`            | Toggle VRAM-only filter                          |
| `enter`        | Select highlighted model                         |
| `?`            | Toggle help panel                                |
| `q` / `ctrl+c` | Quit                                             |

## Scoring Formula

Models are scored on a 0–100 scale using four weighted components inspired by [llmfit](https://github.com/AlexsJones/llmfit):

| Component        | Weight (General) | Description                                           |
| ---------------- | ---------------- | ----------------------------------------------------- |
| **Fit**          | 40%              | How comfortably the model fits in VRAM + system RAM   |
| **Speed**        | 30%              | Estimated throughput based on backend and model size  |
| **Quality**      | 20%              | Benchmark scores (MMLU, Chatbot Arena ELO) + signals  |
| **Context**      | 10%              | Context window capability for the use case            |

Weights change based on the selected **use-case profile**:

| Profile      | Quality | Speed | Fit   | Context | Best For                          |
| ------------ | ------- | ----- | ----- | ------- | --------------------------------- |
| **General**  | 20%     | 30%   | 40%   | 10%     | Balanced everyday use             |
| **Coding**   | 25%     | 35%   | 25%   | 15%     | Code generation, needs speed      |
| **Reasoning**| 35%     | 20%   | 30%   | 15%     | Complex reasoning, quality-first  |
| **Chat**     | 15%     | 40%   | 35%   | 10%     | Conversational, responsiveness    |
| **Multimodal**| 30%    | 25%   | 30%   | 15%     | Vision/audio, balanced            |
| **Embedding**| 20%     | 35%   | 35%   | 10%     | Batch processing, throughput      |

### Fit Score (Hardware Fit)

| Category    | Score Range | Description                                        |
| ----------- | ----------- | -------------------------------------------------- |
| **Perfect** | 95–100      | Fits in VRAM with 50%+ headroom                    |
| **Good**    | 80–95       | Fits in VRAM with 20-50% headroom, or needs RAM overflow with ratio ≥1.0 |
| **Marginal**| 65–80       | Tight fit in VRAM (<20% headroom) or needs RAM overflow (ratio 0.85-1.0) |
| **Too Tight**| 50–65      | Very tight fit, needs significant RAM overflow (ratio 0.7-0.85) |
| **Excluded**| 0           | **Won't run** — model won't fit in available memory (ratio < 0.7) |

### Speed Estimation

Speed is estimated using backend-specific baselines (tokens/sec):

| Backend | Baseline (tok/s) |
| ------- | ---------------- |
| **CUDA** (NVIDIA) | 220 |
| **ROCm** (AMD)    | 180 |
| **Metal** (Apple) | 160 |
| **SYCL** (Intel)  | 100 |
| **CPU ARM**       | 90  |
| **CPU x86**       | 70  |

Formula: `(backend_baseline / model_size_gb) × efficiency_factor(0.55) × quant_factor`

Penalties:
- CPU offload (partial): ×0.5
- CPU-only (no GPU): ×0.3
- MoE expert switching: ×0.8

### Quality Score

Quality is calculated from:
- **80% benchmark performance**: MMLU and Chatbot Arena ELO scores
- **20% parameter count**: Larger models get a bonus (logarithmic scaling)
- **Quantization adjustment**: Higher quantization (Q8, FP16) gets a small bonus, lower quantization (Q3, Q2) gets a penalty

| Available Data        | Base Formula                               |
| --------------------- | ------------------------------------------ |
| Both MMLU + Arena ELO | `0.6 × MMLU + 0.4 × normalized_ELO`        |
| MMLU only             | `MMLU`                                     |
| Arena ELO only        | `normalized_ELO` (scaled 800–1300 → 0–100) |
| Neither               | `50` (neutral default)                     |

Final: `0.8 × base_quality + 0.2 × param_bonus + quant_adjustment`

### Context Score

| Context Length | Score | Category  |
| -------------- | ----- | --------- |
| ≥ 128k tokens  | 100   | Perfect   |
| 32k–128k       | 75–100| Good      |
| 8k–32k         | 50–75 | Marginal  |
| < 8k           | 0–50  | Too Tight |

## Benchmark Database

KnowURLLM ships with an embedded benchmark database (`internal/registry/data/benchmarks.json`) containing MMLU and Chatbot Arena ELO scores for 200+ models. Scores are used in the quality component of the scoring formula.

### Updating Benchmarks

The project includes a scraper to keep benchmarks current by fetching live data from public leaderboards:

```bash
go run scripts/update_benchmarks.go
```

This script:

1. **Auto-discovers trending models** from HuggingFace API (sorted by download count)
2. **Fetches Arena Hard scores** from the `lmarena-ai/arena-leaderboard` space
3. **Fetches MMLU-PRO scores** from the HuggingFace Open LLM Leaderboard dataset
4. **Merges** new data with existing entries (manually-curated scores are never overwritten)
5. **Writes** the updated JSON with a current version date

| Flag                | Description                                  | Default |
| ------------------- | -------------------------------------------- | ------- |
| `--no-discover`     | Skip auto-discovery, use curated list only   | false   |
| `--limit N`         | Max trending models to discover              | 30      |
| `--min-downloads N` | Minimum download count for discovered models | 10000   |

Data sources:

- [LMSYS Chatbot Arena](https://lmarena.ai/leaderboard) — Arena Hard leaderboard CSV
- **Archived** [Open LLM Leaderboard](https://huggingface.co/spaces/open-llm-leaderboard/open_llm_leaderboard) — MMLU-PRO scores
- [HuggingFace trending models](https://huggingface.co/api/models) — auto-discovered by download count

## Environment Variables

| Variable        | Purpose                                       | Default                  |
| --------------- | --------------------------------------------- | ------------------------ |
| `HF_TOKEN`      | Hugging Face API token for higher rate limits | (none)                   |
| `OLLAMA_HOST`   | Local Ollama API address                      | `http://localhost:11434` |
| `MAX_MODELS`    | Max models to fetch per registry              | `200`                    |
| `FETCH_TIMEOUT` | Timeout per registry call                     | `30s`                    |

## Platform Support

| Platform            | CPU | RAM | GPU                       |
| ------------------- | --- | --- | ------------------------- |
| Linux               | Yes | Yes | NVIDIA (NVML), AMD (ROCm) |
| macOS Intel         | Yes | Yes | Intel iGPU                |
| macOS Apple Silicon | Yes | Yes | Unified Memory (Metal)    |
| Windows             | Yes | Yes | NVIDIA (NVML)             |

## Project Structure

```
KnowURLLM/
├── cmd/knowurllm/main.go             # CLI entrypoint
├── internal/
│   ├── models/                       # Shared data structures (ModelEntry, ModelScore, etc.)
│   ├── hardware/                     # Hardware detection (CPU, RAM, GPU via NVML/gopsutil)
│   ├── registry/                     # HuggingFace + Ollama fetcher, benchmark lookup
│   │   ├── fetcher.go                # Registry aggregation logic
│   │   ├── benchmarks.go             # Embedded benchmark database + fuzzy matching
│   │   └── normalize.go              # Model name normalization
│   ├── scorer/                       # Ranking and scoring logic
│   │   ├── formula.go                # Hardware fit, throughput, quality formulas
│   │   └── rank.go                   # Model ranking and sorting
│   └── tui/                          # Bubble Tea interactive UI
│       ├── app.go                    # TUI application setup
│       ├── table.go                  # Model table rendering
│       ├── detail.go                 # Detail panel with score breakdown
│       ├── search.go                 # Search/filter logic
│       └── styles.go                 # Lipgloss theme definitions
├── scripts/
│   ├── build.sh                      # Cross-platform build script
│   └── update_benchmarks.go          # Live benchmark updater (auto-discovers trending models)
├── internal/registry/data/
│   └── benchmarks.json               # Embedded benchmark database (MMLU + Arena ELO)
├── testdata/                         # Mock API responses for testing
└── ARCHITECTURE.md                   # Detailed architecture docs
```

## Architecture

KnowURLLM follows a layered architecture:

```
CLI (main.go)
  → Hardware Detection (hardware/)
  → Model Fetching (registry/fetcher.go)
     ├─ HuggingFace API
     └─ Ollama API
  → Benchmark Enrichment (registry/benchmarks.go)
     └─ Embedded benchmarks.json with fuzzy matching
  → Scoring (scorer/)
     ├─ Hardware Fit (VRAM + RAM capacity)
     ├─ Throughput (tokens/sec estimation)
     └─ Quality (MMLU + Arena ELO)
  → Ranking (scorer/rank.go)
  → TUI Display (tui/)
     ├─ Searchable table
     ├─ Detail panel with score breakdown
     └─ Source/VRAM filters
```

## Testing

```bash
go test ./...
```

Tests cover hardware detection, model parsing, scoring formulas, benchmark enrichment, and TUI rendering.

## License

MIT
