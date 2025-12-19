package tui

import (
	"github.com/kingoftac/gork/internal/models"
)

type WorkflowsLoadedMsg struct {
	Workflows []models.Workflow
	Err       error
}

type RunsLoadedMsg struct {
	Runs []models.Run
	Err  error
}

type StepRunsLoadedMsg struct {
	StepRuns []models.StepRun
	Err      error
}

type WorkflowExecutedMsg struct {
	Run *models.Run
	Err error
}

type WorkflowDeletedMsg struct {
	ID  int64
	Err error
}

type WorkflowCreatedMsg struct {
	Workflow *models.Workflow
	Err      error
}

type WorkflowExportedMsg struct {
	Path string
	Err  error
}

type DataResetMsg struct {
	WorkflowCount int
	RunCount      int
	Err           error
}

type DaemonStartedMsg struct {
	Err error
}

type DaemonStoppedMsg struct {
	Err error
}

type DaemonOutputMsg struct {
	Line string
}

type DaemonStatusMsg struct {
	Running bool
}

type ErrorMsg struct {
	Err error
}

type StatusMsg struct {
	Message string
}

type TickMsg struct{}
