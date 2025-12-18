package scheduler

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/kingoftac/gork/internal/db"
	"github.com/kingoftac/gork/internal/engine"
	"github.com/kingoftac/gork/internal/models"
)

type workflowSchedule struct {
	workflow  *models.Workflow
	timer     *time.Timer
	duration  time.Duration
	running   bool
	cancelRun context.CancelFunc
}

type Scheduler struct {
	db        *db.DB
	eng       *engine.Engine
	schedules map[int64]*workflowSchedule
	mu        sync.Mutex
	ctx       context.Context
	cancel    context.CancelFunc
	wg        sync.WaitGroup
}

func NewScheduler(db *db.DB) *Scheduler {
	return &Scheduler{
		db:        db,
		eng:       engine.NewEngine(db),
		schedules: make(map[int64]*workflowSchedule),
	}
}

func (s *Scheduler) Start(ctx context.Context) {
	slog.Info("Scheduler starting", "component", "scheduler")

	s.ctx, s.cancel = context.WithCancel(ctx)

	s.recoverRuns()
	s.loadWorkflows()

	// Poll for workflow changes (new workflows, updated schedules, deleted workflows)
	// This poll is just for config changes, not for triggering runs
	// TODO: See if this can be event-driven instead of using polling
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	slog.Info("Scheduler started", "component", "scheduler")

	for {
		select {
		case <-s.ctx.Done():
			slog.Info("Scheduler received shutdown signal", "component", "scheduler")
			s.shutdown()
			slog.Info("Scheduler shutdown complete", "component", "scheduler")
			return
		case <-ticker.C:
			s.loadWorkflows()
		}
	}
}

func (s *Scheduler) loadWorkflows() {
	workflows, err := s.db.ListWorkflows()
	if err != nil {
		slog.Error("Failed to list workflows", "component", "scheduler", "error", err)
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	seenIDs := make(map[int64]bool)

	for _, w := range workflows {
		seenIDs[w.ID] = true

		if w.Schedule == "" {
			if sched, exists := s.schedules[w.ID]; exists {
				slog.Info("Removing schedule for workflow (no longer scheduled)", "component", "scheduler", "workflow", w.Name)
				sched.timer.Stop()
				delete(s.schedules, w.ID)
			}
			continue
		}

		duration, err := time.ParseDuration(w.Schedule)
		if err != nil {
			slog.Warn("Invalid schedule duration", "component", "scheduler", "workflow", w.Name, "schedule", w.Schedule, "error", err)
			continue
		}

		if sched, exists := s.schedules[w.ID]; exists {
			if sched.duration == duration {
				sched.workflow = &w
				continue
			}
			slog.Info("Updating workflow schedule", "component", "scheduler", "workflow", w.Name, "old_duration", sched.duration, "new_duration", duration)
			sched.timer.Stop()
		}

		wCopy := w
		s.scheduleWorkflow(&wCopy, duration)
	}

	for id, sched := range s.schedules {
		if !seenIDs[id] {
			slog.Info("Removing deleted workflow from scheduler", "component", "scheduler", "workflow", sched.workflow.Name)
			sched.timer.Stop()
			if sched.cancelRun != nil {
				sched.cancelRun()
			}
			delete(s.schedules, id)
		}
	}
}

func (s *Scheduler) scheduleWorkflow(w *models.Workflow, duration time.Duration) {
	initialDelay := s.calculateInitialDelay(w, duration)

	slog.Info("Scheduling workflow", "component", "scheduler", "workflow", w.Name, "interval", duration, "initial_delay", initialDelay)

	sched := &workflowSchedule{
		workflow: w,
		duration: duration,
		running:  false,
	}

	sched.timer = time.AfterFunc(initialDelay, func() {
		s.runWorkflow(sched)
	})

	s.schedules[w.ID] = sched
}

func (s *Scheduler) calculateInitialDelay(w *models.Workflow, duration time.Duration) time.Duration {
	runs, err := s.db.ListRuns(&w.ID)
	if err != nil || len(runs) == 0 {
		return 0
	}

	lastRun := runs[0]
	if lastRun.CompletedAt.IsZero() {
		return duration
	}

	elapsed := time.Since(lastRun.CompletedAt)
	if elapsed >= duration {
		return 0
	}

	return duration - elapsed
}

func (s *Scheduler) runWorkflow(sched *workflowSchedule) {
	s.mu.Lock()

	if sched.running {
		slog.Debug("Workflow already running, skipping", "component", "scheduler", "workflow", sched.workflow.Name)
		sched.timer = time.AfterFunc(sched.duration, func() {
			s.runWorkflow(sched)
		})
		s.mu.Unlock()
		return
	}

	select {
	case <-s.ctx.Done():
		s.mu.Unlock()
		return
	default:
	}

	sched.running = true
	runCtx, cancel := context.WithCancel(s.ctx)
	sched.cancelRun = cancel
	s.mu.Unlock()

	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		defer func() {
			s.mu.Lock()
			sched.running = false
			sched.cancelRun = nil

			select {
			case <-s.ctx.Done():
			default:
				sched.timer = time.AfterFunc(sched.duration, func() {
					s.runWorkflow(sched)
				})
				slog.Debug("Rescheduled workflow", "component", "scheduler", "workflow", sched.workflow.Name, "next_run_in", sched.duration)
			}
			s.mu.Unlock()
		}()

		slog.Info("Starting scheduled workflow execution", "component", "scheduler", "workflow", sched.workflow.Name, "workflow_id", sched.workflow.ID)

		run, err := s.eng.ExecuteWorkflow(runCtx, sched.workflow, "scheduler")
		if err != nil {
			slog.Error("Failed to execute scheduled workflow", "component", "scheduler", "workflow", sched.workflow.Name, "workflow_id", sched.workflow.ID, "error", err)
		} else {
			slog.Info("Completed scheduled workflow execution", "component", "scheduler", "workflow", sched.workflow.Name, "workflow_id", sched.workflow.ID, "run_id", run.ID, "status", run.Status)
		}
	}()
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

func (s *Scheduler) shutdown() {
	s.mu.Lock()
	activeCount := len(s.schedules)
	slog.Info("Shutting down scheduler", "component", "scheduler", "scheduled_workflows", activeCount)

	for _, sched := range s.schedules {
		sched.timer.Stop()
		if sched.cancelRun != nil {
			slog.Info("Canceling active workflow run", "component", "scheduler", "workflow", sched.workflow.Name)
			sched.cancelRun()
		}
	}
	s.mu.Unlock()

	slog.Info("Waiting for active workflows to complete", "component", "scheduler")
	s.wg.Wait()
	slog.Info("All workflows completed", "component", "scheduler")
}
