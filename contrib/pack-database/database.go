// Package database provides database query tools for agent-go.
//
// This pack includes tools for database operations:
//   - db_query: Execute a SELECT query and return results
//   - db_execute: Execute an INSERT, UPDATE, or DELETE statement
//   - db_transaction: Execute multiple statements in a transaction
//   - db_schema: Get database schema information
//   - db_tables: List tables in the database
//   - db_describe: Describe a table's columns and types
//
// Supports PostgreSQL, MySQL, SQLite, and SQL Server.
// Queries are parameterized to prevent SQL injection.
package database

import (
	"github.com/felixgeelhaar/agent-go/domain/agent"
	"github.com/felixgeelhaar/agent-go/domain/pack"
	"github.com/felixgeelhaar/agent-go/domain/tool"
)

// Pack returns the database tools pack.
func Pack() *pack.Pack {
	return pack.NewBuilder("database").
		WithDescription("Database query and management tools").
		WithVersion("0.1.0").
		AddTools(
			dbQuery(),
			dbExecute(),
			dbTransaction(),
			dbSchema(),
			dbTables(),
			dbDescribe(),
		).
		AllowInState(agent.StateExplore, "db_query", "db_schema", "db_tables", "db_describe").
		AllowInState(agent.StateAct, "db_query", "db_execute", "db_transaction", "db_schema", "db_tables", "db_describe").
		AllowInState(agent.StateValidate, "db_query", "db_schema", "db_tables", "db_describe").
		Build()
}

func dbQuery() tool.Tool {
	return tool.NewBuilder("db_query").
		WithDescription("Execute a SELECT query and return results as JSON").
		ReadOnly().
		MustBuild()
}

func dbExecute() tool.Tool {
	return tool.NewBuilder("db_execute").
		WithDescription("Execute an INSERT, UPDATE, or DELETE statement").
		WithRiskLevel(tool.RiskHigh).
		RequiresApproval().
		MustBuild()
}

func dbTransaction() tool.Tool {
	return tool.NewBuilder("db_transaction").
		WithDescription("Execute multiple statements in a transaction").
		WithRiskLevel(tool.RiskHigh).
		RequiresApproval().
		MustBuild()
}

func dbSchema() tool.Tool {
	return tool.NewBuilder("db_schema").
		WithDescription("Get the database schema as JSON").
		ReadOnly().
		Cacheable().
		MustBuild()
}

func dbTables() tool.Tool {
	return tool.NewBuilder("db_tables").
		WithDescription("List all tables in the database").
		ReadOnly().
		Cacheable().
		MustBuild()
}

func dbDescribe() tool.Tool {
	return tool.NewBuilder("db_describe").
		WithDescription("Describe a table's columns, types, and constraints").
		ReadOnly().
		Cacheable().
		MustBuild()
}
