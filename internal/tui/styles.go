package tui

import "github.com/charmbracelet/lipgloss"

// Color palette
const (
	colorAccent    = lipgloss.Color("#7C3AED")
	colorGold      = lipgloss.Color("#F59E0B")
	colorGreen     = lipgloss.Color("#10B981")
	colorYellow    = lipgloss.Color("#FBBF24")
	colorMuted     = lipgloss.Color("#6B7280")
	colorSelected  = lipgloss.Color("#1E1B4B")
	colorWhite     = lipgloss.Color("#FFFFFF")
	colorDarkGray  = lipgloss.Color("#1F1F1F")
	colorLightGray = lipgloss.Color("#E5E7EB")
	
	// Tier colors
	colorTierS = lipgloss.Color("#FFD700") // Gold
	colorTierA = lipgloss.Color("#10B981") // Green
	colorTierB = lipgloss.Color("#3B82F6") // Blue
	colorTierC = lipgloss.Color("#F59E0B") // Yellow/Orange
	colorTierD = lipgloss.Color("#6B7280") // Gray
)

var (
	// titleStyle styles the app title in the header.
	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(colorAccent).
			Padding(0, 1)

	// headerStyle styles the table column headers.
	headerStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(colorWhite).
			Background(colorDarkGray)

	// selectedRowStyle styles the focused table row.
	selectedRowStyle = lipgloss.NewStyle().
				Foreground(colorWhite).
				Background(colorSelected).
				Bold(true)

	// goldStyle highlights rank 1-3 scores.
	goldStyle = lipgloss.NewStyle().
			Foreground(colorGold).
			Bold(true)

	// tierBadgeStyle styles quality tier (S/A/B/C/D) with tier-specific colors.
	tierBadgeStyle = lipgloss.NewStyle().
				Bold(true).
				Padding(0, 1)

	// vramBadgeStyle is the green VRAM badge.
	vramBadgeStyle = lipgloss.NewStyle().
			Foreground(colorGreen).
			Bold(true).
			Padding(0, 1)

	// moeBadgeStyle is the blue MoE badge.
	moeBadgeStyle = lipgloss.NewStyle().
			Foreground(colorTierB).
			Bold(true).
			Padding(0, 1)

	// ramBadgeStyle is the yellow RAM badge.
	ramBadgeStyle = lipgloss.NewStyle().
			Foreground(colorYellow).
			Bold(true).
			Padding(0, 1)

	// detailStyle styles the detail panel border.
	detailStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(colorAccent).
			Padding(1, 2).
			MarginTop(1)

	// helpStyle styles the bottom help bar.
	helpStyle = lipgloss.NewStyle().
			Foreground(colorMuted).
			Padding(0, 1)

	// searchStyle styles the search input box.
	searchStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(colorAccent).
			Padding(0, 1)

	// filterTagStyle styles active filter tags.
	filterTagStyle = lipgloss.NewStyle().
			Foreground(colorWhite).
			Background(colorAccent).
			Padding(0, 1).
			MarginRight(1).
			Bold(true)

	// mutedTextStyle styles muted/disabled text.
	mutedTextStyle = lipgloss.NewStyle().
			Foreground(colorMuted)

	// boldTextStyle styles bold/important text.
	boldTextStyle = lipgloss.NewStyle().Bold(true)

	// labelStyle styles labels in the detail panel.
	labelStyle = lipgloss.NewStyle().
			Foreground(colorMuted).
			Bold(true)

	// valueStyle styles values in the detail panel.
	valueStyle = lipgloss.NewStyle().
			Foreground(colorWhite)
)

// GetTierColor returns the color for a given quality tier.
func GetTierColor(tier string) lipgloss.Color {
	switch tier {
	case "S":
		return colorTierS
	case "A":
		return colorTierA
	case "B":
		return colorTierB
	case "C":
		return colorTierC
	case "D":
		return colorTierD
	default:
		return colorMuted
	}
}
