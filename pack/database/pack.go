// Package database provides database operation tools.
package database

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/felixgeelhaar/agent-go/domain/agent"
	"github.com/felixgeelhaar/agent-go/domain/pack"
	"github.com/felixgeelhaar/agent-go/domain/tool"
)

// Config configures the database pack.
type Config struct {
	// DB is the database connection pool (required).
	DB *sql.DB

	// QueryTimeout is the timeout for queries (default: 30s).
	QueryTimeout time.Duration

	// MaxRows limits the number of rows returned (default: 1000).
	MaxRows int

	// AllowWrite enables write operations (default: false).
	AllowWrite bool

	// AllowDDL enables DDL operations (default: false).
	AllowDDL bool
}

// Option configures the database pack.
type Option func(*Config)

// WithQueryTimeout sets the query timeout.
func WithQueryTimeout(timeout time.Duration) Option {
	return func(c *Config) {
		c.QueryTimeout = timeout
	}
}

// WithMaxRows sets the maximum rows returned.
func WithMaxRows(max int) Option {
	return func(c *Config) {
		c.MaxRows = max
	}
}

// WithWriteAccess enables write operations.
func WithWriteAccess() Option {
	return func(c *Config) {
		c.AllowWrite = true
	}
}

// WithDDLAccess enables DDL operations (CREATE, ALTER, DROP).
func WithDDLAccess() Option {
	return func(c *Config) {
		c.AllowDDL = true
	}
}

// New creates the database pack.
func New(db *sql.DB, opts ...Option) (*pack.Pack, error) {
	if db == nil {
		return nil, errors.New("database connection is required")
	}

	cfg := Config{
		DB:           db,
		QueryTimeout: 30 * time.Second,
		MaxRows:      1000,
		AllowWrite:   false,
		AllowDDL:     false,
	}

	for _, opt := range opts {
		opt(&cfg)
	}

	builder := pack.NewBuilder("database").
		WithDescription("Database operations").
		WithVersion("1.0.0").
		AddTools(
			queryTool(&cfg),
			tablesTool(&cfg),
			schemaTool(&cfg),
		).
		AllowInState(agent.StateExplore, "db_query", "db_tables", "db_schema").
		AllowInState(agent.StateValidate, "db_query", "db_tables", "db_schema")

	// Add write tools if enabled
	if cfg.AllowWrite {
		builder = builder.AddTools(executeTool(&cfg))
		builder = builder.AllowInState(agent.StateAct, "db_query", "db_execute", "db_tables", "db_schema")
	} else {
		builder = builder.AllowInState(agent.StateAct, "db_query", "db_tables", "db_schema")
	}

	return builder.Build(), nil
}

// queryInput is the input for the db_query tool.
type queryInput struct {
	Query  string        `json:"query"`
	Args   []interface{} `json:"args,omitempty"`
	Limit  int           `json:"limit,omitempty"`
	Offset int           `json:"offset,omitempty"`
}

// queryOutput is the output for the db_query tool.
type queryOutput struct {
	Columns  []string                 `json:"columns"`
	Rows     []map[string]interface{} `json:"rows"`
	RowCount int                      `json:"row_count"`
	Truncated bool                    `json:"truncated,omitempty"`
}

func queryTool(cfg *Config) tool.Tool {
	return tool.NewBuilder("db_query").
		WithDescription("Execute a SELECT query and return results").
		ReadOnly().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var in queryInput
			if err := json.Unmarshal(input, &in); err != nil {
				return tool.Result{}, err
			}

			// Validate query is SELECT
			query := strings.TrimSpace(strings.ToUpper(in.Query))
			if !strings.HasPrefix(query, "SELECT") {
				return tool.Result{}, errors.New("db_query only supports SELECT statements; use db_execute for modifications")
			}

			// Apply timeout
			ctx, cancel := context.WithTimeout(ctx, cfg.QueryTimeout)
			defer cancel()

			// Execute query
			rows, err := cfg.DB.QueryContext(ctx, in.Query, in.Args...)
			if err != nil {
				return tool.Result{}, fmt.Errorf("query failed: %w", err)
			}
			defer rows.Close()

			// Get column names
			columns, err := rows.Columns()
			if err != nil {
				return tool.Result{}, fmt.Errorf("failed to get columns: %w", err)
			}

			// Determine limit
			limit := cfg.MaxRows
			if in.Limit > 0 && in.Limit < limit {
				limit = in.Limit
			}

			// Scan rows
			out := queryOutput{
				Columns: columns,
				Rows:    make([]map[string]interface{}, 0),
			}

			count := 0
			for rows.Next() {
				count++
				if count > limit {
					out.Truncated = true
					break
				}

				// Create a slice of interface{} to scan into
				values := make([]interface{}, len(columns))
				valuePtrs := make([]interface{}, len(columns))
				for i := range values {
					valuePtrs[i] = &values[i]
				}

				if err := rows.Scan(valuePtrs...); err != nil {
					return tool.Result{}, fmt.Errorf("scan failed: %w", err)
				}

				// Convert to map
				row := make(map[string]interface{})
				for i, col := range columns {
					val := values[i]
					// Convert []byte to string for JSON serialization
					if b, ok := val.([]byte); ok {
						val = string(b)
					}
					row[col] = val
				}
				out.Rows = append(out.Rows, row)
			}

			if err := rows.Err(); err != nil {
				return tool.Result{}, fmt.Errorf("row iteration failed: %w", err)
			}

			out.RowCount = len(out.Rows)

			data, _ := json.Marshal(out)
			return tool.Result{Output: data}, nil
		}).
		MustBuild()
}

