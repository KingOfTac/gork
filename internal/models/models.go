package models

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"time"
)

type StepType string

const (
	StepTypeExec   StepType = "exec"
	StepTypeHTTP   StepType = "http"
	StepTypeScript StepType = "script"
)

type RunStatus string

const (
	RunStatusPending  RunStatus = "pending"
	RunStatusRunning  RunStatus = "running"
	RunStatusSuccess  RunStatus = "success"
	RunStatusFailed   RunStatus = "failed"
	RunStatusCanceled RunStatus = "canceled"
	RunStatusTimeout  RunStatus = "timeout"
	RunStatusSkipped  RunStatus = "skipped"
)

type StepStatus string

const (
	StepStatusPending  StepStatus = "pending"
	StepStatusRunning  StepStatus = "running"
	StepStatusSuccess  StepStatus = "success"
	StepStatusFailed   StepStatus = "failed"
	StepStatusCanceled StepStatus = "canceled"
	StepStatusTimeout  StepStatus = "timeout"
	StepStatusSkipped  StepStatus = "skipped"
	StepStatusRetrying StepStatus = "retrying"
)

var (
	// Allowlist
	AllowedCommands = map[string]bool{
		// System commands
		"cmd":        true,
		"powershell": true,
		"bash":       true,
		"sh":         true,
		"curl":       true,
		"wget":       true,

		// Development tools
		"go":      true,
		"python":  true,
		"python3": true,
		"npm":     true,
		"node":    true,
		"javac":   true,
		"java":    true,

		// Build tools
		"make":  true,
		"cmake": true,
		"gcc":   true,
		"g++":   true,
		"clang": true,

		// Version control
		"git": true,

		// File operations
		"cp":    true,
		"mv":    true,
		"rm":    true,
		"mkdir": true,
		"ls":    true,
		"dir":   true,
		"type":  true,
		"cat":   true,
		"echo":  true,
		"find":  true,
		"grep":  true,

		// Windows specific
		"robocopy": true,
		"xcopy":    true,
		"del":      true,
		"timeout":  true,
	}

	// Blocklist
	DangerousCommands = map[string]bool{
		"sudo":      true,
		"su":        true,
		"runas":     true,
		"elevate":   true,
		"pkexec":    true,
		"gksu":      true,
		"kdesu":     true,
		"beesu":     true,
		"chmod":     true,
		"chown":     true,
		"passwd":    true,
		"usermod":   true,
		"mount":     true,
		"umount":    true,
		"fdisk":     true,
		"mkfs":      true,
		"dd":        true,
		"shutdown":  true,
		"reboot":    true,
		"halt":      true,
		"poweroff":  true,
		"systemctl": true,
		"service":   true,
		"init":      true,
		"telinit":   true,
		"crontab":   true,
		"at":        true,
		"ssh":       true,
		"scp":       true,
		"sftp":      true,
		"ftp":       true,
		"nc":        true,
		"ncat":      true,
		"socat":     true,
		"netstat":   true,
		"ss":        true,
		"lsof":      true,
		"ps":        true,
		"top":       true,
		"htop":      true,
		"kill":      true,
		"killall":   true,
		"pkill":     true,
		"taskkill":  true,
	}
)

type Workflow struct {
	ID          int64          `json:"id" yaml:"id"`
	Name        string         `json:"name" yaml:"name"`
	Description string         `json:"description,omitempty" yaml:"description,omitempty"`
	Schedule    string         `json:"schedule,omitempty" yaml:"schedule,omitempty"`
	Steps       []WorkflowStep `json:"steps" yaml:"steps"`
	CreatedAt   time.Time      `json:"created_at" yaml:"created_at"`
	UpdatedAt   time.Time      `json:"updated_at" yaml:"updated_at"`
}

type WorkflowStep struct {
	ID         int64             `json:"id,omitempty" yaml:"id,omitempty"`
	Name       string            `json:"name" yaml:"name"`
	DependsOn  []string          `json:"depends_on,omitempty" yaml:"depends_on,omitempty"`
	Exec       *ExecAction       `json:"exec,omitempty" yaml:"exec,omitempty"`
	HTTP       *HTTPAction       `json:"http,omitempty" yaml:"http,omitempty"`
	Script     *ScriptAction     `json:"script,omitempty" yaml:"script,omitempty"`
	Env        map[string]string `json:"env,omitempty" yaml:"env,omitempty"`
	Inputs     map[string]string `json:"inputs,omitempty" yaml:"inputs,omitempty"`
	Outputs    map[string]string `json:"outputs,omitempty" yaml:"outputs,omitempty"`
	Timeout    time.Duration     `json:"timeout,omitempty" yaml:"timeout,omitempty"`
	Retries    int               `json:"retries,omitempty" yaml:"retries,omitempty"`
	RetryDelay time.Duration     `json:"retry_delay,omitempty" yaml:"retry_delay,omitempty"`
}

