package plannerllm

import (
	"context"
	"errors"
	"strings"
	"testing"

	"go.klarlabs.de/agent/domain/agent"
	"go.klarlabs.de/agent/infrastructure/planner"
)

// scriptedProvider answers each call with the next scripted response, so a test
// can describe a model that gets it wrong and then gets it right.
type scriptedProvider struct {
	replies []string
	err     error
	reqs    []CompletionRequest
}

func (s *scriptedProvider) Complete(_ context.Context, req CompletionRequest) (CompletionResponse, error) {
	s.reqs = append(s.reqs, req)
	if s.err != nil {
		return CompletionResponse{}, s.err
	}
	i := len(s.reqs) - 1
	if i >= len(s.replies) {
		i = len(s.replies) - 1
	}
	return CompletionResponse{Message: Message{Role: "assistant", Content: s.replies[i]}}, nil
}

func (s *scriptedProvider) Name() string { return "scripted" }

func planOnce(t *testing.T, p *LLMPlanner) (agent.Decision, error) {
	t.Helper()
	return p.Plan(t.Context(), planner.PlanRequest{Goal: "investigate the alert"})
}

// TestPlanRepairsProseIntoADecision covers the failure this exists for: the
// model answered in prose, the step died, and the run reported a failure with
// no cause. One corrective turn recovers it.
func TestPlanRepairsProseIntoADecision(t *testing.T) {
	t.Parallel()
	prov := &scriptedProvider{replies: []string{
		"I'll look at the deployment logs first to understand what happened.",
		`{"decision":"call_tool","tool_name":"read_logs","input":{},"reason":"check the deploy"}`,
	}}
	p := NewPlanner(Config{Provider: prov})

	d, err := planOnce(t, p)
	if err != nil {
		t.Fatalf("the repair turn should have recovered this: %v", err)
	}
	if d.Type != agent.DecisionCallTool || d.CallTool.ToolName != "read_logs" {
		t.Fatalf("decision = %+v, want the tool call from the second answer", d)
	}
	if len(prov.reqs) != 2 {
		t.Fatalf("provider called %d times, want 2", len(prov.reqs))
	}

	// The retry must carry the rejected answer and the specific complaint.
	// Re-asking without either gets the same mistake back.
	msgs := prov.reqs[1].Messages
	if len(msgs) != 4 {
		t.Fatalf("retry sent %d messages, want the original two plus the repair pair", len(msgs))
	}
	if msgs[2].Role != "assistant" || !strings.Contains(msgs[2].Content, "deployment logs") {
		t.Errorf("the rejected answer was not echoed back: %+v", msgs[2])
	}
	if msgs[3].Role != "user" || !strings.Contains(msgs[3].Content, "one JSON object") {
		t.Errorf("the repair turn did not say what was wrong: %+v", msgs[3])
	}
}

// TestPlanRepairNamesTheSpecificRule pins that the correction is specific. A
// generic "that was invalid" gets a generically different answer back.
func TestPlanRepairNamesTheSpecificRule(t *testing.T) {
	t.Parallel()
	// "act" is a state, not a decision — the exact shape the engine prompt
	// warns against and models still produce.
	prov := &scriptedProvider{replies: []string{
		`{"decision":"act","reason":"time to do the thing"}`,
		`{"decision":"transition","to_state":"act","reason":"time to do the thing"}`,
	}}

	d, err := planOnce(t, NewPlanner(Config{Provider: prov}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if d.Type != agent.DecisionTransition {
		t.Fatalf("decision = %s, want a transition", d.Type)
	}
	repair := prov.reqs[1].Messages[3].Content
	if !strings.Contains(repair, "to_state") {
		t.Errorf("the correction should name the field that was missing:\n%s", repair)
	}
}

// TestPlanGivesUpAfterTheBound stops a stubborn model from looping.
func TestPlanGivesUpAfterTheBound(t *testing.T) {
	t.Parallel()
	prov := &scriptedProvider{replies: []string{"still prose", "still prose", "still prose"}}

	_, err := planOnce(t, NewPlanner(Config{Provider: prov, RepairAttempts: 2}))
	if !errors.Is(err, ErrRepairExhausted) {
		t.Fatalf("err = %v, want ErrRepairExhausted", err)
	}
	// The last parse error stays reachable: a caller reading the log needs to
	// see what the model kept getting wrong, not just that it did.
	if !errors.Is(err, ErrNoJSON) {
		t.Errorf("err = %v, want the underlying parse error to survive", err)
	}
	if len(prov.reqs) != 3 {
		t.Errorf("provider called %d times, want the first attempt plus 2 repairs", len(prov.reqs))
	}
}

// TestPlanDoesNotRepairAProviderFailure keeps the retry aimed at the model. A
// transport error re-asked is just the outage again, at twice the latency.
func TestPlanDoesNotRepairAProviderFailure(t *testing.T) {
	t.Parallel()
	boom := errors.New("502 bad gateway")
	prov := &scriptedProvider{err: boom}

	_, err := planOnce(t, NewPlanner(Config{Provider: prov}))
	if !errors.Is(err, boom) {
		t.Fatalf("err = %v, want the provider error", err)
	}
	if len(prov.reqs) != 1 {
		t.Errorf("provider called %d times, want 1 — a provider error is not the model's to fix", len(prov.reqs))
	}
}

// TestPlanRepairIsOffWhenDisabled keeps a caller measuring raw model compliance
// able to see the first answer.
func TestPlanRepairIsOffWhenDisabled(t *testing.T) {
	t.Parallel()
	prov := &scriptedProvider{replies: []string{"prose", `{"decision":"finish","summary":"done"}`}}

	_, err := planOnce(t, NewPlanner(Config{Provider: prov, RepairAttempts: -1}))
	if !errors.Is(err, ErrNoJSON) {
		t.Fatalf("err = %v, want the first parse error", err)
	}
	if len(prov.reqs) != 1 {
		t.Errorf("provider called %d times, want 1", len(prov.reqs))
	}
}

// TestPlanDoesNotRepairAGoodAnswer guards the common path against a spurious
// extra completion — every repair is a real token cost.
func TestPlanDoesNotRepairAGoodAnswer(t *testing.T) {
	t.Parallel()
	prov := &scriptedProvider{replies: []string{`{"decision":"finish","summary":"done"}`}}

	if _, err := planOnce(t, NewPlanner(Config{Provider: prov})); err != nil {
		t.Fatal(err)
	}
	if len(prov.reqs) != 1 {
		t.Errorf("provider called %d times for an answer that parsed", len(prov.reqs))
	}
}

func TestIsRepairableRejectsAnUnrelatedError(t *testing.T) {
	t.Parallel()
	if isRepairable(errors.New("connection reset by peer")) {
		t.Error("a transport error must not be treated as the model's mistake")
	}
	if isRepairable(nil) {
		t.Error("nil is not a failure")
	}
	if !isRepairable(errors.New("invalid JSON: unexpected end of input")) {
		t.Error("malformed JSON is exactly what a repair turn fixes")
	}
}
