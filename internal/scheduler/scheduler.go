package scheduler

import (
	"context"
	"log/slog"
	"time"

	"github.com/kingoftac/gork/internal/db"
	"github.com/kingoftac/gork/internal/engine"
	"github.com/kingoftac/gork/internal/models"
)

type Scheduler struct {
	db         *db.DB
	eng        *engine.Engine
	activeRuns map[int64]context.CancelFunc
}

func NewScheduler(db *db.DB) *Scheduler {
	return &Scheduler{
		db:         db,
		eng:        engine.NewEngine(db),
		activeRuns: make(map[int64]context.CancelFunc),
	}
}

func (s *Scheduler) Start(ctx context.Context) {
	slog.Info("Scheduler starting", "component", "scheduler")

	// Recover incomplete runs
	s.recoverRuns()

	// Poll loop
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	pollCount := 0
	slog.Info("Scheduler poll loop started", "component", "scheduler", "interval", "30s")

	for {
		select {
		case <-ctx.Done():
			slog.Info("Scheduler received shutdown signal", "component", "scheduler")
			s.shutdown()
			slog.Info("Scheduler shutdown complete", "component", "scheduler")
			return
		case <-ticker.C:
			pollCount++
			slog.Debug("Scheduler poll cycle starting", "component", "scheduler", "cycle", pollCount)
			s.pollAndRun()
			slog.Debug("Scheduler poll cycle completed", "component", "scheduler", "cycle", pollCount)
		}
	}
}

func (s *Scheduler) recoverRuns() {
	slog.Info("Starting recovery of incomplete runs", "component", "scheduler")

	runs, err := s.db.ListRuns(nil)
	if err != nil {
		slog.Error("Failed to list runs during recovery", "component", "scheduler", "error", err)
		return
	}

	recoveredCount := 0
	for _, r := range runs {
		if r.Status == models.RunStatusRunning || r.Status == models.RunStatusPending {
			// Mark as canceled
			now := time.Now()
			if err := s.db.UpdateRunStatus(r.ID, models.RunStatusCanceled, &now); err != nil {
				slog.Error("Failed to cancel incomplete run", "component", "scheduler", "run_id", r.ID, "error", err)
			} else {
				recoveredCount++
				slog.Info("Recovered incomplete run", "component", "scheduler", "run_id", r.ID, "status", r.Status)
			}
		}
	}

	slog.Info("Recovery complete", "component", "scheduler", "runs_recovered", recoveredCount)
}

func (s *Scheduler) pollAndRun() {
	slog.Info("Starting poll cycle", "component", "scheduler")

	workflows, err := s.db.ListWorkflows()
	if err != nil {
		slog.Error("Failed to list workflows during poll", "component", "scheduler", "error", err)
		return
	}

	scheduledWorkflows := 0
	executedWorkflows := 0
	now := time.Now()

	for _, w := range workflows {
		if w.Schedule == "" {
			continue
		}
		scheduledWorkflows++

		duration, err := time.ParseDuration(w.Schedule)
		if err != nil {
			slog.Warn("Skipping workflow with invalid schedule", "component", "scheduler", "workflow", w.Name, "schedule", w.Schedule, "error", err)
			continue
		}

		// Get last run
		runs, err := s.db.ListRuns(&w.ID)
		if err != nil {
			slog.Error("Failed to list runs for workflow", "component", "scheduler", "workflow", w.Name, "error", err)
			continue
		}

		shouldRun := len(runs) == 0
		if len(runs) > 0 {
			lastRun := runs[0] // since ordered by created_at desc
			if lastRun.CompletedAt.IsZero() {
				// Still running, skip
				slog.Debug("Workflow still running, skipping", "component", "scheduler", "workflow", w.Name, "last_run_id", lastRun.ID)
				continue
			}
			shouldRun = lastRun.CompletedAt.Add(duration).Before(now)
			slog.Debug("Checking workflow schedule", "component", "scheduler", "workflow", w.Name, "last_completed", lastRun.CompletedAt, "next_run", lastRun.CompletedAt.Add(duration), "should_run", shouldRun)
		}

		if shouldRun {
			// Check if already active
			if _, active := s.activeRuns[w.ID]; active {
				slog.Debug("Workflow already active, skipping", "component", "scheduler", "workflow", w.Name)
				continue
			}

			// Run
			slog.Info("Starting scheduled workflow execution", "component", "scheduler", "workflow", w.Name, "workflow_id", w.ID)
			runCtx, cancel := context.WithCancel(context.Background())
			s.activeRuns[w.ID] = cancel

			go func(w models.Workflow) {
				defer delete(s.activeRuns, w.ID)
				run, err := s.eng.ExecuteWorkflow(runCtx, &w, "scheduler")
				if err != nil {
					slog.Error("Failed to execute scheduled workflow", "component", "scheduler", "workflow", w.Name, "workflow_id", w.ID, "error", err)
				} else {
					slog.Info("Completed scheduled workflow execution", "component", "scheduler", "workflow", w.Name, "workflow_id", w.ID, "run_id", run.ID, "status", run.Status)
				}
			}(w)
			executedWorkflows++
		}
	}

	slog.Info("Poll cycle summary", "component", "scheduler", "total_workflows", len(workflows), "scheduled_workflows", scheduledWorkflows, "executed_workflows", executedWorkflows)
}

func (s *Scheduler) shutdown() {
	activeCount := len(s.activeRuns)
	slog.Info("Shutting down scheduler", "component", "scheduler", "active_runs", activeCount)

	for workflowID, cancel := range s.activeRuns {
		slog.Info("Canceling active workflow run", "component", "scheduler", "workflow_id", workflowID)
		cancel()
	}
	// Wait for goroutines to finish? But since async, just cancel.
	slog.Info("Scheduler shutdown initiated", "component", "scheduler", "active_runs_canceled", activeCount)
}
