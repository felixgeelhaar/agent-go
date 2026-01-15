# Technical Design Document

## 1. Design Goals

- Deterministic core
- Explicit control flow
- Orthogonal storage
- Policy-driven safety
- Evolvable without breaking changes

---

## 2. System Architecture

Engine
├─ StateGraph
├─ Planner
├─ ToolRegistry
├─ MiddlewareChain
└─ Storage Capabilities (optional)

The Engine is stable across horizons.

---

## 3. Core Abstractions

### Engine

Configures:

- states
- tools
- planner
- middleware
- storage adapters

---

### Run

A run represents one execution.

Fields:

- RunID
- CurrentState
- Vars
- Budgets
- Ledger
- Artifacts
- Status

Runs are resumable when RunStore is present.

---

### StateGraph

Defines:

- allowed states
- allowed transitions
- tool eligibility per state
- approvals and budget overrides

States are **structural**, not behavioral.

---

## 4. State Semantics (Canonical)

| State    | Responsibility       |
| -------- | -------------------- |
| intake   | Normalize goal       |
| explore  | Gather evidence      |
| decide   | Choose next step     |
| act      | Perform side-effects |
| validate | Confirm outcome      |
| done     | Finalize             |
| failed   | Terminal failure     |

---

## 5. Tool System

### Identity

- Stable string name

### Definition

- Input schema
- Output schema
- Annotations
- Handler
- Artifact emission

### Annotations

- ReadOnly
- Destructive
- Idempotent
- Cacheable
- Risk level

Annotations influence:

- planner scoring
- policy enforcement
- replay eligibility

---

## 6. Planner

### Contract

Input:

- State
- Evidence
- Allowed tools
- Budgets

Output:

- CallTool
- Transition
- AskHuman
- Finish
- Fail

### Guarantees

- Bounded outputs
- No side effects
- Conservative bias
- Deterministic mode

Planner evolves heuristics across horizons, not its contract.

---

## 7. Middleware & Policy

Middleware intercepts:

- planner output
- tool execution
- state transitions

Policies enforce:

- approvals
- budgets
- rate limits
- caching
- auditing

Middleware order is deterministic.

---

## 8. Storage Capabilities (All Horizons)

### RunStore

- Snapshot run state
- Enables pause/resume

### EventStore

- Append-only event log
- Enables audit and replay

### ArtifactStore

- Stores large outputs
- Returns stable ArtifactRef

### Cache

- Tool result caching
- Annotation-driven

### MemoryStore

- Knowledge persistence
- Retrieval support

All stores are optional and pluggable.

---

## 9. Replay & Simulation

Replay modes:

- Narrative replay
- Decision replay
- Full replay (idempotent tools only)

Simulation executes without side effects.

---

## 10. Explainability Model

Every run yields:

- state timeline
- action ledger
- decision rationales
- artifact references

Explainability exists even without persistence.

---

## 11. Failure Model

Failures are explicit and terminal.
No silent recovery.

---

## 12. Determinism & Testing

Required invariants:

1. Tool eligibility
2. Transition validity
3. Approval enforcement
4. Budget enforcement
5. Artifact integrity
6. Cache correctness
7. Replay determinism
8. Ledger completeness

These invariants hold across all horizons.
