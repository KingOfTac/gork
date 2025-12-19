package workflows

import (
	"context"
	"os"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"gopkg.in/yaml.v3"

	"github.com/kingoftac/gork/internal/db"
	"github.com/kingoftac/gork/internal/engine"
	"github.com/kingoftac/gork/internal/models"
	"github.com/kingoftac/gork/internal/tui/common"
)

// WorkflowItem implements list.Item for workflows
type WorkflowItem struct {
	Workflow models.Workflow
}

func (i WorkflowItem) Title() string       { return i.Workflow.Name }
func (i WorkflowItem) Description() string { return i.Workflow.Description }
func (i WorkflowItem) FilterValue() string { return i.Workflow.Name }

// Model represents the workflow management feature
type Model struct {
	db               *db.DB
	list             list.Model
	textInput        textinput.Model
	workflows        []models.Workflow
	selectedWorkflow *models.Workflow
	inputMode        common.InputMode
	width            int
	height           int
	loading          bool
	errMessage       string
	statusMessage    string

	// Reset confirmation state
	showResetConfirm   bool
	resetWorkflowCount int
	resetRunCount      int
}

// New creates a new workflows model
func New(database *db.DB) Model {
	// Create workflow list
	delegate := list.NewDefaultDelegate()
	delegate.Styles.SelectedTitle = common.SelectedItemStyle
	delegate.Styles.SelectedDesc = common.SelectedItemStyle.Foreground(lipgloss.Color("#AAAAAA"))
	delegate.Styles.NormalTitle = common.NormalItemStyle
	delegate.Styles.NormalDesc = common.DimmedItemStyle

	workflowList := list.New([]list.Item{}, delegate, 0, 0)
	workflowList.Title = "Workflows"
	workflowList.Styles.Title = common.ListTitleStyle
	workflowList.SetShowStatusBar(true)
	workflowList.SetFilteringEnabled(true)
	workflowList.SetShowHelp(false)

	// Create text input
	ti := textinput.New()
	ti.Placeholder = "Enter path..."
	ti.CharLimit = 256
	ti.Width = 50

	return Model{
		db:        database,
		list:      workflowList,
		textInput: ti,
		inputMode: common.InputModeNone,
		loading:   true,
	}
}

// Init initializes the workflow model
func (m Model) Init() tea.Cmd {
	return m.LoadWorkflows()
}

// SetSize updates the component size
func (m *Model) SetSize(width, height int) {
	m.width = width
	m.height = height
	m.list.SetSize(width-4, height)
	m.textInput.Width = width - 20
}

// Workflows returns the current workflows
func (m Model) Workflows() []models.Workflow {
	return m.workflows
}

// SelectedWorkflow returns the selected workflow
func (m Model) SelectedWorkflow() *models.Workflow {
	return m.selectedWorkflow
}

// SetSelectedWorkflow sets the selected workflow
func (m *Model) SetSelectedWorkflow(w *models.Workflow) {
	m.selectedWorkflow = w
}

// IsFiltering returns true if the list is in filtering mode
func (m Model) IsFiltering() bool {
	return m.list.FilterState() == list.Filtering
}

// InputMode returns the current input mode
func (m Model) InputMode() common.InputMode {
	return m.inputMode
}

// SetInputMode sets the input mode
func (m *Model) SetInputMode(mode common.InputMode) {
	m.inputMode = mode
}

// ShowResetConfirm returns whether reset confirmation is shown
func (m Model) ShowResetConfirm() bool {
	return m.showResetConfirm
}

// SetShowResetConfirm sets whether reset confirmation is shown
func (m *Model) SetShowResetConfirm(show bool) {
	m.showResetConfirm = show
}

