package tui

import (
	"strings"

	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/kingoftac/gork/internal/db"
	"github.com/kingoftac/gork/internal/tui/common"
	"github.com/kingoftac/gork/internal/tui/daemon"
	"github.com/kingoftac/gork/internal/tui/logs"
	"github.com/kingoftac/gork/internal/tui/runs"
	"github.com/kingoftac/gork/internal/tui/workflows"
)

// App is the main TUI application model that composes all features
type App struct {
	// Core
	db     *db.DB
	width  int
	height int
	help   help.Model
	keys   common.KeyMap

	// Current view state
	currentView  common.View
	previousView common.View

	// Feature models
	workflows *workflows.Model
	runs      *runs.Model
	logs      *logs.Model
	daemon    *daemon.Model

	// Global state
	loading       bool
	statusMessage string
	errMessage    string
}

// NewApp creates a new TUI application
func NewApp(database *db.DB, daemonExePath string) App {
	workflowsModel := workflows.New(database)
	runsModel := runs.New(database)
	logsModel := logs.New(database)
	daemonModel := daemon.New(daemonExePath)

	return App{
		db:          database,
		help:        help.New(),
		keys:        common.DefaultKeyMap,
		currentView: common.ViewWorkflows,
		workflows:   &workflowsModel,
		runs:        &runsModel,
		logs:        &logsModel,
		daemon:      &daemonModel,
		loading:     true,
	}
}

// Init implements tea.Model
func (a App) Init() tea.Cmd {
	return a.workflows.Init()
}

