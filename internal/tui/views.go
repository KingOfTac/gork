package tui

import (
	"context"
	"fmt"
	"os"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"gopkg.in/yaml.v3"

	"github.com/kingoftac/gork/internal/engine"
	"github.com/kingoftac/gork/internal/models"
)

func (m Model) loadWorkflows() tea.Cmd {
	return func() tea.Msg {
		workflows, err := m.db.ListWorkflows()
		return WorkflowsLoadedMsg{Workflows: workflows, Err: err}
	}
}

func (m Model) loadRuns(workflowID int64) tea.Cmd {
	return func() tea.Msg {
		runs, err := m.db.ListRuns(&workflowID)
		return RunsLoadedMsg{Runs: runs, Err: err}
	}
}

func (m Model) loadStepRuns(runID int64) tea.Cmd {
	return func() tea.Msg {
		stepRuns, err := m.db.GetStepRuns(runID)
		return StepRunsLoadedMsg{StepRuns: stepRuns, Err: err}
	}
}

func (m Model) executeWorkflow(workflow *models.Workflow) tea.Cmd {
	return func() tea.Msg {
		eng := engine.NewEngine(m.db)
		run, err := eng.ExecuteWorkflow(context.Background(), workflow, "tui")
		return WorkflowExecutedMsg{Run: run, Err: err}
	}
}

func (m Model) deleteWorkflow(id int64) tea.Cmd {
	return func() tea.Msg {
		err := m.db.DeleteWorkflow(id)
		return WorkflowDeletedMsg{ID: id, Err: err}
	}
}

func (m Model) createWorkflow(path string) tea.Cmd {
	return func() tea.Msg {
		eng := engine.NewEngine(m.db)
		workflow, err := eng.LoadWorkflow(path)
		if err != nil {
			return WorkflowCreatedMsg{Workflow: nil, Err: err}
		}

		if err := m.db.InsertWorkflow(workflow); err != nil {
			return WorkflowCreatedMsg{Workflow: nil, Err: err}
		}

		return WorkflowCreatedMsg{Workflow: workflow, Err: nil}
	}
}

func (m Model) exportWorkflow(id int64, path string) tea.Cmd {
	return func() tea.Msg {
		workflow, err := m.db.GetWorkflow(id)
		if err != nil {
			return WorkflowExportedMsg{Path: "", Err: err}
		}

		data, err := yaml.Marshal(workflow)
		if err != nil {
			return WorkflowExportedMsg{Path: "", Err: err}
		}

		if err := os.WriteFile(path, data, 0644); err != nil {
			return WorkflowExportedMsg{Path: "", Err: err}
		}

		return WorkflowExportedMsg{Path: path, Err: nil}
	}
}

func (m Model) resetAllData() tea.Cmd {
	return func() tea.Msg {
		workflowCount := 0
		runCount := 0

		workflows, _ := m.db.ListWorkflows()
		workflowCount = len(workflows)

		for _, w := range workflows {
			runs, _ := m.db.ListRuns(&w.ID)
			runCount += len(runs)
		}

		err := m.db.ResetAllData()
		return DataResetMsg{WorkflowCount: workflowCount, RunCount: runCount, Err: err}
	}
}

func (m Model) render() string {
	var content string

	header := m.renderHeader()
	footer := m.renderFooter()

	switch m.currentView {
	case ViewWorkflows:
		content = m.workflowList.View()
	case ViewRuns:
		content = m.runList.View()
	case ViewLogs:
		content = m.renderLogView()
	case ViewCreateWorkflow:
		content = m.renderCreateWorkflowView()
	case ViewExportWorkflow:
		content = m.renderExportWorkflowView()
	case ViewConfirmReset:
		content = m.renderResetConfirmView()
	case ViewDaemon:
		content = m.renderDaemonView()
	}

	return lipgloss.JoinVertical(
		lipgloss.Left,
		header,
		content,
		footer,
	)
}

