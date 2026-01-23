package database

import (
	"context"
	"database/sql"
	"encoding/json"
	"testing"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

func setupTestDB(t *testing.T) *sql.DB {
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("failed to open database: %v", err)
	}

	// Create test table
	_, err = db.Exec(`
		CREATE TABLE users (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			name TEXT NOT NULL,
			email TEXT UNIQUE,
			age INTEGER,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)
	`)
	if err != nil {
		t.Fatalf("failed to create table: %v", err)
	}

	// Insert test data
	_, err = db.Exec(`
		INSERT INTO users (name, email, age) VALUES
		('Alice', 'alice@example.com', 30),
		('Bob', 'bob@example.com', 25),
		('Charlie', 'charlie@example.com', 35)
	`)
	if err != nil {
		t.Fatalf("failed to insert data: %v", err)
	}

	return db
}

func TestNew(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	p, err := New(db)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if p == nil {
		t.Fatal("expected non-nil pack")
	}

	if p.Name != "database" {
		t.Errorf("expected name 'database', got '%s'", p.Name)
	}
}

func TestNewWithNilDB(t *testing.T) {
	_, err := New(nil)
	if err == nil {
		t.Error("expected error for nil db")
	}
}

func TestNewWithOptions(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	p, err := New(db,
		WithQueryTimeout(10*time.Second),
		WithMaxRows(500),
		WithWriteAccess(),
		WithDDLAccess(),
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if p == nil {
		t.Error("expected non-nil pack")
	}
}

func TestQueryTool(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	p, err := New(db)
	if err != nil {
		t.Fatalf("failed to create pack: %v", err)
	}

	tool, ok := p.GetTool("db_query")
	if !ok {
		t.Fatal("db_query tool not found")
	}

	// Test basic query
	input, _ := json.Marshal(queryInput{
		Query: "SELECT * FROM users ORDER BY id",
	})

	result, err := tool.Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("query failed: %v", err)
	}

	var out queryOutput
	if err := json.Unmarshal(result.Output, &out); err != nil {
		t.Fatalf("failed to unmarshal output: %v", err)
	}

	if out.RowCount != 3 {
		t.Errorf("expected 3 rows, got %d", out.RowCount)
	}

	if len(out.Columns) < 1 {
		t.Error("expected columns to be populated")
	}
}

func TestQueryToolWithLimit(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	p, err := New(db)
	if err != nil {
		t.Fatalf("failed to create pack: %v", err)
	}

	tool, ok := p.GetTool("db_query")
	if !ok {
		t.Fatal("db_query tool not found")
	}

	input, _ := json.Marshal(queryInput{
		Query: "SELECT * FROM users ORDER BY id",
		Limit: 1,
	})

	result, err := tool.Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("query failed: %v", err)
	}

	var out queryOutput
	if err := json.Unmarshal(result.Output, &out); err != nil {
		t.Fatalf("failed to unmarshal output: %v", err)
	}

	if out.RowCount != 1 {
		t.Errorf("expected 1 row, got %d", out.RowCount)
	}
}

func TestQueryToolRejectsNonSelect(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	p, err := New(db)
	if err != nil {
		t.Fatalf("failed to create pack: %v", err)
	}

	tool, ok := p.GetTool("db_query")
	if !ok {
		t.Fatal("db_query tool not found")
	}

	input, _ := json.Marshal(queryInput{
		Query: "INSERT INTO users (name, email) VALUES ('Test', 'test@example.com')",
	})

	_, err = tool.Execute(context.Background(), input)
	if err == nil {
		t.Error("expected error for non-SELECT query")
	}
}