// Update implements tea.Model
func (a App) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		a.width = msg.Width
		a.height = msg.Height
		a.updateSizes()
		return a, nil

	case tea.KeyMsg:
		// Handle text input mode in workflows
		if a.workflows.InputMode() != common.InputModeNone {
			return a.handleTextInputKey(msg)
		}

		// Don't handle keys while filtering
		if a.currentView == common.ViewWorkflows && a.workflows.IsFiltering() {
			break
		}
		if a.currentView == common.ViewRuns && a.runs.IsFiltering() {
			break
		}

		// Handle confirmation dialogs
		if a.workflows.ShowResetConfirm() {
			return a.handleResetConfirmKey(msg)
		}

		switch {
		case key.Matches(msg, a.keys.Quit):
			if a.daemon.Running() {
				a.daemon.Stop()
			}
			return a, tea.Quit

		case key.Matches(msg, a.keys.Back):
			return a.handleBack()

		case key.Matches(msg, a.keys.Enter):
			return a.handleEnter()

		case key.Matches(msg, a.keys.Run):
			if a.currentView == common.ViewWorkflows {
				return a.handleRunWorkflow()
			}

		case key.Matches(msg, a.keys.Delete):
			if a.currentView == common.ViewWorkflows {
				return a.handleDeleteWorkflow()
			}

		case key.Matches(msg, a.keys.Refresh):
			return a.handleRefresh()

		case key.Matches(msg, a.keys.Create):
			if a.currentView == common.ViewWorkflows {
				return a.handleCreateWorkflow()
			}

		case key.Matches(msg, a.keys.Export):
			if a.currentView == common.ViewWorkflows {
				return a.handleExportWorkflow()
			}

		case key.Matches(msg, a.keys.Reset):
			if a.currentView == common.ViewWorkflows {
				return a.handleReset()
			}

		case key.Matches(msg, a.keys.Daemon):
			return a.handleDaemonView()

		case key.Matches(msg, a.keys.StartStop):
			if a.currentView == common.ViewDaemon {
				return a, a.daemon.Toggle()
			}

		case key.Matches(msg, a.keys.PageUp):
			if a.currentView == common.ViewLogs {
				a.logs.HalfViewUp()
			} else if a.currentView == common.ViewDaemon {
				a.daemon.HalfViewUp()
			}

		case key.Matches(msg, a.keys.PageDown):
			if a.currentView == common.ViewLogs {
				a.logs.HalfViewDown()
			} else if a.currentView == common.ViewDaemon {
				a.daemon.HalfViewDown()
			}

		case key.Matches(msg, a.keys.Home):
			if a.currentView == common.ViewLogs {
				a.logs.GotoTop()
			} else if a.currentView == common.ViewDaemon {
				a.daemon.GotoTop()
			}

		case key.Matches(msg, a.keys.End):
			if a.currentView == common.ViewLogs {
				a.logs.GotoBottom()
			} else if a.currentView == common.ViewDaemon {
				a.daemon.GotoBottom()
			}
		}

	case tea.MouseMsg:
		switch msg.Button {
		case tea.MouseButtonWheelUp:
			if a.currentView == common.ViewLogs {
				a.logs.LineUp(1)
			} else if a.currentView == common.ViewDaemon {
				a.daemon.LineUp(1)
			}
		case tea.MouseButtonWheelDown:
			if a.currentView == common.ViewLogs {
				a.logs.LineDown(1)
			} else if a.currentView == common.ViewDaemon {
				a.daemon.LineDown(1)
			}
		case tea.MouseButtonLeft:
			if msg.Action == tea.MouseActionRelease {
				return a.handleMouseClick(msg.X, msg.Y)
			}
		}

	// Handle workflow messages
	case common.WorkflowsLoadedMsg, common.WorkflowExecutedMsg, common.WorkflowDeletedMsg,
		common.WorkflowCreatedMsg, common.WorkflowExportedMsg, common.DataResetMsg:
		a.loading = false
		newWorkflows, cmd := a.workflows.Update(msg)
		a.workflows = &newWorkflows
		a.statusMessage = a.workflows.StatusMessage()
		a.errMessage = a.workflows.ErrMessage()
		if a.currentView == common.ViewCreateWorkflow || a.currentView == common.ViewExportWorkflow {
			if a.workflows.InputMode() == common.InputModeNone {
				a.currentView = common.ViewWorkflows
			}
		}
		if a.workflows.ShowResetConfirm() == false && a.currentView == common.ViewConfirmReset {
			a.currentView = common.ViewWorkflows
		}
		return a, cmd

	// Handle runs messages
	case common.RunsLoadedMsg:
		a.loading = false
		newRuns, cmd := a.runs.Update(msg)
		a.runs = &newRuns
		a.errMessage = a.runs.ErrMessage()
		return a, cmd

	// Handle logs messages
	case common.StepRunsLoadedMsg:
		a.loading = false
		newLogs, cmd := a.logs.Update(msg)
		a.logs = &newLogs
		a.errMessage = a.logs.ErrMessage()
		return a, cmd

	// Handle daemon messages
	case common.DaemonStartedMsg, common.DaemonStoppedMsg, common.TickMsg:
		if a.currentView == common.ViewDaemon {
			newDaemon, cmd := a.daemon.Update(msg)
			a.daemon = &newDaemon
			a.statusMessage = a.daemon.StatusMessage()
			a.errMessage = a.daemon.ErrMessage()
			return a, cmd
		}
		return a, nil

	case common.StatusMsg:
		a.statusMessage = msg.Message
		return a, nil

	case common.ErrorMsg:
		a.errMessage = msg.Err.Error()
		return a, nil
	}

	// Update active feature component
	var cmd tea.Cmd
	switch a.currentView {
	case common.ViewWorkflows:
		newWorkflows, c := a.workflows.Update(msg)
		a.workflows = &newWorkflows
		cmd = c
	case common.ViewRuns:
		newRuns, c := a.runs.Update(msg)
		a.runs = &newRuns
		cmd = c
	case common.ViewLogs:
		newLogs, c := a.logs.Update(msg)
		a.logs = &newLogs
		cmd = c
	case common.ViewDaemon:
		newDaemon, c := a.daemon.Update(msg)
		a.daemon = &newDaemon
		cmd = c
	case common.ViewCreateWorkflow, common.ViewExportWorkflow:
		cmd = a.workflows.UpdateTextInput(msg)
	}
	cmds = append(cmds, cmd)

	return a, tea.Batch(cmds...)
}

// View implements tea.Model
func (a App) View() string {
	header := a.renderHeader()
	footer := a.renderFooter()

	var content string
	switch a.currentView {
	case common.ViewWorkflows:
		content = a.workflows.View()
	case common.ViewRuns:
		content = a.runs.View()
	case common.ViewLogs:
		content = a.logs.ViewFull(a.runs.SelectedRun())
	case common.ViewCreateWorkflow:
		content = a.workflows.ViewCreateForm()
	case common.ViewExportWorkflow:
		content = a.workflows.ViewExportForm()
	case common.ViewConfirmReset:
		content = a.workflows.ViewResetConfirm()
	case common.ViewDaemon:
		content = a.daemon.View()
	}

	return lipgloss.JoinVertical(
		lipgloss.Left,
		header,
		content,
		footer,
	)
}

