// Package messaging provides message queue tools for agent-go.
//
// Tools operate through a Broker backend. MemoryBroker is included for tests
// and local use; RabbitMQ/Kafka/NATS adapters can implement Broker.
//
// mq_subscribe is a pull-style consume (next N messages), not a long-lived
// streaming subscription — better suited to agent tool calls.
package messaging

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"

	"go.klarlabs.de/agent/domain/agent"
	"go.klarlabs.de/agent/domain/pack"
	"go.klarlabs.de/agent/domain/tool"
)

// ErrQueueNotFound indicates the queue does not exist.
var ErrQueueNotFound = errors.New("queue not found")

// ErrMessageNotFound indicates an ack/nack target was not found.
var ErrMessageNotFound = errors.New("message not found")

// Message is a queue message.
type Message struct {
	ID        string            `json:"id"`
	Queue     string            `json:"queue"`
	Body      string            `json:"body"`
	Headers   map[string]string `json:"headers,omitempty"`
	Timestamp time.Time         `json:"timestamp"`
}

// QueueInfo describes a queue.
type QueueInfo struct {
	Name     string `json:"name"`
	Depth    int    `json:"depth"`
	InFlight int    `json:"in_flight"`
}

// Broker is the messaging backend.
type Broker interface {
	Publish(ctx context.Context, queue, body string, headers map[string]string) (Message, error)
	Consume(ctx context.Context, queue string, max int) ([]Message, error)
	Ack(ctx context.Context, queue, messageID string) error
	Nack(ctx context.Context, queue, messageID string, requeue bool) error
	ListQueues(ctx context.Context) ([]QueueInfo, error)
	QueueInfo(ctx context.Context, queue string) (QueueInfo, error)
	Purge(ctx context.Context, queue string) (int, error)
}

// Pack returns messaging tools backed by broker.
func Pack(broker Broker) *pack.Pack {
	if broker == nil {
		panic("messaging.Pack: broker is required")
	}
	p := &mqPack{broker: broker}
	return pack.NewBuilder("messaging").
		WithDescription("Message queue tools for pub/sub and queue operations").
		WithVersion("0.1.0").
		AddTools(
			p.mqPublish(),
			p.mqSubscribe(),
			p.mqAck(),
			p.mqNack(),
			p.mqListQueues(),
			p.mqQueueInfo(),
			p.mqPurge(),
		).
		AllowInState(agent.StateExplore, "mq_list_queues", "mq_queue_info").
		AllowInState(agent.StateAct, "mq_publish", "mq_subscribe", "mq_ack", "mq_nack", "mq_list_queues", "mq_queue_info", "mq_purge").
		Build()
}

type mqPack struct {
	broker Broker
}

func resultOK(v any) (tool.Result, error) {
	out, err := json.Marshal(v)
	if err != nil {
		return tool.Result{}, err
	}
	return tool.Result{Output: out}, nil
}

func decode[T any](raw json.RawMessage) (T, error) {
	var v T
	if err := json.Unmarshal(raw, &v); err != nil {
		return v, fmt.Errorf("%w: %v", tool.ErrInvalidInput, err)
	}
	return v, nil
}

func (p *mqPack) mqPublish() tool.Tool {
	return tool.NewBuilder("mq_publish").
		WithDescription("Publish a message to a topic or queue").
		WithRiskLevel(tool.RiskMedium).
		WithInputSchema(tool.ObjectSchema(map[string]json.RawMessage{
			"queue":   json.RawMessage(`{"type":"string"}`),
			"body":    json.RawMessage(`{"type":"string"}`),
			"headers": json.RawMessage(`{"type":"object"}`),
		}, []string{"queue", "body"})).
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			in, err := decode[struct {
				Queue   string            `json:"queue"`
				Body    string            `json:"body"`
				Headers map[string]string `json:"headers"`
			}](input)
			if err != nil {
				return tool.Result{}, err
			}
			if in.Queue == "" {
				return tool.Result{}, fmt.Errorf("%w: queue is required", tool.ErrInvalidInput)
			}
			msg, err := p.broker.Publish(ctx, in.Queue, in.Body, in.Headers)
			if err != nil {
				return tool.Result{}, err
			}
			return resultOK(map[string]any{"message": msg, "published": true})
		}).
		MustBuild()
}

