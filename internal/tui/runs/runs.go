package runs

import (
	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/kingoftac/gork/internal/db"
	"github.com/kingoftac/gork/internal/models"
	"github.com/kingoftac/gork/internal/tui/common"
)

// RunItem implements list.Item for runs
type RunItem struct {
	Run          models.Run
	WorkflowName string
}

func (i RunItem) Title() string {
	return common.StatusStyle(string(i.Run.Status)).Render(string(i.Run.Status)) + " - " + i.WorkflowName
}

func (i RunItem) Description() string {
	if i.Run.StartedAt.IsZero() {
		return "Not started"
	}
	return "Started: " + i.Run.StartedAt.Format("2006-01-02 15:04:05")
}

func (i RunItem) FilterValue() string { return i.WorkflowName }

// Model represents the run listing feature
type Model struct {
	db           *db.DB
	list         list.Model
	runs         []models.Run
	selectedRun  *models.Run
	workflowName string
	width        int
	height       int
	loading      bool
	errMessage   string
}

// New creates a new runs model
func New(database *db.DB) Model {
	delegate := list.NewDefaultDelegate()
	delegate.Styles.SelectedTitle = common.SelectedItemStyle
	delegate.Styles.SelectedDesc = common.SelectedItemStyle.Foreground(lipgloss.Color("#AAAAAA"))
	delegate.Styles.NormalTitle = common.NormalItemStyle
	delegate.Styles.NormalDesc = common.DimmedItemStyle

	runList := list.New([]list.Item{}, delegate, 0, 0)
	runList.Title = "Runs"
	runList.Styles.Title = common.ListTitleStyle
	runList.SetShowStatusBar(true)
	runList.SetFilteringEnabled(true)
	runList.SetShowHelp(false)

	return Model{
		db:   database,
		list: runList,
	}
}

// SetSize updates the component size
func (m *Model) SetSize(width, height int) {
	m.width = width
	m.height = height
	m.list.SetSize(width-4, height)
}

// Runs returns the current runs
func (m Model) Runs() []models.Run {
	return m.runs
}

// SelectedRun returns the selected run
func (m Model) SelectedRun() *models.Run {
	return m.selectedRun
}

// SetSelectedRun sets the selected run
func (m *Model) SetSelectedRun(r *models.Run) {
	m.selectedRun = r
}

// IsFiltering returns true if the list is in filtering mode
func (m Model) IsFiltering() bool {
	return m.list.FilterState() == list.Filtering
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

// SetWorkflowName sets the workflow name for display
func (m *Model) SetWorkflowName(name string) {
	m.workflowName = name
}

// WorkflowName returns the workflow name
func (m Model) WorkflowName() string {
	return m.workflowName
}

// Clear clears the runs data
func (m *Model) Clear() {
	m.runs = nil
	m.selectedRun = nil
	m.workflowName = ""
}

// Update handles messages for the runs model
func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case common.RunsLoadedMsg:
		m.loading = false
		if msg.Err != nil {
			m.errMessage = msg.Err.Error()
			return m, nil
		}
		m.runs = msg.Runs
		m.updateList()
		return m, nil
	}

	m.list, cmd = m.list.Update(msg)
	return m, cmd
}

func (m *Model) updateList() {
	items := make([]list.Item, len(m.runs))
	for i, r := range m.runs {
		items[i] = RunItem{Run: r, WorkflowName: m.workflowName}
	}
	m.list.SetItems(items)
}

// View renders the runs list
func (m Model) View() string {
	return m.list.View()
}

// Commands

// LoadRuns loads runs for a workflow
func (m Model) LoadRuns(workflowID int64) tea.Cmd {
	return func() tea.Msg {
		runs, err := m.db.ListRuns(&workflowID)
		return common.RunsLoadedMsg{Runs: runs, Err: err}
	}
}

// GetSelectedRun returns the currently selected run from the list
func (m Model) GetSelectedRun() *models.Run {
	if item, ok := m.list.SelectedItem().(RunItem); ok {
		return &item.Run
	}
	return nil
}

// Select selects an item at the given index
func (m *Model) Select(index int) {
	m.list.Select(index)
}
