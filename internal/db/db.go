package db

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/kingoftac/gork/internal/models"
	_ "modernc.org/sqlite"
)

type DB struct {
	*sql.DB
}

func NewDB(dbPath string) (*DB, error) {
	if err := os.MkdirAll(filepath.Dir(dbPath), 0755); err != nil {
		return nil, fmt.Errorf("failed to create database directory: %w", err)
	}

	sqlDB, err := sql.Open("sqlite", dbPath+"?_pragma=journal_mode(WAL)&_pragma=synchronous(NORMAL)&_pragma=cache_size(1000)&_pragma=foreign_keys(on)&_pragma=busy_timeout(5000)")
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	if err := sqlDB.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	db := &DB{sqlDB}

	if err := db.migrate(); err != nil {
		return nil, fmt.Errorf("failed to migrate database: %w", err)
	}

	return db, nil
}

func retryDBOperation(operation func() error) error {
	maxRetries := 5
	baseDelay := 50 * time.Millisecond

	for i := 0; i < maxRetries; i++ {
		err := operation()
		if err == nil {
			return nil
		}

		if strings.Contains(err.Error(), "database os locked") || strings.Contains(err.Error(), "SQLITE_BUSY") {
			if i < maxRetries-1 {
				delay := time.Duration(1<<uint(i)) * baseDelay
				time.Sleep(delay)
				continue
			}
		}

		return err
	}

	return fmt.Errorf("max retries exceeded for database operation")
}

func (db *DB) migrate() error {
	queries := []string{
		`CREATE TABLE IF NOT EXISTS workflows (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			name TEXT NOT NULL UNIQUE,
			description TEXT,
			schedule TEXT,
			steps TEXT NOT NULL,
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS runs (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			workflow_id INTEGER NOT NULL,
			status TEXT NOT NULL,
			started_at DATETIME,
			completed_at DATETIME,
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			trigger TEXT,
			FOREIGN KEY (workflow_id) REFERENCES workflows(id)
		)`,
		`CREATE TABLE IF NOT EXISTS step_runs (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			run_id INTEGER NOT NULL,
			step_name TEXT NOT NULL,
			status TEXT NOT NULL,
			attempt INTEGER NOT NULL DEFAULT 0,
			started_at DATETIME,
			completed_at DATETIME,
			error TEXT,
			logs TEXT,
			FOREIGN KEY (run_id) REFERENCES runs(id)
		)`,
		`CREATE TABLE IF NOT EXISTS step_data (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			run_id INTEGER NOT NULL,
			step_name TEXT NOT NULL,
			key TEXT NOT NULL,
			value TEXT NOT NULL,
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (run_id) REFERENCES runs(id),
			UNIQUE(run_id, step_name, key)
		)`,
	}

	for _, query := range queries {
		if _, err := db.Exec(query); err != nil {
			return fmt.Errorf("failed to execute migration query: %w", err)
		}
	}

	return nil
}

func (db *DB) InsertWorkflow(w *models.Workflow) error {
	stepsJSON, err := json.Marshal(w.Steps)
	if err != nil {
		return fmt.Errorf("failed to marshal steps: %w", err)
	}

	query := `INSERT OR REPLACE INTO workflows (id, name, description, schedule, steps, created_at, updated_at) VALUES ((SELECT id FROM workflows WHERE name = ?), ?, ?, ?, ?, (SELECT created_at FROM workflows WHERE name = ?), ?)`
	now := time.Now()
	_, err = db.Exec(query, w.Name, w.Name, w.Description, w.Schedule, string(stepsJSON), w.Name, now)
	if err != nil {
		return fmt.Errorf("failed to insert workflow: %w", err)
	}
	return nil
}

func (db *DB) GetWorkflow(id int64) (*models.Workflow, error) {
	query := `SELECT id, name, description, schedule, steps, created_at, updated_at FROM workflows WHERE id = ?`
	row := db.QueryRow(query, id)

	var w models.Workflow
	var stepsJSON string
	err := row.Scan(&w.ID, &w.Name, &w.Description, &w.Schedule, &stepsJSON, &w.CreatedAt, &w.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("failed to get workflow: %w", err)
	}

	if err := json.Unmarshal([]byte(stepsJSON), &w.Steps); err != nil {
		return nil, fmt.Errorf("failed to unmarshal steps: %w", err)
	}

	return &w, nil
}