func TestTablesTool(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	p, err := New(db)
	if err != nil {
		t.Fatalf("failed to create pack: %v", err)
	}

	tool, ok := p.GetTool("db_tables")
	if !ok {
		t.Fatal("db_tables tool not found")
	}

	result, err := tool.Execute(context.Background(), json.RawMessage("{}"))
	if err != nil {
		t.Fatalf("tables query failed: %v", err)
	}

	var out tablesOutput
	if err := json.Unmarshal(result.Output, &out); err != nil {
		t.Fatalf("failed to unmarshal output: %v", err)
	}

	if out.Count < 1 {
		t.Error("expected at least 1 table")
	}

	// Check that users table is present
	found := false
	for _, table := range out.Tables {
		if table.Name == "users" {
			found = true
			break
		}
	}
	if !found {
		t.Error("users table not found")
	}
}

func TestSchemaTool(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	p, err := New(db)
	if err != nil {
		t.Fatalf("failed to create pack: %v", err)
	}

	tool, ok := p.GetTool("db_schema")
	if !ok {
		t.Fatal("db_schema tool not found")
	}

	input, _ := json.Marshal(schemaInput{
		Table: "users",
	})

	result, err := tool.Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("schema query failed: %v", err)
	}

	var out schemaOutput
	if err := json.Unmarshal(result.Output, &out); err != nil {
		t.Fatalf("failed to unmarshal output: %v", err)
	}

	if out.Table != "users" {
		t.Errorf("expected table 'users', got '%s'", out.Table)
	}

	if len(out.Columns) != 5 {
		t.Errorf("expected 5 columns, got %d", len(out.Columns))
	}

	// Check id column
	var idCol *columnInfo
	for i := range out.Columns {
		if out.Columns[i].Name == "id" {
			idCol = &out.Columns[i]
			break
		}
	}

	if idCol == nil {
		t.Error("id column not found")
	} else if !idCol.PrimaryKey {
		t.Error("expected id to be primary key")
	}
}

func TestSchemaToolMissingTable(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	p, err := New(db)
	if err != nil {
		t.Fatalf("failed to create pack: %v", err)
	}

	tool, ok := p.GetTool("db_schema")
	if !ok {
		t.Fatal("db_schema tool not found")
	}

	input, _ := json.Marshal(schemaInput{
		Table: "",
	})

	_, err = tool.Execute(context.Background(), input)
	if err == nil {
		t.Error("expected error for missing table name")
	}
}

func TestSchemaToolNonExistentTable(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	p, err := New(db)
	if err != nil {
		t.Fatalf("failed to create pack: %v", err)
	}

	tool, ok := p.GetTool("db_schema")
	if !ok {
		t.Fatal("db_schema tool not found")
	}

	input, _ := json.Marshal(schemaInput{
		Table: "nonexistent",
	})

	_, err = tool.Execute(context.Background(), input)
	if err == nil {
		t.Error("expected error for non-existent table")
	}
}

func TestExecuteToolWithWriteAccess(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	p, err := New(db, WithWriteAccess())
	if err != nil {
		t.Fatalf("failed to create pack: %v", err)
	}

	tool, ok := p.GetTool("db_execute")
	if !ok {
		t.Fatal("db_execute tool not found")
	}

	// Insert a new user
	input, _ := json.Marshal(executeInput{
		Query: "INSERT INTO users (name, email, age) VALUES (?, ?, ?)",
		Args:  []interface{}{"Dave", "dave@example.com", 40},
	})

	result, err := tool.Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("execute failed: %v", err)
	}

	var out executeOutput
	if err := json.Unmarshal(result.Output, &out); err != nil {
		t.Fatalf("failed to unmarshal output: %v", err)
	}

	if out.RowsAffected != 1 {
		t.Errorf("expected 1 row affected, got %d", out.RowsAffected)
	}

	// Verify the insert
	var count int
	db.QueryRow("SELECT COUNT(*) FROM users WHERE name = 'Dave'").Scan(&count)
	if count != 1 {
		t.Error("user was not inserted")
	}
}