// executeInput is the input for the db_execute tool.
type executeInput struct {
	Query string        `json:"query"`
	Args  []interface{} `json:"args,omitempty"`
}

// executeOutput is the output for the db_execute tool.
type executeOutput struct {
	RowsAffected int64 `json:"rows_affected"`
	LastInsertID int64 `json:"last_insert_id,omitempty"`
}

func executeTool(cfg *Config) tool.Tool {
	return tool.NewBuilder("db_execute").
		WithDescription("Execute an INSERT, UPDATE, or DELETE statement").
		Destructive().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var in executeInput
			if err := json.Unmarshal(input, &in); err != nil {
				return tool.Result{}, err
			}

			// Validate query is not SELECT
			query := strings.TrimSpace(strings.ToUpper(in.Query))
			if strings.HasPrefix(query, "SELECT") {
				return tool.Result{}, errors.New("db_execute does not support SELECT; use db_query")
			}

			// Check for DDL
			if !cfg.AllowDDL {
				for _, ddl := range []string{"CREATE", "ALTER", "DROP", "TRUNCATE"} {
					if strings.HasPrefix(query, ddl) {
						return tool.Result{}, fmt.Errorf("DDL operations (%s) are not allowed", ddl)
					}
				}
			}

			// Apply timeout
			ctx, cancel := context.WithTimeout(ctx, cfg.QueryTimeout)
			defer cancel()

			// Execute query
			result, err := cfg.DB.ExecContext(ctx, in.Query, in.Args...)
			if err != nil {
				return tool.Result{}, fmt.Errorf("execution failed: %w", err)
			}

			out := executeOutput{}
			if affected, err := result.RowsAffected(); err == nil {
				out.RowsAffected = affected
			}
			if lastID, err := result.LastInsertId(); err == nil {
				out.LastInsertID = lastID
			}

			data, _ := json.Marshal(out)
			return tool.Result{Output: data}, nil
		}).
		MustBuild()
}

// tablesOutput is the output for the db_tables tool.
type tablesOutput struct {
	Tables []tableInfo `json:"tables"`
	Count  int         `json:"count"`
}

type tableInfo struct {
	Name   string `json:"name"`
	Schema string `json:"schema,omitempty"`
	Type   string `json:"type,omitempty"`
}

func tablesTool(cfg *Config) tool.Tool {
	return tool.NewBuilder("db_tables").
		WithDescription("List all tables in the database").
		ReadOnly().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			// Apply timeout
			ctx, cancel := context.WithTimeout(ctx, cfg.QueryTimeout)
			defer cancel()

			// Try different database-specific queries
			queries := []string{
				// PostgreSQL
				"SELECT table_name, table_schema, table_type FROM information_schema.tables WHERE table_schema NOT IN ('pg_catalog', 'information_schema')",
				// MySQL
				"SELECT table_name, table_schema, table_type FROM information_schema.tables WHERE table_schema = DATABASE()",
				// SQLite
				"SELECT name, '', 'BASE TABLE' FROM sqlite_master WHERE type='table' AND name NOT LIKE 'sqlite_%'",
			}

			var tables []tableInfo
			var lastErr error

			for _, q := range queries {
				rows, err := cfg.DB.QueryContext(ctx, q)
				if err != nil {
					lastErr = err
					continue
				}

				tables = make([]tableInfo, 0)
				for rows.Next() {
					var t tableInfo
					if err := rows.Scan(&t.Name, &t.Schema, &t.Type); err != nil {
						rows.Close()
						lastErr = err
						tables = nil
						break
					}
					tables = append(tables, t)
				}
				rows.Close()

				if tables != nil {
					break
				}
			}

			if tables == nil {
				return tool.Result{}, fmt.Errorf("failed to list tables: %w", lastErr)
			}

			out := tablesOutput{
				Tables: tables,
				Count:  len(tables),
			}

			data, _ := json.Marshal(out)
			return tool.Result{Output: data}, nil
		}).
		MustBuild()
}

