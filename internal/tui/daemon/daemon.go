package daemon

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"sync"
	"time"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/kingoftac/gork/internal/tui/common"
)

// AnimatedLogEntry represents a log entry with animation state
type AnimatedLogEntry struct {
	Text      string
	Frame     int  // Current animation frame (0 to AnimationFrames-1)
	Animating bool // Whether this entry is still animating
}

// Process manages the daemon subprocess
type Process struct {
	cmd     *exec.Cmd
	running bool
	mu      sync.Mutex
	logs    []string
	logsMu  sync.Mutex
}

// Model represents the daemon management feature
type Model struct {
	process       *Process
	viewport      viewport.Model
	logs          []AnimatedLogEntry
	exePath       string
	animating     bool
	width         int
	height        int
	statusMessage string
	errMessage    string
}

// New creates a new daemon model
func New(exePath string) Model {
	vp := viewport.New(0, 0)
	vp.Style = common.PanelStyle

	return Model{
		process:  &Process{},
		viewport: vp,
		logs:     make([]AnimatedLogEntry, 0),
		exePath:  exePath,
	}
}

// SetSize updates the component size
func (m *Model) SetSize(width, height int) {
	m.width = width
	m.height = height
	m.viewport.Width = width - 6
	m.viewport.Height = height - 4
}

// Running returns whether the daemon is running
func (m Model) Running() bool {
	return m.process.running
}

// Animating returns whether animations are in progress
func (m Model) Animating() bool {
	return m.animating
}

// StatusMessage returns the status message
func (m Model) StatusMessage() string {
	return m.statusMessage
}

// SetStatusMessage sets the status message
func (m *Model) SetStatusMessage(msg string) {
	m.statusMessage = msg
}

// ErrMessage returns the error message
func (m Model) ErrMessage() string {
	return m.errMessage
}

// SetErrMessage sets the error message
func (m *Model) SetErrMessage(msg string) {
	m.errMessage = msg
}

// ClearMessages clears status and error messages
func (m *Model) ClearMessages() {
	m.errMessage = ""
	m.statusMessage = ""
}

// Update handles messages for the daemon model
func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case common.DaemonStartedMsg:
		if msg.Err != nil {
			m.errMessage = msg.Err.Error()
			m.process.running = false
			return m, nil
		}
		m.statusMessage = "Daemon started"
		m.process.running = true
		// Clear the TUI logs to sync with the daemon's fresh log buffer
		m.logs = make([]AnimatedLogEntry, 0)
		m.animating = false
		m.updateViewport()
		return m, m.Tick()

	case common.DaemonStoppedMsg:
		m.process.running = false
		if msg.Err != nil {
			m.errMessage = msg.Err.Error()
			return m, nil
		}
		m.statusMessage = "Daemon stopped"
		return m, nil

	case common.TickMsg:
		var needsUpdate bool

		// Check for new logs from daemon
		if m.process.running {
			m.process.logsMu.Lock()
			if len(m.process.logs) > len(m.logs) {
				// Add new logs with animation
				for i := len(m.logs); i < len(m.process.logs); i++ {
					m.logs = append(m.logs, AnimatedLogEntry{
						Text:      m.process.logs[i],
						Frame:     0,
						Animating: true,
					})
				}
				m.animating = true
				needsUpdate = true
			}
			m.process.logsMu.Unlock()
		}

		// Advance animation frames for entries still animating
		if m.animating {
			stillAnimating := false
			for i := range m.logs {
				if m.logs[i].Animating {
					m.logs[i].Frame++
					if m.logs[i].Frame >= common.AnimationFrames {
						m.logs[i].Frame = common.AnimationFrames - 1
						m.logs[i].Animating = false
					} else {
						stillAnimating = true
					}
				}
			}
			m.animating = stillAnimating
			needsUpdate = true
		}

		if needsUpdate {
			m.updateViewport()
			m.viewport.GotoBottom()
		}

		// Continue ticking if daemon is running or animating
		if m.process.running || m.animating {
			return m, m.Tick()
		}
		return m, nil
	}

	m.viewport, cmd = m.viewport.Update(msg)
	return m, cmd
}

func (m *Model) updateViewport() {
	content := m.renderLogs()
	m.viewport.SetContent(content)
}