func (db *DB) GetWorkflowByName(name string) (*models.Workflow, error) {
	query := `SELECT id, name, description, schedule, steps, created_at, updated_at FROM workflows WHERE name = ?`
	row := db.QueryRow(query, name)

	var w models.Workflow
	var stepsJSON string
	err := row.Scan(&w.ID, &w.Name, &w.Description, &w.Schedule, &stepsJSON, &w.CreatedAt, &w.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("failed to get workflow: %w", err)
	}

	if err := json.Unmarshal([]byte(stepsJSON), &w.Steps); err != nil {
		return nil, fmt.Errorf("failed to unmarshal steps: %w", err)
	}

	return &w, nil
}

func (db *DB) ListWorkflows() ([]models.Workflow, error) {
	query := `SELECT id, name, description, schedule, steps, created_at, updated_at FROM workflows ORDER BY name`
	rows, err := db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("failed to list workflows: %w", err)
	}
	defer rows.Close()

	var workflows []models.Workflow
	for rows.Next() {
		var w models.Workflow
		var stepsJSON string
		err := rows.Scan(&w.ID, &w.Name, &w.Description, &w.Schedule, &stepsJSON, &w.CreatedAt, &w.UpdatedAt)
		if err != nil {
			return nil, fmt.Errorf("failed to scan workflow: %w", err)
		}

		if err := json.Unmarshal([]byte(stepsJSON), &w.Steps); err != nil {
			return nil, fmt.Errorf("failed to unmarshal steps: %w", err)
		}

		workflows = append(workflows, w)
	}

	return workflows, nil
}

func (db *DB) DeleteWorkflow(id int64) error {
	stepDataQuery := `DELETE FROM step_data WHERE run_id IN (SELECT id FROM runs WHERE workflow_id = ?)`
	if err := retryDBOperation(func() error {
		_, err := db.Exec(stepDataQuery, id)
		return err
	}); err != nil {
		return fmt.Errorf("failed to delete step data: %w", err)
	}

	stepRunsQuery := `DELETE FROM step_runs WHERE run_id IN (SELECT id FROM runs WHERE workflow_id = ?)`
	if err := retryDBOperation(func() error {
		_, err := db.Exec(stepRunsQuery, id)
		return err
	}); err != nil {
		return fmt.Errorf("failed to delete step runs: %w", err)
	}

	runsQuery := `DELETE FROM runs WHERE workflow_id = ?`
	if err := retryDBOperation(func() error {
		_, err := db.Exec(runsQuery, id)
		return err
	}); err != nil {
		return fmt.Errorf("failed to delete runs: %w", err)
	}

	workflowQuery := `DELETE FROM workflows WHERE id = ?`
	if err := retryDBOperation(func() error {
		_, err := db.Exec(workflowQuery, id)
		return err
	}); err != nil {
		return fmt.Errorf("failed to delete workflow: %w", err)
	}

	return nil
}

func (db *DB) ResetAllData() error {

	stepDataQuery := `DELETE FROM step_data`
	if err := retryDBOperation(func() error {
		_, err := db.Exec(stepDataQuery)
		return err
	}); err != nil {
		return fmt.Errorf("failed to delete step data: %w", err)
	}

	stepRunsQuery := `DELETE FROM step_runs`
	if err := retryDBOperation(func() error {
		_, err := db.Exec(stepRunsQuery)
		return err
	}); err != nil {
		return fmt.Errorf("failed to delete step runs: %w", err)
	}

	runsQuery := `DELETE FROM runs`
	if err := retryDBOperation(func() error {
		_, err := db.Exec(runsQuery)
		return err
	}); err != nil {
		return fmt.Errorf("failed to delete runs: %w", err)
	}

	workflowsQuery := `DELETE FROM workflows`
	if err := retryDBOperation(func() error {
		_, err := db.Exec(workflowsQuery)
		return err
	}); err != nil {
		return fmt.Errorf("failed to delete workflows: %w", err)
	}

	return nil
}

