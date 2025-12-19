package tui

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"sync"
	"time"

	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/kingoftac/gork/internal/db"
	"github.com/kingoftac/gork/internal/models"
)

const (
	headerHeight = 4
	footerHeight = 3
)

type View int

const (
	ViewWorkflows View = iota
	ViewRuns
	ViewLogs
	ViewCreateWorkflow
	ViewExportWorkflow
	ViewConfirmReset
	ViewDaemon
)

type InputMode int

const (
	InputModeNone InputMode = iota
	InputModeCreate
	InputModeExport
)

type KeyMap struct {
	Up        key.Binding
	Down      key.Binding
	Enter     key.Binding
	Back      key.Binding
	Run       key.Binding
	Delete    key.Binding
	Refresh   key.Binding
	Quit      key.Binding
	Help      key.Binding
	Create    key.Binding
	Export    key.Binding
	Reset     key.Binding
	Daemon    key.Binding
	Tab       key.Binding
	Yes       key.Binding
	No        key.Binding
	StartStop key.Binding
	PageUp    key.Binding
	PageDown  key.Binding
	Home      key.Binding
	End       key.Binding
}

var DefaultKeyMap = KeyMap{
	Up: key.NewBinding(
		key.WithKeys("up", "k"),
		key.WithHelp("↑/k", "up"),
	),
	Down: key.NewBinding(
		key.WithKeys("down", "j"),
		key.WithHelp("↓/j", "down"),
	),
	Enter: key.NewBinding(
		key.WithKeys("enter"),
		key.WithHelp("enter", "select"),
	),
	Back: key.NewBinding(
		key.WithKeys("esc", "backspace"),
		key.WithHelp("esc", "back"),
	),
	Run: key.NewBinding(
		key.WithKeys("r"),
		key.WithHelp("r", "run workflow"),
	),
	Delete: key.NewBinding(
		key.WithKeys("d"),
		key.WithHelp("d", "delete"),
	),
	Refresh: key.NewBinding(
		key.WithKeys("R"),
		key.WithHelp("R", "refresh"),
	),
	Quit: key.NewBinding(
		key.WithKeys("q", "ctrl+c"),
		key.WithHelp("q", "quit"),
	),
	Help: key.NewBinding(
		key.WithKeys("?"),
		key.WithHelp("?", "help"),
	),
	Create: key.NewBinding(
		key.WithKeys("c"),
		key.WithHelp("c", "create"),
	),
	Export: key.NewBinding(
		key.WithKeys("e"),
		key.WithHelp("e", "export"),
	),
	Reset: key.NewBinding(
		key.WithKeys("X"),
		key.WithHelp("X", "reset all"),
	),
	Daemon: key.NewBinding(
		key.WithKeys("D"),
		key.WithHelp("D", "daemon"),
	),
	Tab: key.NewBinding(
		key.WithKeys("tab"),
		key.WithHelp("tab", "next field"),
	),
	Yes: key.NewBinding(
		key.WithKeys("y", "Y"),
		key.WithHelp("y", "yes"),
	),
	No: key.NewBinding(
		key.WithKeys("n", "N"),
		key.WithHelp("n", "no"),
	),
	StartStop: key.NewBinding(
		key.WithKeys("s"),
		key.WithHelp("s", "start/stop"),
	),
	PageUp: key.NewBinding(
		key.WithKeys("pgup", "ctrl+u"),
		key.WithHelp("pgup", "page up"),
	),
	PageDown: key.NewBinding(
		key.WithKeys("pgdown", "ctrl+d"),
		key.WithHelp("pgdn", "page down"),
	),
	Home: key.NewBinding(
		key.WithKeys("home", "g"),
		key.WithHelp("home", "top"),
	),
	End: key.NewBinding(
		key.WithKeys("end", "G"),
		key.WithHelp("end", "bottom"),
	),
}

type WorkflowItem struct {
	workflow models.Workflow
}

