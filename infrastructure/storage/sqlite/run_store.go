package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"time"

	"github.com/felixgeelhaar/agent-go/domain/agent"
	"github.com/felixgeelhaar/agent-go/domain/run"
)

// RunStore is a SQLite-backed implementation of run.Store.
type RunStore struct {
	db *sql.DB
}

// NewRunStore creates a new SQLite run store with the given configuration.
func NewRunStore(cfg Config, opts ...Option) (*RunStore, error) {
	// Apply options
	for _, opt := range opts {
		opt(&cfg)
	}

	db, err := openDB(cfg)
	if err != nil {
		return nil, err
	}

	s := &RunStore{db: db}

	// Auto-migrate if enabled
	if cfg.AutoMigrate {
		if err := s.migrate(); err != nil {
			_ = db.Close()
			return nil, err
		}
	}

	return s, nil
}

// NewRunStoreFromDB creates a run store from an existing database connection.
func NewRunStoreFromDB(db *sql.DB) (*RunStore, error) {
	s := &RunStore{db: db}

	if err := s.migrate(); err != nil {
		return nil, err
	}

	return s, nil
}

// migrate creates the runs table if it doesn't exist.
func (s *RunStore) migrate() error {
	schema := `
		CREATE TABLE IF NOT EXISTS runs (
			id TEXT PRIMARY KEY,
			goal TEXT NOT NULL,
			status TEXT NOT NULL,
			current_state TEXT NOT NULL,
			result TEXT,
			error TEXT,
			data BLOB NOT NULL,
			start_time INTEGER NOT NULL,
			end_time INTEGER,
			created_at INTEGER NOT NULL,
			updated_at INTEGER NOT NULL
		);
		CREATE INDEX IF NOT EXISTS idx_runs_status ON runs(status);
		CREATE INDEX IF NOT EXISTS idx_runs_start_time ON runs(start_time);
		CREATE INDEX IF NOT EXISTS idx_runs_current_state ON runs(current_state);
	`

	_, err := s.db.Exec(schema)
	if err != nil {
		return errors.Join(ErrMigrationFailed, err)
	}

	return nil
}

