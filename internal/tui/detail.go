package tui

import (
	"fmt"
	"strings"

	"github.com/kevo-1/KnowURLLM/internal/domain"
)

// renderDetailPanel renders the detail panel for the currently focused model.
// The expanded parameter controls whether all fields or only key fields are shown.
func renderDetailPanel(result domain.RankedModel, width int, expanded bool) string {
	if width < 40 {
		return ""
	}

	if expanded {
		return renderDetailPanelExpanded(result, width)
	}
	return renderDetailPanelCondensed(result, width)
}

// renderDetailPanelCondensed renders a compact view with only key information.
func renderDetailPanelCondensed(result domain.RankedModel, width int) string {
	m := result.Model
	q := result.Quality
	hw := result.Hardware

	var lines []string

	// Tier badge and fit badge
	tierBadge := tierBadgeStyle.Copy().Foreground(GetTierColor(string(q.Tier))).Render(string(q.Tier))
	fitBadge := renderFitBadge(hw.FitLabel)
	lines = append(lines, formatDetailLine("Tier:", tierBadge+"  |  Fit: "+fitBadge))

	// Quality score with confidence
	qualityLine := fmt.Sprintf("%.0f/100", q.Overall)
	if q.Confidence < 0.5 {
		qualityLine += fmt.Sprintf(" (±%.0f, low confidence)", (100-q.Overall)*0.2)
	}
	lines = append(lines, formatDetailLine("Quality:", qualityLine))

	// Performance estimate
	lines = append(lines, formatDetailLine("Perf:", formatTPS(hw.EstimatedTPS)))

	// Model size and quantization
	sizeLine := formatBytes(m.ModelSizeBytes)
	if hw.BestQuant != "" {
		sizeLine += "  |  Best: " + hw.BestQuant
	}
	lines = append(lines, formatDetailLine("Size:", sizeLine))

	content := strings.Join(lines, "\n")
	return detailStyle.Width(width - 4).Render(content)
}

// renderDetailPanelExpanded renders the full detail view with all fields.
func renderDetailPanelExpanded(result domain.RankedModel, width int) string {
	m := result.Model
	q := result.Quality
	hw := result.Hardware

	var lines []string

	// Tier badge and fit badge
	tierBadge := tierBadgeStyle.Copy().Foreground(GetTierColor(string(q.Tier))).Render(string(q.Tier))
	fitBadge := renderFitBadge(hw.FitLabel)
	lines = append(lines, formatDetailLine("Tier:", tierBadge+"  |  Fit: "+fitBadge))
	lines = append(lines, "")

	// Quality breakdown
	var qualityParts []string
	if q.ArenaELO > 0 {
		qualityParts = append(qualityParts, fmt.Sprintf("Arena ELO: %.0f", q.ArenaELO))
	}
	if q.MMLUPro > 0 {
		qualityParts = append(qualityParts, fmt.Sprintf("MMLU-PRO: %.1f", q.MMLUPro))
	}
	if q.IFEval > 0 {
		qualityParts = append(qualityParts, fmt.Sprintf("IFEval: %.1f", q.IFEval))
	}
	if q.GSM8K > 0 {
		qualityParts = append(qualityParts, fmt.Sprintf("GSM8K: %.1f", q.GSM8K))
	}
	
	qualityScore := fmt.Sprintf("Overall: %.0f/100 (Confidence: %.0f%%)", q.Overall, q.Confidence*100)
	if len(qualityParts) > 0 {
		qualityScore += "  |  " + strings.Join(qualityParts, ", ")
	}
	lines = append(lines, formatDetailLine("Quality:", qualityScore))
	lines = append(lines, "")

	// Category scores
	if len(q.CategoryScores) > 0 {
		var catParts []string
		for cat, score := range q.CategoryScores {
			catParts = append(catParts, fmt.Sprintf("%s: %.0f", formatCategoryName(cat), score))
		}
		lines = append(lines, formatDetailLine("Categories:", strings.Join(catParts, "  |  ")))
		lines = append(lines, "")
	}

	// Performance estimate
	lines = append(lines, formatDetailLine("TPS:", formatTPS(hw.EstimatedTPS)))
	lines = append(lines, formatDetailLine("Run Mode:", formatRunMode(hw.Mode)))
	lines = append(lines, formatDetailLine("Quant:", hw.BestQuant))
	lines = append(lines, "")

	// Memory utilization
	var memParts []string
	if hw.VRAMUtilPct > 0 {
		memParts = append(memParts, fmt.Sprintf("VRAM: %.0f%%", hw.VRAMUtilPct*100))
	}
	if hw.RAMUtilPct > 0 {
		memParts = append(memParts, fmt.Sprintf("RAM: %.0f%%", hw.RAMUtilPct*100))
	}
	if len(memParts) > 0 {
		lines = append(lines, formatDetailLine("Memory:", strings.Join(memParts, "  |  ")))
	}

	// Model info
	lines = append(lines, "")
	lines = append(lines, formatDetailLine("Source:", m.Source))
	if m.ContextLength > 0 {
		lines = append(lines, formatDetailLine("Context:", formatContext(m.ContextLength)))
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
		return vramBadgeStyle.Render("VRAM ✓")
	case "Good":
		return moeBadgeStyle.Render("Good")
	case "Marginal":
		return ramBadgeStyle.Render("RAM")
	case "Too Tight":
		return mutedTextStyle.Render("Too Tight")
	default:
		return mutedTextStyle.Render(category)
	}
}

// formatCategoryName formats a category score name for display.
func formatCategoryName(cat string) string {
	switch cat {
	case "general_chat":
		return "Chat"
	case "coding":
		return "Code"
	case "reasoning":
		return "Reason"
	case "long_context":
		return "Context"
	case "multimodal":
		return "Multi"
	default:
		return cat
	}
}

// formatRunMode formats a run mode for display.
func formatRunMode(mode domain.RunMode) string {
	switch mode {
	case domain.RunModeGPU:
		return "GPU"
	case domain.RunModeMoE:
		return "MoE"
	case domain.RunModeCPUGPU:
		return "CPU+GPU"
	case domain.RunModeCPU:
		return "CPU"
	default:
		return "Unknown"
	}
}