func (i WorkflowItem) Title() string       { return i.workflow.Name }
func (i WorkflowItem) Description() string { return i.workflow.Description }
func (i WorkflowItem) FilterValue() string { return i.workflow.Name }

type RunItem struct {
	run          models.Run
	workflowName string
}

func (i RunItem) Title() string {
	return StatusStyle(string(i.run.Status)).Render(string(i.run.Status)) + " - " + i.workflowName
}
func (i RunItem) Description() string {
	if i.run.StartedAt.IsZero() {
		return "Not started"
	}
	return "Started: " + i.run.StartedAt.Format("2006-01-02 15:04:05")
}
func (i RunItem) FilterValue() string { return i.workflowName }

type DaemonProcess struct {
	cmd     *exec.Cmd
	running bool
	mu      sync.Mutex
	logs    []string
	logsMu  sync.Mutex
}

type AnimatedLogEntry struct {
	Text      string
	Frame     int
	Animating bool
}

type Model struct {
	db                 *db.DB
	width              int
	height             int
	currentView        View
	previousView       View
	workflowList       list.Model
	runList            list.Model
	logViewport        viewport.Model
	daemonViewport     viewport.Model
	help               help.Model
	keys               KeyMap
	workflows          []models.Workflow
	runs               []models.Run
	stepRuns           []models.StepRun
	selectedWorkflow   *models.Workflow
	selectedRun        *models.Run
	statusMessage      string
	errMessage         string
	loading            bool
	textInput          textinput.Model
	inputMode          InputMode
	daemon             *DaemonProcess
	daemonLogs         []AnimatedLogEntry
	daemonExePath      string
	animating          bool
	resetWorkflowCount int
	resetRunCount      int
}

func NewModel(database *db.DB, daemonExePath string) Model {
	workflowDelegate := list.NewDefaultDelegate()
	workflowDelegate.Styles.SelectedTitle = SelectedItemStyle
	workflowDelegate.Styles.SelectedDesc = SelectedItemStyle.Foreground(lipgloss.Color("#AAAAAA"))
	workflowDelegate.Styles.NormalTitle = NormalItemStyle
	workflowDelegate.Styles.NormalDesc = DimmedItemStyle

	workflowList := list.New([]list.Item{}, workflowDelegate, 0, 0)
	workflowList.Title = "Workflows"
	workflowList.Styles.Title = ListTitleStyle
	workflowList.SetShowStatusBar(true)
	workflowList.SetFilteringEnabled(true)
	workflowList.SetShowHelp(false)

	runDelegate := list.NewDefaultDelegate()
	runDelegate.Styles.SelectedTitle = SelectedItemStyle
	runDelegate.Styles.SelectedDesc = SelectedItemStyle.Foreground(lipgloss.Color("#AAAAAA"))
	runDelegate.Styles.NormalTitle = NormalItemStyle
	runDelegate.Styles.NormalDesc = DimmedItemStyle

	runList := list.New([]list.Item{}, runDelegate, 0, 0)
	runList.Title = "Runs"
	runList.Styles.Title = ListTitleStyle
	runList.SetShowStatusBar(true)
	runList.SetFilteringEnabled(true)
	runList.SetShowHelp(false)

	logViewport := viewport.New(0, 0)
	logViewport.Style = PanelStyle

	daemonViewport := viewport.New(0, 0)
	daemonViewport.Style = PanelStyle

	ti := textinput.New()
	ti.Placeholder = "Enter path..."
	ti.CharLimit = 256
	ti.Width = 50

	return Model{
		db:             database,
		currentView:    ViewWorkflows,
		workflowList:   workflowList,
		runList:        runList,
		logViewport:    logViewport,
		daemonViewport: daemonViewport,
		help:           help.New(),
		keys:           DefaultKeyMap,
		loading:        true,
		textInput:      ti,
		inputMode:      InputModeNone,
		daemon:         &DaemonProcess{},
		daemonLogs:     make([]AnimatedLogEntry, 0),
		daemonExePath:  daemonExePath,
	}
}

