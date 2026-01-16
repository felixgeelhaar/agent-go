package queue

import (
	"context"
	"encoding/json"
	"testing"
	"time"
)

func TestNewTask(t *testing.T) {
	payload := json.RawMessage(`{"key": "value"}`)
	task := NewTask(TaskTypeToolCall, payload)

	if task.ID == "" {
		t.Error("expected task ID to be set")
	}
	if task.Type != TaskTypeToolCall {
		t.Errorf("expected type %v, got %v", TaskTypeToolCall, task.Type)
	}
	if task.CreatedAt.IsZero() {
		t.Error("expected created_at to be set")
	}
	if task.Metadata == nil {
		t.Error("expected metadata to be initialized")
	}
}

func TestNewToolCallTask(t *testing.T) {
	input := json.RawMessage(`{"filename": "test.txt"}`)
	task, err := NewToolCallTask("run-123", "read_file", input, "explore")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if task.RunID != "run-123" {
		t.Errorf("expected run ID run-123, got %s", task.RunID)
	}
	if task.Type != TaskTypeToolCall {
		t.Errorf("expected type %v, got %v", TaskTypeToolCall, task.Type)
	}

	var payload ToolCallPayload
	if err := json.Unmarshal(task.Payload, &payload); err != nil {
		t.Fatalf("failed to unmarshal payload: %v", err)
	}

	if payload.ToolName != "read_file" {
		t.Errorf("expected tool name read_file, got %s", payload.ToolName)
	}
	if payload.State != "explore" {
		t.Errorf("expected state explore, got %s", payload.State)
	}
}

func TestNewPlanningTask(t *testing.T) {
	evidence := json.RawMessage(`{"files": ["a.txt"]}`)
	task, err := NewPlanningTask("run-456", "decide", evidence, []string{"tool1", "tool2"}, map[string]int{"calls": 10})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if task.RunID != "run-456" {
		t.Errorf("expected run ID run-456, got %s", task.RunID)
	}
	if task.Type != TaskTypePlanning {
		t.Errorf("expected type %v, got %v", TaskTypePlanning, task.Type)
	}

	var payload PlanningPayload
	if err := json.Unmarshal(task.Payload, &payload); err != nil {
		t.Fatalf("failed to unmarshal payload: %v", err)
	}

	if payload.State != "decide" {
		t.Errorf("expected state decide, got %s", payload.State)
	}
	if len(payload.AllowedTools) != 2 {
		t.Errorf("expected 2 allowed tools, got %d", len(payload.AllowedTools))
	}
	if payload.Budgets["calls"] != 10 {
		t.Errorf("expected budget calls=10, got %d", payload.Budgets["calls"])
	}
}

func TestMemoryQueueEnqueueDequeue(t *testing.T) {
	ctx := context.Background()
	q := NewMemoryQueue()

	task := NewTask(TaskTypeToolCall, json.RawMessage(`{}`))
	task.Priority = 1

	err := q.Enqueue(ctx, task)
	if err != nil {
		t.Fatalf("enqueue failed: %v", err)
	}

	size, err := q.Size(ctx)
	if err != nil {
		t.Fatalf("size failed: %v", err)
	}
	if size != 1 {
		t.Errorf("expected size 1, got %d", size)
	}

	dequeued, err := q.Dequeue(ctx)
	if err != nil {
		t.Fatalf("dequeue failed: %v", err)
	}

	if dequeued.ID != task.ID {
		t.Errorf("expected task ID %s, got %s", task.ID, dequeued.ID)
	}
}