// View renders the daemon view
func (m Model) View() string {
	var statusText string
	if m.process.running {
		statusText = common.StatusSuccessStyle.Render("● Running")
	} else {
		statusText = common.StatusFailedStyle.Render("○ Stopped")
	}

	title := common.TitleStyle.Render("Daemon Management")
	status := common.SubtitleStyle.Render(fmt.Sprintf("Status: %s", statusText))

	logsTitle := common.SubtitleStyle.Render("Logs:")

	return lipgloss.JoinVertical(
		lipgloss.Left,
		title,
		status,
		"",
		logsTitle,
		m.viewport.View(),
	)
}

func (m Model) renderLogs() string {
	if len(m.logs) == 0 {
		return common.DimmedItemStyle.Render("No daemon logs yet. Press 's' to start the daemon.")
	}

	var sb fmt.Stringer = &logBuilder{}
	builder := sb.(*logBuilder)
	for i, entry := range m.logs {
		lineNum := common.LogLineNumberStyle.Render(fmt.Sprintf("%d", i+1))

		// Apply fade-in animation based on frame
		var logStyle lipgloss.Style
		if entry.Frame < len(common.FadeColors) {
			logStyle = lipgloss.NewStyle().Foreground(common.FadeColors[entry.Frame])
		} else {
			logStyle = common.LogStyle
		}

		builder.WriteString(fmt.Sprintf("%s %s\n", lineNum, logStyle.Render(entry.Text)))
	}
	return builder.String()
}

type logBuilder struct {
	data []byte
}

func (b *logBuilder) WriteString(s string) {
	b.data = append(b.data, s...)
}

func (b *logBuilder) String() string {
	return string(b.data)
}

// Commands

// Tick returns a tick command for polling daemon output
func (m Model) Tick() tea.Cmd {
	// Use faster tick rate when animating for smooth fade-in
	tickRate := 200 * time.Millisecond
	if m.animating {
		tickRate = time.Duration(common.AnimationFrameTime) * time.Millisecond
	}
	return tea.Tick(tickRate, func(t time.Time) tea.Msg {
		return common.TickMsg{}
	})
}

// Toggle toggles the daemon on/off
func (m *Model) Toggle() tea.Cmd {
	if m.process.running {
		return func() tea.Msg {
			m.Stop()
			return common.DaemonStoppedMsg{Err: nil}
		}
	}
	return m.Start()
}

// Start starts the daemon
func (m *Model) Start() tea.Cmd {
	return func() tea.Msg {
		// Check if daemon executable exists
		if _, err := os.Stat(m.exePath); os.IsNotExist(err) {
			return common.DaemonStartedMsg{Err: fmt.Errorf("daemon executable not found at '%s'. Build it with 'go build -o gork-daemon.exe ./cmd/daemon' or set GORK_DAEMON_PATH environment variable", m.exePath)}
		}

		cmd := exec.Command(m.exePath)

		stdout, err := cmd.StdoutPipe()
		if err != nil {
			return common.DaemonStartedMsg{Err: err}
		}

		stderr, err := cmd.StderrPipe()
		if err != nil {
			return common.DaemonStartedMsg{Err: err}
		}

		if err := cmd.Start(); err != nil {
			return common.DaemonStartedMsg{Err: fmt.Errorf("failed to start daemon: %w", err)}
		}

		m.process.mu.Lock()
		m.process.cmd = cmd
		m.process.running = true
		m.process.logs = make([]string, 0)
		m.process.mu.Unlock()

		// Start goroutine to read stdout
		go func() {
			scanner := bufio.NewScanner(stdout)
			scanner.Buffer(make([]byte, 64*1024), 1024*1024)
			for scanner.Scan() {
				line := scanner.Text()
				m.process.logsMu.Lock()
				m.process.logs = append(m.process.logs, line)
				m.process.logsMu.Unlock()
			}
		}()

		// Start goroutine to read stderr
		go func() {
			scanner := bufio.NewScanner(stderr)
			scanner.Buffer(make([]byte, 64*1024), 1024*1024)
			for scanner.Scan() {
				line := "[stderr] " + scanner.Text()
				m.process.logsMu.Lock()
				m.process.logs = append(m.process.logs, line)
				m.process.logsMu.Unlock()
			}
		}()

		// Wait for process to exit in background
		go func() {
			cmd.Wait()
			m.process.mu.Lock()
			m.process.running = false
			m.process.mu.Unlock()
		}()

		return common.DaemonStartedMsg{Err: nil}
	}
}

// Stop stops the daemon
func (m *Model) Stop() {
	m.process.mu.Lock()
	defer m.process.mu.Unlock()

	if m.process.cmd != nil && m.process.cmd.Process != nil {
		m.process.cmd.Process.Kill()
		m.process.cmd.Wait()
		m.process.cmd = nil
		m.process.running = false
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
