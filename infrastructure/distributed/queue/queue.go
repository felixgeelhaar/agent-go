// Package queue provides task queue abstractions for distributed execution.
package queue

import (
	"context"
	"encoding/json"
	"errors"
	"time"
)

// Task represents a unit of work to be executed.
type Task struct {
	ID        string          `json:"id"`
	RunID     string          `json:"run_id"`
	Type      TaskType        `json:"type"`
	Payload   json.RawMessage `json:"payload"`
	Priority  int             `json:"priority"`
	CreatedAt time.Time       `json:"created_at"`
	Metadata  map[string]any  `json:"metadata,omitempty"`
}

// TaskType categorizes tasks.
type TaskType string

const (
	TaskTypeToolCall   TaskType = "tool_call"
	TaskTypePlanning   TaskType = "planning"
	TaskTypeValidation TaskType = "validation"
	TaskTypeCustom     TaskType = "custom"
)

// TaskStatus represents the current state of a task.
type TaskStatus string

const (
	TaskStatusPending    TaskStatus = "pending"
	TaskStatusProcessing TaskStatus = "processing"
	TaskStatusCompleted  TaskStatus = "completed"
	TaskStatusFailed     TaskStatus = "failed"
	TaskStatusRetrying   TaskStatus = "retrying"
)

// TaskResult holds the result of task execution.
type TaskResult struct {
	TaskID      string          `json:"task_id"`
	Status      TaskStatus      `json:"status"`
	Result      json.RawMessage `json:"result,omitempty"`
	Error       string          `json:"error,omitempty"`
	CompletedAt time.Time       `json:"completed_at"`
	Duration    time.Duration   `json:"duration_ns"`
	WorkerID    string          `json:"worker_id"`
}

// Queue defines the interface for task queues.
type Queue interface {
	// Enqueue adds a task to the queue.
	Enqueue(ctx context.Context, task Task) error

	// Dequeue retrieves the next task from the queue.
	// Blocks until a task is available or context is cancelled.
	Dequeue(ctx context.Context) (*Task, error)

	// Acknowledge marks a task as successfully completed.
	Acknowledge(ctx context.Context, taskID string, result TaskResult) error

	// Reject marks a task as failed, optionally requeueing it.
	Reject(ctx context.Context, taskID string, reason string, requeue bool) error

	// Peek returns the next task without removing it from the queue.
	Peek(ctx context.Context) (*Task, error)

	// Size returns the current number of tasks in the queue.
	Size(ctx context.Context) (int, error)

	// Close releases queue resources.
	Close() error
}

// PriorityQueue extends Queue with priority support.
type PriorityQueue interface {
	Queue

	// EnqueueWithPriority adds a task with explicit priority.
	EnqueueWithPriority(ctx context.Context, task Task, priority int) error
}

// DelayedQueue extends Queue with delayed execution support.
type DelayedQueue interface {
	Queue

	// EnqueueDelayed adds a task to be executed after a delay.
	EnqueueDelayed(ctx context.Context, task Task, delay time.Duration) error
}

// Common errors.
var (
	ErrQueueEmpty     = errors.New("queue is empty")
	ErrQueueClosed    = errors.New("queue is closed")
	ErrTaskNotFound   = errors.New("task not found")
	ErrTaskInProgress = errors.New("task is being processed")
)

// ToolCallPayload is the payload for tool execution tasks.
type ToolCallPayload struct {
	ToolName string          `json:"tool_name"`
	Input    json.RawMessage `json:"input"`
	State    string          `json:"state"`
	Timeout  time.Duration   `json:"timeout_ns,omitempty"`
}

// PlanningPayload is the payload for planning tasks.
type PlanningPayload struct {
	State        string          `json:"state"`
	Evidence     json.RawMessage `json:"evidence"`
	AllowedTools []string        `json:"allowed_tools"`
	Budgets      map[string]int  `json:"budgets"`
}

// NewTask creates a new task with a generated ID.
func NewTask(taskType TaskType, payload json.RawMessage) Task {
	return Task{
		ID:        generateID(),
		Type:      taskType,
		Payload:   payload,
		CreatedAt: time.Now(),
		Metadata:  make(map[string]any),
	}
}

// NewToolCallTask creates a task for tool execution.
func NewToolCallTask(runID, toolName string, input json.RawMessage, state string) (Task, error) {
	payload := ToolCallPayload{
		ToolName: toolName,
		Input:    input,
		State:    state,
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return Task{}, err
	}
	task := NewTask(TaskTypeToolCall, data)
	task.RunID = runID
	return task, nil
}

// NewPlanningTask creates a task for planning decisions.
func NewPlanningTask(runID, state string, evidence json.RawMessage, allowedTools []string, budgets map[string]int) (Task, error) {
	payload := PlanningPayload{
		State:        state,
		Evidence:     evidence,
		AllowedTools: allowedTools,
		Budgets:      budgets,
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return Task{}, err
	}
	task := NewTask(TaskTypePlanning, data)
	task.RunID = runID
	return task, nil
}

// generateID creates a simple unique ID.
func generateID() string {
	return time.Now().Format("20060102150405.000000000")
}