// ResetCounts returns the workflow and run counts for reset confirmation
func (m Model) ResetCounts() (int, int) {
	return m.resetWorkflowCount, m.resetRunCount
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

// StatusMessage returns the status message
func (m Model) StatusMessage() string {
	return m.statusMessage
}

// SetStatusMessage sets the status message
func (m *Model) SetStatusMessage(msg string) {
	m.statusMessage = msg
}

// ClearMessages clears status and error messages
func (m *Model) ClearMessages() {
	m.errMessage = ""
	m.statusMessage = ""
}

// TextInput returns the text input model
func (m Model) TextInput() textinput.Model {
	return m.textInput
}

// Update handles messages for the workflow model
func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case common.WorkflowsLoadedMsg:
		m.loading = false
		if msg.Err != nil {
			m.errMessage = msg.Err.Error()
			return m, nil
		}
		m.workflows = msg.Workflows
		m.updateList()
		return m, nil

	case common.WorkflowExecutedMsg:
		m.loading = false
		if msg.Err != nil {
			m.errMessage = msg.Err.Error()
			return m, nil
		}
		m.statusMessage = "Workflow completed with status: " + string(msg.Run.Status)
		return m, m.LoadWorkflows()

	case common.WorkflowDeletedMsg:
		m.loading = false
		if msg.Err != nil {
			m.errMessage = msg.Err.Error()
			return m, nil
		}
		m.statusMessage = "Workflow deleted successfully"
		return m, m.LoadWorkflows()

	case common.WorkflowCreatedMsg:
		m.loading = false
		m.inputMode = common.InputModeNone
		if msg.Err != nil {
			m.errMessage = msg.Err.Error()
			return m, nil
		}
		m.statusMessage = "Workflow '" + msg.Workflow.Name + "' created successfully"
		return m, m.LoadWorkflows()

	case common.WorkflowExportedMsg:
		m.loading = false
		m.inputMode = common.InputModeNone
		if msg.Err != nil {
			m.errMessage = msg.Err.Error()
			return m, nil
		}
		m.statusMessage = "Workflow exported to: " + msg.Path
		return m, nil

	case common.DataResetMsg:
		m.loading = false
		m.showResetConfirm = false
		if msg.Err != nil {
			m.errMessage = msg.Err.Error()
			return m, nil
		}
		m.statusMessage = "Reset complete"
		return m, m.LoadWorkflows()
	}

	// Update list component
	m.list, cmd = m.list.Update(msg)
	return m, cmd
}

// UpdateTextInput updates the text input with a message
func (m *Model) UpdateTextInput(msg tea.Msg) tea.Cmd {
	var cmd tea.Cmd
	m.textInput, cmd = m.textInput.Update(msg)
	return cmd
}

func (m *Model) updateList() {
	items := make([]list.Item, len(m.workflows))
	for i, w := range m.workflows {
		items[i] = WorkflowItem{Workflow: w}
	}
	m.list.SetItems(items)
}

// View renders the workflow list
func (m Model) View() string {
	return m.list.View()
}

// ViewCreateForm renders the create workflow form
func (m Model) ViewCreateForm() string {
	return lipgloss.JoinVertical(
		lipgloss.Left,
		common.TitleStyle.Render("Create Workflow"),
		"",
		common.SubtitleStyle.Render("Enter the path to a workflow YAML file:"),
		"",
		m.textInput.View(),
	)
}

// ViewExportForm renders the export workflow form
func (m Model) ViewExportForm() string {
	workflowName := "Unknown"
	if m.selectedWorkflow != nil {
		workflowName = m.selectedWorkflow.Name
	}

	return lipgloss.JoinVertical(
		lipgloss.Left,
		common.TitleStyle.Render("Export Workflow"),
		common.SubtitleStyle.Render("Workflow: "+workflowName),
		"",
		common.SubtitleStyle.Render("Enter the output file path:"),
		"",
		m.textInput.View(),
	)
}

// ViewResetConfirm renders the reset confirmation dialog
func (m Model) ViewResetConfirm() string {
	return lipgloss.JoinVertical(
		lipgloss.Left,
		common.TitleStyle.Render("⚠️  Reset All Data"),
		"",
		common.ErrorBoxStyle.Render("This will permanently delete all workflows and runs!"),
		"",
		common.SubtitleStyle.Render("Press 'y' to confirm or 'n' to cancel"),
	)
}

// Commands

// LoadWorkflows loads all workflows from the database
func (m Model) LoadWorkflows() tea.Cmd {
	return func() tea.Msg {
		workflows, err := m.db.ListWorkflows()
		return common.WorkflowsLoadedMsg{Workflows: workflows, Err: err}
	}
}