func (a *App) updateSizes() {
	availableHeight := a.height - common.HeaderHeight - common.FooterHeight

	a.workflows.SetSize(a.width, availableHeight)
	a.runs.SetSize(a.width, availableHeight)
	a.logs.SetSize(a.width, availableHeight)
	a.daemon.SetSize(a.width, availableHeight)
}

func (a App) renderHeader() string {
	title := common.HeaderStyle.Render(" 🔧 GORK - Workflow Orchestration Engine ")

	// Breadcrumb
	var breadcrumb string
	switch a.currentView {
	case common.ViewWorkflows:
		breadcrumb = common.BreadcrumbActiveStyle.Render("Workflows")
	case common.ViewRuns:
		workflowName := "Unknown"
		if a.workflows.SelectedWorkflow() != nil {
			workflowName = a.workflows.SelectedWorkflow().Name
		}
		breadcrumb = common.BreadcrumbStyle.Render("Workflows > ") + common.BreadcrumbActiveStyle.Render(workflowName)
	case common.ViewLogs:
		workflowName := "Unknown"
		if a.workflows.SelectedWorkflow() != nil {
			workflowName = a.workflows.SelectedWorkflow().Name
		}
		breadcrumb = common.BreadcrumbStyle.Render("Workflows > "+workflowName+" > ") + common.BreadcrumbActiveStyle.Render("Logs")
	case common.ViewCreateWorkflow:
		breadcrumb = common.BreadcrumbStyle.Render("Workflows > ") + common.BreadcrumbActiveStyle.Render("Create")
	case common.ViewExportWorkflow:
		workflowName := "Unknown"
		if a.workflows.SelectedWorkflow() != nil {
			workflowName = a.workflows.SelectedWorkflow().Name
		}
		breadcrumb = common.BreadcrumbStyle.Render("Workflows > "+workflowName+" > ") + common.BreadcrumbActiveStyle.Render("Export")
	case common.ViewConfirmReset:
		breadcrumb = common.BreadcrumbStyle.Render("Workflows > ") + common.BreadcrumbActiveStyle.Render("Reset All Data")
	case common.ViewDaemon:
		breadcrumb = common.BreadcrumbActiveStyle.Render("Daemon")
	}

	headerContent := lipgloss.JoinVertical(
		lipgloss.Left,
		title,
		breadcrumb,
	)

	return common.BaseStyle.Render(headerContent)
}

func (a App) renderFooter() string {
	var statusLine string
	if a.loading {
		statusLine = common.InfoBoxStyle.Render("⏳ Loading...")
	} else if a.errMessage != "" {
		statusLine = common.ErrorBoxStyle.Render("❌ " + a.errMessage)
	} else if a.statusMessage != "" {
		statusLine = common.InfoBoxStyle.Render("ℹ️  " + a.statusMessage)
	}

	helpLine := a.renderHelp()

	return common.FooterStyle.Render(lipgloss.JoinVertical(
		lipgloss.Left,
		statusLine,
		helpLine,
	))
}

