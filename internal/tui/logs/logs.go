package logs

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/kingoftac/gork/internal/db"
	"github.com/kingoftac/gork/internal/models"
	"github.com/kingoftac/gork/internal/tui/common"
)

// Model represents the log viewing feature
type Model struct {
	db         *db.DB
	viewport   viewport.Model
	stepRuns   []models.StepRun
	runID      int64
	width      int
	height     int
	loading    bool
	errMessage string
}

// New creates a new logs model
func New(database *db.DB) Model {
	vp := viewport.New(0, 0)
	vp.Style = common.PanelStyle

	return Model{
		db:       database,
		viewport: vp,
	}
}

// SetSize updates the component size
func (m *Model) SetSize(width, height int) {
	m.width = width
	m.height = height
	m.viewport.Width = width - 6
	m.viewport.Height = height - 4 // Account for title and run info
}

// StepRuns returns the current step runs
func (m Model) StepRuns() []models.StepRun {
	return m.stepRuns
}

// Loading returns the loading state
func (m Model) Loading() bool {
	return m.loading
}

// SetLoading sets the loading state
func (m *Model) SetLoading(loading bool) {
	m.loading = loading
}

// ErrMessage returns the error message
func (m Model) ErrMessage() string {
	return m.errMessage
}

// SetErrMessage sets the error message
func (m *Model) SetErrMessage(msg string) {
	m.errMessage = msg
}

// RunID returns the current run ID
func (m Model) RunID() int64 {
	return m.runID
}

// Clear clears the logs data
func (m *Model) Clear() {
	m.stepRuns = nil
	m.runID = 0
}

// Update handles messages for the logs model
func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case common.StepRunsLoadedMsg:
		m.loading = false
		if msg.Err != nil {
			m.errMessage = msg.Err.Error()
			return m, nil
		}
		m.stepRuns = msg.StepRuns
		m.updateViewport()
		return m, nil
	}

	m.viewport, cmd = m.viewport.Update(msg)
	return m, cmd
}

func (m *Model) updateViewport() {
	content := m.renderLogs()
	m.viewport.SetContent(content)
}

// View renders the logs viewport
func (m Model) View() string {
	return m.viewport.View()
}

// ViewFull renders the full log view with title
func (m Model) ViewFull(run *models.Run) string {
	if run == nil {
		return "No run selected"
	}

	title := common.TitleStyle.Render(fmt.Sprintf("Logs for Run #%d", run.ID))
	status := common.SubtitleStyle.Render(fmt.Sprintf("Status: %s", common.StatusStyle(string(run.Status)).Render(string(run.Status))))

	return fmt.Sprintf("%s\n%s\n\n%s", title, status, m.viewport.View())
}

func (m Model) renderLogs() string {
	if len(m.stepRuns) == 0 {
		return common.DimmedItemStyle.Render("No logs available for this run.")
	}

	var sb strings.Builder
	for _, sr := range m.stepRuns {
		// Step header
		statusStyle := common.StatusStyle(string(sr.Status))
		stepHeader := fmt.Sprintf("━━━ Step: %s [%s] ━━━",
			sr.StepName,
			statusStyle.Render(string(sr.Status)))
		sb.WriteString(common.TitleStyle.Render(stepHeader))
		sb.WriteString("\n")

		// Timing info
		if !sr.StartedAt.IsZero() {
			sb.WriteString(common.DimmedItemStyle.Render(fmt.Sprintf("Started: %s", sr.StartedAt.Format("15:04:05"))))
			if !sr.CompletedAt.IsZero() {
				duration := sr.CompletedAt.Sub(sr.StartedAt)
				sb.WriteString(common.DimmedItemStyle.Render(fmt.Sprintf(" | Duration: %s", duration.Round(100*1e6))))
			}
			sb.WriteString("\n")
		}

		// Output (from Logs field)
		if len(sr.Logs) > 0 {
			sb.WriteString("\n")
			for i, line := range sr.Logs {
				lineNum := common.LogLineNumberStyle.Render(fmt.Sprintf("%d", i+1))
				sb.WriteString(fmt.Sprintf("%s %s\n", lineNum, common.LogStyle.Render(line)))
			}
		}

		// Error output
		if sr.Error != "" {
			sb.WriteString(common.ErrorBoxStyle.Render("Error:"))
			sb.WriteString("\n")
			sb.WriteString(common.StatusFailedStyle.Render(sr.Error))
			sb.WriteString("\n")
		}

		sb.WriteString("\n")
	}

	return sb.String()
}

// Commands

// LoadStepRuns loads step runs for a run
func (m *Model) LoadStepRuns(runID int64) tea.Cmd {
	m.runID = runID
	return func() tea.Msg {
		stepRuns, err := m.db.GetStepRuns(runID)
		return common.StepRunsLoadedMsg{StepRuns: stepRuns, Err: err}
	}
}

// Scroll operations

// LineUp scrolls up by n lines
func (m *Model) LineUp(n int) {
	m.viewport.LineUp(n)
}

// LineDown scrolls down by n lines
func (m *Model) LineDown(n int) {
	m.viewport.LineDown(n)
}

// HalfViewUp scrolls up by half the viewport height
func (m *Model) HalfViewUp() {
	m.viewport.HalfViewUp()
}

// HalfViewDown scrolls down by half the viewport height
func (m *Model) HalfViewDown() {
	m.viewport.HalfViewDown()
}

// GotoTop scrolls to the top
func (m *Model) GotoTop() {
	m.viewport.GotoTop()
}

// GotoBottom scrolls to the bottom
func (m *Model) GotoBottom() {
	m.viewport.GotoBottom()
}
