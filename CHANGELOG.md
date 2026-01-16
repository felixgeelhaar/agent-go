# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

## [0.1.0] - 2026-01-15

### Added

#### Core Runtime
- State-driven agent runtime with canonical state machine (intake, explore, decide, act, validate, done, failed)
- Tool system with annotations (ReadOnly, Destructive, Idempotent, Cacheable, RiskLevel)
- Decision types: CallTool, Transition, AskHuman, Finish, Fail
- Policy enforcement: tool eligibility, state transitions, budget tracking
- Append-only audit ledger for all operations
- Run lifecycle management with evidence accumulation

#### LLM Providers
- Anthropic Claude provider with message API support
- OpenAI GPT provider with chat completions API
- Google Gemini provider
- Ollama provider for local models
- AWS Bedrock provider with Claude and other models
- Cohere provider (Command-R, Command-R+)
- Streaming support via StreamingProvider interface

#### Domain Packs
- **Database Pack**: PostgreSQL, SQLite, Redis tools for queries, schema management
- **Git Pack**: Repository operations, commits, branches, diffs
- **Kubernetes Pack**: Pod, service, deployment management tools
- **Cloud Pack**: Provider interface with S3, GCS, Azure Blob implementations

#### Infrastructure
- **State Machine**: Statekit integration with guards, actions, and interpreter
- **Resilience**: Fortify integration (circuit breaker, retry, rate limiter, bulkhead, timeout)
- **Observability**: OpenTelemetry tracing and metrics, structured logging with Bolt
- **Distributed**: Worker pools, message queues (Redis, memory), distributed locks
- **MCP Integration**: Model Context Protocol server/client using felixgeelhaar/mcp-go

#### Security
- Input validation middleware
- WASM tool sandboxing with wazero runtime
- Secret management integration (environment, file-based stores)
- Audit logging for security-relevant events

#### Developer Experience
- Fluent API for tool and engine construction
- ScriptedPlanner for deterministic testing
- MockPlanner for unit tests
- Comprehensive test coverage for critical packages

### Security
- Tool sandboxing with WebAssembly isolation (configurable memory/time limits)
- Input validation before tool execution
- Secret management with secure storage backends

### Dependencies
- Go 1.25.5+
- github.com/felixgeelhaar/statekit v1.0.1
- github.com/felixgeelhaar/fortify v1.1.2
- github.com/felixgeelhaar/bolt/v3 v3.1.2
- github.com/felixgeelhaar/mcp-go v1.5.0
- github.com/tetratelabs/wazero v1.11.0
- go.opentelemetry.io/otel v1.39.0
- And various cloud SDKs (AWS, GCP, Azure)

[unreleased]: https://github.com/felixgeelhaar/agent-go/compare/v0.1.0...HEAD
[0.1.0]: https://github.com/felixgeelhaar/agent-go/releases/tag/v0.1.0
