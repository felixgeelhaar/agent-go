package messaging_test

import (
	"context"
	"encoding/json"
	"testing"

	messaging "go.klarlabs.de/agent/contrib/pack-messaging"
	"go.klarlabs.de/agent/domain/tool"
)

func toolMap(b messaging.Broker) map[string]tool.Tool {
	p := messaging.Pack(b)
	m := make(map[string]tool.Tool, len(p.Tools))
	for _, tt := range p.Tools {
		m[tt.Name()] = tt
	}
	return m
}

func TestPackNotStub(t *testing.T) {
	p := messaging.Pack(messaging.NewMemoryBroker())
	if p.Metadata["status"] == "stub" {
		t.Fatal("should not be stub")
	}
	if len(p.Tools) != 7 {
		t.Fatalf("expected 7 tools, got %d", len(p.Tools))
	}
}

func TestPublishConsumeAck(t *testing.T) {
	broker := messaging.NewMemoryBroker()
	tools := toolMap(broker)
	ctx := context.Background()

	res, err := tools["mq_publish"].Execute(ctx, json.RawMessage(`{"queue":"jobs","body":"hello"}`))
	if err != nil {
		t.Fatal(err)
	}
	var pub struct {
		Message messaging.Message `json:"message"`
	}
	if err := json.Unmarshal(res.Output, &pub); err != nil {
		t.Fatal(err)
	}
	if pub.Message.ID == "" {
		t.Fatal("expected message id")
	}

	res, err = tools["mq_subscribe"].Execute(ctx, json.RawMessage(`{"queue":"jobs","max":5}`))
	if err != nil {
		t.Fatal(err)
	}
	var sub struct {
		Count int `json:"count"`
		Messages []messaging.Message `json:"messages"`
	}
	if err := json.Unmarshal(res.Output, &sub); err != nil {
		t.Fatal(err)
	}
	if sub.Count != 1 || sub.Messages[0].Body != "hello" {
		t.Fatalf("unexpected subscribe: %+v", sub)
	}

	_, err = tools["mq_ack"].Execute(ctx, json.RawMessage(
		`{"queue":"jobs","message_id":"`+sub.Messages[0].ID+`"}`))
	if err != nil {
		t.Fatal(err)
	}

	infoRes, err := tools["mq_queue_info"].Execute(ctx, json.RawMessage(`{"queue":"jobs"}`))
	if err != nil {
		t.Fatal(err)
	}
	var info messaging.QueueInfo
	_ = json.Unmarshal(infoRes.Output, &info)
	if info.Depth != 0 || info.InFlight != 0 {
		t.Fatalf("expected empty queue after ack, got %+v", info)
	}
}

func TestNackRequeueAndPurge(t *testing.T) {
	broker := messaging.NewMemoryBroker()
	tools := toolMap(broker)
	ctx := context.Background()

	_, _ = tools["mq_publish"].Execute(ctx, json.RawMessage(`{"queue":"q","body":"x"}`))
	res, _ := tools["mq_subscribe"].Execute(ctx, json.RawMessage(`{"queue":"q","max":1}`))
	var sub struct {
		Messages []messaging.Message `json:"messages"`
	}
	_ = json.Unmarshal(res.Output, &sub)

	_, err := tools["mq_nack"].Execute(ctx, json.RawMessage(
		`{"queue":"q","message_id":"`+sub.Messages[0].ID+`","requeue":true}`))
	if err != nil {
		t.Fatal(err)
	}

	_, err = tools["mq_purge"].Execute(ctx, json.RawMessage(`{"queue":"q","confirm":false}`))
	if err == nil {
		t.Fatal("expected confirm required")
	}
	res, err = tools["mq_purge"].Execute(ctx, json.RawMessage(`{"queue":"q","confirm":true}`))
	if err != nil {
		t.Fatal(err)
	}
	var purged struct {
		Purged int `json:"purged"`
	}
	_ = json.Unmarshal(res.Output, &purged)
	if purged.Purged != 1 {
		t.Fatalf("expected 1 purged, got %d", purged.Purged)
	}
}