func (m Model) renderHeader() string {
	title := HeaderStyle.Render(" 🔧 GORK - Workflow Orchestration Engine ")

	var breadcrumb string
	switch m.currentView {
	case ViewWorkflows:
		breadcrumb = BreadcrumbActiveStyle.Render("Workflows")
	case ViewRuns:
		workflowName := "Unknown"
		if m.selectedWorkflow != nil {
			workflowName = m.selectedWorkflow.Name
		}
		breadcrumb = BreadcrumbStyle.Render("Workflows > ") + BreadcrumbActiveStyle.Render(workflowName+" > Runs")
	case ViewLogs:
		workflowName := "Unknown"
		if m.selectedWorkflow != nil {
			workflowName = m.selectedWorkflow.Name
		}
		runID := "?"
		if m.selectedRun != nil {
			runID = fmt.Sprintf("%d", m.selectedRun.ID)
		}
		breadcrumb = BreadcrumbStyle.Render("Workflows > "+workflowName+" > Runs > ") + BreadcrumbActiveStyle.Render("Run #"+runID)
	case ViewCreateWorkflow:
		breadcrumb = BreadcrumbStyle.Render("Workflows > ") + BreadcrumbActiveStyle.Render("Create Workflow")
	case ViewExportWorkflow:
		workflowName := "Unknown"
		if m.selectedWorkflow != nil {
			workflowName = m.selectedWorkflow.Name
		}
		breadcrumb = BreadcrumbStyle.Render("Workflows > "+workflowName+" > ") + BreadcrumbActiveStyle.Render("Export")
	case ViewConfirmReset:
		breadcrumb = BreadcrumbStyle.Render("Workflows > ") + BreadcrumbActiveStyle.Render("Reset All Data")
	case ViewDaemon:
		breadcrumb = BreadcrumbActiveStyle.Render("Daemon")
	}

	headerContent := lipgloss.JoinVertical(
		lipgloss.Left,
		title,
		breadcrumb,
	)

	return BaseStyle.Render(headerContent)
}

func (m Model) renderFooter() string {
	var statusLine string
	if m.loading {
		statusLine = InfoBoxStyle.Render("⏳ Loading...")
	} else if m.errMessage != "" {
		statusLine = ErrorBoxStyle.Render("❌ " + m.errMessage)
	} else if m.statusMessage != "" {
		statusLine = InfoBoxStyle.Render("ℹ️  " + m.statusMessage)
	}

	helpLine := m.renderHelp()

	return lipgloss.JoinVertical(
		lipgloss.Left,
		statusLine,
		helpLine,
	)
}

