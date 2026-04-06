# KnowURLLM

> A Go-based TUI tool that detects your hardware, fetches LLM models from Hugging Face and Ollama, ranks them by **Arena-style quality tiers** with **Bayesian fusion** of benchmarks, then filters by hardware compatibility, and displays results in an interactive terminal interface.

## Features

- **Automatic hardware detection** — CPU cores, RAM, GPU VRAM (NVIDIA via nvidia-smi/NVML, AMD ROCm, Apple Silicon unified memory)
- **Arena-style quality ranking** — Bayesian fusion of 5+ benchmarks (Arena ELO, MMLU-PRO, IFEval, GSM8K, ARC)
- **Quality tiers** — S/A/B/C/D tier classification with confidence intervals
- **Category-specific scoring** — General Chat, Coding, Reasoning, Long Context, Multimodal
- **Hardware compatibility filtering** — Only shows models that run on your hardware (GPU/MoE/CPU+GPU/CPU)
- **Dynamic quantization** — Auto-selects best quant (Q8_0 → Q2_K) that fits your memory; preserves explicit quantization when provided
- **MoE support** — 146+ Mixture-of-Experts models with active parameter tracking and validation
- **Correct MoE VRAM calculation** — Quantization-aware active parameter size estimation (fixes unit mismatch bug)
- **Interactive TUI** — Searchable, filterable table with tier badges, quality scores, and expandable detail panels with snapshot tests
- **Cross-platform** — Linux, macOS (Intel + Apple Silicon), Windows

## Recent Improvements

### v2.0 (April 2026)

- **Fixed MoE VRAM calculation** — Eliminated hardcoded 0.563 constant; now uses quantization-aware `bytesPerParamForQuant()` for accurate active parameter size estimation
- **Removed code duplication** — Unified GPU bandwidth lookup into single source of truth (`internal/domain/hardware/gpuinfo.go`)
- **Eliminated dead code** — Removed duplicate `internal/models/` package and legacy `internal/scorer/` scoring system
- **Extended benchmark support** — All 5 quality signals now flow through the pipeline (MMLU, ArenaELO, IFEval, GSM8K, ARC)
- **Improved safety** — Replaced unsafe custom float parser with `strconv.ParseFloat` (supports scientific notation)
- **MoE field validation** — Invalid MoE states detected and corrected during ingestion (zero active params, active > total)
- **Enhanced reliability** — Ollama fetch now has concurrency control (max 3) and retry logic with exponential backoff for 429/503 errors
- **Faster timeout** — Ollama HTTP timeout reduced from 30s to 5s for quicker failure detection
- **Structured logging** — All logging migrated from `log.Printf` to `log/slog` with component field for filtering
- **TUI snapshot tests** — Golden file tests added using `charmbracelet/x/exp/golden` for regression-free UI changes

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
2. **Fetch available models** — from the embedded registry (960+ models, 146+ MoE)
3. **Rank by quality** — Arena-style Bayesian fusion → Quality tiers (S/A/B/C/D)
4. **Filter by hardware** — Only models that run on your hardware
5. **Launch the TUI** — browse, search, filter, and select models with tier badges

When you select a model and press **Enter**, it prints details and the `ollama run` command if applicable.

### Key Bindings

| Key            | Action                          |
| -------------- | ------------------------------- |
| `↑` / `k`      | Move up in the list             |
| `↓` / `j`      | Move down in the list           |
| `/`            | Open search                     |
| `esc`          | Clear search / exit search mode |
| `v`            | Toggle VRAM-only filter         |
| `tab`          | Toggle expanded detail panel    |
| `enter`        | Select highlighted model        |
| `?`            | Toggle help panel               |
| `q` / `ctrl+c` | Quit                            |

## Ranking System

