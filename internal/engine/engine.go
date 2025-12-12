package engine

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"gopkg.in/yaml.v3"

	"github.com/kingoftac/gork/internal/db"
	"github.com/kingoftac/gork/internal/models"
	"github.com/kingoftac/gork/internal/runner"
)

type Engine struct {
	db *db.DB
	mu sync.Mutex
}

func NewEngine(db *db.DB) *Engine {
	return &Engine{db: db}
}

func (e *Engine) LoadWorkflow(filePath string) (*models.Workflow, error) {
	// Security: Validate file path to prevent directory traversal
	cleanPath := filepath.Clean(filePath)
	if filepath.IsAbs(cleanPath) {
		return nil, fmt.Errorf("absolute file paths are not allowed")
	}
	if strings.Contains(cleanPath, "..") {
		return nil, fmt.Errorf("file path cannot contain '..' (directory traversal)")
	}

	// Only allow files in workflows/ directory or current directory
	if !strings.HasPrefix(cleanPath, "workflows/") && !strings.HasPrefix(cleanPath, "./workflows/") &&
		!strings.HasPrefix(cleanPath, "workflows\\") && !strings.HasPrefix(cleanPath, ".\\workflows\\") &&
		filepath.Dir(cleanPath) != "." {
		return nil, fmt.Errorf("workflows must be loaded from the workflows/ directory")
	}

	data, err := os.ReadFile(cleanPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	var workflow models.Workflow
	if err := yaml.Unmarshal(data, &workflow); err != nil {
		return nil, fmt.Errorf("failed to unmarshal YAML: %w", err)
	}

	if err := workflow.Validate(); err != nil {
		return nil, fmt.Errorf("invalid workflow: %w", err)
	}

	return &workflow, nil
}

func (e *Engine) ExecuteWorkflow(ctx context.Context, workflow *models.Workflow, trigger string) (*models.Run, error) {
	run := &models.Run{
		WorkflowID: workflow.ID,
		Status:     models.RunStatusPending,
		StartedAt:  time.Now(),
		Trigger:    trigger,
	}

	runID, err := e.db.InsertRun(run)
	if err != nil {
		return nil, fmt.Errorf("failed to insert run: %w", err)
	}
	run.ID = runID

	if err := e.db.UpdateRunStatus(runID, models.RunStatusRunning, nil); err != nil {
		return nil, fmt.Errorf("failed to update run status: %w", err)
	}
	run.Status = models.RunStatusRunning

	// Build dependency graph
	stepMap := make(map[string]models.WorkflowStep)
	for _, step := range workflow.Steps {
		stepMap[step.Name] = step
	}

	// Channels for signaling completion
	doneChans := make(map[string]chan struct{})
	for name := range stepMap {
		doneChans[name] = make(chan struct{})
	}

	// Goroutines for each step
	errCh := make(chan error, len(workflow.Steps))
	for _, step := range workflow.Steps {
		go e.executeStep(ctx, runID, step, stepMap, doneChans, errCh)
	}

	// Wait for all steps to complete
	for i := 0; i < len(workflow.Steps); i++ {
		if err := <-errCh; err != nil {
			// On error, cancel context if not already
			// But since goroutines may still be running, perhaps mark run as failed
			e.db.UpdateRunStatus(runID, models.RunStatusFailed, &time.Time{})
			return run, err
		}
	}

	// Check if all steps succeeded
	stepRuns, err := e.db.GetStepRuns(runID)
	if err != nil {
		return nil, fmt.Errorf("failed to get step runs: %w", err)
	}

	runStatus := models.RunStatusSuccess
	for _, sr := range stepRuns {
		if sr.Status == models.StepStatusFailed || sr.Status == models.StepStatusTimeout {
			runStatus = models.RunStatusFailed
			break
		}
	}

	completedAt := time.Now()
	if err := e.db.UpdateRunStatus(runID, runStatus, &completedAt); err != nil {
		return nil, fmt.Errorf("failed to update run status: %w", err)
	}
	run.Status = runStatus
	run.CompletedAt = completedAt

	return run, nil
}

func (e *Engine) executeStep(ctx context.Context, runID int64, step models.WorkflowStep, stepMap map[string]models.WorkflowStep, doneChans map[string]chan struct{}, errCh chan<- error) {
	// Wait for dependencies
	for _, dep := range step.DependsOn {
		select {
		case <-doneChans[dep]:
		case <-ctx.Done():
			errCh <- ctx.Err()
			return
		}
	}

	// Resolve inputs from previous step outputs
	resolvedStep, err := e.resolveStepInputs(runID, step)
	if err != nil {
		errCh <- fmt.Errorf("failed to resolve step inputs: %w", err)
		return
	}

	// Insert step run
	stepRun := &models.StepRun{
		RunID:    runID,
		StepName: step.Name,
		Status:   models.StepStatusPending,
		Attempt:  0,
		Logs:     []string{},
	}
	stepRunID, err := e.db.InsertStepRun(stepRun)
	if err != nil {
		errCh <- fmt.Errorf("failed to insert step run: %w", err)
		return
	}
	stepRun.ID = stepRunID

	// Update to running
	if err := e.db.UpdateStepRun(stepRunID, models.StepStatusRunning, nil, "", []string{}); err != nil {
		errCh <- fmt.Errorf("failed to update step run: %w", err)
		return
	}

	// Execute with retries
	var lastErr error
	for attempt := 0; attempt <= step.Retries; attempt++ {
		stepRun.Attempt = attempt
		if attempt > 0 {
			e.mu.Lock()
			if err := e.db.UpdateStepRun(stepRunID, models.StepStatusRetrying, nil, "", stepRun.Logs); err != nil {
				e.mu.Unlock()
				errCh <- fmt.Errorf("failed to update step run: %w", err)
				return
			}
			e.mu.Unlock()
			select {
			case <-time.After(step.RetryDelay):
			case <-ctx.Done():
				errCh <- ctx.Err()
				return
			}
		}

		stepCtx := ctx
		var cancel context.CancelFunc
		if step.Timeout > 0 {
			stepCtx, cancel = context.WithTimeout(ctx, time.Duration(step.Timeout))
			defer cancel()
		}

		logs, err := runner.RunStep(stepCtx, resolvedStep)
		stepRun.Logs = append(stepRun.Logs, logs...)

		e.mu.Lock()
		if err := e.db.AppendLogs(stepRunID, logs); err != nil {
			e.mu.Unlock()
			errCh <- fmt.Errorf("failed to append logs: %w", err)
			return
		}
		e.mu.Unlock()

		if err == nil {
			// Store outputs
			if err := e.storeStepOutputs(runID, step, logs); err != nil {
				errCh <- fmt.Errorf("failed to store step outputs: %w", err)
				return
			}

			// Success
			completedAt := time.Now()
			e.mu.Lock()
			if err := e.db.UpdateStepRun(stepRunID, models.StepStatusSuccess, &completedAt, "", stepRun.Logs); err != nil {
				e.mu.Unlock()
				errCh <- fmt.Errorf("failed to update step run: %w", err)
				return
			}
			e.mu.Unlock()
			break
		}

		lastErr = err
		if attempt < step.Retries {
			continue
		}

		// Failed after retries
		completedAt := time.Now()
		status := models.StepStatusFailed
		if stepCtx.Err() == context.DeadlineExceeded {
			status = models.StepStatusTimeout
		}
		e.mu.Lock()
		if err := e.db.UpdateStepRun(stepRunID, status, &completedAt, lastErr.Error(), stepRun.Logs); err != nil {
			e.mu.Unlock()
			errCh <- fmt.Errorf("failed to update step run: %w", err)
			return
		}
		e.mu.Unlock()
		errCh <- fmt.Errorf("step %s failed: %w", step.Name, lastErr)
		return
	}

	// Signal completion
	close(doneChans[step.Name])
	errCh <- nil
}

// topologicalSort is not needed since we use channels, but for validation we already have it in models
func topologicalSort(steps map[string]models.WorkflowStep) ([]string, error) {
	inDegree := make(map[string]int)
	graph := make(map[string][]string)

	for name, step := range steps {
		if inDegree[name] == 0 {
			inDegree[name] = 0
		}
		for _, dep := range step.DependsOn {
			graph[dep] = append(graph[dep], name)
			inDegree[name]++
		}
	}

	var queue []string
	for name, degree := range inDegree {
		if degree == 0 {
			queue = append(queue, name)
		}
	}

	var result []string
	for len(queue) > 0 {
		sort.Strings(queue)
		current := queue[0]
		queue = queue[1:]
		result = append(result, current)

		for _, neighbor := range graph[current] {
			inDegree[neighbor]--
			if inDegree[neighbor] == 0 {
				queue = append(queue, neighbor)
			}
		}
	}

	if len(result) != len(steps) {
		return nil, fmt.Errorf("cycle detected")
	}

	return result, nil
}

func (e *Engine) resolveStepInputs(runID int64, step models.WorkflowStep) (models.WorkflowStep, error) {
	resolvedStep := step

	// Load all step data for this run
	stepData, err := e.db.GetAllStepData(runID)
	if err != nil {
		return resolvedStep, fmt.Errorf("failed to get step data: %w", err)
	}

	// Resolve inputs
	for inputKey, inputSpec := range step.Inputs {
		// inputSpec format: "step_name.key_name"
		parts := strings.Split(inputSpec, ".")
		if len(parts) != 2 {
			return resolvedStep, fmt.Errorf("invalid input spec %s: expected format step_name.key_name", inputSpec)
		}
		sourceStep, keyName := parts[0], parts[1]

		if stepOutputs, exists := stepData[sourceStep]; exists {
			if value, hasKey := stepOutputs[keyName]; hasKey {
				// Add to environment variables
				if resolvedStep.Env == nil {
					resolvedStep.Env = make(map[string]string)
				}
				resolvedStep.Env[inputKey] = value
			} else {
				return resolvedStep, fmt.Errorf("input %s references non-existent output %s from step %s", inputKey, keyName, sourceStep)
			}
		} else {
			return resolvedStep, fmt.Errorf("input %s references non-existent step %s", inputKey, sourceStep)
		}
	}

	return resolvedStep, nil
}

func (e *Engine) storeStepOutputs(runID int64, step models.WorkflowStep, logs []string) error {
	for outputKey, outputSpec := range step.Outputs {
		// Simple extraction: find line containing the spec and extract everything after it
		for _, log := range logs {
			if idx := strings.Index(log, outputSpec); idx >= 0 {
				value := strings.TrimSpace(log[idx+len(outputSpec):])
				if value != "" {
					if err := e.db.StoreStepData(runID, step.Name, outputKey, value); err != nil {
						return fmt.Errorf("failed to store output %s: %w", outputKey, err)
					}
				}
			}
		}
	}
	return nil
}