func (m Model) renderHelp() string {
	var keys []string

	switch m.currentView {
	case ViewWorkflows:
		keys = []string{
			HelpKeyStyle.Render("enter") + HelpDescStyle.Render(" select"),
			HelpKeyStyle.Render("r") + HelpDescStyle.Render(" run"),
			HelpKeyStyle.Render("c") + HelpDescStyle.Render(" create"),
			HelpKeyStyle.Render("e") + HelpDescStyle.Render(" export"),
			HelpKeyStyle.Render("d") + HelpDescStyle.Render(" delete"),
			HelpKeyStyle.Render("D") + HelpDescStyle.Render(" daemon"),
			HelpKeyStyle.Render("X") + HelpDescStyle.Render(" reset"),
			HelpKeyStyle.Render("R") + HelpDescStyle.Render(" refresh"),
			HelpKeyStyle.Render("/") + HelpDescStyle.Render(" filter"),
			HelpKeyStyle.Render("q") + HelpDescStyle.Render(" quit"),
		}
	case ViewRuns:
		keys = []string{
			HelpKeyStyle.Render("enter") + HelpDescStyle.Render(" view logs"),
			HelpKeyStyle.Render("esc") + HelpDescStyle.Render(" back"),
			HelpKeyStyle.Render("R") + HelpDescStyle.Render(" refresh"),
			HelpKeyStyle.Render("/") + HelpDescStyle.Render(" filter"),
			HelpKeyStyle.Render("q") + HelpDescStyle.Render(" quit"),
		}
	case ViewLogs:
		keys = []string{
			HelpKeyStyle.Render("↑/↓") + HelpDescStyle.Render(" scroll"),
			HelpKeyStyle.Render("pgup/pgdn") + HelpDescStyle.Render(" page"),
			HelpKeyStyle.Render("g/G") + HelpDescStyle.Render(" top/bottom"),
			HelpKeyStyle.Render("esc") + HelpDescStyle.Render(" back"),
			HelpKeyStyle.Render("R") + HelpDescStyle.Render(" refresh"),
			HelpKeyStyle.Render("q") + HelpDescStyle.Render(" quit"),
		}
	case ViewCreateWorkflow, ViewExportWorkflow:
		keys = []string{
			HelpKeyStyle.Render("enter") + HelpDescStyle.Render(" submit"),
			HelpKeyStyle.Render("esc") + HelpDescStyle.Render(" cancel"),
		}
	case ViewConfirmReset:
		keys = []string{
			HelpKeyStyle.Render("y") + HelpDescStyle.Render(" confirm"),
			HelpKeyStyle.Render("n") + HelpDescStyle.Render(" cancel"),
		}
	case ViewDaemon:
		if m.daemon.running {
			keys = []string{
				HelpKeyStyle.Render("s") + HelpDescStyle.Render(" stop"),
				HelpKeyStyle.Render("↑/↓") + HelpDescStyle.Render(" scroll"),
				HelpKeyStyle.Render("pgup/pgdn") + HelpDescStyle.Render(" page"),
				HelpKeyStyle.Render("g/G") + HelpDescStyle.Render(" top/bottom"),
				HelpKeyStyle.Render("esc") + HelpDescStyle.Render(" back"),
			}
		} else {
			keys = []string{
				HelpKeyStyle.Render("s") + HelpDescStyle.Render(" start"),
				HelpKeyStyle.Render("↑/↓") + HelpDescStyle.Render(" scroll"),
				HelpKeyStyle.Render("pgup/pgdn") + HelpDescStyle.Render(" page"),
				HelpKeyStyle.Render("g/G") + HelpDescStyle.Render(" top/bottom"),
				HelpKeyStyle.Render("esc") + HelpDescStyle.Render(" back"),
			}
		}
	}

	return HelpStyle.Render(strings.Join(keys, "  •  "))
}

func (m Model) renderLogView() string {
	if m.selectedRun == nil {
		return "No run selected"
	}

	title := TitleStyle.Render(fmt.Sprintf("Logs for Run #%d", m.selectedRun.ID))

	runInfo := m.renderRunInfo()

	return lipgloss.JoinVertical(
		lipgloss.Left,
		title,
		runInfo,
		m.logViewport.View(),
	)
}

func (m Model) renderRunInfo() string {
	if m.selectedRun == nil {
		return ""
	}

	r := m.selectedRun
	status := StatusStyle(string(r.Status)).Render(string(r.Status))

	started := "N/A"
	if !r.StartedAt.IsZero() {
		started = r.StartedAt.Format("2006-01-02 15:04:05")
	}

	completed := "N/A"
	if !r.CompletedAt.IsZero() {
		completed = r.CompletedAt.Format("2006-01-02 15:04:05")
	}

	info := fmt.Sprintf(
		"Status: %s  |  Started: %s  |  Completed: %s  |  Trigger: %s",
		status, started, completed, r.Trigger,
	)

	return SubtitleStyle.Render(info)
}

