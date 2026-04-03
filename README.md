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

Models are scored on a 0–100 scale using three weighted components:

| Component        | Weight | Description                                           |
| ---------------- | ------ | ----------------------------------------------------- |
| **Hardware Fit** | 50%    | How comfortably the model fits in VRAM + system RAM   |
| **Throughput**   | 30%    | Estimated tokens/sec based on model size and hardware |
| **Quality**      | 20%    | Benchmark scores (MMLU, Chatbot Arena ELO)            |

### Hardware Fit

| Fit Ratio | Score  | Behavior                                           |
| --------- | ------ | -------------------------------------------------- |
| < 0.7     | 0      | **Excluded** — model won't fit in available memory |
| 0.7–1.0   | 50–80  | Fits with RAM overflow (VRAM + 50% system RAM)     |
| 1.0–1.3   | 80–100 | Fits fully in memory                               |
| ≥ 1.3     | 100    | Fits in VRAM with headroom                         |

### Throughput

Estimated using a log-scale formula based on:

- **GPU inference**: `(VRAM_GB × 80) / (model_size_GB × quant_factor)`
- **CPU inference**: `(cpu_cores × 3.0) / (model_size_GB × quant_factor)`

Quantization factors: Q4_K_M = 1.0× (baseline), Q5 = 0.85×, Q3 = 0.7×, Q2 = 0.6×, FP16 = 0.4×, FP32 = 0.2×

### Quality

| Available Data        | Formula                                    |
| --------------------- | ------------------------------------------ |
| Both MMLU + Arena ELO | `0.6 × MMLU + 0.4 × normalized_ELO`        |
| MMLU only             | `MMLU`                                     |
| Arena ELO only        | `normalized_ELO` (scaled 800–1300 → 0–100) |
| Neither               | `50` (neutral default)                     |

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