func (a App) renderHelp() string {
	var keys []string

	switch a.currentView {
	case common.ViewWorkflows:
		keys = []string{
			common.HelpKeyStyle.Render("enter") + common.HelpDescStyle.Render(" select"),
			common.HelpKeyStyle.Render("r") + common.HelpDescStyle.Render(" run"),
			common.HelpKeyStyle.Render("c") + common.HelpDescStyle.Render(" create"),
			common.HelpKeyStyle.Render("e") + common.HelpDescStyle.Render(" export"),
			common.HelpKeyStyle.Render("d") + common.HelpDescStyle.Render(" delete"),
			common.HelpKeyStyle.Render("D") + common.HelpDescStyle.Render(" daemon"),
			common.HelpKeyStyle.Render("X") + common.HelpDescStyle.Render(" reset"),
			common.HelpKeyStyle.Render("q") + common.HelpDescStyle.Render(" quit"),
		}
	case common.ViewRuns:
		keys = []string{
			common.HelpKeyStyle.Render("enter") + common.HelpDescStyle.Render(" view logs"),
			common.HelpKeyStyle.Render("esc") + common.HelpDescStyle.Render(" back"),
			common.HelpKeyStyle.Render("R") + common.HelpDescStyle.Render(" refresh"),
			common.HelpKeyStyle.Render("q") + common.HelpDescStyle.Render(" quit"),
		}
	case common.ViewLogs:
		keys = []string{
			common.HelpKeyStyle.Render("↑/↓") + common.HelpDescStyle.Render(" scroll"),
			common.HelpKeyStyle.Render("pgup/pgdn") + common.HelpDescStyle.Render(" page"),
			common.HelpKeyStyle.Render("g/G") + common.HelpDescStyle.Render(" top/bottom"),
			common.HelpKeyStyle.Render("esc") + common.HelpDescStyle.Render(" back"),
			common.HelpKeyStyle.Render("R") + common.HelpDescStyle.Render(" refresh"),
			common.HelpKeyStyle.Render("q") + common.HelpDescStyle.Render(" quit"),
		}
	case common.ViewCreateWorkflow, common.ViewExportWorkflow:
		keys = []string{
			common.HelpKeyStyle.Render("enter") + common.HelpDescStyle.Render(" submit"),
			common.HelpKeyStyle.Render("esc") + common.HelpDescStyle.Render(" cancel"),
		}
	case common.ViewConfirmReset:
		keys = []string{
			common.HelpKeyStyle.Render("y") + common.HelpDescStyle.Render(" confirm"),
			common.HelpKeyStyle.Render("n") + common.HelpDescStyle.Render(" cancel"),
		}
	case common.ViewDaemon:
		if a.daemon.Running() {
			keys = []string{
				common.HelpKeyStyle.Render("s") + common.HelpDescStyle.Render(" stop"),
				common.HelpKeyStyle.Render("↑/↓") + common.HelpDescStyle.Render(" scroll"),
				common.HelpKeyStyle.Render("pgup/pgdn") + common.HelpDescStyle.Render(" page"),
				common.HelpKeyStyle.Render("g/G") + common.HelpDescStyle.Render(" top/bottom"),
				common.HelpKeyStyle.Render("esc") + common.HelpDescStyle.Render(" back"),
			}
		} else {
			keys = []string{
				common.HelpKeyStyle.Render("s") + common.HelpDescStyle.Render(" start"),
				common.HelpKeyStyle.Render("↑/↓") + common.HelpDescStyle.Render(" scroll"),
				common.HelpKeyStyle.Render("pgup/pgdn") + common.HelpDescStyle.Render(" page"),
				common.HelpKeyStyle.Render("g/G") + common.HelpDescStyle.Render(" top/bottom"),
				common.HelpKeyStyle.Render("esc") + common.HelpDescStyle.Render(" back"),
			}
		}
	}

	return common.HelpStyle.Render(strings.Join(keys, "  •  "))
}

// Event handlers

func (a App) handleBack() (tea.Model, tea.Cmd) {
	switch a.currentView {
	case common.ViewRuns:
		a.currentView = common.ViewWorkflows
		a.workflows.SetSelectedWorkflow(nil)
		a.runs.Clear()
	case common.ViewLogs:
		a.currentView = common.ViewRuns
		a.runs.SetSelectedRun(nil)
		a.logs.Clear()
	case common.ViewCreateWorkflow, common.ViewExportWorkflow:
		a.currentView = common.ViewWorkflows
		a.workflows.CancelInput()
	case common.ViewConfirmReset:
		a.currentView = common.ViewWorkflows
		a.workflows.SetShowResetConfirm(false)
	case common.ViewDaemon:
		a.currentView = common.ViewWorkflows
	}
	a.errMessage = ""
	a.statusMessage = ""
	return a, nil
}

func (a App) handleEnter() (tea.Model, tea.Cmd) {
	switch a.currentView {
	case common.ViewWorkflows:
		if workflow := a.workflows.GetSelectedWorkflow(); workflow != nil {
			a.workflows.SetSelectedWorkflow(workflow)
			a.currentView = common.ViewRuns
			a.runs.SetWorkflowName(workflow.Name)
			a.loading = true
			return a, a.runs.LoadRuns(workflow.ID)
		}
	case common.ViewRuns:
		if run := a.runs.GetSelectedRun(); run != nil {
			a.runs.SetSelectedRun(run)
			a.currentView = common.ViewLogs
			a.loading = true
			return a, a.logs.LoadStepRuns(run.ID)
		}
	}
	return a, nil
}

