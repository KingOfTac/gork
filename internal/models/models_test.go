package models

import (
	"strings"
	"testing"
	"time"
)

func TestValidateWorkflowSuccess(t *testing.T) {
	w := Workflow{
		Name: "example",
		Steps: []WorkflowStep{
			{
				Name: "fetch",
				Exec: &ExecAction{Command: "echo", Args: []string{"hello"}},
			},
			{
				Name:      "notify",
				DependsOn: []string{"fetch"},
				HTTP: &HTTPAction{
					Method: "POST",
					URL:    "https://example.com/hooks",
				},
				RetryDelay: time.Second,
				Retries:    1,
			},
		},
	}

	if err := w.Validate(); err != nil {
		t.Fatalf("expected workflow to validate, got error: %v", err)
	}
}

func TestValidateWorkflowDuplicateStep(t *testing.T) {
	w := Workflow{
		Name: "dup",
		Steps: []WorkflowStep{
			{Name: "step", Exec: &ExecAction{Command: "echo", Args: []string{"first"}}},
			{Name: "step", Exec: &ExecAction{Command: "echo", Args: []string{"second"}}},
		},
	}

	err := w.Validate()
	if err == nil || !strings.Contains(err.Error(), "duplicate step name") {
		t.Fatalf("expected duplicate step name error, got: %v", err)
	}
}

func TestValidateWorkflowMissingAction(t *testing.T) {
	w := Workflow{
		Name: "missing-action",
		Steps: []WorkflowStep{
			{Name: "step-without-action"},
		},
	}

	err := w.Validate()
	if err == nil || !strings.Contains(err.Error(), "must define exactly one action") {
		t.Fatalf("expected missing action error, got: %v", err)
	}
}

func TestValidateWorkflowMultipleActions(t *testing.T) {
	w := Workflow{
		Name: "multi-action",
		Steps: []WorkflowStep{
			{
				Name: "step",
				Exec: &ExecAction{Command: "echo", Args: []string{"first"}},
				HTTP: &HTTPAction{URL: "https://example.com"},
			},
		},
	}

	err := w.Validate()
	if err == nil || !strings.Contains(err.Error(), "only one action") {
		t.Fatalf("expected single action enforcement error, got: %v", err)
	}
}

func TestValidateWorkflowUnknownDependency(t *testing.T) {
	w := Workflow{
		Name: "unknown-dep",
		Steps: []WorkflowStep{
			{
				Name:      "step",
				DependsOn: []string{"missing"},
				Exec:      &ExecAction{Command: "echo", Args: []string{"hi"}},
			},
		},
	}

	err := w.Validate()
	if err == nil || !strings.Contains(err.Error(), "depends on unknown step") {
		t.Fatalf("expected unknown dependency error, got: %v", err)
	}
}

func TestValidateWorkflowCycle(t *testing.T) {
	w := Workflow{
		Name: "cycle",
		Steps: []WorkflowStep{
			{
				Name:      "first",
				DependsOn: []string{"second"},
				Exec:      &ExecAction{Command: "echo", Args: []string{"first"}},
			},
			{
				Name:      "second",
				DependsOn: []string{"first"},
				HTTP:      &HTTPAction{URL: "https://example.com"},
			},
		},
	}

	err := w.Validate()
	if err == nil || !strings.Contains(err.Error(), "cycle") {
		t.Fatalf("expected cycle detection error, got: %v", err)
	}
}

func TestExecActionValidation(t *testing.T) {
	step := WorkflowStep{
		Name: "invalid-exec",
		Exec: &ExecAction{},
	}

	err := step.Validate()
	if err == nil || !strings.Contains(err.Error(), "exec command is required") {
		t.Fatalf("expected exec validation error, got: %v", err)
	}
}