type ExecAction struct {
	Command    string            `json:"command" yaml:"command"`
	Args       []string          `json:"args,omitempty" yaml:"args,omitempty"`
	Env        map[string]string `json:"env,omitempty" yaml:"env,omitempty"`
	WorkingDir string            `json:"working_dir,omitempty" yaml:"working_dir,omitempty"`
}

type HTTPAction struct {
	Method  string            `json:"method" yaml:"method"`
	URL     string            `json:"url" yaml:"url"`
	Headers map[string]string `json:"headers,omitempty" yaml:"headers,omitempty"`
	Body    string            `json:"body,omitempty" yaml:"body,omitempty"`
}

type ScriptAction struct {
	Language string `json:"language" yaml:"language"`
	Source   string `json:"source,omitempty" yaml:"source,omitempty"`
	Inline   string `json:"inline,omitempty" yaml:"inline,omitempty"`
}

type Run struct {
	ID          int64     `json:"id" yaml:"id"`
	WorkflowID  int64     `json:"workflow_id" yaml:"workflow_id"`
	Status      RunStatus `json:"status" yaml:"status"`
	StartedAt   time.Time `json:"started_at,omitempty" yaml:"started_at,omitempty"`
	CompletedAt time.Time `json:"completed_at,omitempty" yaml:"completed_at,omitempty"`
	CreatedAt   time.Time `json:"created_at" yaml:"created_at"`
	UpdatedAt   time.Time `json:"updated_at" yaml:"updated_at"`
	Trigger     string    `json:"trigger,omitempty" yaml:"trigger,omitempty"`
}

type StepRun struct {
	ID          int64      `json:"id" yaml:"id"`
	RunID       int64      `json:"run_id" yaml:"run_id"`
	StepName    string     `json:"step_name" yaml:"step_name"`
	Status      StepStatus `json:"status" yaml:"status"`
	Attempt     int        `json:"attempt" yaml:"attempt"`
	StartedAt   time.Time  `json:"started_at,omitempty" yaml:"started_at,omitempty"`
	CompletedAt time.Time  `json:"completed_at,omitempty" yaml:"completed_at,omitempty"`
	Error       string     `json:"error,omitempty" yaml:"error,omitempty"`
	Logs        []string   `json:"logs,omitempty" yaml:"logs,omitempty"`
}

func (s StepStatus) IsTerminal() bool {
	switch s {
	case StepStatusSuccess, StepStatusFailed, StepStatusCanceled, StepStatusTimeout, StepStatusSkipped:
		return true
	default:
		return false
	}
}

func (s RunStatus) IsTerminal() bool {
	switch s {
	case RunStatusSuccess, RunStatusFailed, RunStatusCanceled, RunStatusTimeout, RunStatusSkipped:
		return true
	default:
		return false
	}
}

func (w Workflow) Validate() error {
	if strings.TrimSpace(w.Name) == "" {
		return errors.New("workflow name is required")
	}
	if len(w.Steps) == 0 {
		return errors.New("workflow must contain at least one step")
	}

	stepsByName := make(map[string]WorkflowStep, len(w.Steps))
	for _, step := range w.Steps {
		if _, exists := stepsByName[step.Name]; exists {
			return fmt.Errorf("duplicate step name %q", step.Name)
		}

		if err := step.Validate(); err != nil {
			return fmt.Errorf("step %q: %w", step.Name, err)
		}

		stepsByName[step.Name] = step
	}

	for _, step := range w.Steps {
		for _, dep := range step.DependsOn {
			if dep == step.Name {
				return fmt.Errorf("step %q cannot depend on itself", step.Name)
			}

			if _, ok := stepsByName[dep]; !ok {
				return fmt.Errorf("step %q depends on unknown step %q", step.Name, dep)
			}
		}
	}

	if err := detectCycles(stepsByName); err != nil {
		return err
	}

	return nil
}