func (a App) handleRunWorkflow() (tea.Model, tea.Cmd) {
	if workflow := a.workflows.GetSelectedWorkflow(); workflow != nil {
		a.loading = true
		a.statusMessage = "Running workflow..."
		return a, a.workflows.ExecuteWorkflow(workflow)
	}
	return a, nil
}

func (a App) handleDeleteWorkflow() (tea.Model, tea.Cmd) {
	if workflow := a.workflows.GetSelectedWorkflow(); workflow != nil {
		a.loading = true
		return a, a.workflows.DeleteWorkflow(workflow.ID)
	}
	return a, nil
}

func (a App) handleRefresh() (tea.Model, tea.Cmd) {
	a.loading = true
	a.errMessage = ""
	a.statusMessage = ""

	switch a.currentView {
	case common.ViewWorkflows:
		return a, a.workflows.LoadWorkflows()
	case common.ViewRuns:
		if workflow := a.workflows.SelectedWorkflow(); workflow != nil {
			return a, a.runs.LoadRuns(workflow.ID)
		}
	case common.ViewLogs:
		if a.logs.RunID() != 0 {
			return a, a.logs.LoadStepRuns(a.logs.RunID())
		}
	}
	return a, nil
}

func (a App) handleCreateWorkflow() (tea.Model, tea.Cmd) {
	a.currentView = common.ViewCreateWorkflow
	return a, a.workflows.PrepareCreate()
}

func (a App) handleExportWorkflow() (tea.Model, tea.Cmd) {
	if a.workflows.GetSelectedWorkflow() != nil {
		a.currentView = common.ViewExportWorkflow
		return a, a.workflows.PrepareExport()
	}
	return a, nil
}

func (a App) handleReset() (tea.Model, tea.Cmd) {
	a.workflows.PrepareReset()
	a.currentView = common.ViewConfirmReset
	return a, nil
}

func (a App) handleDaemonView() (tea.Model, tea.Cmd) {
	a.previousView = a.currentView
	a.currentView = common.ViewDaemon
	return a, nil
}

func (a App) handleTextInputKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyEsc:
		a.workflows.CancelInput()
		a.currentView = common.ViewWorkflows
		return a, nil
	case tea.KeyEnter:
		path := a.workflows.TextInput().Value()
		if path == "" {
			a.errMessage = "Path cannot be empty"
			return a, nil
		}

		a.loading = true
		switch a.workflows.InputMode() {
		case common.InputModeCreate:
			return a, a.workflows.CreateWorkflow(path)
		case common.InputModeExport:
			if workflow := a.workflows.SelectedWorkflow(); workflow != nil {
				return a, a.workflows.ExportWorkflow(workflow.ID, path)
			}
		}
		return a, nil
	}

	// Update text input
	cmd := a.workflows.UpdateTextInput(msg)
	return a, cmd
}

func (a App) handleResetConfirmKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case key.Matches(msg, a.keys.Yes):
		a.loading = true
		return a, a.workflows.ResetAllData()
	case key.Matches(msg, a.keys.No), key.Matches(msg, a.keys.Back):
		a.currentView = common.ViewWorkflows
		a.workflows.SetShowResetConfirm(false)
		return a, nil
	}
	return a, nil
}

func (a App) handleMouseClick(x, y int) (tea.Model, tea.Cmd) {
	// Adjust for header offset
	listY := y - common.HeaderHeight

	if listY < 0 {
		return a, nil
	}

	switch a.currentView {
	case common.ViewWorkflows:
		if listY >= 0 && listY < len(a.workflows.Workflows()) {
			itemHeight := 2
			titleOffset := 2
			clickedIndex := (listY - titleOffset) / itemHeight
			if clickedIndex >= 0 && clickedIndex < len(a.workflows.Workflows()) {
				a.workflows.Select(clickedIndex)
				return a, nil
			}
		}
	case common.ViewRuns:
		if listY >= 0 && listY < len(a.runs.Runs()) {
			itemHeight := 2
			titleOffset := 2
			clickedIndex := (listY - titleOffset) / itemHeight
			if clickedIndex >= 0 && clickedIndex < len(a.runs.Runs()) {
				a.runs.Select(clickedIndex)
				return a, nil
			}
		}
	}

	return a, nil
}
