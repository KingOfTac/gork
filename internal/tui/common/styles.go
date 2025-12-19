package common

import (
	"github.com/charmbracelet/lipgloss"
)

// Animation constants
const (
	AnimationFrames    = 6  // Number of frames for fade-in animation
	AnimationFrameTime = 50 // Milliseconds per frame
)

// FadeColors for log entry animation (dark to bright)
var FadeColors = []lipgloss.Color{
	lipgloss.Color("#2A2A2A"), // Very dim
	lipgloss.Color("#4A4A4A"),
	lipgloss.Color("#7A7A7A"),
	lipgloss.Color("#AAAAAA"),
	lipgloss.Color("#DADADA"),
	lipgloss.Color("#FAFAFA"), // Full brightness
}

var (
	// Colors
	PrimaryColor   = lipgloss.Color("#7D56F4")
	SecondaryColor = lipgloss.Color("#6C6C6C")
	AccentColor    = lipgloss.Color("#04B575")
	ErrorColor     = lipgloss.Color("#FF5F56")
	WarningColor   = lipgloss.Color("#FFBD2E")
	InfoColor      = lipgloss.Color("#27C7FA")
	SubtleColor    = lipgloss.Color("#383838")
	HighlightColor = lipgloss.Color("#E0E0E0")

	// Base styles
	BaseStyle = lipgloss.NewStyle().
			Padding(1, 2)

	// Title styles
	TitleStyle = lipgloss.NewStyle().
			Foreground(PrimaryColor).
			Bold(true).
			MarginBottom(1)

	SubtitleStyle = lipgloss.NewStyle().
			Foreground(SecondaryColor).
			Italic(true)

	// List styles
	ListTitleStyle = lipgloss.NewStyle().
			Foreground(PrimaryColor).
			Bold(true).
			Padding(0, 1).
			MarginBottom(1)

	SelectedItemStyle = lipgloss.NewStyle().
				Foreground(HighlightColor).
				Background(PrimaryColor).
				Bold(true).
				Padding(0, 1)

	NormalItemStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FAFAFA")).
			Padding(0, 1)

	DimmedItemStyle = lipgloss.NewStyle().
			Foreground(SecondaryColor).
			Padding(0, 1)

	// Status styles
	StatusPendingStyle = lipgloss.NewStyle().
				Foreground(WarningColor).
				Bold(true)

	StatusRunningStyle = lipgloss.NewStyle().
				Foreground(InfoColor).
				Bold(true)

	StatusSuccessStyle = lipgloss.NewStyle().
				Foreground(AccentColor).
				Bold(true)

	StatusFailedStyle = lipgloss.NewStyle().
				Foreground(ErrorColor).
				Bold(true)

	// Panel styles
	PanelStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(SubtleColor).
			Padding(1, 2)

	ActivePanelStyle = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(PrimaryColor).
				Padding(1, 2)

	// Help styles
	HelpStyle = lipgloss.NewStyle().
			Foreground(SecondaryColor).
			MarginTop(1)

	HelpKeyStyle = lipgloss.NewStyle().
			Foreground(PrimaryColor).
			Bold(true)

	HelpDescStyle = lipgloss.NewStyle().
			Foreground(SecondaryColor)

	// Log styles
	LogStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FAFAFA"))

	LogLineNumberStyle = lipgloss.NewStyle().
				Foreground(SecondaryColor).
				Width(5).
				Align(lipgloss.Right).
				MarginRight(1)

	// Header/Footer
	HeaderStyle = lipgloss.NewStyle().
			Foreground(HighlightColor).
			Background(PrimaryColor).
			Bold(true).
			Padding(0, 1)

	FooterStyle = lipgloss.NewStyle().
			Foreground(SecondaryColor).
			Border(lipgloss.NormalBorder(), true, false, false, false).
			BorderForeground(SubtleColor).
			Padding(0, 1)

	// Breadcrumb
	BreadcrumbStyle = lipgloss.NewStyle().
			Foreground(SecondaryColor)

	BreadcrumbActiveStyle = lipgloss.NewStyle().
				Foreground(PrimaryColor).
				Bold(true)

	// Error/Info boxes
	ErrorBoxStyle = lipgloss.NewStyle().
			Foreground(ErrorColor).
			Border(lipgloss.RoundedBorder()).
			BorderForeground(ErrorColor).
			Padding(0, 1)

	InfoBoxStyle = lipgloss.NewStyle().
			Foreground(InfoColor).
			Border(lipgloss.RoundedBorder()).
			BorderForeground(InfoColor).
			Padding(0, 1)
)

// StatusStyle returns the appropriate style for a given status
func StatusStyle(status string) lipgloss.Style {
	switch status {
	case "pending":
		return StatusPendingStyle
	case "running":
		return StatusRunningStyle
	case "success", "succeeded", "completed":
		return StatusSuccessStyle
	case "failed", "error":
		return StatusFailedStyle
	default:
		return lipgloss.NewStyle().Foreground(SecondaryColor)
	}
}