func (s WorkflowStep) Validate() error {
	if strings.TrimSpace(s.Name) == "" {
		return errors.New("step name is required")
	}

	actionCount := 0
	if s.Exec != nil {
		actionCount++
		if err := s.Exec.Validate(); err != nil {
			return fmt.Errorf("exec action: %w", err)
		}
	}
	if s.HTTP != nil {
		actionCount++
		if err := s.HTTP.Validate(); err != nil {
			return fmt.Errorf("http action: %w", err)
		}
	}
	if s.Script != nil {
		actionCount++
		if err := s.Script.Validate(); err != nil {
			return fmt.Errorf("script action: %w", err)
		}
	}

	if actionCount == 0 {
		return errors.New("step must define exactly one action")
	}
	if actionCount > 1 {
		return errors.New("only one action may be defined per step")
	}

	if s.Retries < 0 {
		return errors.New("retries cannot be negative")
	}
	if s.RetryDelay < 0 {
		return errors.New("retry delay cannot be negative")
	}
	if s.Timeout < 0 {
		return errors.New("timeout cannot be negative")
	}

	for k := range s.Env {
		if strings.Contains(k, "=") {
			return fmt.Errorf("environment variable key '%s' cannot contain '='", k)
		}
	}

	for inputKey, inputSpec := range s.Inputs {
		if !strings.Contains(inputSpec, ".") {
			return fmt.Errorf("input '%s' must be in format 'step_name.key_name'", inputKey)
		}
		parts := strings.Split(inputSpec, ".")
		if len(parts) != 2 {
			return fmt.Errorf("input '%s' has invalid format", inputKey)
		}
		if strings.Contains(inputKey, "=") || strings.Contains(inputKey, ";") || strings.Contains(inputKey, "|") {
			return fmt.Errorf("input key '%s' contains invalid characters", inputKey)
		}
	}

	if len(s.DependsOn) > 0 {
		seen := make(map[string]struct{}, len(s.DependsOn))
		for _, dep := range s.DependsOn {
			if strings.TrimSpace(dep) == "" {
				return errors.New("dependency names must be non-empty")
			}
			if _, exists := seen[dep]; exists {
				return fmt.Errorf("duplicate dependency %q", dep)
			}
			seen[dep] = struct{}{}
		}
	}

	return nil
}

func (e ExecAction) Validate() error {
	command := strings.TrimSpace(e.Command)
	if command == "" {
		return errors.New("exec command is required")
	}

	if DangerousCommands[strings.ToLower(command)] {
		return fmt.Errorf("command '%s' is not allowed for security reasons", command)
	}

	isLocalExec := strings.HasPrefix(command, "./") || strings.HasPrefix(command, ".\\")
	isPathBased := strings.Contains(command, "/") || strings.Contains(command, "\\")
	if !AllowedCommands[strings.ToLower(command)] && !filepath.IsAbs(command) && !isPathBased && !isLocalExec {
		return fmt.Errorf("command '%s' is not in the allowed commands list", command)
	}

	if e.WorkingDir != "" {
		workingDir := filepath.Clean(e.WorkingDir)
		if filepath.IsAbs(workingDir) {
			return errors.New("absolute working directories are not allowed")
		}
		if strings.Contains(workingDir, "..") {
			return errors.New("working directory cannot contain '..' (parent directory traversal)")
		}
	}

	for k := range e.Env {
		if strings.Contains(k, "=") {
			return fmt.Errorf("environment variable key '%s' cannot contain '='", k)
		}
	}

	return nil
}

func (h HTTPAction) Validate() error {
	if strings.TrimSpace(h.URL) == "" {
		return errors.New("http url is required")
	}
	return nil
}

func (s ScriptAction) Validate() error {
	if strings.TrimSpace(s.Inline) == "" {
		return errors.New("script inline content is required")
	}
	return nil
}

func detectCycles(steps map[string]WorkflowStep) error {
	type visitState int
	const (
		stateUnvisited visitState = iota
		stateVisiting
		stateVisited
	)

	state := make(map[string]visitState, len(steps))

	var visit func(name string) error
	visit = func(name string) error {
		switch state[name] {
		case stateVisiting:
			return fmt.Errorf("dependency cycle detected at step %q", name)
		case stateVisited:
			return nil
		}

		state[name] = stateVisiting
		step := steps[name]
		for _, dep := range step.DependsOn {
			if err := visit(dep); err != nil {
				return err
			}
		}
		state[name] = stateVisited
		return nil
	}

	for name := range steps {
		if state[name] == stateUnvisited {
			if err := visit(name); err != nil {
				return err
			}
		}
	}

	return nil
}

func (s WorkflowStep) ActionType() StepType {
	if s.Exec != nil {
		return StepTypeExec
	}
	if s.HTTP != nil {
		return StepTypeHTTP
	}
	if s.Script != nil {
		return StepTypeScript
	}
	return ""
}