func TestMemoryQueuePriority(t *testing.T) {
	ctx := context.Background()
	q := NewMemoryQueue()

	low := NewTask(TaskTypeToolCall, json.RawMessage(`{"name": "low"}`))
	low.Priority = 1

	high := NewTask(TaskTypeToolCall, json.RawMessage(`{"name": "high"}`))
	high.Priority = 10

	// Enqueue low priority first
	q.Enqueue(ctx, low)
	q.Enqueue(ctx, high)

	// Should get high priority first
	first, _ := q.Dequeue(ctx)
	if first.Priority != 10 {
		t.Errorf("expected high priority task first, got priority %d", first.Priority)
	}

	second, _ := q.Dequeue(ctx)
	if second.Priority != 1 {
		t.Errorf("expected low priority task second, got priority %d", second.Priority)
	}
}

func TestMemoryQueueAcknowledge(t *testing.T) {
	ctx := context.Background()
	q := NewMemoryQueue()

	task := NewTask(TaskTypeToolCall, json.RawMessage(`{}`))
	q.Enqueue(ctx, task)

	dequeued, _ := q.Dequeue(ctx)

	// Task should be in processing
	if q.ProcessingCount() != 1 {
		t.Errorf("expected 1 processing, got %d", q.ProcessingCount())
	}

	result := TaskResult{
		TaskID:      dequeued.ID,
		Status:      TaskStatusCompleted,
		CompletedAt: time.Now(),
	}
	err := q.Acknowledge(ctx, dequeued.ID, result)
	if err != nil {
		t.Fatalf("acknowledge failed: %v", err)
	}

	// Task should be removed from processing
	if q.ProcessingCount() != 0 {
		t.Errorf("expected 0 processing, got %d", q.ProcessingCount())
	}

	// Result should be stored
	storedResult, exists := q.GetResult(dequeued.ID)
	if !exists {
		t.Error("expected result to be stored")
	}
	if storedResult.Status != TaskStatusCompleted {
		t.Errorf("expected status completed, got %v", storedResult.Status)
	}
}

func TestMemoryQueueReject(t *testing.T) {
	ctx := context.Background()
	q := NewMemoryQueue()

	task := NewTask(TaskTypeToolCall, json.RawMessage(`{}`))
	q.Enqueue(ctx, task)

	dequeued, _ := q.Dequeue(ctx)

	// Reject without requeue
	err := q.Reject(ctx, dequeued.ID, "test failure", false)
	if err != nil {
		t.Fatalf("reject failed: %v", err)
	}

	// Result should show failure
	result, exists := q.GetResult(dequeued.ID)
	if !exists {
		t.Error("expected result to be stored")
	}
	if result.Status != TaskStatusFailed {
		t.Errorf("expected status failed, got %v", result.Status)
	}
	if result.Error != "test failure" {
		t.Errorf("expected error 'test failure', got '%s'", result.Error)
	}
}

func TestMemoryQueueRejectRequeue(t *testing.T) {
	ctx := context.Background()
	q := NewMemoryQueue()

	task := NewTask(TaskTypeToolCall, json.RawMessage(`{}`))
	q.Enqueue(ctx, task)

	dequeued, _ := q.Dequeue(ctx)

	// Reject with requeue
	err := q.Reject(ctx, dequeued.ID, "retry needed", true)
	if err != nil {
		t.Fatalf("reject failed: %v", err)
	}

	// Task should be back in queue
	size, _ := q.Size(ctx)
	if size != 1 {
		t.Errorf("expected size 1 after requeue, got %d", size)
	}
}

func TestMemoryQueuePeek(t *testing.T) {
	ctx := context.Background()
	q := NewMemoryQueue()

	task := NewTask(TaskTypeToolCall, json.RawMessage(`{}`))
	q.Enqueue(ctx, task)

	peeked, err := q.Peek(ctx)
	if err != nil {
		t.Fatalf("peek failed: %v", err)
	}

	if peeked.ID != task.ID {
		t.Errorf("expected task ID %s, got %s", task.ID, peeked.ID)
	}

	// Size should not change
	size, _ := q.Size(ctx)
	if size != 1 {
		t.Errorf("expected size 1 after peek, got %d", size)
	}
}