func (db *DB) InsertRun(r *models.Run) (int64, error) {
	query := `INSERT INTO runs (workflow_id, status, started_at, completed_at, created_at, updated_at, trigger) VALUES (?, ?, ?, ?, ?, ?, ?)`
	now := time.Now()
	var result sql.Result
	err := retryDBOperation(func() error {
		var err error
		result, err = db.Exec(query, r.WorkflowID, r.Status, r.StartedAt, r.CompletedAt, now, now, r.Trigger)
		return err
	})
	if err != nil {
		return 0, fmt.Errorf("failed to insert run: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("failed to get last insert id: %w", err)
	}

	return id, nil
}

func (db *DB) UpdateRunStatus(id int64, status models.RunStatus, completedAt *time.Time) error {
	query := `UPDATE runs SET status = ?, completed_at = ?, updated_at = ? WHERE id = ?`
	now := time.Now()
	err := retryDBOperation(func() error {
		_, err := db.Exec(query, status, completedAt, now, id)
		return err
	})
	if err != nil {
		return fmt.Errorf("failed to update run status: %w", err)
	}
	return nil
}

func (db *DB) GetRun(id int64) (*models.Run, error) {
	query := `SELECT id, workflow_id, status, started_at, completed_at, created_at, updated_at, trigger FROM runs WHERE id = ?`
	row := db.QueryRow(query, id)

	var r models.Run
	var completedAt sql.NullTime
	err := row.Scan(&r.ID, &r.WorkflowID, &r.Status, &r.StartedAt, &completedAt, &r.CreatedAt, &r.UpdatedAt, &r.Trigger)
	if err != nil {
		return nil, fmt.Errorf("failed to get run: %w", err)
	}
	if completedAt.Valid {
		r.CompletedAt = completedAt.Time
	}

	return &r, nil
}

func (db *DB) ListRuns(workflowID *int64) ([]models.Run, error) {
	var query string
	var args []interface{}
	if workflowID != nil {
		query = `SELECT id, workflow_id, status, started_at, completed_at, created_at, updated_at, trigger FROM runs WHERE workflow_id = ? ORDER BY created_at DESC`
		args = []interface{}{*workflowID}
	} else {
		query = `SELECT id, workflow_id, status, started_at, completed_at, created_at, updated_at, trigger FROM runs ORDER BY created_at DESC`
		args = []interface{}{}
	}

	rows, err := db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to list runs: %w", err)
	}
	defer rows.Close()

	var runs []models.Run
	for rows.Next() {
		var r models.Run
		var completedAt sql.NullTime
		err := rows.Scan(&r.ID, &r.WorkflowID, &r.Status, &r.StartedAt, &completedAt, &r.CreatedAt, &r.UpdatedAt, &r.Trigger)
		if err != nil {
			return nil, fmt.Errorf("failed to scan run: %w", err)
		}
		if completedAt.Valid {
			r.CompletedAt = completedAt.Time
		}
		runs = append(runs, r)
	}

	return runs, nil
}

