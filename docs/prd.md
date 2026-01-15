# Product Requirements Document (PRD)

## Product Objective

Build a **state-driven agent runtime** that allows developers to create AI-powered systems that are safe, explainable, testable, and evolvable across time.

This product must scale from:

- in-memory, single-process runs
  to
- distributed, policy-governed systems

without changing core abstractions.

---

## Users

- Backend engineers
- Platform / infra teams
- AI engineers building production systems

---

## Non-Negotiable Principles

These apply to **all horizons**:

- Agents operate inside explicit state graphs
- Tools are always explicitly allowed or denied
- Policies are enforced structurally
- Behavior is inspectable
- Infrastructure is optional

---

## Functional Scope by Horizon

---

## Horizon 1 — Controlled Autonomy

### Capabilities

- State-driven execution engine
- Default state graph and planner
- String-identified tools with schema validation
- Tool metadata for safety (read-only, destructive, idempotent)
- Middleware-based policy enforcement
- Artifact handling
- In-memory operation by default

### Default States

- intake
- explore
- decide
- act
- validate
- done
- failed

### Guarantees

- Planner outputs are bounded
- Tools cannot run outside allowed states
- Destructive actions require approval
- Budgets are enforced
- Explainability exists without persistence

---

## Horizon 2 — Operational Intelligence

### Expanded Capabilities

- Durable RunStore (pause/resume)
- EventStore for audit and replay
- Multiple replay modes (narrative, decision, full)
- Simulation / dry-run execution
- Redis-backed caching
- Domain packs (ops, research, data)
- Cross-run analysis

### Guarantees

- Replay does not change semantics
- Cached results are policy-aware
- Artifacts are reference-stable

---

## Horizon 3 — Governed Adaptivity

### Exploratory Capabilities

- Agent-suggested state graph changes
- Policy recommendations
- Automated playbook proposals
- Pattern detection across runs

### Hard Constraints

- No unsupervised self-modification
- All changes require human approval
- No silent policy changes

---

## Storage & Infrastructure (All Horizons)

### Guiding Rule

> Storage is a capability, not a requirement.

Supported capability types:

- RunStore (snapshots)
- EventStore (audit/replay)
- ArtifactStore (large outputs)
- Cache (tool results)
- MemoryStore (knowledge / retrieval)

Each is:

- optional
- independently configurable
- replaceable

---

## Quality & Test Requirements

- Deterministic execution mode
- Testable without LLMs
- No required external services
- Explicit failure modes
- Invariant-driven test suite

---

## Explicit Non-Goals

- Multi-agent orchestration (for now)
- Dynamic state creation by LLMs
- UI dashboards (initially)
- Model-specific abstractions

---

## Summary

This product prioritizes **trust before autonomy** and **structure before intelligence**.

Future capability emerges from **explicit design**, not increased freedom.
