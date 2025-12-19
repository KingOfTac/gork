package common

import (
	"github.com/kingoftac/gork/internal/models"
)

// Messages for async operations

// WorkflowsLoadedMsg is sent when workflows are loaded from the database
type WorkflowsLoadedMsg struct {
	Workflows []models.Workflow
	Err       error
}

// RunsLoadedMsg is sent when runs are loaded from the database
type RunsLoadedMsg struct {
	Runs []models.Run
	Err  error
}

// StepRunsLoadedMsg is sent when step runs are loaded from the database
type StepRunsLoadedMsg struct {
	StepRuns []models.StepRun
	Err      error
}

// WorkflowExecutedMsg is sent when a workflow execution completes
type WorkflowExecutedMsg struct {
	Run *models.Run
	Err error
}

// WorkflowDeletedMsg is sent when a workflow is deleted
type WorkflowDeletedMsg struct {
	ID  int64
	Err error
}

// WorkflowCreatedMsg is sent when a workflow is created
type WorkflowCreatedMsg struct {
	Workflow *models.Workflow
	Err      error
}

// WorkflowExportedMsg is sent when a workflow is exported
type WorkflowExportedMsg struct {
	Path string
	Err  error
}

// DataResetMsg is sent when all data is reset
type DataResetMsg struct {
	WorkflowCount int
	RunCount      int
	Err           error
}

// DaemonStartedMsg is sent when the daemon starts
type DaemonStartedMsg struct {
	Err error
}

// DaemonStoppedMsg is sent when the daemon stops
type DaemonStoppedMsg struct {
	Err error
}

// DaemonStatusMsg is sent with daemon status updates
type DaemonStatusMsg struct {
	Running bool
}

// DaemonOutputMsg is sent with daemon output
type DaemonOutputMsg struct {
	Line string
}

// StatusMsg is sent for status messages
type StatusMsg struct {
	Message string
}

// ErrorMsg is sent for error messages
type ErrorMsg struct {
	Err error
}

// TickMsg is sent for periodic updates
type TickMsg struct{}

// NavigateMsg is sent when navigating between views
type NavigateMsg struct {
	View View
}