func TestExecuteToolRejectsSelect(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	p, err := New(db, WithWriteAccess())
	if err != nil {
		t.Fatalf("failed to create pack: %v", err)
	}

	tool, ok := p.GetTool("db_execute")
	if !ok {
		t.Fatal("db_execute tool not found")
	}

	input, _ := json.Marshal(executeInput{
		Query: "SELECT * FROM users",
	})

	_, err = tool.Execute(context.Background(), input)
	if err == nil {
		t.Error("expected error for SELECT query")
	}
}

func TestExecuteToolRejectsDDLWithoutAccess(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	p, err := New(db, WithWriteAccess())
	if err != nil {
		t.Fatalf("failed to create pack: %v", err)
	}

	tool, ok := p.GetTool("db_execute")
	if !ok {
		t.Fatal("db_execute tool not found")
	}

	ddlStatements := []string{
		"CREATE TABLE test (id INTEGER)",
		"ALTER TABLE users ADD COLUMN foo TEXT",
		"DROP TABLE users",
		"TRUNCATE TABLE users",
	}

	for _, stmt := range ddlStatements {
		input, _ := json.Marshal(executeInput{
			Query: stmt,
		})

		_, err = tool.Execute(context.Background(), input)
		if err == nil {
			t.Errorf("expected error for DDL statement: %s", stmt)
		}
	}
}

func TestExecuteToolAllowsDDLWithAccess(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	p, err := New(db, WithWriteAccess(), WithDDLAccess())
	if err != nil {
		t.Fatalf("failed to create pack: %v", err)
	}

	tool, ok := p.GetTool("db_execute")
	if !ok {
		t.Fatal("db_execute tool not found")
	}

	input, _ := json.Marshal(executeInput{
		Query: "CREATE TABLE test_table (id INTEGER PRIMARY KEY, value TEXT)",
	})

	_, err = tool.Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("expected DDL to succeed with DDL access: %v", err)
	}

	// Verify table was created
	var tableName string
	err = db.QueryRow("SELECT name FROM sqlite_master WHERE type='table' AND name='test_table'").Scan(&tableName)
	if err != nil {
		t.Error("table was not created")
	}
}

func TestExecuteToolWithoutWriteAccess(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	p, err := New(db) // No write access
	if err != nil {
		t.Fatalf("failed to create pack: %v", err)
	}

	_, ok := p.GetTool("db_execute")
	if ok {
		t.Error("expected db_execute tool to not exist without write access")
	}
}

func TestQueryToolWithArgs(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	p, err := New(db)
	if err != nil {
		t.Fatalf("failed to create pack: %v", err)
	}

	tool, ok := p.GetTool("db_query")
	if !ok {
		t.Fatal("db_query tool not found")
	}

	input, _ := json.Marshal(queryInput{
		Query: "SELECT * FROM users WHERE age > ?",
		Args:  []interface{}{28},
	})

	result, err := tool.Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("query failed: %v", err)
	}

	var out queryOutput
	if err := json.Unmarshal(result.Output, &out); err != nil {
		t.Fatalf("failed to unmarshal output: %v", err)
	}

	if out.RowCount != 2 {
		t.Errorf("expected 2 rows (age > 28), got %d", out.RowCount)
	}
}

func TestToolAnnotations(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	p, err := New(db, WithWriteAccess())
	if err != nil {
		t.Fatalf("failed to create pack: %v", err)
	}

	// Check query tool is read-only
	if queryTool, ok := p.GetTool("db_query"); ok {
		annotations := queryTool.Annotations()
		if !annotations.ReadOnly {
			t.Error("db_query should be read-only")
		}
	}

	// Check tables tool is read-only and cacheable
	if tablesTool, ok := p.GetTool("db_tables"); ok {
		annotations := tablesTool.Annotations()
		if !annotations.ReadOnly {
			t.Error("db_tables should be read-only")
		}
		if !annotations.Cacheable {
			t.Error("db_tables should be cacheable")
		}
	}

	// Check execute tool is destructive
	if executeTool, ok := p.GetTool("db_execute"); ok {
		annotations := executeTool.Annotations()
		if !annotations.Destructive {
			t.Error("db_execute should be destructive")
		}
	}
}