func (p *mqPack) mqSubscribe() tool.Tool {
	return tool.NewBuilder("mq_subscribe").
		WithDescription("Pull up to N messages from a queue (consume)").
		WithRiskLevel(tool.RiskLow).
		WithInputSchema(tool.ObjectSchema(map[string]json.RawMessage{
			"queue": json.RawMessage(`{"type":"string"}`),
			"max":   json.RawMessage(`{"type":"integer"}`),
		}, []string{"queue"})).
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			in, err := decode[struct {
				Queue string `json:"queue"`
				Max   int    `json:"max"`
			}](input)
			if err != nil {
				return tool.Result{}, err
			}
			if in.Max <= 0 {
				in.Max = 1
			}
			msgs, err := p.broker.Consume(ctx, in.Queue, in.Max)
			if err != nil {
				return tool.Result{}, err
			}
			return resultOK(map[string]any{"messages": msgs, "count": len(msgs)})
		}).
		MustBuild()
}

func (p *mqPack) mqAck() tool.Tool {
	return tool.NewBuilder("mq_ack").
		WithDescription("Acknowledge message processing").
		WithRiskLevel(tool.RiskLow).
		WithInputSchema(tool.ObjectSchema(map[string]json.RawMessage{
			"queue":      json.RawMessage(`{"type":"string"}`),
			"message_id": json.RawMessage(`{"type":"string"}`),
		}, []string{"queue", "message_id"})).
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			in, err := decode[struct {
				Queue     string `json:"queue"`
				MessageID string `json:"message_id"`
			}](input)
			if err != nil {
				return tool.Result{}, err
			}
			if err := p.broker.Ack(ctx, in.Queue, in.MessageID); err != nil {
				return tool.Result{}, err
			}
			return resultOK(map[string]any{"acked": true, "message_id": in.MessageID})
		}).
		MustBuild()
}

func (p *mqPack) mqNack() tool.Tool {
	return tool.NewBuilder("mq_nack").
		WithDescription("Negative acknowledge a message (optionally requeue)").
		WithRiskLevel(tool.RiskLow).
		WithInputSchema(tool.ObjectSchema(map[string]json.RawMessage{
			"queue":      json.RawMessage(`{"type":"string"}`),
			"message_id": json.RawMessage(`{"type":"string"}`),
			"requeue":    json.RawMessage(`{"type":"boolean"}`),
		}, []string{"queue", "message_id"})).
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			in, err := decode[struct {
				Queue     string `json:"queue"`
				MessageID string `json:"message_id"`
				Requeue   bool   `json:"requeue"`
			}](input)
			if err != nil {
				return tool.Result{}, err
			}
			if err := p.broker.Nack(ctx, in.Queue, in.MessageID, in.Requeue); err != nil {
				return tool.Result{}, err
			}
			return resultOK(map[string]any{"nacked": true, "message_id": in.MessageID, "requeued": in.Requeue})
		}).
		MustBuild()
}

func (p *mqPack) mqListQueues() tool.Tool {
	return tool.NewBuilder("mq_list_queues").
		WithDescription("List available queues").
		ReadOnly().
		Cacheable().
		WithHandler(func(ctx context.Context, _ json.RawMessage) (tool.Result, error) {
			qs, err := p.broker.ListQueues(ctx)
			if err != nil {
				return tool.Result{}, err
			}
			return resultOK(map[string]any{"queues": qs, "count": len(qs)})
		}).
		MustBuild()
}

func (p *mqPack) mqQueueInfo() tool.Tool {
	return tool.NewBuilder("mq_queue_info").
		WithDescription("Get queue statistics and metadata").
		ReadOnly().
		Cacheable().
		WithInputSchema(tool.ObjectSchema(map[string]json.RawMessage{
			"queue": json.RawMessage(`{"type":"string"}`),
		}, []string{"queue"})).
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			in, err := decode[struct {
				Queue string `json:"queue"`
			}](input)
			if err != nil {
				return tool.Result{}, err
			}
			info, err := p.broker.QueueInfo(ctx, in.Queue)
			if err != nil {
				return tool.Result{}, err
			}
			return resultOK(info)
		}).
		MustBuild()
}

