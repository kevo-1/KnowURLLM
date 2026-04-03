# KnowURLLM

> A Go-based TUI tool that detects your hardware, fetches LLM models from Hugging Face and Ollama, ranks them by hardware compatibility and performance, and displays results in an interactive terminal interface.

## Features

- **Automatic hardware detection** — CPU, RAM, GPU (NVIDIA, AMD, Apple Silicon)
- **Model registry aggregation** — fetches from both Hugging Face and Ollama APIs
- **Intelligent scoring** — ranks models by hardware fit, estimated throughput, and benchmark quality
- **Interactive TUI** — searchable, filterable table with live detail panels
- **Cross-platform** — Linux, macOS (Intel + Apple Silicon), Windows

## Installation

```bash
go install github.com/KnowURLLM/cmd/knowurllm@latest
```

Or build from source:

```bash
git clone https://github.com/KnowURLLM/knowurllm.git
cd knowurllm
go build -o knowurllm ./cmd/knowurllm/
```

## Usage

Just run:

```bash
knowurllm
```

The tool will:

1. Detect your hardware (CPU, RAM, GPU)
2. Fetch available models from Hugging Face and Ollama
3. Score and rank models based on your hardware
4. Launch an interactive TUI for browsing and selecting models

When you select a model and press `Enter`, it prints the model details and the `ollama run` command (if applicable).

## Environment Variables

| Variable        | Purpose                                       | Default                  |
| --------------- | --------------------------------------------- | ------------------------ |
| `HF_TOKEN`      | Hugging Face API token for higher rate limits | (none)                   |
| `OLLAMA_HOST`   | Local Ollama API address                      | `http://localhost:11434` |
| `MAX_MODELS`    | Max models to fetch per registry              | `200`                    |
| `FETCH_TIMEOUT` | Timeout per registry call                     | `30s`                    |

## Key Bindings

| Key            | Action                                           |
| -------------- | ------------------------------------------------ |
| `↑` / `k`      | Move up                                          |
| `↓` / `j`      | Move down                                        |
| `/`            | Open search                                      |
| `esc`          | Clear search / exit search                       |
| `f`            | Cycle source filter (all → huggingface → ollama) |
| `v`            | Toggle VRAM-only filter                          |
| `enter`        | Select highlighted model                         |
| `?`            | Toggle help                                      |
| `q` / `ctrl+c` | Quit                                             |

## Platform Support

| Platform            | CPU | RAM | GPU                |
| ------------------- | --- | --- | ------------------ |
| Linux               | Yes | Yes | Yes NVIDIA, AMD    |
| macOS Intel         | Yes | Yes | Yes Intel iGPU     |
| macOS Apple Silicon | Yes | Yes | Yes Unified Memory |
| Windows             | Yes | Yes | Yes NVIDIA         |

## Scoring Formula

Models are scored on a 0–100 scale using three weighted components:

| Component        | Weight | Description                                           |
| ---------------- | ------ | ----------------------------------------------------- |
| **Hardware Fit** | 50%    | How comfortably the model fits in VRAM/RAM            |
| **Throughput**   | 30%    | Estimated tokens/sec based on model size and hardware |
| **Quality**      | 20%    | Benchmark scores (MMLU, Chatbot Arena ELO)            |

Models that cannot fit in available memory (fit ratio < 0.7) are excluded from results.

## Project Structure

```
KnowURLLM/
├── cmd/knowurllm/main.go        # CLI entrypoint
├── internal/
│   ├── models/                  # Shared data structures
│   ├── hardware/                # Hardware detection
│   ├── registry/                # Hugging Face + Ollama fetcher
│   ├── scorer/                  # Ranking and scoring logic
│   └── tui/                     # Bubble Tea interactive UI
├── testdata/                    # Mock API responses for testing
├── scripts/build.sh             # Cross-platform build script
└── ARCHITECTURE.md              # Detailed architecture docs
```

## License

MIT
