# 06 - Distributed Execution

Demonstrates scaling agent execution across multiple workers using queues and distributed locks.

## What This Example Shows

- Creating a task queue for work distribution
- Starting multiple workers to process tasks
- Using distributed locks for coordination
- Collecting worker metrics

## Run It

```bash
go run main.go
```

## Expected Output

```
=== Distributed Execution Example ===

Infrastructure created:
  - Queue: MemoryQueue (use Redis/NATS in production)
  - Lock: MemoryLock (use Redis in production)

Starting 3 workers...

Submitting 10 tasks to the queue...

Processing tasks...
  [Worker worker-1] Processed item: item-3
  [Worker worker-2] Processed item: item-1
  [Worker worker-3] Processed item: item-2
  ...

=== Worker Metrics ===
Worker 1:
  Tasks Started:   4
  Tasks Completed: 4
  Tasks Failed:    0
  Avg Duration:    52ms
...
```

## Architecture

```
┌─────────────┐     ┌─────────────┐     ┌─────────────┐
│   Client    │     │   Client    │     │   Client    │
└──────┬──────┘     └──────┬──────┘     └──────┬──────┘
       │                   │                   │
       └───────────────────┴───────────────────┘
                           │
                           ▼
                    ┌─────────────┐
                    │    Queue    │  ← Redis, NATS, or Memory
                    └──────┬──────┘
                           │
       ┌───────────────────┼───────────────────┐
       │                   │                   │
       ▼                   ▼                   ▼
┌─────────────┐     ┌─────────────┐     ┌─────────────┐
│  Worker 1   │     │  Worker 2   │     │  Worker 3   │
└─────────────┘     └─────────────┘     └─────────────┘
       │                   │                   │
       └───────────────────┴───────────────────┘
                           │
                           ▼
                    ┌─────────────┐
                    │    Lock     │  ← Prevents duplicate processing
                    └─────────────┘
```

## Queue Configuration

### Memory Queue (Development)

```go
import "github.com/felixgeelhaar/agent-go/contrib/distributed"

q := distributed.NewMemoryQueue()
```

### Redis Queue (Production)

```go
import "github.com/felixgeelhaar/agent-go/contrib/distributed/redis"

q, _ := redis.NewQueue(
    redis.WithAddress("localhost:6379"),
    redis.WithPassword("secret"),
    redis.WithDB(0),
)
```

### NATS Queue (High Throughput)

```go
import "github.com/felixgeelhaar/agent-go/contrib/distributed/nats"

q, _ := nats.NewQueue(
    nats.WithURL("nats://localhost:4222"),
    nats.WithStream("agent-tasks"),
)
```

## Lock Configuration

### Memory Lock (Development)

```go
import "github.com/felixgeelhaar/agent-go/contrib/distributed"

l := distributed.NewMemoryLock()
```

### Redis Lock (Production)

```go
import "github.com/felixgeelhaar/agent-go/contrib/distributed/redis"

l, _ := redis.NewLock(
    redis.WithAddress("localhost:6379"),
)
```

## Worker Configuration

```go
worker := distributed.NewWorker(distributed.WorkerConfig{
    ID:       "worker-1",
    Queue:    taskQueue,
    Lock:     distLock,
    Registry: toolRegistry,
},
    distributed.WithConcurrency(4),           // Parallel task processing
    distributed.WithPollInterval(100*time.Millisecond),
    distributed.WithTaskTimeout(30*time.Second),
    distributed.WithLockTTL(60*time.Second),
)
```

## Task Priority

Tasks can have different priorities:

```go
task.Priority = 10  // Higher priority = processed first

// Or use EnqueueWithPriority
queue.EnqueueWithPriority(ctx, task, 100)
```

## Custom Task Handlers

Register handlers for different task types:

```go
worker.RegisterHandler(queue.TaskTypeToolCall, func(ctx context.Context, task queue.Task) (json.RawMessage, error) {
    // Process tool call task
    return result, nil
})

worker.RegisterHandler(queue.TaskTypePlanning, func(ctx context.Context, task queue.Task) (json.RawMessage, error) {
    // Process planning task
    return result, nil
})
```

## Graceful Shutdown

```go
// Stop accepting new tasks and wait for current tasks to complete
worker.Stop()

// Or with timeout
ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
defer cancel()
worker.StopWithContext(ctx)
```

## Next Steps

- **[07-production](../07-production/)** - Full production setup with all components