func (m Model) Init() tea.Cmd {
	return m.loadWorkflows()
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.updateSizes()
		return m, nil

	case tea.KeyMsg:
		if m.inputMode != InputModeNone {
			return m.handleTextInputKey(msg)
		}

		if m.workflowList.FilterState() == list.Filtering || m.runList.FilterState() == list.Filtering {
			break
		}

		if m.currentView == ViewConfirmReset {
			return m.handleResetConfirmKey(msg)
		}

		switch {
		case key.Matches(msg, m.keys.Quit):
			if m.daemon.running {
				m.stopDaemon()
			}
			return m, tea.Quit

		case key.Matches(msg, m.keys.Back):
			return m.handleBack()

		case key.Matches(msg, m.keys.Enter):
			return m.handleEnter()

		case key.Matches(msg, m.keys.Run):
			if m.currentView == ViewWorkflows {
				return m.handleRunWorkflow()
			}

		case key.Matches(msg, m.keys.Delete):
			if m.currentView == ViewWorkflows {
				return m.handleDeleteWorkflow()
			}

		case key.Matches(msg, m.keys.Refresh):
			return m.handleRefresh()

		case key.Matches(msg, m.keys.Create):
			if m.currentView == ViewWorkflows {
				return m.handleCreateWorkflow()
			}

		case key.Matches(msg, m.keys.Export):
			if m.currentView == ViewWorkflows {
				return m.handleExportWorkflow()
			}

		case key.Matches(msg, m.keys.Reset):
			if m.currentView == ViewWorkflows {
				return m.handleReset()
			}

		case key.Matches(msg, m.keys.Daemon):
			return m.handleDaemonView()

		case key.Matches(msg, m.keys.StartStop):
			if m.currentView == ViewDaemon {
				return m, m.toggleDaemon()
			}

		case key.Matches(msg, m.keys.PageUp):
			if m.currentView == ViewLogs {
				m.logViewport.HalfViewUp()
			} else if m.currentView == ViewDaemon {
				m.daemonViewport.HalfViewUp()
			}

		case key.Matches(msg, m.keys.PageDown):
			if m.currentView == ViewLogs {
				m.logViewport.HalfViewDown()
			} else if m.currentView == ViewDaemon {
				m.daemonViewport.HalfViewDown()
			}

		case key.Matches(msg, m.keys.Home):
			if m.currentView == ViewLogs {
				m.logViewport.GotoTop()
			} else if m.currentView == ViewDaemon {
				m.daemonViewport.GotoTop()
			}

		case key.Matches(msg, m.keys.End):
			if m.currentView == ViewLogs {
				m.logViewport.GotoBottom()
			} else if m.currentView == ViewDaemon {
				m.daemonViewport.GotoBottom()
			}
		}

	case tea.MouseMsg:
		switch msg.Button {
		case tea.MouseButtonWheelUp:
			if m.currentView == ViewLogs {
				m.logViewport.LineUp(1)
			} else if m.currentView == ViewDaemon {
				m.daemonViewport.LineUp(1)
			}
		case tea.MouseButtonWheelDown:
			if m.currentView == ViewLogs {
				m.logViewport.LineDown(1)
			} else if m.currentView == ViewDaemon {
				m.daemonViewport.LineDown(1)
			}
		case tea.MouseButtonLeft:
			if msg.Action == tea.MouseActionRelease {
				return m.handleMouseClick(msg.X, msg.Y)
			}
		}

	case WorkflowsLoadedMsg:
		m.loading = false
		if msg.Err != nil {
			m.errMessage = msg.Err.Error()
			return m, nil
		}
		m.workflows = msg.Workflows
		m.updateWorkflowList()
		return m, nil

	case RunsLoadedMsg:
		m.loading = false
		if msg.Err != nil {
			m.errMessage = msg.Err.Error()
			return m, nil
		}
		m.runs = msg.Runs
		m.updateRunList()
		return m, nil

	case StepRunsLoadedMsg:
		m.loading = false
		if msg.Err != nil {
			m.errMessage = msg.Err.Error()
			return m, nil
		}
		m.stepRuns = msg.StepRuns
		m.updateLogViewport()
		return m, nil

	case WorkflowExecutedMsg:
		m.loading = false
		if msg.Err != nil {
			m.errMessage = msg.Err.Error()
			return m, nil
		}
		m.statusMessage = "Workflow completed with status: " + string(msg.Run.Status)
		return m, m.loadWorkflows()

	case WorkflowDeletedMsg:
		m.loading = false
		if msg.Err != nil {
			m.errMessage = msg.Err.Error()
			return m, nil
		}
		m.statusMessage = "Workflow deleted successfully"
		return m, m.loadWorkflows()

	case WorkflowCreatedMsg:
		m.loading = false
		m.inputMode = InputModeNone
		m.currentView = ViewWorkflows
		if msg.Err != nil {
			m.errMessage = msg.Err.Error()
			return m, nil
		}
		m.statusMessage = "Workflow '" + msg.Workflow.Name + "' created successfully"
		return m, m.loadWorkflows()

	case WorkflowExportedMsg:
		m.loading = false
		m.inputMode = InputModeNone
		m.currentView = ViewWorkflows
		if msg.Err != nil {
			m.errMessage = msg.Err.Error()
			return m, nil
		}
		m.statusMessage = "Workflow exported to: " + msg.Path
		return m, nil

	case DataResetMsg:
		m.loading = false
		m.currentView = ViewWorkflows
		if msg.Err != nil {
			m.errMessage = msg.Err.Error()
			return m, nil
		}
		m.statusMessage = "Reset complete. Deleted " + string(rune(msg.WorkflowCount+'0')) + " workflows and " + string(rune(msg.RunCount+'0')) + " runs."
		return m, m.loadWorkflows()

	case DaemonStartedMsg:
		m.loading = false
		if msg.Err != nil {
			m.errMessage = msg.Err.Error()
			m.daemon.running = false
			return m, nil
		}
		m.statusMessage = "Daemon started"
		m.daemon.running = true
		m.daemonLogs = make([]AnimatedLogEntry, 0)
		m.animating = false
		m.updateDaemonViewport()
		return m, m.readDaemonOutput()

	case DaemonStoppedMsg:
		m.daemon.running = false
		if msg.Err != nil {
			m.errMessage = msg.Err.Error()
			return m, nil
		}
		m.statusMessage = "Daemon stopped"
		return m, nil

	case DaemonStatusMsg:
		m.daemon.running = msg.Running
		return m, nil

	case StatusMsg:
		m.statusMessage = msg.Message
		return m, nil

	case ErrorMsg:
		m.errMessage = msg.Err.Error()
		return m, nil

	case TickMsg:
		if m.currentView == ViewDaemon {
			var needsUpdate bool

			if m.daemon.running {
				m.daemon.logsMu.Lock()
				if len(m.daemon.logs) > len(m.daemonLogs) {
					for i := len(m.daemonLogs); i < len(m.daemon.logs); i++ {
						m.daemonLogs = append(m.daemonLogs, AnimatedLogEntry{
							Text:      m.daemon.logs[i],
							Frame:     0,
							Animating: true,
						})
					}
					m.animating = true
					needsUpdate = true
				}
				m.daemon.logsMu.Unlock()
			}

			if m.animating {
				stillAnimating := false
				for i := range m.daemonLogs {
					if m.daemonLogs[i].Animating {
						m.daemonLogs[i].Frame++
						if m.daemonLogs[i].Frame >= AnimationFrames {
							m.daemonLogs[i].Frame = AnimationFrames - 1
							m.daemonLogs[i].Animating = false
						} else {
							stillAnimating = true
						}
					}
				}
				m.animating = stillAnimating
				needsUpdate = true
			}

			if needsUpdate {
				m.updateDaemonViewport()
				m.daemonViewport.GotoBottom()
			}

			if m.daemon.running || m.animating {
				return m, m.readDaemonOutput()
			}
		}
		return m, nil
	}

	switch m.currentView {
	case ViewWorkflows:
		newList, cmd := m.workflowList.Update(msg)
		m.workflowList = newList
		cmds = append(cmds, cmd)
	case ViewRuns:
		newList, cmd := m.runList.Update(msg)
		m.runList = newList
		cmds = append(cmds, cmd)
	case ViewLogs:
		newViewport, cmd := m.logViewport.Update(msg)
		m.logViewport = newViewport
		cmds = append(cmds, cmd)
	case ViewDaemon:
		newViewport, cmd := m.daemonViewport.Update(msg)
		m.daemonViewport = newViewport
		cmds = append(cmds, cmd)
	case ViewCreateWorkflow, ViewExportWorkflow:
		newInput, cmd := m.textInput.Update(msg)
		m.textInput = newInput
		cmds = append(cmds, cmd)
	}

	return m, tea.Batch(cmds...)
}