// ExecuteWorkflow executes a workflow
func (m Model) ExecuteWorkflow(workflow *models.Workflow) tea.Cmd {
	return func() tea.Msg {
		eng := engine.NewEngine(m.db)
		run, err := eng.ExecuteWorkflow(context.Background(), workflow, "tui")
		return common.WorkflowExecutedMsg{Run: run, Err: err}
	}
}

// DeleteWorkflow deletes a workflow
func (m Model) DeleteWorkflow(id int64) tea.Cmd {
	return func() tea.Msg {
		err := m.db.DeleteWorkflow(id)
		return common.WorkflowDeletedMsg{ID: id, Err: err}
	}
}

// CreateWorkflow creates a workflow from a YAML file
func (m Model) CreateWorkflow(path string) tea.Cmd {
	return func() tea.Msg {
		eng := engine.NewEngine(m.db)
		workflow, err := eng.LoadWorkflow(path)
		if err != nil {
			return common.WorkflowCreatedMsg{Workflow: nil, Err: err}
		}

		if err := m.db.InsertWorkflow(workflow); err != nil {
			return common.WorkflowCreatedMsg{Workflow: nil, Err: err}
		}

		return common.WorkflowCreatedMsg{Workflow: workflow, Err: nil}
	}
}

// ExportWorkflow exports a workflow to a YAML file
func (m Model) ExportWorkflow(id int64, path string) tea.Cmd {
	return func() tea.Msg {
		workflow, err := m.db.GetWorkflow(id)
		if err != nil {
			return common.WorkflowExportedMsg{Path: "", Err: err}
		}

		data, err := yaml.Marshal(workflow)
		if err != nil {
			return common.WorkflowExportedMsg{Path: "", Err: err}
		}

		if err := os.WriteFile(path, data, 0644); err != nil {
			return common.WorkflowExportedMsg{Path: "", Err: err}
		}

		return common.WorkflowExportedMsg{Path: path, Err: nil}
	}
}

// ResetAllData resets all workflows and runs
func (m Model) ResetAllData() tea.Cmd {
	return func() tea.Msg {
		workflowCount := 0
		runCount := 0

		workflows, _ := m.db.ListWorkflows()
		workflowCount = len(workflows)

		for _, w := range workflows {
			runs, _ := m.db.ListRuns(&w.ID)
			runCount += len(runs)
			m.db.DeleteWorkflow(w.ID)
		}

		return common.DataResetMsg{
			WorkflowCount: workflowCount,
			RunCount:      runCount,
			Err:           nil,
		}
	}
}

// PrepareReset prepares for reset confirmation
func (m *Model) PrepareReset() {
	m.resetWorkflowCount = len(m.workflows)
	totalRuns := 0
	for _, w := range m.workflows {
		runs, err := m.db.ListRuns(&w.ID)
		if err == nil {
			totalRuns += len(runs)
		}
	}
	m.resetRunCount = totalRuns
	m.showResetConfirm = true
}

// PrepareCreate prepares for workflow creation
func (m *Model) PrepareCreate() tea.Cmd {
	m.inputMode = common.InputModeCreate
	m.textInput.SetValue("")
	m.textInput.Placeholder = "Enter workflow YAML file path..."
	m.textInput.Focus()
	return textinput.Blink
}

// PrepareExport prepares for workflow export
func (m *Model) PrepareExport() tea.Cmd {
	if item, ok := m.list.SelectedItem().(WorkflowItem); ok {
		m.selectedWorkflow = &item.Workflow
		m.inputMode = common.InputModeExport
		m.textInput.SetValue("")
		m.textInput.Placeholder = "Enter output file path..."
		m.textInput.Focus()
		return textinput.Blink
	}
	return nil
}

// CancelInput cancels the current input operation
func (m *Model) CancelInput() {
	m.inputMode = common.InputModeNone
	m.textInput.SetValue("")
}

// GetSelectedWorkflow returns the currently selected workflow from the list
func (m Model) GetSelectedWorkflow() *models.Workflow {
	if item, ok := m.list.SelectedItem().(WorkflowItem); ok {
		return &item.Workflow
	}
	return nil
}

// Select selects an item at the given index
func (m *Model) Select(index int) {
	m.list.Select(index)
}
