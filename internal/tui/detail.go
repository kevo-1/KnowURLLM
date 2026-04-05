package tui

import (
	"fmt"
	"strings"

	"github.com/kevo-1/KnowURLLM/internal/models"
)

// renderDetailPanel renders the detail panel for the currently focused model.
// The expanded parameter controls whether all fields or only key fields are shown.
func renderDetailPanel(result models.RankResult, width int, expanded bool) string {
	if width < 40 {
		return ""
	}

	if expanded {
		return renderDetailPanelExpanded(result, width)
	}
	return renderDetailPanelCondensed(result, width)
}

// renderDetailPanelCondensed renders a compact view with only key information.
func renderDetailPanelCondensed(result models.RankResult, width int) string {
	m := result.Model
	s := result.Score

	var lines []string

	// Fit badge and reason
	fitBadge := renderFitBadge(s.FitCategory)
	lines = append(lines, formatDetailLine("Fit:", fitBadge+" "+s.FitReason))

	// Total score with breakdown
	scoreLine := fmt.Sprintf("Total %.1f  |  Fit %.1f  |  Speed %.1f  |  Quality %.1f",
		s.TotalScore, s.HardwareFitScore, s.ThroughputScore, s.QualityScore)
	lines = append(lines, formatDetailLine("Score:", scoreLine))

	// Performance estimate
	lines = append(lines, formatDetailLine("Perf:", formatTPS(s.EstimatedTPS)))

	// Model size and quantization
	sizeLine := formatBytes(m.ModelSizeBytes)
	if m.Quantization != "" {
		sizeLine += "  |  Quant: " + m.Quantization
	}
	lines = append(lines, formatDetailLine("Size:", sizeLine))

	content := strings.Join(lines, "\n")
	return detailStyle.Width(width - 4).Render(content)
}

// renderDetailPanelExpanded renders the full detail view with all fields.
func renderDetailPanelExpanded(result models.RankResult, width int) string {
	m := result.Model
	s := result.Score

	var lines []string

	// Fit reason with category badge
	fitBadge := renderFitBadge(s.FitCategory)
	lines = append(lines, formatDetailLine("Fit:", fitBadge+" "+s.FitReason))

	// Score breakdown — now with 5 dimensions
	scoreLine := fmt.Sprintf("Total %.1f  |  Fit %.1f  |  Speed %.1f  |  Quality %.1f  |  Context %.1f",
		s.TotalScore, s.HardwareFitScore, s.ThroughputScore, s.QualityScore, s.ContextScore)
	lines = append(lines, formatDetailLine("Score:", scoreLine))

	// Performance estimate
	lines = append(lines, formatDetailLine("Perf:", formatTPS(s.EstimatedTPS)))

	// Quality metrics — always show QualityScore, show raw benchmarks if available
	var qualityParts []string
	if m.MMLUScore > 0 {
		qualityParts = append(qualityParts, fmt.Sprintf("MMLU %.1f", m.MMLUScore))
	}
	if m.ArenaELO > 0 {
		qualityParts = append(qualityParts, fmt.Sprintf("ELO %.0f", m.ArenaELO))
	}

	if len(qualityParts) > 0 {
		// Show raw benchmark scores + QualityScore
		lines = append(lines, formatDetailLine("Quality:",
			strings.Join(qualityParts, "  |  ")+fmt.Sprintf("  |  Score %.1f", s.QualityScore)))
	} else {
		// No raw benchmarks available — show computed QualityScore only
		lines = append(lines, formatDetailLine("Quality:",
			fmt.Sprintf("Score %.1f (estimated)", s.QualityScore)))
	}

	// Size, quantization, context
	var sizeParts []string
	sizeParts = append(sizeParts, formatBytes(m.ModelSizeBytes))
	if m.Quantization != "" {
		sizeParts = append(sizeParts, "Quant: "+m.Quantization)
	}
	if m.ContextLength > 0 {
		sizeParts = append(sizeParts, "Context: "+formatContext(m.ContextLength))
	}
	lines = append(lines, formatDetailLine("Size:", strings.Join(sizeParts, "  |  ")))

	// Source and tags
	var tagParts []string
	tagParts = append(tagParts, "Source: "+m.Source)
	if len(m.Tags) > 0 {
		tagParts = append(tagParts, "Tags: "+strings.Join(m.Tags, ", "))
	}
	lines = append(lines, formatDetailLine("", strings.Join(tagParts, "  |  ")))

	// URL
	if m.URL != "" {
		lines = append(lines, formatDetailLine("URL:", m.URL))
	}

	content := strings.Join(lines, "\n")
	return detailStyle.Width(width - 4).Render(content)
}

// formatDetailLine formats a single line in the detail panel.
func formatDetailLine(label, value string) string {
	if label == "" {
		return valueStyle.Render(value)
	}
	return labelStyle.Render(label) + " " + valueStyle.Render(value)
}

// formatBytes converts bytes to a human-readable string with appropriate
// decimal precision for hardware decision-making.
func formatBytes(b uint64) string {
	const (
		KB = 1024
		MB = KB * 1024
		GB = MB * 1024
		TB = GB * 1024
	)

	switch {
	case b >= TB:
		return fmt.Sprintf("%.2f TB", float64(b)/float64(TB))
	case b >= GB:
		return fmt.Sprintf("%.1f GB", float64(b)/float64(GB))
	case b >= MB:
		return fmt.Sprintf("%.1f MB", float64(b)/float64(MB))
	case b >= KB:
		return fmt.Sprintf("%.1f KB", float64(b)/float64(KB))
	default:
		return fmt.Sprintf("%d B", b)
	}
}

// formatContext converts a context length to a short string like "128k", "4k".
func formatContext(n int) string {
	if n >= 1000 {
		return fmt.Sprintf("%dk", n/1000)
	}
	return fmt.Sprintf("%d", n)
}

// formatTPS formats tokens-per-second to a string like "~42 tok/s".
func formatTPS(f float64) string {
	return fmt.Sprintf("~%d tok/s", int(f))
}

// renderFitBadge renders a colored badge for the fit category.
func renderFitBadge(category string) string {
	switch category {
	case "Perfect":
		return "[Perfect]"
	case "Good":
		return "[Good]"
	case "Marginal":
		return "[Marginal]"
	case "Too Tight":
		return "[Too Tight]"
	default:
		return "[" + category + "]"
	}
}
