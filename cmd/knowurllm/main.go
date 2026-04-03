// Command knowurllm is a CLI tool that detects user hardware, fetches LLM models
// from Hugging Face and Ollama, ranks them by hardware compatibility and performance,
// and displays results in an interactive terminal interface.
package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/kevo-1/KnowURLLM/internal/hardware"
	"github.com/kevo-1/KnowURLLM/internal/registry"
	"github.com/kevo-1/KnowURLLM/internal/scorer"
	"github.com/kevo-1/KnowURLLM/internal/tui"
)

func main() {
	// 1. Detect hardware
	hw, err := hardware.Detect()
	if err != nil {
		log.Printf("warning: hardware detection partial: %v", err)
		// continue — partial profile is acceptable
	}

	// 2. Fetch models from registries
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	fetcher := registry.NewFetcher()
	entries, err := fetcher.FetchAll(ctx)
	if err != nil {
		log.Fatalf("failed to fetch models: %v", err)
	}
	if len(entries) == 0 {
		fmt.Fprintln(os.Stderr, "no models found from any registry")
		os.Exit(1)
	}

	// 3. Score and rank models
	s := scorer.NewScorer()
	results, err := s.Rank(hw, entries)
	if err != nil {
		log.Fatalf("scoring failed: %v", err)
	}
	if len(results) == 0 {
		fmt.Fprintln(os.Stderr, "no models fit your hardware")
		os.Exit(0)
	}

	// 4. Launch TUI
	app := tui.NewApp(results)
	selected, err := app.Run()
	if err != nil {
		log.Fatalf("TUI error: %v", err)
	}

	// 5. Output result
	if selected.ID == "" {
		fmt.Println("No model selected.")
		return
	}

	fmt.Printf("\n✅ Selected: %s\n", selected.DisplayName)
	fmt.Printf("   Source:   %s\n", selected.Source)
	fmt.Printf("   Size:     %s\n", formatBytes(selected.ModelSizeBytes))

	if selected.Source == "ollama" || selected.Source == "huggingface+ollama" {
		fmt.Printf("\n💡 Run it:\n   ollama run %s\n", selected.ID)
	} else {
		fmt.Printf("\n🔗 Model page:\n   %s\n", selected.URL)
	}
}

func formatBytes(b uint64) string {
	const gb = 1 << 30
	const mb = 1 << 20
	switch {
	case b >= gb:
		return fmt.Sprintf("%.1f GB", float64(b)/gb)
	case b >= mb:
		return fmt.Sprintf("%.0f MB", float64(b)/mb)
	default:
		return fmt.Sprintf("%d B", b)
	}
}
