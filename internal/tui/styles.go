package tui

import (
	"github.com/charmbracelet/lipgloss"
)

const (
	AnimationFrames    = 6  // Number of frames for fade-in animation
	AnimationFrameTime = 50 // Milliseconds per frame
)

var FadeColors = []lipgloss.Color{
	lipgloss.Color("#2A2A2A"),
	lipgloss.Color("#4A4A4A"),
	lipgloss.Color("#7A7A7A"),
	lipgloss.Color("#AAAAAA"),
	lipgloss.Color("#DADADA"),
	lipgloss.Color("#FAFAFA"),
}

var (
	primaryColor   = lipgloss.Color("#7D56F4")
	secondaryColor = lipgloss.Color("#6C6C6C")
	accentColor    = lipgloss.Color("#04B575")
	errorColor     = lipgloss.Color("#FF5F56")
	warningColor   = lipgloss.Color("#FFBD2E")
	infoColor      = lipgloss.Color("#27C7FA")
	subtleColor    = lipgloss.Color("#383838")
	highlightColor = lipgloss.Color("#E0E0E0")

	BaseStyle = lipgloss.NewStyle().
			Padding(1, 2)

	TitleStyle = lipgloss.NewStyle().
			Foreground(primaryColor).
			Bold(true).
			MarginBottom(1)

	SubtitleStyle = lipgloss.NewStyle().
			Foreground(secondaryColor).
			Italic(true)

	ListTitleStyle = lipgloss.NewStyle().
			Foreground(primaryColor).
			Bold(true).
			Padding(0, 1).
			MarginBottom(1)

	SelectedItemStyle = lipgloss.NewStyle().
				Foreground(highlightColor).
				Background(primaryColor).
				Bold(true).
				Padding(0, 1)

	NormalItemStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FAFAFA")).
			Padding(0, 1)

	DimmedItemStyle = lipgloss.NewStyle().
			Foreground(secondaryColor).
			Padding(0, 1)

	StatusPendingStyle = lipgloss.NewStyle().
				Foreground(warningColor).
				Bold(true)

	StatusRunningStyle = lipgloss.NewStyle().
				Foreground(infoColor).
				Bold(true)

	StatusSuccessStyle = lipgloss.NewStyle().
				Foreground(accentColor).
				Bold(true)

	StatusFailedStyle = lipgloss.NewStyle().
				Foreground(errorColor).
				Bold(true)

	PanelStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(subtleColor).
			Padding(1, 2)

	ActivePanelStyle = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(primaryColor).
				Padding(1, 2)

	HelpStyle = lipgloss.NewStyle().
			Foreground(secondaryColor).
			MarginTop(1)

	HelpKeyStyle = lipgloss.NewStyle().
			Foreground(primaryColor).
			Bold(true)

	HelpDescStyle = lipgloss.NewStyle().
			Foreground(secondaryColor)

	LogStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FAFAFA"))

	LogLineNumberStyle = lipgloss.NewStyle().
				Foreground(secondaryColor).
				Width(5).
				Align(lipgloss.Right).
				MarginRight(1)

	HeaderStyle = lipgloss.NewStyle().
			Foreground(highlightColor).
			Background(primaryColor).
			Bold(true).
			Padding(0, 1)

	FooterStyle = lipgloss.NewStyle().
			Foreground(secondaryColor).
			Border(lipgloss.NormalBorder(), true, false, false, false).
			BorderForeground(subtleColor).
			Padding(0, 1)

	BreadcrumbStyle = lipgloss.NewStyle().
			Foreground(secondaryColor)

	BreadcrumbActiveStyle = lipgloss.NewStyle().
				Foreground(primaryColor).
				Bold(true)

	ErrorBoxStyle = lipgloss.NewStyle().
			Foreground(errorColor).
			Border(lipgloss.RoundedBorder()).
			BorderForeground(errorColor).
			Padding(0, 1)

	InfoBoxStyle = lipgloss.NewStyle().
			Foreground(infoColor).
			Border(lipgloss.RoundedBorder()).
			BorderForeground(infoColor).
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
		return lipgloss.NewStyle().Foreground(secondaryColor)
	}
}