func (p *mqPack) mqPurge() tool.Tool {
	return tool.NewBuilder("mq_purge").
		WithDescription("Purge all messages from a queue").
		Destructive().
		WithInputSchema(tool.ObjectSchema(map[string]json.RawMessage{
			"queue":   json.RawMessage(`{"type":"string"}`),
			"confirm": json.RawMessage(`{"type":"boolean"}`),
		}, []string{"queue", "confirm"})).
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			in, err := decode[struct {
				Queue   string `json:"queue"`
				Confirm bool   `json:"confirm"`
			}](input)
			if err != nil {
				return tool.Result{}, err
			}
			if !in.Confirm {
				return tool.Result{}, fmt.Errorf("%w: confirm must be true to purge", tool.ErrInvalidInput)
			}
			n, err := p.broker.Purge(ctx, in.Queue)
			if err != nil {
				return tool.Result{}, err
			}
			return resultOK(map[string]any{"queue": in.Queue, "purged": n})
		}).
		MustBuild()
}

// ---------------------------------------------------------------------------
// MemoryBroker
// ---------------------------------------------------------------------------

type memQueue struct {
	pending  []Message
	inflight map[string]Message
}

// MemoryBroker is an in-process Broker for tests and local use.
type MemoryBroker struct {
	mu     sync.Mutex
	queues map[string]*memQueue
}

// NewMemoryBroker creates an empty in-memory broker.
func NewMemoryBroker() *MemoryBroker {
	return &MemoryBroker{queues: make(map[string]*memQueue)}
}

func (b *MemoryBroker) ensure(name string) *memQueue {
	q, ok := b.queues[name]
	if !ok {
		q = &memQueue{inflight: make(map[string]Message)}
		b.queues[name] = q
	}
	return q
}

func (b *MemoryBroker) Publish(_ context.Context, queue, body string, headers map[string]string) (Message, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	q := b.ensure(queue)
	msg := Message{
		ID:        newMessageID(),
		Queue:     queue,
		Body:      body,
		Headers:   headers,
		Timestamp: time.Now().UTC(),
	}
	q.pending = append(q.pending, msg)
	return msg, nil
}

func (b *MemoryBroker) Consume(_ context.Context, queue string, max int) ([]Message, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	q := b.ensure(queue)
	if max <= 0 {
		max = 1
	}
	n := max
	if n > len(q.pending) {
		n = len(q.pending)
	}
	out := make([]Message, n)
	copy(out, q.pending[:n])
	q.pending = q.pending[n:]
	for _, m := range out {
		q.inflight[m.ID] = m
	}
	return out, nil
}

func (b *MemoryBroker) Ack(_ context.Context, queue, messageID string) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	q, ok := b.queues[queue]
	if !ok {
		return ErrQueueNotFound
	}
	if _, ok := q.inflight[messageID]; !ok {
		return ErrMessageNotFound
	}
	delete(q.inflight, messageID)
	return nil
}

func (b *MemoryBroker) Nack(_ context.Context, queue, messageID string, requeue bool) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	q, ok := b.queues[queue]
	if !ok {
		return ErrQueueNotFound
	}
	msg, ok := q.inflight[messageID]
	if !ok {
		return ErrMessageNotFound
	}
	delete(q.inflight, messageID)
	if requeue {
		q.pending = append([]Message{msg}, q.pending...)
	}
	return nil
}

func (b *MemoryBroker) ListQueues(_ context.Context) ([]QueueInfo, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	out := make([]QueueInfo, 0, len(b.queues))
	for name, q := range b.queues {
		out = append(out, QueueInfo{Name: name, Depth: len(q.pending), InFlight: len(q.inflight)})
	}
	return out, nil
}

func (b *MemoryBroker) QueueInfo(_ context.Context, queue string) (QueueInfo, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	q, ok := b.queues[queue]
	if !ok {
		return QueueInfo{}, ErrQueueNotFound
	}
	return QueueInfo{Name: queue, Depth: len(q.pending), InFlight: len(q.inflight)}, nil
}

func (b *MemoryBroker) Purge(_ context.Context, queue string) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	q, ok := b.queues[queue]
	if !ok {
		return 0, ErrQueueNotFound
	}
	n := len(q.pending)
	q.pending = nil
	return n, nil
}

func newMessageID() string {
	buf := make([]byte, 16)
	_, _ = rand.Read(buf)
	return hex.EncodeToString(buf)
}