func (m Model) View() string {
	return m.render()
}

func (m *Model) updateSizes() {
	availableHeight := m.height - headerHeight - footerHeight

	m.workflowList.SetSize(m.width-4, availableHeight)
	m.runList.SetSize(m.width-4, availableHeight)
	m.logViewport.Width = m.width - 6
	m.logViewport.Height = availableHeight - 4
	m.daemonViewport.Width = m.width - 6
	m.daemonViewport.Height = availableHeight - 4
	m.textInput.Width = m.width - 20
}

func (m *Model) updateDaemonViewport() {
	content := m.renderDaemonLogs()
	m.daemonViewport.SetContent(content)
}

func (m *Model) updateWorkflowList() {
	items := make([]list.Item, len(m.workflows))
	for i, w := range m.workflows {
		items[i] = WorkflowItem{workflow: w}
	}
	m.workflowList.SetItems(items)
}

func (m *Model) updateRunList() {
	items := make([]list.Item, len(m.runs))
	for i, r := range m.runs {
		workflowName := "Unknown"
		if m.selectedWorkflow != nil {
			workflowName = m.selectedWorkflow.Name
		}
		items[i] = RunItem{run: r, workflowName: workflowName}
	}
	m.runList.SetItems(items)
}

func (m *Model) updateLogViewport() {
	content := m.renderLogs()
	m.logViewport.SetContent(content)
}

