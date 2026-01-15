# Governed Adaptivity Example

This example demonstrates Horizon 3: Governed Adaptivity features including pattern detection, suggestion generation, proposal workflow, and visual inspectors.

## Running the Example

```bash
go run ./example/governed_adaptivity
```

## What It Demonstrates

### 1. Simulated Agent Runs
Creates 5 agent runs with events including:
- State transitions (intake → explore → decide → act → validate → done)
- Tool calls (read_file, analyze_content, write_file)
- Some runs include failures for pattern detection

### 2. Pattern Detection
Uses composite pattern detectors to find:
- Tool sequence patterns (repeated tool call sequences)
- Failure patterns (recurring tool failures)

### 3. Suggestion Generation
Generates policy improvement suggestions from detected patterns:
- Eligibility suggestions (add/remove tools from states)
- Budget suggestions (increase/decrease limits)

### 4. Proposal Workflow
Full human-governed policy evolution:
- Create proposal with changes
- Submit for review
- Approve (requires human actor)
- Apply to create new policy version
- Rollback to previous version

### 5. Visual Exports
Exports for visualization:
- **JSON**: Run details with events and timeline
- **DOT**: State machine for Graphviz
- **Mermaid**: State machine for Markdown
- **Metrics**: Dashboard data (success rates, durations)

## Output Files

The example creates:
- `state_machine.dot` - Graphviz DOT format
- `state_machine.mmd` - Mermaid diagram format

Render DOT with Graphviz:
```bash
dot -Tpng state_machine.dot -o state_machine.png
```

## Key Constraint

**No unsupervised self-modification** - All policy changes require explicit human approval through the proposal workflow.
