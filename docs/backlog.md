
## Domain Layer - Core Aggregates

Implement the domain layer with DDD patterns including: Agent aggregate (state, decision, run, evidence), Tool aggregate (tool interface, annotations, schema, registry), Policy subdomain (budget, approval, constraint), Ledger subdomain (audit logging), Artifact subdomain (artifact references and storage), and supporting domains (cache, event sourcing, patterns, proposals, suggestions).

---

## Infrastructure - State Machine Integration

Integrate statekit library for canonical agent statechart with states: intake, explore, decide, act, validate, done, failed. Includes guards, actions, interpreter, and state transitions. Currently has type mismatch issue between ir.MachineConfig and statekit.MachineConfig that needs resolution.

---

## Infrastructure - Resilience Patterns

Integrate fortify library for resilient tool execution with patterns: Bulkhead (concurrency control), Timeout, Circuit Breaker (failure isolation), Retry with exponential backoff. Composition: Bulkhead → Timeout → Circuit Breaker → Retry. Currently has import issues with fortify package structure.

---

## Infrastructure - LLM Planners

LLM provider integrations for the planner contract including: OpenAI/compatible APIs, AWS Bedrock (Claude), and mock/scripted planners for testing. Bedrock provider has AWS SDK import issues to resolve.

---

## Infrastructure - Storage Implementations

Storage adapters for repositories including: Memory (tool registry, cache, run store, event store, pattern store), Redis (cache), PostgreSQL (run store, event store), and filesystem artifact storage.

---

## Infrastructure - Cloud Provider Packs

Cloud storage provider integrations: AWS S3 provider with bucket/object operations, Azure Blob Storage provider, Google Cloud Storage provider. S3 provider has AWS SDK import compilation issues to resolve.

---

## Application Layer - Engine Orchestration

Main orchestration service that coordinates the agent runtime including: Run lifecycle management, planner integration, tool execution with middleware, event publishing, and replay capabilities for debugging/testing.

---

## Middleware System

Composable middleware chain for tool execution including: Eligibility checking (tool allowed in state), Approval workflow (destructive actions), Ledger recording (audit trail), and Logging middleware.

---

## Analytics and Pattern Detection

Runtime analytics aggregation and pattern detection system including: Sequence patterns, performance patterns, failure patterns, composite detector, and suggestion generation for optimization recommendations.

---

## Policy Proposal Workflow

Change management for policy updates including: Proposal creation, status workflow (draft, pending, approved, rejected), versioned policy snapshots, and applier for safe policy transitions.

---

## Inspector and Export

Runtime inspection and state export capabilities including: JSON formatter, DOT graph formatter, Mermaid diagram formatter for visualizing agent state machines and run history.

---

## Design Invariant Test Suite

Comprehensive test suite validating 8 design invariants: Tool eligibility, transition validity, approval enforcement, budget enforcement, state semantics (only Act has side effects), tool registration uniqueness, run lifecycle, and evidence accumulation.

---

## MemoryStore - Knowledge Persistence

Implement MemoryStore capability for knowledge persistence and retrieval support. This enables RAG-style patterns where agents can store and retrieve knowledge across runs. Should support: vector embeddings storage, semantic search/retrieval, knowledge graph relationships, and pluggable backends (in-memory, PostgreSQL with pgvector, dedicated vector DBs).

---

## Invariant Test - Cache Correctness

Add explicit invariant test for cache correctness as specified in TDD. Test should verify: cached results match original execution, cache keys are deterministic, annotation-driven caching respects Cacheable flag, cache invalidation works correctly, and policy-aware caching honors budget/approval constraints.

---

## Invariant Test - Replay Determinism

Add explicit invariant test for replay determinism as specified in TDD. Test should verify: narrative replay produces same timeline, decision replay reaches same decisions given same evidence, full replay with idempotent tools produces identical results, and replay does not change semantics.

---

## Invariant Test - Artifact Integrity

Add explicit invariant test for artifact integrity as specified in TDD. Test should verify: ArtifactRef is stable and unique, stored artifacts match retrieved content, artifact metadata is preserved, large outputs are correctly stored and retrieved, and artifacts survive across run pause/resume.

---

## Ledger Completeness Invariant Test

Add Invariant Test 12 for ledger completeness as specified in TDD. Test should verify: all tool executions are recorded, all state transitions are recorded, ledger entries are immutable and append-only, ledger entries have correct timestamps and ordering, and ledger can be queried by run ID, time range, and event type.

---

## Rate Limiting Middleware

Implement rate limiting middleware for tool execution using fortify's RateLimiter. Features: global rate limit across all tools, per-tool rate limits with custom configurations, per-run rate limits, configurable rate/burst/interval, fail-open/fail-closed modes, callbacks for rate limit events (exceeded/allowed), and integration with existing middleware chain. Files: domain/policy/ratelimit.go (errors), infrastructure/middleware/ratelimit.go (implementation), interfaces/api/agent.go (WithRateLimit option).

---

## Webhook Notifications

Implement webhook notification system for external event delivery. Features: subscribe to event store events, filter events by type per endpoint, batch events before delivery, HMAC-SHA256 payload signing for security, retry with exponential backoff using fortify, circuit breaker per endpoint, multiple webhook endpoints with independent configs. Domain layer: domain/notification/ (webhook config, delivery result, event filter, errors). Infrastructure layer: infrastructure/notification/ (webhook_notifier, signing, batcher, sender). Public API: interfaces/api/notification.go.

---

## CLI Tool

Implement CLI tool for running agents from command line. Commands: 'agent run' (execute agent with goal), 'agent validate' (validate config file), 'agent export-schema' (export JSON schema), 'agent list-packs' (list available packs), 'agent inspect' (inspect state machine/run/tools), 'agent version'. Global flags: --config, --log-level, --log-format, --quiet, --verbose. Run flags: --pack, --planner, --model, --max-steps, --timeout, --budget, --var, --approval, --dry-run, --output, --trace. Files: cmd/agent/main.go, interfaces/cli/ (app.go, run.go, validate.go, etc.).

---

## YAML/JSON Configuration System

Implement configuration file support for agent setup. Schema includes: version, metadata (name, description), planner config (type, model, provider-specific), packs to load with options, custom eligibility overrides, state transitions, budgets, execution limits, approval config, resilience config, middleware config, logging config, output config, initial variables. Environment variable expansion with ${VAR} syntax. Domain layer: domain/config/ (config.go, validation.go, errors.go). Infrastructure layer: infrastructure/config/ (loader.go, parser.go, env.go, defaults.go, schema.go, builder.go). Supports YAML and JSON formats.

---

## OpenTelemetry Metrics Export

Enhance observability with OpenTelemetry metrics export. Metrics to track: tool execution counts and durations, state transition counts, budget consumption, rate limit hits, cache hit/miss ratios, circuit breaker state changes, webhook delivery success/failure rates. Integration with existing bolt logging and otel tracing. Files: infrastructure/telemetry/metrics.go, infrastructure/middleware/metrics.go (metrics middleware).

---
