# Package `application`

**Import path:** `go.klarlabs.de/agent/application`

## Overview

package application // import "go.klarlabs.de/agent/application"

Package application provides application services.

Package application provides the application layer for the agent runtime.

Package application provides application services.

Package application provides application services.

## Full API Reference

```
package application // import "go.klarlabs.de/agent/application"

Package application provides application services.

Package application provides the application layer for the agent runtime.

Package application provides application services.

Package application provides application services.

VARIABLES

var ErrInvalidStep = errors.New("invalid step index")
    ErrInvalidStep indicates a fork/reconstruct was requested at a step index
    below 1 (steps are 1-based).


TYPES

type DetectionService struct {
	// Has unexported fields.
}
    DetectionService manages pattern detection.

func NewDetectionService(detector pattern.Detector, patternStore pattern.Store) *DetectionService
    NewDetectionService creates a new detection service.

func (s *DetectionService) DeletePattern(ctx context.Context, id string) error
    DeletePattern removes a pattern.

func (s *DetectionService) DetectPatterns(ctx context.Context, opts pattern.DetectionOptions) ([]pattern.Pattern, error)
    DetectPatterns runs pattern detection and stores the results.

func (s *DetectionService) GetPattern(ctx context.Context, id string) (*pattern.Pattern, error)
    GetPattern retrieves a pattern by ID.

func (s *DetectionService) GetSupportedPatternTypes() []pattern.PatternType
    GetSupportedPatternTypes returns the pattern types the detector can find.

func (s *DetectionService) ListPatterns(ctx context.Context, filter pattern.ListFilter) ([]*pattern.Pattern, error)
    ListPatterns returns patterns matching the filter.

type Engine struct {
	// Has unexported fields.
}
    Engine is the main orchestration service for agent execution.

func NewEngine(config EngineConfig) (*Engine, error)
    NewEngine creates a new engine with the given configuration.

func NewEngineWithOptions(opts ...Option) (*Engine, error)
    NewEngineWithOptions creates an engine with functional options.

func (e *Engine) ContinueRun(ctx context.Context, run *agent.Run) (*agent.Run, error)
    ContinueRun drives a reconstructed run (e.g. the product of Fork or a
    replay) further from its current state. It re-attaches a fresh state
    machine and a new per-run Governor to the run and resumes the step loop from
    run.CurrentState, re-applying the full pipeline: the structural act-gate,
    governance authorization (budget + approval), and the 12-event stream.

    The run's tool-call budget is seeded with the calls already consumed (the
    persisted tool-result evidence count) so a continued fork does not silently
    reset its run-spanning budget to full. A nil or already-terminal run is
    returned unchanged with an error.

    ContinueRun is the run-advancing counterpart to Fork: Fork reconstructs and
    inspects, ContinueRun re-drives. Pair them to branch a run and explore the
    branch under the same governance and act-gate guarantees as the original.

func (e *Engine) Fork(ctx context.Context, runID string, stepN int) (*agent.Run, error)
    Fork creates a new run branched from an existing run's event history at a
    given step. The source run is reconstructed up to and including its first
    stepN executed steps (decision.made boundaries), and the resulting state —
    goal, variables, evidence, and current state — is materialized into a fresh
    run with a new ID. The fork is persisted (when a run store is configured)
    and a lineage event linking parent and child is recorded in the new run's
    event stream.

    Fork enables "what-if" branching and deterministic re-exploration from any
    point in a run's history (pair with WithClock for fully reproducible forks).
    The returned run is in its reconstructed state and not yet re-executed;
    callers may inspect it, or drive it further with ContinueRun, which resumes
    the step loop from the fork's current state under the same governance and
    structural act-gate guarantees.

    Requires an event store (use WithEventStore). stepN must be >= 1.

func (e *Engine) Knowledge() knowledge.Store
    Knowledge returns the knowledge store, if configured.

func (e *Engine) ResumeWithInput(ctx context.Context, run *agent.Run, input string) (*agent.Run, error)
    ResumeWithInput continues a paused run with human-provided input.

func (e *Engine) Run(ctx context.Context, goal string) (*agent.Run, error)
    Run executes the agent with the given goal.

func (e *Engine) RunInTask(ctx context.Context, goal string, tc *task.Context) (*agent.Run, error)
    RunInTask executes the agent within a shared task context. The task context
    enables sharing variables, evidence, and artifact references between parent
    and child agents in a delegation hierarchy.

func (e *Engine) RunWithVars(ctx context.Context, goal string, vars map[string]any) (*agent.Run, error)
    RunWithVars executes the agent with the given goal and initial variables.

func (e *Engine) Stream(ctx context.Context, goal string) (string, <-chan event.Event, error)
    Stream executes the agent in the background and returns a channel of events.
    The returned channel receives events as the agent executes. It is closed
    when the run completes, fails, or the context is cancelled. Requires an
    EventStore to be configured via WithEventStore.

type EngineConfig struct {
	Registry     tool.Registry
	Planner      domainplanner.Planner
	Executor     *resilience.Executor
	Artifacts    artifact.Store
	Knowledge    knowledge.Store
	Eligibility  *policy.ToolEligibility
	Transitions  *policy.StateTransitions
	Approver     policy.Approver
	BudgetLimits map[string]int
	MaxSteps     int
	// MaxNoProgress aborts a run after this many consecutive steps that make no
	// progress — no state change AND no new evidence (e.g. the planner keeps
	// emitting self-transitions or repeats). This is loop detection: it catches a
	// stuck agent cheaply, long before MaxSteps. Zero uses the default (6).
	MaxNoProgress int
	Middleware    *middleware.Registry
	Tracer        telemetry.Tracer
	Meter         telemetry.Meter
	RunStore      run.Store
	EventStore    event.Store
	TaskContext   *task.Context
	// Governance selects the governance backend (budget + approval +
	// evidence). When nil, the full-delegation KernelFactory is used: each run
	// is ONE axi session, so budget, the destructive-tool approval gate, and
	// the evidence chain are all axi-native.
	Governance governance.Factory
	// Logger is the injected structured logger. When nil, a no-op logger is
	// used and the engine emits no logs — the execution path never falls back
	// to the package-level logging singleton.
	Logger *logging.Logger
	// Clock supplies the time used for run IDs, run start timestamps, and
	// event timestamps. When nil, the system clock is used. Inject a fixed or
	// statekit FakeClock for deterministic replay and tests.
	Clock clock.Clock
	// StrictAudit, when non-nil, overrides the default audit policy. When nil,
	// strict audit is enabled whenever EventStore is configured. Strict mode
	// fails the run if EventStore.Append or RunStore Save/Update returns an
	// error, instead of logging and continuing.
	//
	// Audit clarity: the per-run ledger created by the engine is ephemeral
	// (in-memory for that run only). Durable, queryable audit requires an
	// EventStore (and optionally a RunStore). Prefer WithEventStore in
	// production; without it there is no durable audit trail.
	StrictAudit *bool
}
    EngineConfig contains configuration for the engine.

type EventIterator struct {
	// Has unexported fields.
}
    EventIterator allows iterating over events one at a time.

func (it *EventIterator) Index() int
    Index returns the current position.

func (it *EventIterator) Len() int
    Len returns the total number of events.

func (it *EventIterator) Next() *event.Event
    Next returns the next event, or nil if done.

func (it *EventIterator) Peek() *event.Event
    Peek returns the next event without advancing.

func (it *EventIterator) Reset()
    Reset returns to the beginning.

type EvolutionService struct {
	// Has unexported fields.
}
    EvolutionService manages policy evolution through the suggestion-proposal
    workflow.

func NewEvolutionService(
	generator suggestion.Generator,
	suggestionStore suggestion.Store,
	workflow *infraProposal.WorkflowService,
	patternStore pattern.Store,
) *EvolutionService
    NewEvolutionService creates a new evolution service.

func (s *EvolutionService) AcceptSuggestion(ctx context.Context, suggestionID, actor string) (*proposal.Proposal, error)
    AcceptSuggestion converts a suggestion into a proposal.

func (s *EvolutionService) ApplyProposal(ctx context.Context, proposalID string) error
    ApplyProposal applies an approved proposal.

func (s *EvolutionService) ApproveProposal(ctx context.Context, proposalID, approver, reason string) error
    ApproveProposal approves a proposal.

func (s *EvolutionService) CreateProposal(ctx context.Context, title, description, creator string) (*proposal.Proposal, error)
    CreateProposal creates a new proposal manually.

func (s *EvolutionService) GenerateSuggestions(ctx context.Context, patternIDs []string) ([]suggestion.Suggestion, error)
    GenerateSuggestions creates suggestions from detected patterns.

func (s *EvolutionService) GetProposal(ctx context.Context, id string) (*proposal.Proposal, error)
    GetProposal retrieves a proposal by ID.

func (s *EvolutionService) GetSuggestion(ctx context.Context, id string) (*suggestion.Suggestion, error)
    GetSuggestion retrieves a suggestion by ID.

func (s *EvolutionService) ListSuggestions(ctx context.Context, filter suggestion.ListFilter) ([]*suggestion.Suggestion, error)
    ListSuggestions returns suggestions matching the filter.

func (s *EvolutionService) RejectProposal(ctx context.Context, proposalID, rejector, reason string) error
    RejectProposal rejects a proposal.

func (s *EvolutionService) RejectSuggestion(ctx context.Context, suggestionID, actor, reason string) error
    RejectSuggestion rejects a suggestion.

func (s *EvolutionService) RollbackProposal(ctx context.Context, proposalID, reason string) error
    RollbackProposal rolls back an applied proposal.

func (s *EvolutionService) SubmitProposal(ctx context.Context, proposalID, submitter string) error
    SubmitProposal submits a proposal for review.

type InspectionService struct {
	// Has unexported fields.
}
    InspectionService provides inspection and export capabilities.

func NewInspectionService(insp inspector.Inspector) *InspectionService
    NewInspectionService creates a new inspection service.

func (s *InspectionService) ExportMetrics(ctx context.Context, filter inspector.MetricsFilter, format inspector.ExportFormat) ([]byte, error)
    ExportMetrics exports aggregated metrics.

func (s *InspectionService) ExportRun(ctx context.Context, runID string, format inspector.ExportFormat) ([]byte, error)
    ExportRun exports run data in the specified format.

func (s *InspectionService) ExportStateMachine(ctx context.Context, format inspector.ExportFormat) ([]byte, error)
    ExportStateMachine exports the state machine graph.

func (s *InspectionService) GetMetricsAsJSON(ctx context.Context, filter inspector.MetricsFilter) ([]byte, error)
    GetMetricsAsJSON exports metrics as JSON (convenience method).

func (s *InspectionService) GetRunAsJSON(ctx context.Context, runID string) ([]byte, error)
    GetRunAsJSON exports run data as JSON (convenience method).

func (s *InspectionService) GetStateMachineAsDOT(ctx context.Context) ([]byte, error)
    GetStateMachineAsDOT exports the state machine as DOT graph (convenience
    method).

func (s *InspectionService) GetStateMachineAsMermaid(ctx context.Context) ([]byte, error)
    GetStateMachineAsMermaid exports the state machine as Mermaid diagram
    (convenience method).

type Option func(*EngineConfig)
    Option configures the engine.

func WithApprover(a policy.Approver) Option
    WithApprover sets the approval handler.

func WithArtifactStore(s artifact.Store) Option
    WithArtifactStore sets the artifact store.

func WithBudgets(limits map[string]int) Option
    WithBudgets sets budget limits.

func WithClock(c clock.Clock) Option
    WithClock sets the clock used for run IDs, run start timestamps, and event
    timestamps. When unset, the system clock is used. Inject a fixed or statekit
    FakeClock for deterministic replay and tests.

func WithEligibility(e *policy.ToolEligibility) Option
    WithEligibility sets the tool eligibility configuration.

func WithEventStore(s event.Store) Option
    WithEventStore sets the event store for event sourcing and streaming. When
    configured, domain events (run.started, tool.called, state.transitioned,
    etc.) are published throughout the run. Required for Stream() to work.

func WithExecutor(e *resilience.Executor) Option
    WithExecutor sets the resilient executor.

func WithLogger(l *logging.Logger) Option
    WithLogger sets the injected structured logger for the engine. When unset,
    the engine uses a no-op logger and emits nothing — the execution path never
    depends on the package-level logging singleton.

func WithMaxSteps(n int) Option
    WithMaxSteps sets the maximum number of steps.

func WithMeter(m telemetry.Meter) Option
    WithMeter sets the OpenTelemetry meter for metrics collection. When
    configured, the engine records run duration, step counts, and tool latency.

func WithMiddleware(m *middleware.Registry) Option
    WithMiddleware sets a custom middleware registry. If not set, the engine
    uses a default middleware chain with: - Eligibility middleware (tool access
    control per state) - Approval middleware (human approval for destructive
    tools) - Logging middleware (execution timing and results)

func WithPlanner(p domainplanner.Planner) Option
    WithPlanner sets the planner.

func WithRegistry(r tool.Registry) Option
    WithRegistry sets the tool registry.

func WithRunStore(s run.Store) Option
    WithRunStore sets the run store for persistent run state. When configured,
    runs are automatically saved on creation and updated on each step and
    terminal state. Persistence is best-effort — failures are logged but do not
    abort the run.

func WithTaskContext(tc *task.Context) Option
    WithTaskContext sets a shared task context for multi-agent coordination.
    When configured, the engine merges shared variables, propagates evidence to
    the task context, and sets ParentRunID/TaskID on runs.

func WithTracer(t telemetry.Tracer) Option
    WithTracer sets the OpenTelemetry tracer for distributed tracing. When
    configured, the engine creates spans for runs, steps, planner decisions,
    and tool executions. Pass nil to disable tracing.

func WithTransitions(t *policy.StateTransitions) Option
    WithTransitions sets the state transitions configuration.

type Replay struct {
	// Has unexported fields.
}
    Replay provides event replay capabilities.

func NewReplay(eventStore event.Store) *Replay
    NewReplay creates a new replay engine.

func (r *Replay) NewEventIterator(ctx context.Context, runID string) (*EventIterator, error)
    NewEventIterator creates an iterator over events.

func (r *Replay) NewTimeline(ctx context.Context, runID string) (*Timeline, error)
    NewTimeline creates a timeline from events.

func (r *Replay) ReconstructRun(ctx context.Context, runID string) (*agent.Run, error)
    ReconstructRun rebuilds a run's state from its event history.

func (r *Replay) ReconstructRunAtStep(ctx context.Context, runID string, n int) (*agent.Run, error)
    ReconstructRunAtStep rebuilds a run's state from its event history up to and
    including the first n executed steps. A step boundary is a decision.made
    event, so the reconstructed run reflects the effects of the first n planner
    decisions (transitions, tool results, variables, evidence) and stops before
    the (n+1)th decision.

    n must be >= 1. If the run has fewer than n steps, the full history is
    applied. This is the truncation primitive used by Engine.Fork.

func (r *Replay) ReconstructRunFrom(ctx context.Context, runID string, fromSeq uint64) (*agent.Run, error)
    ReconstructRunFrom rebuilds a run's state from a starting sequence.

type StateTransition struct {
	From      agent.State
	To        agent.State
	Reason    string
	Timestamp time.Time
}
    StateTransition represents a state change.

type Timeline struct {
	// Has unexported fields.
}
    Timeline provides a time-based view of events.

func (tl *Timeline) Duration() time.Duration
    Duration returns the total duration of the run.

func (tl *Timeline) EventsByType(eventType event.Type) []event.Event
    EventsByType returns events of a specific type.

func (tl *Timeline) EventsInRange(from, to time.Time) []event.Event
    EventsInRange returns events within a time range.

func (tl *Timeline) StateTransitions() []StateTransition
    StateTransitions returns all state transition events.

func (tl *Timeline) ToolCalls() []ToolCall
    ToolCalls returns all tool call events with their results.

type ToolCall struct {
	ToolName  string
	Input     json.RawMessage
	Output    json.RawMessage
	StartTime time.Time
	Duration  time.Duration
	State     agent.State
	Success   bool
	Cached    bool
	Error     string
}
    ToolCall represents a tool invocation with its result.
```