func (m Model) renderLogs() string {
	if len(m.stepRuns) == 0 {
		return DimmedItemStyle.Render("No logs available")
	}

	var sb strings.Builder

	for _, sr := range m.stepRuns {
		stepHeader := fmt.Sprintf("\n━━━ Step: %s ━━━", sr.StepName)
		sb.WriteString(TitleStyle.Render(stepHeader))
		sb.WriteString("\n")

		status := StatusStyle(string(sr.Status)).Render(string(sr.Status))
		sb.WriteString(fmt.Sprintf("Status: %s\n", status))

		if sr.Error != "" {
			sb.WriteString(ErrorBoxStyle.Render("Error: "+sr.Error) + "\n")
		}

		if len(sr.Logs) == 0 {
			sb.WriteString(DimmedItemStyle.Render("  (no output)") + "\n")
		} else {
			for i, logLine := range sr.Logs {
				lineNum := LogLineNumberStyle.Render(fmt.Sprintf("%d", i+1))
				sb.WriteString(fmt.Sprintf("%s %s\n", lineNum, LogStyle.Render(logLine)))
			}
		}
	}

	return sb.String()
}

func (m Model) renderCreateWorkflowView() string {
	title := TitleStyle.Render("Create Workflow from YAML")

	prompt := SubtitleStyle.Render("Enter the path to a workflow YAML file:")

	inputBox := PanelStyle.
		Width(m.width - 10).
		Render(m.textInput.View())

	return lipgloss.JoinVertical(
		lipgloss.Left,
		title,
		"",
		prompt,
		"",
		inputBox,
	)
}

func (m Model) renderExportWorkflowView() string {
	workflowName := "Unknown"
	if m.selectedWorkflow != nil {
		workflowName = m.selectedWorkflow.Name
	}

	title := TitleStyle.Render(fmt.Sprintf("Export Workflow: %s", workflowName))

	prompt := SubtitleStyle.Render("Enter the output file path:")

	inputBox := PanelStyle.
		Width(m.width - 10).
		Render(m.textInput.View())

	return lipgloss.JoinVertical(
		lipgloss.Left,
		title,
		"",
		prompt,
		"",
		inputBox,
	)
}

func (m Model) renderResetConfirmView() string {
	title := TitleStyle.Render("⚠️  Reset All Data")

	warning := ErrorBoxStyle.
		Width(m.width - 10).
		Render(fmt.Sprintf(
			"WARNING: This will permanently delete:\n\n"+
				"  • %d workflows\n"+
				"  • %d runs (with all step data)\n\n"+
				"This action CANNOT be undone!",
			m.resetWorkflowCount,
			m.resetRunCount,
		))

	prompt := SubtitleStyle.Render("Are you sure you want to continue?")

	return lipgloss.JoinVertical(
		lipgloss.Left,
		title,
		"",
		warning,
		"",
		prompt,
	)
}

func (m Model) renderDaemonView() string {
	var statusText string
	if m.daemon.running {
		statusText = StatusSuccessStyle.Render("● Running")
	} else {
		statusText = StatusFailedStyle.Render("○ Stopped")
	}

	title := TitleStyle.Render("Daemon Management")
	status := SubtitleStyle.Render(fmt.Sprintf("Status: %s", statusText))

	logsTitle := SubtitleStyle.Render("Logs:")

	return lipgloss.JoinVertical(
		lipgloss.Left,
		title,
		status,
		"",
		logsTitle,
		m.daemonViewport.View(),
	)
}

func (m Model) renderDaemonLogs() string {
	if len(m.daemonLogs) == 0 {
		return DimmedItemStyle.Render("No daemon logs yet. Press 's' to start the daemon.")
	}

	var sb strings.Builder
	for i, entry := range m.daemonLogs {
		lineNum := LogLineNumberStyle.Render(fmt.Sprintf("%d", i+1))

		var logStyle lipgloss.Style
		if entry.Frame < len(FadeColors) {
			logStyle = lipgloss.NewStyle().Foreground(FadeColors[entry.Frame])
		} else {
			logStyle = LogStyle
		}

		sb.WriteString(fmt.Sprintf("%s %s\n", lineNum, logStyle.Render(entry.Text)))
	}
	return sb.String()
}
