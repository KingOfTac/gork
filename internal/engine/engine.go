package engine

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"gopkg.in/yaml.v3"

	"github.com/kingoftac/gork/internal/db"
	"github.com/kingoftac/gork/internal/models"
	"github.com/kingoftac/gork/internal/runner"
)

type Engine struct {
	db          *db.DB
	mu          sync.Mutex
	verboseLogs bool
}

func NewEngine(db *db.DB) *Engine {
	return &Engine{db: db, verboseLogs: false}
}

func NewEngineWithVerboseLogs(db *db.DB) *Engine {
	return &Engine{db: db, verboseLogs: true}
}

func (e *Engine) LoadWorkflow(filePath string) (*models.Workflow, error) {
	cleanPath := filepath.Clean(filePath)
	if filepath.IsAbs(cleanPath) {
		return nil, fmt.Errorf("absolute file paths are not allowed")
	}
	if strings.Contains(cleanPath, "..") {
		return nil, fmt.Errorf("file path cannot contain '..' (directory traversal)")
	}

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

	stepMap := make(map[string]models.WorkflowStep)
	for _, step := range workflow.Steps {
		stepMap[step.Name] = step
	}

	doneChans := make(map[string]chan struct{})
	for name := range stepMap {
		doneChans[name] = make(chan struct{})
	}

	errCh := make(chan error, len(workflow.Steps))
	for _, step := range workflow.Steps {
		go e.executeStep(ctx, runID, step, stepMap, doneChans, errCh)
	}

	for i := 0; i < len(workflow.Steps); i++ {
		if err := <-errCh; err != nil {
			e.db.UpdateRunStatus(runID, models.RunStatusFailed, &time.Time{})
			return run, err
		}
	}

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
	for _, dep := range step.DependsOn {
		select {
		case <-doneChans[dep]:
		case <-ctx.Done():
			errCh <- ctx.Err()
			return
		}
	}

	resolvedStep, err := e.resolveStepInputs(runID, step)
	if err != nil {
		errCh <- fmt.Errorf("failed to resolve step inputs: %w", err)
		return
	}

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

	if err := e.db.UpdateStepRun(stepRunID, models.StepStatusRunning, nil, "", []string{}); err != nil {
		errCh <- fmt.Errorf("failed to update step run: %w", err)
		return
	}

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

		if e.verboseLogs && len(logs) > 0 {
			slog.Info("Step output", "step", step.Name, "attempt", attempt+1)
			for _, line := range logs {
				fmt.Printf("  [%s] %s\n", step.Name, line)
			}
			os.Stdout.Sync()
		}

		e.mu.Lock()
		if err := e.db.AppendLogs(stepRunID, logs); err != nil {
			e.mu.Unlock()
			errCh <- fmt.Errorf("failed to append logs: %w", err)
			return
		}
		e.mu.Unlock()

		if err == nil {
			if err := e.storeStepOutputs(runID, step, logs); err != nil {
				errCh <- fmt.Errorf("failed to store step outputs: %w", err)
				return
			}

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

	close(doneChans[step.Name])
	errCh <- nil
}

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

	stepData, err := e.db.GetAllStepData(runID)
	if err != nil {
		return resolvedStep, fmt.Errorf("failed to get step data: %w", err)
	}

	for inputKey, inputSpec := range step.Inputs {
		parts := strings.Split(inputSpec, ".")
		if len(parts) != 2 {
			return resolvedStep, fmt.Errorf("invalid input spec %s: expected format step_name.key_name", inputSpec)
		}
		sourceStep, keyName := parts[0], parts[1]

		if stepOutputs, exists := stepData[sourceStep]; exists {
			if value, hasKey := stepOutputs[keyName]; hasKey {
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
	for _, log := range logs {
		if strings.HasPrefix(log, "HTTP_STATUS:") {
			value := strings.TrimPrefix(log, "HTTP_STATUS:")
			if err := e.db.StoreStepData(runID, step.Name, "status", value); err != nil {
				return fmt.Errorf("failed to store http status: %w", err)
			}
		} else if strings.HasPrefix(log, "HTTP_BODY:") {
			value := strings.TrimPrefix(log, "HTTP_BODY:")
			if err := e.db.StoreStepData(runID, step.Name, "body", value); err != nil {
				return fmt.Errorf("failed to store http body: %w", err)
			}
		} else if strings.HasPrefix(log, "HTTP_HEADER_") {
			rest := strings.TrimPrefix(log, "HTTP_HEADER_")
			if idx := strings.Index(rest, ":"); idx >= 0 {
				headerName := strings.ToLower(rest[:idx])
				headerValue := rest[idx+1:]
				if err := e.db.StoreStepData(runID, step.Name, "header_"+headerName, headerValue); err != nil {
					return fmt.Errorf("failed to store http header %s: %w", headerName, err)
				}
			}
		}
	}

	for outputKey, outputSpec := range step.Outputs {
		var value string

		switch {
		case strings.HasPrefix(outputSpec, "json_path:"):
			jsonPath := strings.TrimPrefix(outputSpec, "json_path:")
			for _, log := range logs {
				if strings.HasPrefix(log, "HTTP_BODY:") {
					body := strings.TrimPrefix(log, "HTTP_BODY:")
					extracted, err := extractJSONPath(body, jsonPath)
					if err == nil && extracted != "" {
						value = extracted
					}
					break
				}
			}

		case strings.HasPrefix(outputSpec, "regex:"):
			pattern := strings.TrimPrefix(outputSpec, "regex:")
			re, err := regexp.Compile(pattern)
			if err == nil {
				fullLog := strings.Join(logs, "\n")
				matches := re.FindStringSubmatch(fullLog)
				if len(matches) > 1 {
					value = matches[1]
				} else if len(matches) == 1 {
					value = matches[0]
				}
			}

		case outputSpec == "body":
			for _, log := range logs {
				if strings.HasPrefix(log, "HTTP_BODY:") {
					value = strings.TrimPrefix(log, "HTTP_BODY:")
					break
				}
			}

		case outputSpec == "status":
			for _, log := range logs {
				if strings.HasPrefix(log, "HTTP_STATUS:") {
					value = strings.TrimPrefix(log, "HTTP_STATUS:")
					break
				}
			}

		case outputSpec == "full_output":
			value = strings.Join(logs, "\n")

		default:
			for _, log := range logs {
				if idx := strings.Index(log, outputSpec); idx >= 0 {
					value = strings.TrimSpace(log[idx+len(outputSpec):])
					if value != "" {
						break
					}
				}
			}
		}

		if value != "" {
			if err := e.db.StoreStepData(runID, step.Name, outputKey, value); err != nil {
				return fmt.Errorf("failed to store output %s: %w", outputKey, err)
			}
		}
	}
	return nil
}

// extractJSONPath extracts a value from JSON using a simple path syntax
// Supports: $.key, $.key.nested, $.array[0], $.array[*].field
func extractJSONPath(jsonStr, path string) (string, error) {
	if !strings.HasPrefix(path, "$.") {
		return "", fmt.Errorf("json path must start with $.")
	}

	var data interface{}
	if err := json.Unmarshal([]byte(jsonStr), &data); err != nil {
		return "", err
	}

	path = strings.TrimPrefix(path, "$.")
	parts := splitJSONPath(path)

	current := data
	for _, part := range parts {
		if part == "" {
			continue
		}

		if idx := strings.Index(part, "["); idx >= 0 {
			fieldName := part[:idx]
			indexStr := strings.TrimSuffix(strings.TrimPrefix(part[idx:], "["), "]")

			if fieldName != "" {
				if m, ok := current.(map[string]interface{}); ok {
					current = m[fieldName]
				} else {
					return "", fmt.Errorf("cannot access field %s on non-object", fieldName)
				}
			}

			arr, ok := current.([]interface{})
			if !ok {
				return "", fmt.Errorf("cannot index non-array")
			}

			if indexStr == "*" {
				current = arr
			} else {
				i, err := strconv.Atoi(indexStr)
				if err != nil || i < 0 || i >= len(arr) {
					return "", fmt.Errorf("invalid array index: %s", indexStr)
				}
				current = arr[i]
			}
		} else {
			switch v := current.(type) {
			case map[string]interface{}:
				current = v[part]
			case []interface{}:
				var results []interface{}
				for _, item := range v {
					if m, ok := item.(map[string]interface{}); ok {
						if val, exists := m[part]; exists {
							results = append(results, val)
						}
					}
				}
				current = results
			default:
				return "", fmt.Errorf("cannot access field %s on type %T", part, current)
			}
		}

		if current == nil {
			return "", fmt.Errorf("path not found")
		}
	}

	switch v := current.(type) {
	case string:
		return v, nil
	case []interface{}:
		b, err := json.Marshal(v)
		if err != nil {
			return "", err
		}
		return string(b), nil
	default:
		b, err := json.Marshal(v)
		if err != nil {
			return "", err
		}
		return string(b), nil
	}
}

func splitJSONPath(path string) []string {
	var parts []string
	var current strings.Builder
	inBracket := false

	for _, ch := range path {
		switch ch {
		case '.':
			if !inBracket {
				if current.Len() > 0 {
					parts = append(parts, current.String())
					current.Reset()
				}
			} else {
				current.WriteRune(ch)
			}
		case '[':
			inBracket = true
			current.WriteRune(ch)
		case ']':
			inBracket = false
			current.WriteRune(ch)
		default:
			current.WriteRune(ch)
		}
	}

	if current.Len() > 0 {
		parts = append(parts, current.String())
	}

	return parts
}