func (m Model) handleBack() (tea.Model, tea.Cmd) {
	switch m.currentView {
	case ViewRuns:
		m.currentView = ViewWorkflows
		m.selectedWorkflow = nil
		m.runs = nil
	case ViewLogs:
		m.currentView = ViewRuns
		m.selectedRun = nil
		m.stepRuns = nil
	case ViewCreateWorkflow, ViewExportWorkflow:
		m.currentView = ViewWorkflows
		m.inputMode = InputModeNone
		m.textInput.SetValue("")
	case ViewConfirmReset:
		m.currentView = ViewWorkflows
	case ViewDaemon:
		m.currentView = ViewWorkflows
	}
	m.errMessage = ""
	m.statusMessage = ""
	return m, nil
}

func (m Model) handleEnter() (tea.Model, tea.Cmd) {
	switch m.currentView {
	case ViewWorkflows:
		if item, ok := m.workflowList.SelectedItem().(WorkflowItem); ok {
			m.selectedWorkflow = &item.workflow
			m.currentView = ViewRuns
			m.loading = true
			return m, m.loadRuns(item.workflow.ID)
		}
	case ViewRuns:
		if item, ok := m.runList.SelectedItem().(RunItem); ok {
			m.selectedRun = &item.run
			m.currentView = ViewLogs
			m.loading = true
			return m, m.loadStepRuns(item.run.ID)
		}
	}
	return m, nil
}