func (db *DB) InsertStepRun(sr *models.StepRun) (int64, error) {
	logsJSON, err := json.Marshal(sr.Logs)
	if err != nil {
		return 0, fmt.Errorf("failed to marshal logs: %w", err)
	}

	query := `INSERT INTO step_runs (run_id, step_name, status, attempt, started_at, completed_at, error, logs) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`
	var result sql.Result
	err = retryDBOperation(func() error {
		var err error
		result, err = db.Exec(query, sr.RunID, sr.StepName, sr.Status, sr.Attempt, sr.StartedAt, sr.CompletedAt, sr.Error, string(logsJSON))
		return err
	})
	if err != nil {
		return 0, fmt.Errorf("failed to insert step run: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("failed to get last insert id: %w", err)
	}

	return id, nil
}

func (db *DB) UpdateStepRun(id int64, status models.StepStatus, completedAt *time.Time, errorMsg string, logs []string) error {
	logsJSON, err := json.Marshal(logs)
	if err != nil {
		return fmt.Errorf("failed to marshal logs: %w", err)
	}

	query := `UPDATE step_runs SET status = ?, completed_at = ?, error = ?, logs = ? WHERE id = ?`
	err = retryDBOperation(func() error {
		_, err := db.Exec(query, status, completedAt, errorMsg, string(logsJSON), id)
		return err
	})
	if err != nil {
		return fmt.Errorf("failed to update step run: %w", err)
	}
	return nil
}

func (db *DB) GetStepRuns(runID int64) ([]models.StepRun, error) {
	query := `SELECT id, run_id, step_name, status, attempt, started_at, completed_at, error, logs FROM step_runs WHERE run_id = ? ORDER BY started_at`
	rows, err := db.Query(query, runID)
	if err != nil {
		return nil, fmt.Errorf("failed to get step runs: %w", err)
	}
	defer rows.Close()

	var stepRuns []models.StepRun
	for rows.Next() {
		var sr models.StepRun
		var logsJSON string
		var startedAt, completedAt sql.NullTime
		err := rows.Scan(&sr.ID, &sr.RunID, &sr.StepName, &sr.Status, &sr.Attempt, &startedAt, &completedAt, &sr.Error, &logsJSON)
		if err != nil {
			return nil, fmt.Errorf("failed to scan step run: %w", err)
		}

		if startedAt.Valid {
			sr.StartedAt = startedAt.Time
		}
		if completedAt.Valid {
			sr.CompletedAt = completedAt.Time
		}

		if err := json.Unmarshal([]byte(logsJSON), &sr.Logs); err != nil {
			return nil, fmt.Errorf("failed to unmarshal logs: %w", err)
		}

		stepRuns = append(stepRuns, sr)
	}

	return stepRuns, nil
}

func (db *DB) AppendLogs(stepRunID int64, logs []string) error {
	return retryDBOperation(func() error {

		query := `SELECT logs FROM step_runs WHERE id = ?`
		row := db.QueryRow(query, stepRunID)

		var logsJSON string
		err := row.Scan(&logsJSON)
		if err != nil {
			return fmt.Errorf("failed to get current logs: %w", err)
		}

		var currentLogs []string
		if err := json.Unmarshal([]byte(logsJSON), &currentLogs); err != nil {
			return fmt.Errorf("failed to unmarshal current logs: %w", err)
		}

		currentLogs = append(currentLogs, logs...)

		newLogsJSON, err := json.Marshal(currentLogs)
		if err != nil {
			return fmt.Errorf("failed to marshal new logs: %w", err)
		}

		updateQuery := `UPDATE step_runs SET logs = ? WHERE id = ?`
		_, err = db.Exec(updateQuery, string(newLogsJSON), stepRunID)
		if err != nil {
			return fmt.Errorf("failed to update logs: %w", err)
		}

		return nil
	})
}

func (db *DB) StoreStepData(runID int64, stepName, key, value string) error {
	if runID <= 0 {
		return fmt.Errorf("invalid run ID")
	}
	if strings.TrimSpace(stepName) == "" {
		return fmt.Errorf("step name cannot be empty")
	}
	if strings.TrimSpace(key) == "" {
		return fmt.Errorf("key cannot be empty")
	}
	if len(value) > 10000 {
		return fmt.Errorf("value too large (max 10000 characters)")
	}

	query := `INSERT OR REPLACE INTO step_data (run_id, step_name, key, value) VALUES (?, ?, ?, ?)`
	_, err := db.Exec(query, runID, stepName, key, value)
	if err != nil {
		return fmt.Errorf("failed to store step data: %w", err)
	}
	return nil
}

func (db *DB) GetStepData(runID int64, stepName, key string) (string, error) {
	if runID <= 0 {
		return "", fmt.Errorf("invalid run ID")
	}
	if strings.TrimSpace(stepName) == "" {
		return "", fmt.Errorf("step name cannot be empty")
	}
	if strings.TrimSpace(key) == "" {
		return "", fmt.Errorf("key cannot be empty")
	}

	query := `SELECT value FROM step_data WHERE run_id = ? AND step_name = ? AND key = ?`
	row := db.QueryRow(query, runID, stepName, key)

	var value string
	err := row.Scan(&value)
	if err != nil {
		return "", fmt.Errorf("failed to get step data: %w", err)
	}
	return value, nil
}

func (db *DB) GetAllStepData(runID int64) (map[string]map[string]string, error) {
	query := `SELECT step_name, key, value FROM step_data WHERE run_id = ?`
	rows, err := db.Query(query, runID)
	if err != nil {
		return nil, fmt.Errorf("failed to query step data: %w", err)
	}
	defer rows.Close()

	data := make(map[string]map[string]string)
	for rows.Next() {
		var stepName, key, value string
		if err := rows.Scan(&stepName, &key, &value); err != nil {
			return nil, fmt.Errorf("failed to scan step data: %w", err)
		}
		if data[stepName] == nil {
			data[stepName] = make(map[string]string)
		}
		data[stepName][key] = value
	}
	return data, nil
}