// Save persists a new run.
func (s *RunStore) Save(ctx context.Context, r *agent.Run) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	if r.ID == "" {
		return run.ErrInvalidRunID
	}

	data, err := json.Marshal(r)
	if err != nil {
		return err
	}

	now := time.Now().Unix()
	startTime := r.StartTime.Unix()

	var endTime sql.NullInt64
	if !r.EndTime.IsZero() {
		endTime = sql.NullInt64{Int64: r.EndTime.Unix(), Valid: true}
	}

	_, err = s.db.ExecContext(ctx,
		`INSERT INTO runs (id, goal, status, current_state, result, error, data, start_time, end_time, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		r.ID, r.Goal, string(r.Status), string(r.CurrentState), r.Result, r.Error,
		data, startTime, endTime, now, now,
	)

	if err != nil {
		// Check for duplicate key
		if isUniqueViolation(err) {
			return run.ErrRunExists
		}
		return err
	}

	return nil
}

// Get retrieves a run by ID.
func (s *RunStore) Get(ctx context.Context, id string) (*agent.Run, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	if id == "" {
		return nil, run.ErrInvalidRunID
	}

	var data []byte
	err := s.db.QueryRowContext(ctx,
		"SELECT data FROM runs WHERE id = ?",
		id,
	).Scan(&data)

	if errors.Is(err, sql.ErrNoRows) {
		return nil, run.ErrRunNotFound
	}
	if err != nil {
		return nil, err
	}

	var r agent.Run
	if err := json.Unmarshal(data, &r); err != nil {
		return nil, err
	}

	return &r, nil
}

// Update updates an existing run.
func (s *RunStore) Update(ctx context.Context, r *agent.Run) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	if r.ID == "" {
		return run.ErrInvalidRunID
	}

	data, err := json.Marshal(r)
	if err != nil {
		return err
	}

	now := time.Now().Unix()

	var endTime sql.NullInt64
	if !r.EndTime.IsZero() {
		endTime = sql.NullInt64{Int64: r.EndTime.Unix(), Valid: true}
	}

	result, err := s.db.ExecContext(ctx,
		`UPDATE runs SET
			goal = ?, status = ?, current_state = ?, result = ?, error = ?,
			data = ?, end_time = ?, updated_at = ?
		 WHERE id = ?`,
		r.Goal, string(r.Status), string(r.CurrentState), r.Result, r.Error,
		data, endTime, now, r.ID,
	)

	if err != nil {
		return err
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if rows == 0 {
		return run.ErrRunNotFound
	}

	return nil
}

// Delete removes a run by ID.
func (s *RunStore) Delete(ctx context.Context, id string) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	if id == "" {
		return run.ErrInvalidRunID
	}

	result, err := s.db.ExecContext(ctx, "DELETE FROM runs WHERE id = ?", id)
	if err != nil {
		return err
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if rows == 0 {
		return run.ErrRunNotFound
	}

	return nil
}

// List returns runs matching the filter.
func (s *RunStore) List(ctx context.Context, filter run.ListFilter) ([]*agent.Run, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	query, args := s.buildListQuery(filter, false)

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var runs []*agent.Run
	for rows.Next() {
		var data []byte
		if err := rows.Scan(&data); err != nil {
			return nil, err
		}

		var r agent.Run
		if err := json.Unmarshal(data, &r); err != nil {
			continue // Skip malformed entries
		}

		runs = append(runs, &r)
	}

	return runs, rows.Err()
}

// Count returns the number of runs matching the filter.
func (s *RunStore) Count(ctx context.Context, filter run.ListFilter) (int64, error) {
	if err := ctx.Err(); err != nil {
		return 0, err
	}

	query, args := s.buildListQuery(filter, true)

	var count int64
	err := s.db.QueryRowContext(ctx, query, args...).Scan(&count)
	return count, err
}

// Summary returns aggregate statistics.
func (s *RunStore) Summary(ctx context.Context, filter run.ListFilter) (run.Summary, error) {
	if err := ctx.Err(); err != nil {
		return run.Summary{}, err
	}

	// Build WHERE clause
	where, args := s.buildWhereClause(filter)

	query := `
		SELECT
			COUNT(*) as total,
			SUM(CASE WHEN status = 'completed' THEN 1 ELSE 0 END) as completed,
			SUM(CASE WHEN status = 'failed' THEN 1 ELSE 0 END) as failed,
			SUM(CASE WHEN status = 'running' THEN 1 ELSE 0 END) as running,
			COALESCE(AVG(CASE WHEN end_time IS NOT NULL THEN end_time - start_time ELSE NULL END), 0) as avg_duration
		FROM runs
	`

	if where != "" {
		query += " WHERE " + where
	}

	var summary run.Summary
	var avgDurationSec float64

	err := s.db.QueryRowContext(ctx, query, args...).Scan(
		&summary.TotalRuns,
		&summary.CompletedRuns,
		&summary.FailedRuns,
		&summary.RunningRuns,
		&avgDurationSec,
	)

	if err != nil {
		return run.Summary{}, err
	}

	summary.AverageDuration = time.Duration(avgDurationSec * float64(time.Second))

	return summary, nil
}

// buildListQuery builds the SQL query for listing runs.
func (s *RunStore) buildListQuery(filter run.ListFilter, countOnly bool) (string, []interface{}) {
	var query string
	if countOnly {
		query = "SELECT COUNT(*) FROM runs"
	} else {
		query = "SELECT data FROM runs"
	}

	where, args := s.buildWhereClause(filter)

	if where != "" {
		query += " WHERE " + where
	}

	if !countOnly {
		// Add ORDER BY
		orderBy := "start_time"
		switch filter.OrderBy {
		case run.OrderByEndTime:
			orderBy = "end_time"
		case run.OrderByID:
			orderBy = "id"
		case run.OrderByStatus:
			orderBy = "status"
		}

		query += " ORDER BY " + orderBy
		if filter.Descending {
			query += " DESC"
		}

		// Add LIMIT and OFFSET
		if filter.Limit > 0 {
			query += " LIMIT ?"
			args = append(args, filter.Limit)
		}

		if filter.Offset > 0 {
			query += " OFFSET ?"
			args = append(args, filter.Offset)
		}
	}

	return query, args
}

// buildWhereClause builds the WHERE clause for filtering.
func (s *RunStore) buildWhereClause(filter run.ListFilter) (string, []interface{}) {
	var conditions []string
	var args []interface{}

	// Filter by status
	if len(filter.Status) > 0 {
		placeholders := ""
		for i, status := range filter.Status {
			if i > 0 {
				placeholders += ", "
			}
			placeholders += "?"
			args = append(args, string(status))
		}
		conditions = append(conditions, "status IN ("+placeholders+")")
	}

	// Filter by state
	if len(filter.States) > 0 {
		placeholders := ""
		for i, state := range filter.States {
			if i > 0 {
				placeholders += ", "
			}
			placeholders += "?"
			args = append(args, string(state))
		}
		conditions = append(conditions, "current_state IN ("+placeholders+")")
	}

	// Filter by time range
	if !filter.FromTime.IsZero() {
		conditions = append(conditions, "start_time >= ?")
		args = append(args, filter.FromTime.Unix())
	}

	if !filter.ToTime.IsZero() {
		conditions = append(conditions, "start_time <= ?")
		args = append(args, filter.ToTime.Unix())
	}

	// Filter by goal pattern
	if filter.GoalPattern != "" {
		conditions = append(conditions, "goal LIKE ?")
		args = append(args, "%"+filter.GoalPattern+"%")
	}

	where := ""
	for i, cond := range conditions {
		if i > 0 {
			where += " AND "
		}
		where += cond
	}

	return where, args
}

// Close closes the database connection.
func (s *RunStore) Close() error {
	return s.db.Close()
}

// DB returns the underlying database connection.
func (s *RunStore) DB() *sql.DB {
	return s.db
}

// isUniqueViolation checks if the error is a unique constraint violation.
func isUniqueViolation(err error) bool {
	if err == nil {
		return false
	}
	return errors.Is(err, sql.ErrNoRows) ||
		// SQLite unique constraint error
		containsString(err.Error(), "UNIQUE constraint failed")
}

func containsString(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && (s[:len(substr)] == substr || containsString(s[1:], substr)))
}

// Ensure RunStore implements run.Store and run.SummaryProvider
var (
	_ run.Store           = (*RunStore)(nil)
	_ run.SummaryProvider = (*RunStore)(nil)
)