func TestMemoryQueueClose(t *testing.T) {
	ctx := context.Background()
	q := NewMemoryQueue()

	err := q.Close()
	if err != nil {
		t.Fatalf("close failed: %v", err)
	}

	// Enqueue should fail
	task := NewTask(TaskTypeToolCall, json.RawMessage(`{}`))
	err = q.Enqueue(ctx, task)
	if err != ErrQueueClosed {
		t.Errorf("expected ErrQueueClosed, got %v", err)
	}

	// Size should return error
	_, err = q.Size(ctx)
	if err != ErrQueueClosed {
		t.Errorf("expected ErrQueueClosed, got %v", err)
	}
}

func TestMemoryQueueDequeueBlocking(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	q := NewMemoryQueue()

	// Dequeue on empty queue should block until context cancellation
	_, err := q.Dequeue(ctx)
	if err != context.DeadlineExceeded {
		t.Errorf("expected context.DeadlineExceeded, got %v", err)
	}
}

func TestMemoryQueueDequeueUnblocks(t *testing.T) {
	ctx := context.Background()
	q := NewMemoryQueue()

	done := make(chan struct{})
	var dequeued *Task

	go func() {
		dequeued, _ = q.Dequeue(ctx)
		close(done)
	}()

	// Give goroutine time to block
	time.Sleep(10 * time.Millisecond)

	// Enqueue task
	task := NewTask(TaskTypeToolCall, json.RawMessage(`{}`))
	q.Enqueue(ctx, task)

	// Wait for dequeue to complete
	select {
	case <-done:
		if dequeued.ID != task.ID {
			t.Errorf("expected task ID %s, got %s", task.ID, dequeued.ID)
		}
	case <-time.After(100 * time.Millisecond):
		t.Error("dequeue did not unblock")
	}
}

func TestMemoryQueuePeekEmpty(t *testing.T) {
	ctx := context.Background()
	q := NewMemoryQueue()

	_, err := q.Peek(ctx)
	if err != ErrQueueEmpty {
		t.Errorf("expected ErrQueueEmpty, got %v", err)
	}
}

func TestMemoryQueueAcknowledgeNotFound(t *testing.T) {
	ctx := context.Background()
	q := NewMemoryQueue()

	result := TaskResult{TaskID: "nonexistent"}
	err := q.Acknowledge(ctx, "nonexistent", result)
	if err != ErrTaskNotFound {
		t.Errorf("expected ErrTaskNotFound, got %v", err)
	}
}

func TestMemoryQueueRejectNotFound(t *testing.T) {
	ctx := context.Background()
	q := NewMemoryQueue()

	err := q.Reject(ctx, "nonexistent", "reason", false)
	if err != ErrTaskNotFound {
		t.Errorf("expected ErrTaskNotFound, got %v", err)
	}
}

func TestMemoryQueueEnqueueWithPriority(t *testing.T) {
	ctx := context.Background()
	q := NewMemoryQueue()

	task := NewTask(TaskTypeToolCall, json.RawMessage(`{}`))
	err := q.EnqueueWithPriority(ctx, task, 100)
	if err != nil {
		t.Fatalf("enqueue with priority failed: %v", err)
	}

	dequeued, _ := q.Dequeue(ctx)
	if dequeued.Priority != 100 {
		t.Errorf("expected priority 100, got %d", dequeued.Priority)
	}
}

func TestTaskTypes(t *testing.T) {
	types := []TaskType{
		TaskTypeToolCall,
		TaskTypePlanning,
		TaskTypeValidation,
		TaskTypeCustom,
	}

	for _, tt := range types {
		if string(tt) == "" {
			t.Error("task type should not be empty")
		}
	}
}

func TestTaskStatuses(t *testing.T) {
	statuses := []TaskStatus{
		TaskStatusPending,
		TaskStatusProcessing,
		TaskStatusCompleted,
		TaskStatusFailed,
		TaskStatusRetrying,
	}

	for _, s := range statuses {
		if string(s) == "" {
			t.Error("task status should not be empty")
		}
	}
}
