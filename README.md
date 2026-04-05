# KnowURLLM

> A Go-based TUI tool that detects your hardware, fetches LLM models from Hugging Face and Ollama, ranks them by hardware compatibility and benchmark quality, and displays results in an interactive terminal interface.

## Features

- **Automatic hardware detection** — CPU cores, RAM, GPU VRAM (NVIDIA via nvidia-smi/NVML, AMD ROCm, Apple Silicon unified memory)
- **Model registry aggregation** — 960+ models from an embedded Hugging Face database, with optional live Ollama API queries
- **Intelligent scoring** — ranks models by hardware fit, estimated throughput, benchmark quality, and context capability
- **Embedded benchmark database** — curated MMLU and Chatbot Arena ELO scores with fuzzy model name matching
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
2. **Fetch available models** — from the embedded registry (960+ models)
3. **Score and rank** — using hardware fit, estimated throughput, benchmark quality, and context
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

Models are scored on a 0–100 composite scale using four weighted components inspired by [llmfit](https://github.com/AlexsJones/llmfit):

| Component        | Weight (General) | Description                                           |
| ---------------- | ---------------- | ----------------------------------------------------- |
| **Quality**      | 35%              | Benchmark scores (MMLU + Chatbot Arena ELO)           |
| **Speed**        | 25%              | Estimated throughput (tokens/sec) based on hardware   |
| **Fit**          | 25%              | How comfortably the model fits in VRAM + system RAM   |
| **Context**      | 15%              | Context window capability for the use case            |

Weights change based on the selected **use-case profile**:

| Profile      | Quality | Speed | Fit   | Context | Best For                          |
| ------------ | ------- | ----- | ----- | ------- | --------------------------------- |
| **General**  | 35%     | 25%   | 25%   | 15%     | Balanced everyday use             |
| **Coding**   | 45%     | 25%   | 20%   | 10%     | Code generation, quality-first    |
| **Reasoning**| 55%     | 15%   | 20%   | 10%     | Complex reasoning, quality-critical |
| **Chat**     | 25%     | 35%   | 25%   | 15%     | Conversational, responsiveness    |
| **Multimodal**| 40%    | 20%   | 30%   | 10%     | Vision/audio, hardware-heavy      |
| **Embedding**| 50%     | 30%   | 20%   | 0%      | Batch processing, quality-critical |

### Fit Score (Hardware Fit)

The scorer first detects the **run mode** for each model:
- **GPU** — model fits entirely in VRAM
- **MoE** — Mixture-of-Experts: active experts in VRAM, inactive in RAM
- **CPU+GPU** — model spills from VRAM into system RAM
- **CPU** — no GPU available, system RAM only

It then dynamically selects the highest-quality quantization (Q8_0 → Q6_K → Q5_K_M → Q4_K_M → Q3_K_M → Q2_K) that fits available memory.

| Category    | Score | Description                                        |
| ----------- | ----- | -------------------------------------------------- |
| **Perfect** | 100   | Fits in VRAM with ≥10% headroom                    |
| **Good**    | 75    | Fits in VRAM (tight), or MoE/CPU+GPU mode          |
| **Marginal**| 40    | CPU-only inference                                 |
| **Too Tight**| 0    | **Excluded** — model won't fit in available memory  |

### Speed Estimation

Speed is estimated using a **two-path approach**:

**Primary path (bandwidth-based)** — when a known GPU is detected:

```
TPS = (GPU_bandwidth_GB/s / model_size_GB) × 0.85 efficiency
```

The GPU bandwidth is looked up from a table of 35+ known GPUs (NVIDIA RTX/A-series/H-series/GTX 16-series, Apple M-series, AMD RX). For example:
- RTX 4090: 1008 GB/s
- RTX 3090: 936 GB/s
- Apple M2 Max: 400 GB/s (unified memory)
- RX 7900 XTX: 960 GB/s
- GTX 1650: 128 GB/s

The lookup prefers longer/more specific matches (e.g., "GTX 1650 SUPER" before "GTX 1650") to avoid false positives.

**Fallback path (parameter-count-based)** — when GPU bandwidth is unknown:

```
TPS = K_backend / params_billions × quant_multiplier × mode_penalty
```

| Backend | K Constant |
| ------- | ---------- |
| **CUDA** (NVIDIA) | 30.0 |
| **Metal** (Apple) | 20.0 |
| **ROCm** (AMD)    | 18.0 |
| **CPU x86**       | 8.0  |
| **CPU ARM**       | 10.0 |

Mode penalties:
- CPU+GPU offload: ×0.5
- CPU-only (no GPU): ×0.3
- MoE expert switching: ×0.8

The final TPS score is normalized to 0–100: `score = min(100, log2(TPS + 1) × 12.5)`

### Quality Score

Quality is calculated from available benchmark data:

| Available Data        | Formula                                    |
| --------------------- | ------------------------------------------ |
| Both MMLU + Arena ELO | `0.6 × MMLU + 0.4 × normalized_ELO`        |
| MMLU only             | `MMLU`                                     |
| Arena ELO only        | `normalized_ELO` (scaled 800–1300 → 0–100) |
| Neither               | `50` (neutral default)                     |

Arena ELO is normalized from the 800–1300 range to 0–100. When both benchmarks are available, MMLU gets 60% weight and Arena ELO gets 40% weight.

### Context Score

Context capability is scored based on the model's maximum context length:

| Context Length | Score |
| -------------- | ----- |
| ≥ 128k tokens  | 100   |
| ≥ 64k tokens   | 85    |
| ≥ 32k tokens   | 70    |
| ≥ 16k tokens   | 55    |
| ≥ 8k tokens    | 40    |
| < 8k tokens    | 20    |

## Benchmark Database

KnowURLLM ships with an embedded benchmark database (`internal/registry/data/benchmarks.json`) containing curated MMLU and Chatbot Arena ELO scores for known models. Models without benchmark data receive a neutral quality score of 50. Scores are used in the quality component of the scoring formula.

Benchmark enrichment uses regex-based fuzzy model name matching that handles version suffixes (`-v0.2`, `_v1`, etc.) and size-specific placeholders (a 3B model won't match an 8B entry).

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

| Platform            | CPU | RAM | GPU                               |
| ------------------- | --- | --- | --------------------------------- |
| Linux               | Yes | Yes | NVIDIA (nvidia-smi/NVML), AMD (ROCm/sysfs) |
| macOS Intel         | Yes | Yes | Intel iGPU                        |
| macOS Apple Silicon | Yes | Yes | Unified Memory (Metal)            |
| Windows             | Yes | Yes | NVIDIA (nvidia-smi)               |

## Project Structure

```
KnowURLLM/
├── cmd/knowurllm/main.go             # CLI entrypoint (5-step pipeline)
├── internal/
│   ├── models/                       # Shared data structures (contract layer)
│   │   └── models.go                 # HardwareProfile, GPUInfo, ModelEntry, ModelScore, RankResult
│   ├── hardware/                     # Hardware detection module
│   │   ├── detect.go                 # Main Detect() orchestrator
│   │   ├── cache.go                  # sync.Once caching + ResetCache()
│   │   ├── detector.go               # HardwareDetector interface + MockDetector
│   │   ├── cpu.go                    # CPU detection (gopsutil + platform fallbacks)
│   │   ├── memory.go                 # RAM detection (gopsutil + platform fallbacks)
│   │   ├── gpu.go                    # GPU coordinator (dispatches by platform)
│   │   ├── gpu_nvidia.go             # NVIDIA detection via nvidia-smi
│   │   ├── gpu_nvidia_nvml.go        # NVIDIA NVML direct detection (Linux only)
│   │   ├── gpu_amd.go                # AMD GPU detection (Linux)
│   │   ├── gpu_apple.go              # Apple Silicon GPU detection
│   │   ├── gpu_bandwidth.go          # Memory bandwidth lookup table (35+ GPUs incl. GTX 16-series)
│   │   ├── gpu_utils.go              # VRAM calculation, vendor normalization
│   │   ├── error.go                  # GPUDetectionError + helpers
│   │   └── hardware_test.go          # Unit + integration tests
│   ├── registry/                     # Model registry + benchmark module
│   │   ├── fetcher.go                # Embedded hf_models.json + Ollama API
│   │   ├── benchmarks.go             # Embedded benchmarks.json + fuzzy matching
│   │   ├── normalize.go              # Model name normalization, quantization parsing
│   │   └── data/
│   │       ├── hf_models.json        # 960+ embedded models
│   │       └── benchmarks.json       # Curated MMLU + Arena ELO scores
│   ├── scorer/                       # Scoring and ranking module
│   │   ├── scorer.go                 # Scorer struct + validation
│   │   ├── formula.go                # Run mode detection, quant selection, scoring
│   │   ├── rank.go                   # Ranking, sorting, filtering
│   │   ├── profiles.go               # ValidProfiles() helper
│   │   └── scorer_test.go            # Comprehensive scorer tests
│   └── tui/                          # Bubble Tea interactive UI
│       ├── app.go                    # App setup + Run()
│       ├── model.go                  # Bubble Tea model state + key bindings
│       ├── view.go                   # Render layouts (small/normal/wide)
│       ├── table.go                  # Table rendering
│       ├── detail.go                 # Detail panel (condensed + expanded)
│       ├── search.go                 # Search input component
│       ├── update.go                 # Event handling, filtering
│       └── styles.go                 # Lipgloss theme
├── scripts/
│   ├── build.sh                      # Cross-platform build script
│   └── update_benchmarks.go          # Live benchmark updater
├── testdata/                         # Mock API responses for testing
└── tests/e2e/                        # End-to-end tests
    └── hardware_e2e_test.go          # Real hardware + pipeline tests
```

## Architecture

KnowURLLM follows a layered architecture with `internal/models/` as the contract layer:

```
CLI (cmd/knowurllm/main.go)
  → Hardware Detection (hardware/)
     ├─ CPU (gopsutil → /proc/cpuinfo → sysctl → wmic)
     ├─ Memory (gopsutil → platform fallbacks)
     └─ GPU (nvidia-smi/NVML, AMD ROCm, Apple Metal)
  → Model Fetching (registry/fetcher.go)
     ├─ Embedded hf_models.json (960+ models, go:embed)
     └─ Ollama API (optional, parallel concurrent queries across 9 model families)
  → Benchmark Enrichment (registry/benchmarks.go)
     └─ Embedded benchmarks.json with fuzzy name matching
  → Scoring (scorer/)
     ├─ Run mode detection (GPU / MoE / CPU+GPU / CPU)
     ├─ Dynamic quantization selection (Q8_0 → Q2_K)
     ├─ Speed estimation (bandwidth-based or param-count fallback)
     ├─ Quality scoring (MMLU + Arena ELO)
     ├─ Context scoring (6 tiers: 8k → 128k+)
     └─ Use-case weighted composite (General/Coding/Reasoning/Chat/Multimodal/Embedding)
  → Ranking (scorer/rank.go)
     └─ Multi-level tiebreaker sorting (FitCategory > Quality > Downloads > Name)
  → TUI Display (tui/)
     ├─ Responsive layouts (small <60w, normal 60-119w, wide ≥120w)
     ├─ Searchable table with live filtering
     ├─ Expandable detail panel with score breakdown
     ├─ VRAM-only filter toggle
     └─ Help panel with key bindings
```

## Testing

```bash
go test ./...
```

Tests cover hardware detection, model parsing, scoring formulas, benchmark enrichment, and TUI rendering.

## License

MIT