func (m Model) handleRunWorkflow() (tea.Model, tea.Cmd) {
	if item, ok := m.workflowList.SelectedItem().(WorkflowItem); ok {
		m.loading = true
		m.statusMessage = "Running workflow..."
		return m, m.executeWorkflow(&item.workflow)
	}
	return m, nil
}

func (m Model) handleDeleteWorkflow() (tea.Model, tea.Cmd) {
	if item, ok := m.workflowList.SelectedItem().(WorkflowItem); ok {
		m.loading = true
		return m, m.deleteWorkflow(item.workflow.ID)
	}
	return m, nil
}

func (m Model) handleRefresh() (tea.Model, tea.Cmd) {
	m.loading = true
	m.errMessage = ""
	m.statusMessage = ""

	switch m.currentView {
	case ViewWorkflows:
		return m, m.loadWorkflows()
	case ViewRuns:
		if m.selectedWorkflow != nil {
			return m, m.loadRuns(m.selectedWorkflow.ID)
		}
	case ViewLogs:
		if m.selectedRun != nil {
			return m, m.loadStepRuns(m.selectedRun.ID)
		}
	}
	return m, nil
}

func (m Model) handleMouseClick(x, y int) (tea.Model, tea.Cmd) {
	listY := y - headerHeight

	if listY < 0 {
		return m, nil
	}

	switch m.currentView {
	case ViewWorkflows:
		if listY >= 0 && listY < len(m.workflows) {
			itemHeight := 2
			titleOffset := 2

			clickedIndex := (listY - titleOffset) / itemHeight
			if clickedIndex >= 0 && clickedIndex < len(m.workflows) {
				m.workflowList.Select(clickedIndex)
				return m, nil
			}
		}
	case ViewRuns:
		if listY >= 0 && listY < len(m.runs) {
			itemHeight := 2
			titleOffset := 2

			clickedIndex := (listY - titleOffset) / itemHeight
			if clickedIndex >= 0 && clickedIndex < len(m.runs) {
				m.runList.Select(clickedIndex)
				return m, nil
			}
		}
	case ViewLogs:
		return m, nil
	}

	return m, nil
}

func (m Model) handleCreateWorkflow() (tea.Model, tea.Cmd) {
	m.currentView = ViewCreateWorkflow
	m.inputMode = InputModeCreate
	m.textInput.SetValue("")
	m.textInput.Placeholder = "Enter workflow YAML file path..."
	m.textInput.Focus()
	return m, textinput.Blink
}

func (m Model) handleExportWorkflow() (tea.Model, tea.Cmd) {
	if item, ok := m.workflowList.SelectedItem().(WorkflowItem); ok {
		m.selectedWorkflow = &item.workflow
		m.currentView = ViewExportWorkflow
		m.inputMode = InputModeExport
		m.textInput.SetValue("")
		m.textInput.Placeholder = "Enter output file path..."
		m.textInput.Focus()
		return m, textinput.Blink
	}
	return m, nil
}

func (m Model) handleReset() (tea.Model, tea.Cmd) {
	m.resetWorkflowCount = len(m.workflows)
	totalRuns := 0
	for _, w := range m.workflows {
		runs, err := m.db.ListRuns(&w.ID)
		if err == nil {
			totalRuns += len(runs)
		}
	}
	m.resetRunCount = totalRuns
	m.currentView = ViewConfirmReset
	return m, nil
}

func (m Model) handleDaemonView() (tea.Model, tea.Cmd) {
	m.previousView = m.currentView
	m.currentView = ViewDaemon
	return m, nil
}

