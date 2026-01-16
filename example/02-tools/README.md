# 02 - Tools

Demonstrates creating multiple tools with different annotations and how tool eligibility controls access.

## What This Example Shows

- Creating tools with various annotation combinations
- Understanding `ReadOnly`, `Destructive`, `Idempotent`, `Cacheable`
- Using `RiskLevel` to classify tool danger
- Setting up tool eligibility per state
- How state transitions enable different tools

## Run It

```bash
go run main.go
```

## Expected Output

```
=== Tools Example ===
Status: done
Steps: 5

Output file contents: Processed output!

=== Tool Annotations Summary ===
read_file:   ReadOnly=true,  Destructive=false, Risk=Low
write_file:  ReadOnly=false, Destructive=false, Risk=Medium
delete_file: ReadOnly=false, Destructive=true,  Risk=High
```

## Tool Annotations Explained

### ReadOnly

```go
WithAnnotations(agent.Annotations{
    ReadOnly: true,  // Tool does not modify external state
})
```

- `true`: Tool only reads, never writes. Safe for `explore` and `validate` states.
- `false`: Tool may modify state. Typically restricted to `act` state.

### Destructive

```go
WithAnnotations(agent.Annotations{
    Destructive: true,  // May cause irreversible changes
})
```

- `true`: Action cannot be undone (delete, send email). Requires approval if configured.
- `false`: Action is recoverable (overwrite a file, update a record).

### Idempotent

```go
WithAnnotations(agent.Annotations{
    Idempotent: true,  // Same input always produces same result
})
```

- `true`: Safe to retry on failure. The runtime will automatically retry with exponential backoff.
- `false`: Not safe to retry (e.g., sending an email would send multiple copies).

### Cacheable

```go
WithAnnotations(agent.Annotations{
    Cacheable: true,  // Results can be cached
})
```

- `true`: For the same input, cached results can be returned.
- `false`: Must execute fresh every time.

### RiskLevel

```go
WithAnnotations(agent.Annotations{
    RiskLevel: agent.RiskHigh,  // None, Low, Medium, High, Critical
})
```

Used for governance and approval decisions. Higher risk = more scrutiny.

## Tool Eligibility

Control which tools are available in which states:

```go
eligibility := agent.NewToolEligibility()

// Read-only in explore (information gathering)
eligibility.Allow(agent.StateExplore, "read_file")

// All tools in act (execution)
eligibility.Allow(agent.StateAct, "read_file", "write_file", "delete_file")
```

This ensures that destructive operations can only happen in the `act` state.

## Next Steps

- **[03-policies](../03-policies/)** - Add budgets and approval requirements
- **[04-llm-planner](../04-llm-planner/)** - Use a real LLM instead of scripted steps