KnowURLLM uses a **two-tier ranking system** inspired by [LMSYS Chatbot Arena](https://lmarena.ai/leaderboard):

```
Tier 1: Quality Ranking (Arena-style, hardware-agnostic)
  ↓
Tier 2: Hardware Compatibility Filter + Performance Sub-sort
```

**Unlike composite scoring** (which mixes quality and hardware into one score), this approach:

- ✅ Ranks models by **pure quality** first (what the model can do)
- ✅ Filters out models that **don't run on your hardware**
- ✅ Sub-sorts by **hardware fit** within each quality tier

### Quality Tiers

Models are classified into quality tiers based on **Bayesian fusion** of multiple benchmark signals:

| Tier  | Percentile | Description           | Example Use Cases                          |
| ----- | ---------- | --------------------- | ------------------------------------------ |
| **S** | Top 5%     | State-of-the-art      | Production systems, quality-critical tasks |
| **A** | Top 15%    | Excellent performance | Daily use, coding, reasoning               |
| **B** | Top 35%    | Good capability       | General tasks, balanced performance        |
| **C** | Top 60%    | Moderate performance  | Lightweight use, resource-constrained      |
| **D** | Bottom 40% | Basic capability      | Simple tasks, experimentation              |

**Confidence-aware classification:**

- Models with few benchmarks get wider confidence intervals
- Low-confidence models are conservatively downgraded (e.g., S-tier → A-tier)
- Transparent: shows confidence % in detail panel

### Bayesian Fusion (Quality Scoring)

Instead of simple weighted averages, KnowURLLM uses **Bayesian fusion** to combine multiple benchmark signals with confidence weighting:

**Benchmark signals** (automatically detected from available data):

| Benchmark     | Weight | Confidence | Description                                           |
| ------------- | ------ | ---------- | ----------------------------------------------------- |
| **Arena ELO** | 50%    | 95%        | LMSYS Chatbot Arena (human preference, gold standard) |
| **MMLU-PRO**  | 30%    | 85%        | Academic knowledge & reasoning benchmark              |
| **IFEval**    | 10%    | 60%        | Instruction following capability                      |
| **GSM8K**     | 10%    | 60%        | Mathematical reasoning (grade school math)            |
| **ARC**       | 10%    | 60%        | Science question & answers                            |

**Bayesian fusion formula:**

```
Quality Score = Σ(value × weight × confidence) / Σ(weight × confidence)
```

**Missing data handling:**

- Models without benchmarks get neutral prior (50) with zero confidence
- More benchmarks → higher confidence → more reliable score
- Confidence intervals: ±2 points (high confidence) to ±20 points (low confidence)

### Category-Specific Scoring

Different use cases weight benchmarks differently:

| Category         | Arena ELO | MMLU-PRO | IFEval | GSM8K | ARC | Best For                            |
| ---------------- | --------- | -------- | ------ | ----- | --- | ----------------------------------- |
| **General Chat** | 70%       | 20%      | 10%    | —     | —   | Conversational AI, daily assistance |
| **Coding**       | 40%       | 30%      | —      | 30%   | —   | Code generation, debugging          |
| **Reasoning**    | 30%       | 20%      | —      | 40%   | 10% | Math, logic, multi-step tasks       |
| **Long Context** | 50%       | 50%      | —      | —     | —   | Document analysis, summarization    |
| **Multimodal**   | 60%       | 40%      | —      | —     | —   | Vision, cross-modal tasks           |

### Hardware Compatibility (Filtering)

After quality ranking, models are filtered by hardware compatibility:

**Run mode detection:**

- **GPU** — model fits entirely in VRAM (fastest)
- **MoE** — Mixture-of-Experts: active experts in VRAM, inactive in RAM (146+ models)
- **CPU+GPU** — model spills from VRAM into system RAM (moderate speed)
- **CPU** — no GPU available, system RAM only (slowest)

**Dynamic quantization selection:**
Walks from best quality → most compressed (Q8_0 → Q6_K → Q5_K_M → Q4_K_M → Q3_K_M → Q2_K) and picks the highest quantization that fits available memory.

**Fit classification:**

| Category      | Description                                        | Typical Mode      |
| ------------- | -------------------------------------------------- | ----------------- |
| **Perfect**   | Fits in VRAM with ≥10% headroom                    | GPU               |
| **Good**      | Fits in VRAM (tight), or MoE/CPU+GPU mode          | GPU, MoE, CPU+GPU |
| **Marginal**  | CPU-only inference                                 | CPU               |
| **Too Tight** | **Excluded** — model won't fit in available memory | —                 |

### Speed Estimation

Performance (tokens/sec) is estimated using a **two-path approach**:

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

| Backend           | K Constant |
| ----------------- | ---------- |
| **CUDA** (NVIDIA) | 30.0       |
| **Metal** (Apple) | 20.0       |
| **ROCm** (AMD)    | 18.0       |
| **CPU x86**       | 8.0        |
| **CPU ARM**       | 10.0       |

Mode penalties:

- CPU+GPU offload: ×0.5
- CPU-only (no GPU): ×0.3
- MoE expert switching: ×0.8

The final TPS score is normalized to 0–100: `score = min(100, log2(TPS + 1) × 12.5)`

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

KnowURLLM ships with an embedded benchmark database (`internal/registry/data/benchmarks.json`) containing curated scores from multiple benchmarks:

| Benchmark             | Description                                             | Scale                          |
| --------------------- | ------------------------------------------------------- | ------------------------------ |
| **Chatbot Arena ELO** | LMSYS human preference voting                           | 800–1350 (normalized to 0–100) |
| **MMLU-PRO**          | Massive Multitask Language Understanding (professional) | 0–100                          |
| **IFEval**            | Instruction Following evaluation                        | 0–100                          |
| **GSM8K**             | Grade School Math (8K problems)                         | 0–100                          |
| **ARC**               | AI2 Reasoning Challenge (science QA)                    | 0–100                          |

Models without benchmark data receive a neutral quality score of 50 with zero confidence. Scores are fused using Bayesian methodology (see above).

Benchmark enrichment uses regex-based fuzzy model name matching that handles version suffixes (`-v0.2`, `_v1`, etc.) and size-specific placeholders (a 3B model won't match an 8B entry).

**MoE models:** 146+ Mixture-of-Experts models with active parameter tracking (e.g., Mixtral 8x7B: 46.7B total params, 12.9B active per token).

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

**Note:** Ollama library fetch uses 5s HTTP timeout with retry logic (max 3 attempts) and exponential backoff for 429/503 errors.

## Platform Support

| Platform            | CPU | RAM | GPU                                        |
| ------------------- | --- | --- | ------------------------------------------ |
| Linux               | Yes | Yes | NVIDIA (nvidia-smi/NVML), AMD (ROCm/sysfs) |
| macOS Intel         | Yes | Yes | Intel iGPU                                 |
| macOS Apple Silicon | Yes | Yes | Unified Memory (Metal)                     |
| Windows             | Yes | Yes | NVIDIA (nvidia-smi)                        |

## Project Structure

```
KnowURLLM/
├── cmd/knowurllm/main.go             # CLI entrypoint (uses service layer)
├── internal/
│   ├── domain/                       # Contract layer (pure business logic)
│   │   ├── models.go                 # HardwareProfile, ModelEntry, RankedModel, QualityTier
│   │   ├── filters.go                # FilterOptions + filtering logic
│   │   ├── quality/                  # Arena-style quality scoring
│   │   │   ├── scorer.go             # Main orchestrator, percentile calculation
│   │   │   ├── arena.go              # Arena ELO normalization (800–1350 → 0–100)
│   │   │   ├── benchmarks.go         # Multi-signal extraction (5 benchmarks)
│   │   │   ├── confidence.go         # Bayesian fusion, confidence intervals
│   │   │   ├── categories.go         # Category-specific scoring (5 categories)
│   │   │   └── quality_test.go       # 25+ comprehensive tests
│   │   └── hardware/                 # Hardware compatibility (domain layer)
│   │       ├── fit.go                # Run mode detection, quant selection
│   │       ├── performance.go        # TPS estimation (bandwidth + fallback)
│   │       ├── gpuinfo.go            # GPU bandwidth lookup table (35+ GPUs)
│   │       └── hardware_test.go      # 15+ tests
│   ├── services/                     # Orchestration layer
│   │   ├── ranker.go                 # Two-tier ranking (quality → hardware)
│   │   └── ranker_test.go            # Integration tests
│   ├── hardware/                     # Hardware detection (infrastructure)
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
│   │   ├── gpu_bandwidth.go          # Memory bandwidth lookup table (35+ GPUs)
│   │   ├── gpu_utils.go              # VRAM calculation, vendor normalization
│   │   ├── error.go                  # GPUDetectionError + helpers
│   │   └── hardware_test.go          # Unit + integration tests
│   ├── registry/                     # Model registry + benchmark module
│   │   ├── fetcher.go                # Embedded hf_models.json + Ollama API
│   │   ├── benchmarks.go             # Embedded benchmarks.json + fuzzy matching
│   │   ├── normalize.go              # Model name normalization, quantization parsing
│   │   ├── moe_test.go               # MoE model parsing tests (146+ models)
│   │   └── data/
│   │       ├── hf_models.json        # 960+ embedded models (146+ MoE)
│   │       └── benchmarks.json       # Curated benchmark scores
│   ├── scorer/                       # Legacy scoring (backward compatibility)
│   │   ├── scorer.go                 # Scorer struct + validation
│   │   ├── formula.go                # Run mode detection, quant selection, scoring
│   │   ├── rank.go                   # Ranking, sorting, filtering
│   │   ├── profiles.go               # ValidProfiles() helper
│   │   └── scorer_test.go            # Comprehensive scorer tests
│   └── tui/                          # Bubble Tea interactive UI
│       ├── app.go                    # App setup + Run()
│       ├── model.go                  # Bubble Tea model state + key bindings
│       ├── view.go                   # Render layouts (small/normal/wide)
│       ├── table.go                  # Table rendering (Tier, Quality columns)
│       ├── detail.go                 # Detail panel (quality breakdown, categories)
│       ├── search.go                 # Search input component
│       ├── update.go                 # Event handling, filtering
│       ├── styles.go                 # Lipgloss theme (tier colors)
│       └── tui_test.go               # TUI tests
├── scripts/
│   ├── build.sh                      # Cross-platform build script
│   └── update_benchmarks.go          # Live benchmark updater
├── testdata/                         # Mock API responses for testing
└── tests/e2e/                        # End-to-end tests
    └── hardware_e2e_test.go          # Real hardware + pipeline tests
```

## Architecture

KnowURLLM follows **clean architecture** with `internal/domain/` as the contract layer:

```
CLI (cmd/knowurllm/main.go)
  → Hardware Detection (hardware/)
     ├─ CPU (gopsutil → /proc/cpuinfo → sysctl → wmic)
     ├─ Memory (gopsutil → platform fallbacks)
     └─ GPU (nvidia-smi/NVML, AMD ROCm, Apple Metal)
  → Model Fetching (registry/fetcher.go)
     ├─ Embedded hf_models.json (960+ models, 146+ MoE, go:embed)
     └─ Ollama API (optional, parallel concurrent queries across 9 model families)
  → Benchmark Enrichment (registry/benchmarks.go)
     └─ Embedded benchmarks.json with fuzzy name matching
  → Ranking (services/ranker.go)
     ├─ Step 1: Quality Scoring (domain/quality/)
     │   ├─ Bayesian fusion of 5 benchmarks (Arena ELO, MMLU, IFEval, GSM8K, ARC)
     │   ├─ Confidence intervals and percentile calculation
     │   ├─ Quality tier assignment (S/A/B/C/D)
     │   └─ Category-specific scores (Chat, Coding, Reasoning, etc.)
     ├─ Step 2: Hardware Filtering (domain/hardware/)
     │   ├─ Run mode detection (GPU / MoE / CPU+GPU / CPU)
     │   ├─ Dynamic quantization selection (Q8_0 → Q2_K)
     │   └─ Speed estimation (bandwidth-based or param-count fallback)
     └─ Step 3: Tier-Based Sorting
         ├─ Primary: Quality tier (S > A > B > C > D)
         ├─ Secondary: Quality score within tier
         ├─ Tertiary: Hardware fit quality
         └─ Quaternary: TPS (faster first)
  → TUI Display (tui/)
     ├─ Responsive layouts (small <60w, normal 60-119w, wide ≥120w)
     ├─ Searchable table with tier badges and quality scores
     │   └─ Columns: Rank | Tier | Model | Size | TPS | Quality | Fit
     ├─ Expandable detail panel (Tab key)
     │   ├─ Quality tier with confidence
     │   ├─ Benchmark breakdown (Arena ELO, MMLU, etc.)
     │   ├─ Category scores (Chat, Coding, Reasoning, etc.)
     │   └─ Hardware fit details (mode, quant, memory usage)
     ├─ VRAM-only filter toggle
     └─ Help panel with key bindings
```

### Architectural Principles

- **Domain layer** (`internal/domain/`): Pure business logic, zero dependencies on infrastructure
- **Services layer** (`internal/services/`): Orchestration, imports domain + infrastructure
- **Infrastructure** (`hardware/`, `registry/`): External concerns, import only domain
- **Presentation** (`tui/`): UI only, imports domain (receives pre-ranked results)
- **No circular dependencies**: Strict unidirectional flow (domain ← services ← infrastructure)

## Testing

```bash
go test ./...
```

Tests cover:

- **Hardware detection**: CPU, memory, GPU (NVIDIA, AMD, Apple Silicon)
- **Quality scoring**: Bayesian fusion, ELO normalization, confidence intervals, category scoring (25+ tests)
- **Hardware compatibility**: Run mode detection, quantization selection, MoE handling (15+ tests)
- **Model parsing**: Embedded data, Ollama API, MoE model parsing (146+ MoE models validated)
- **Benchmark enrichment**: Fuzzy matching, score normalization
- **TUI rendering**: Navigation, filtering, search, detail panels
- **End-to-end**: Real hardware detection + full pipeline tests

**Test coverage:**

```
✅ internal/domain/quality/     — 25 tests (Bayesian fusion, tiers, categories)
✅ internal/domain/hardware/    — 15 tests (compatibility, performance, MoE)
✅ internal/hardware/           — Integration tests (real detection)
✅ internal/registry/           — MoE parsing, benchmarks, normalization
✅ internal/tui/                — Navigation, filtering, rendering
✅ tests/e2e/                   — Real hardware + pipeline validation
```

## License

MIT
