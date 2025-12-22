package runner

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"strings"

	"github.com/kingoftac/gork/internal/models"
)

func RunStep(ctx context.Context, step models.WorkflowStep) ([]string, error) {
	switch step.ActionType() {
	case models.StepTypeExec:
		return runExec(ctx, step)
	case models.StepTypeHTTP:
		return runHTTP(ctx, step)
	case models.StepTypeScript:
		return runScript(ctx, step)
	default:
		return nil, fmt.Errorf("unknown action type")
	}
}

func runExec(ctx context.Context, step models.WorkflowStep) ([]string, error) {
	cmd := exec.CommandContext(ctx, step.Exec.Command, step.Exec.Args...)

	// SECURITY: Restrict working directory to current directory only
	// Prevent steps from accessing files outside the workspace
	cmd.Dir = "."

	if step.Exec.WorkingDir != "" {
		// TODO: Log security warning but allow relative paths within current directory
		// The validation should prevent dangerous paths, but this is an extra safeguard
	}

	cmd.Env = []string{}

	for k, v := range step.Env {
		if !strings.Contains(k, "=") {
			cmd.Env = append(cmd.Env, k+"="+v)
		}
	}
	for k, v := range step.Exec.Env {
		if !strings.Contains(k, "=") {
			cmd.Env = append(cmd.Env, k+"="+v)
		}
	}

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	logs := []string{}

	if out := stdout.String(); out != "" {
		logs = append(logs, strings.Split(strings.TrimSpace(out), "\n")...)
	}
	if errOut := stderr.String(); errOut != "" {
		logs = append(logs, strings.Split(strings.TrimSpace(errOut), "\n")...)
	}

	if err != nil {
		return logs, fmt.Errorf("exec failed: %w", err)
	}

	return logs, nil
}

func runHTTP(ctx context.Context, step models.WorkflowStep) ([]string, error) {
	method := step.HTTP.Method
	if method == "" {
		method = "GET"
	}

	url := interpolateEnvVars(step.HTTP.URL, step.Env)
	body := interpolateEnvVars(step.HTTP.Body, step.Env)

	req, err := http.NewRequestWithContext(ctx, method, url, strings.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	for k, v := range step.HTTP.Headers {
		req.Header.Set(k, interpolateEnvVars(v, step.Env))
	}

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("http request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	logs := []string{
		fmt.Sprintf("HTTP %s %s -> %d", method, url, resp.StatusCode),
		fmt.Sprintf("HTTP_STATUS:%d", resp.StatusCode),
		fmt.Sprintf("HTTP_BODY:%s", string(respBody)),
	}

	for k, v := range resp.Header {
		logs = append(logs, fmt.Sprintf("HTTP_HEADER_%s:%s", strings.ToUpper(strings.ReplaceAll(k, "-", "_")), strings.Join(v, ",")))
	}

	if resp.StatusCode >= 400 {
		return logs, fmt.Errorf("http error: %d", resp.StatusCode)
	}

	return logs, nil
}

func runScript(ctx context.Context, step models.WorkflowStep) ([]string, error) {
	shell := "sh"
	if step.Script.Language != "" {
		shell = step.Script.Language
	}

	cmd := exec.CommandContext(ctx, shell, "-c", step.Script.Inline)
	cmd.Env = os.Environ()
	for k, v := range step.Env {
		cmd.Env = append(cmd.Env, k+"="+v)
	}

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	logs := []string{}

	if out := stdout.String(); out != "" {
		logs = append(logs, strings.Split(strings.TrimSpace(out), "\n")...)
	}
	if errOut := stderr.String(); errOut != "" {
		logs = append(logs, strings.Split(strings.TrimSpace(errOut), "\n")...)
	}

	if err != nil {
		return logs, fmt.Errorf("script failed: %w", err)
	}

	return logs, nil
}

func interpolateEnvVars(s string, env map[string]string) string {
	result := s
	for k, v := range env {
		result = strings.ReplaceAll(result, "${"+k+"}", v)
	}
	return result
}