func (m Model) handleTextInputKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyEsc:
		m.inputMode = InputModeNone
		m.currentView = ViewWorkflows
		m.textInput.SetValue("")
		return m, nil
	case tea.KeyEnter:
		path := m.textInput.Value()
		if path == "" {
			m.errMessage = "Path cannot be empty"
			return m, nil
		}

		m.loading = true
		switch m.inputMode {
		case InputModeCreate:
			return m, m.createWorkflow(path)
		case InputModeExport:
			if m.selectedWorkflow != nil {
				return m, m.exportWorkflow(m.selectedWorkflow.ID, path)
			}
		}
		return m, nil
	}

	var cmd tea.Cmd
	m.textInput, cmd = m.textInput.Update(msg)
	return m, cmd
}

func (m Model) handleResetConfirmKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case key.Matches(msg, m.keys.Yes):
		m.loading = true
		return m, m.resetAllData()
	case key.Matches(msg, m.keys.No), key.Matches(msg, m.keys.Back):
		m.currentView = ViewWorkflows
		return m, nil
	}
	return m, nil
}

func (m *Model) stopDaemon() {
	m.daemon.mu.Lock()
	defer m.daemon.mu.Unlock()

	if m.daemon.cmd != nil && m.daemon.cmd.Process != nil {
		m.daemon.cmd.Process.Kill()
		m.daemon.cmd.Wait()
		m.daemon.cmd = nil
		m.daemon.running = false
	}
}

func (m Model) startDaemon() tea.Cmd {
	return func() tea.Msg {
		if _, err := os.Stat(m.daemonExePath); os.IsNotExist(err) {
			return DaemonStartedMsg{Err: fmt.Errorf("daemon executable not found at '%s'. Build it with 'go build -o gork-daemon.exe ./cmd/daemon' or set GORK_DAEMON_PATH environment variable", m.daemonExePath)}
		}

		cmd := exec.Command(m.daemonExePath)

		stdout, err := cmd.StdoutPipe()
		if err != nil {
			return DaemonStartedMsg{Err: err}
		}

		stderr, err := cmd.StderrPipe()
		if err != nil {
			return DaemonStartedMsg{Err: err}
		}

		if err := cmd.Start(); err != nil {
			return DaemonStartedMsg{Err: fmt.Errorf("failed to start daemon: %w", err)}
		}

		m.daemon.mu.Lock()
		m.daemon.cmd = cmd
		m.daemon.running = true
		m.daemon.logs = make([]string, 0)
		m.daemon.mu.Unlock()

		go func() {
			scanner := bufio.NewScanner(stdout)
			scanner.Buffer(make([]byte, 64*1024), 1024*1024)
			for scanner.Scan() {
				line := scanner.Text()
				m.daemon.logsMu.Lock()
				m.daemon.logs = append(m.daemon.logs, line)
				m.daemon.logsMu.Unlock()
			}
		}()

		go func() {
			scanner := bufio.NewScanner(stderr)
			scanner.Buffer(make([]byte, 64*1024), 1024*1024)
			for scanner.Scan() {
				line := "[stderr] " + scanner.Text()
				m.daemon.logsMu.Lock()
				m.daemon.logs = append(m.daemon.logs, line)
				m.daemon.logsMu.Unlock()
			}
		}()

		go func() {
			cmd.Wait()
			m.daemon.mu.Lock()
			m.daemon.running = false
			m.daemon.mu.Unlock()
		}()

		return DaemonStartedMsg{Err: nil}
	}
}

func (m Model) readDaemonOutput() tea.Cmd {
	tickRate := 200 * time.Millisecond
	if m.animating {
		tickRate = time.Duration(AnimationFrameTime) * time.Millisecond
	}
	return tea.Tick(tickRate, func(t time.Time) tea.Msg {
		return TickMsg{}
	})
}

func (m Model) toggleDaemon() tea.Cmd {
	if m.daemon.running {
		return func() tea.Msg {
			m.stopDaemon()
			return DaemonStoppedMsg{Err: nil}
		}
	}
	return m.startDaemon()
}