// schemaInput is the input for the db_schema tool.
type schemaInput struct {
	Table string `json:"table"`
}

// schemaOutput is the output for the db_schema tool.
type schemaOutput struct {
	Table   string       `json:"table"`
	Columns []columnInfo `json:"columns"`
}

type columnInfo struct {
	Name       string `json:"name"`
	Type       string `json:"type"`
	Nullable   bool   `json:"nullable"`
	Default    string `json:"default,omitempty"`
	PrimaryKey bool   `json:"primary_key,omitempty"`
}

func schemaTool(cfg *Config) tool.Tool {
	return tool.NewBuilder("db_schema").
		WithDescription("Get schema information for a table").
		ReadOnly().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var in schemaInput
			if err := json.Unmarshal(input, &in); err != nil {
				return tool.Result{}, err
			}

			if in.Table == "" {
				return tool.Result{}, errors.New("table name is required")
			}

			// Apply timeout
			ctx, cancel := context.WithTimeout(ctx, cfg.QueryTimeout)
			defer cancel()

			// Try different database-specific queries
			columns, err := getPostgresSchema(ctx, cfg.DB, in.Table)
			if err != nil {
				columns, err = getMySQLSchema(ctx, cfg.DB, in.Table)
			}
			if err != nil {
				columns, err = getSQLiteSchema(ctx, cfg.DB, in.Table)
			}
			if err != nil {
				return tool.Result{}, fmt.Errorf("failed to get schema for table %s: %w", in.Table, err)
			}

			out := schemaOutput{
				Table:   in.Table,
				Columns: columns,
			}

			data, _ := json.Marshal(out)
			return tool.Result{Output: data}, nil
		}).
		MustBuild()
}

func getPostgresSchema(ctx context.Context, db *sql.DB, table string) ([]columnInfo, error) {
	query := `
		SELECT
			c.column_name,
			c.data_type,
			c.is_nullable = 'YES',
			COALESCE(c.column_default, ''),
			COALESCE(tc.constraint_type = 'PRIMARY KEY', false)
		FROM information_schema.columns c
		LEFT JOIN information_schema.key_column_usage kcu
			ON c.column_name = kcu.column_name
			AND c.table_name = kcu.table_name
		LEFT JOIN information_schema.table_constraints tc
			ON kcu.constraint_name = tc.constraint_name
			AND tc.constraint_type = 'PRIMARY KEY'
		WHERE c.table_name = $1
		ORDER BY c.ordinal_position
	`

	rows, err := db.QueryContext(ctx, query, table)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var columns []columnInfo
	for rows.Next() {
		var col columnInfo
		if err := rows.Scan(&col.Name, &col.Type, &col.Nullable, &col.Default, &col.PrimaryKey); err != nil {
			return nil, err
		}
		columns = append(columns, col)
	}

	if len(columns) == 0 {
		return nil, fmt.Errorf("table %s not found or has no columns", table)
	}

	return columns, rows.Err()
}

func getMySQLSchema(ctx context.Context, db *sql.DB, table string) ([]columnInfo, error) {
	query := `
		SELECT
			COLUMN_NAME,
			DATA_TYPE,
			IS_NULLABLE = 'YES',
			COALESCE(COLUMN_DEFAULT, ''),
			COLUMN_KEY = 'PRI'
		FROM INFORMATION_SCHEMA.COLUMNS
		WHERE TABLE_NAME = ? AND TABLE_SCHEMA = DATABASE()
		ORDER BY ORDINAL_POSITION
	`

	rows, err := db.QueryContext(ctx, query, table)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var columns []columnInfo
	for rows.Next() {
		var col columnInfo
		if err := rows.Scan(&col.Name, &col.Type, &col.Nullable, &col.Default, &col.PrimaryKey); err != nil {
			return nil, err
		}
		columns = append(columns, col)
	}

	if len(columns) == 0 {
		return nil, fmt.Errorf("table %s not found or has no columns", table)
	}

	return columns, rows.Err()
}

func getSQLiteSchema(ctx context.Context, db *sql.DB, table string) ([]columnInfo, error) {
	query := fmt.Sprintf("PRAGMA table_info(%s)", table)

	rows, err := db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var columns []columnInfo
	for rows.Next() {
		var cid int
		var col columnInfo
		var notNull int
		var pk int
		var dflt sql.NullString
		if err := rows.Scan(&cid, &col.Name, &col.Type, &notNull, &dflt, &pk); err != nil {
			return nil, err
		}
		col.Nullable = notNull == 0
		col.PrimaryKey = pk > 0
		if dflt.Valid {
			col.Default = dflt.String
		}
		columns = append(columns, col)
	}

	if len(columns) == 0 {
		return nil, fmt.Errorf("table %s not found or has no columns", table)
	}

	return columns, rows.Err()
}
