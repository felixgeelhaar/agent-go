package queue

import (
	"container/heap"
	"context"
	"sync"
	"time"
)

// MemoryQueue implements Queue using in-memory storage.
// Useful for testing and single-node deployments.
type MemoryQueue struct {
	mu         sync.Mutex
	cond       *sync.Cond
	tasks      priorityHeap
	processing map[string]*Task
	results    map[string]TaskResult
	closed     bool
}

// MemoryQueueOption configures the memory queue.
type MemoryQueueOption func(*MemoryQueue)

// NewMemoryQueue creates a new in-memory queue.
func NewMemoryQueue(opts ...MemoryQueueOption) *MemoryQueue {
	q := &MemoryQueue{
		tasks:      make(priorityHeap, 0),
		processing: make(map[string]*Task),
		results:    make(map[string]TaskResult),
	}
	q.cond = sync.NewCond(&q.mu)
	heap.Init(&q.tasks)

	for _, opt := range opts {
		opt(q)
	}
	return q
}

// Enqueue adds a task to the queue.
func (q *MemoryQueue) Enqueue(ctx context.Context, task Task) error {
	q.mu.Lock()
	defer q.mu.Unlock()

	if q.closed {
		return ErrQueueClosed
	}

	item := &taskItem{
		task:     task,
		priority: task.Priority,
	}
	heap.Push(&q.tasks, item)
	q.cond.Signal()
	return nil
}

// EnqueueWithPriority adds a task with explicit priority.
func (q *MemoryQueue) EnqueueWithPriority(ctx context.Context, task Task, priority int) error {
	task.Priority = priority
	return q.Enqueue(ctx, task)
}

// Dequeue retrieves the next task from the queue.
func (q *MemoryQueue) Dequeue(ctx context.Context) (*Task, error) {
	q.mu.Lock()
	defer q.mu.Unlock()

	// Wait for a task or context cancellation
	for q.tasks.Len() == 0 && !q.closed {
		// Release lock and wait
		done := make(chan struct{})
		go func() {
			select {
			case <-ctx.Done():
				q.mu.Lock()
				q.cond.Broadcast()
				q.mu.Unlock()
			case <-done:
			}
		}()
		q.cond.Wait()
		close(done)

		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
	}

	if q.closed && q.tasks.Len() == 0 {
		return nil, ErrQueueClosed
	}

	if q.tasks.Len() == 0 {
		return nil, ErrQueueEmpty
	}

	item := heap.Pop(&q.tasks).(*taskItem)
	task := item.task
	q.processing[task.ID] = &task
	return &task, nil
}

// Acknowledge marks a task as successfully completed.
func (q *MemoryQueue) Acknowledge(ctx context.Context, taskID string, result TaskResult) error {
	q.mu.Lock()
	defer q.mu.Unlock()

	if _, exists := q.processing[taskID]; !exists {
		return ErrTaskNotFound
	}

	delete(q.processing, taskID)
	q.results[taskID] = result
	return nil
}

// Reject marks a task as failed, optionally requeueing it.
func (q *MemoryQueue) Reject(ctx context.Context, taskID string, reason string, requeue bool) error {
	q.mu.Lock()
	defer q.mu.Unlock()

	task, exists := q.processing[taskID]
	if !exists {
		return ErrTaskNotFound
	}

	delete(q.processing, taskID)

	if requeue {
		item := &taskItem{
			task:     *task,
			priority: task.Priority,
		}
		heap.Push(&q.tasks, item)
		q.cond.Signal()
	} else {
		q.results[taskID] = TaskResult{
			TaskID:      taskID,
			Status:      TaskStatusFailed,
			Error:       reason,
			CompletedAt: time.Now(),
		}
	}
	return nil
}

// Peek returns the next task without removing it from the queue.
func (q *MemoryQueue) Peek(ctx context.Context) (*Task, error) {
	q.mu.Lock()
	defer q.mu.Unlock()

	if q.closed {
		return nil, ErrQueueClosed
	}

	if q.tasks.Len() == 0 {
		return nil, ErrQueueEmpty
	}

	task := q.tasks[0].task
	return &task, nil
}

// Size returns the current number of tasks in the queue.
func (q *MemoryQueue) Size(ctx context.Context) (int, error) {
	q.mu.Lock()
	defer q.mu.Unlock()

	if q.closed {
		return 0, ErrQueueClosed
	}

	return q.tasks.Len(), nil
}

// Close releases queue resources.
func (q *MemoryQueue) Close() error {
	q.mu.Lock()
	defer q.mu.Unlock()

	q.closed = true
	q.cond.Broadcast()
	return nil
}

// ProcessingCount returns the number of tasks being processed.
func (q *MemoryQueue) ProcessingCount() int {
	q.mu.Lock()
	defer q.mu.Unlock()
	return len(q.processing)
}

// GetResult retrieves the result for a completed task.
func (q *MemoryQueue) GetResult(taskID string) (TaskResult, bool) {
	q.mu.Lock()
	defer q.mu.Unlock()
	result, exists := q.results[taskID]
	return result, exists
}

// Priority heap implementation for task ordering.
type taskItem struct {
	task     Task
	priority int
	index    int
}

type priorityHeap []*taskItem

func (h priorityHeap) Len() int { return len(h) }

func (h priorityHeap) Less(i, j int) bool {
	// Higher priority first, then earlier creation time
	if h[i].priority != h[j].priority {
		return h[i].priority > h[j].priority
	}
	return h[i].task.CreatedAt.Before(h[j].task.CreatedAt)
}

func (h priorityHeap) Swap(i, j int) {
	h[i], h[j] = h[j], h[i]
	h[i].index = i
	h[j].index = j
}

func (h *priorityHeap) Push(x any) {
	n := len(*h)
	item := x.(*taskItem)
	item.index = n
	*h = append(*h, item)
}

func (h *priorityHeap) Pop() any {
	old := *h
	n := len(old)
	item := old[n-1]
	old[n-1] = nil
	item.index = -1
	*h = old[0 : n-1]
	return item
}
