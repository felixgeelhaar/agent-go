package plannerllm

import (
	"errors"
	"fmt"
	"strings"
)

// DefaultRepairAttempts is how many times a planner re-asks after the model
// returns something that is not a usable decision.
//
// One is enough for the failure this addresses. A model that answered in prose
// almost always produces the JSON when told, in the same turn, exactly what was
// wrong with what it just said; a model that gets it wrong twice is not going
// to get it right on the fifth try, and each attempt spends the run's token
// budget.
const DefaultRepairAttempts = 1

// ErrRepairExhausted reports that the model never produced a usable decision.
// It wraps the last parse error, so a caller can still see what the model kept
// getting wrong.
var ErrRepairExhausted = errors.New("the model did not return a usable decision")

// parseErrors are the failures that are the model's to fix. Every one of them
// means a completion arrived and said the wrong thing — as opposed to a
// transport or provider error, where re-asking would just repeat the outage.
var parseErrors = []error{
	ErrEmptyResponse,
	ErrNoJSON,
	ErrUnknownDecision,
	ErrMissingToolName,
	ErrMissingReason,
	ErrInvalidState,
	ErrInvalidToolArgs,
}

// isRepairable reports whether re-asking the model could plausibly help.
//
// Malformed JSON is included by prefix rather than by sentinel because
// ParseDecisionJSON wraps encoding/json's own error, which has no sentinel to
// match on.
func isRepairable(err error) bool {
	if err == nil {
		return false
	}
	for _, target := range parseErrors {
		if errors.Is(err, target) {
			return true
		}
	}
	return strings.HasPrefix(err.Error(), "invalid JSON:")
}

// repairMessages appends the exchange that asks the model to try again: what it
// said, and what was wrong with it.
//
// The rejected output is echoed back as the assistant turn it actually was.
// Leaving it out would ask the model to correct something it can no longer see,
// and models then tend to repeat the mistake verbatim.
func repairMessages(msgs []Message, said Message, err error) []Message {
	echo := said
	echo.Role = "assistant"
	if strings.TrimSpace(echo.Content) == "" {
		// A rejected native tool call carries no text, so describing it is the
		// only way the model can see what it just did. "(no content)" would ask
		// it to fix something invisible.
		echo.Content = describeToolCalls(said.ToolCalls)
	}
	return append(msgs,
		echo,
		Message{Role: "user", Content: repairInstruction(err)},
	)
}

// describeToolCalls renders a tool-call turn as text for the repair echo.
func describeToolCalls(calls []ToolCall) string {
	if len(calls) == 0 {
		return "(no content)"
	}
	var b strings.Builder
	b.WriteString("(tool call)")
	for _, c := range calls {
		fmt.Fprintf(&b, " %s(%s)", c.Function.Name, c.Function.Arguments)
	}
	return b.String()
}

// repairInstruction names the defect. A generic "that was invalid, try again"
// gets a generically different answer; naming the specific rule that was broken
// gets the rule followed.
func repairInstruction(err error) string {
	var b strings.Builder
	b.WriteString("That response could not be used: ")
	b.WriteString(err.Error())
	b.WriteString(".\n\n")

	switch {
	case errors.Is(err, ErrNoJSON), errors.Is(err, ErrEmptyResponse):
		b.WriteString("Your entire reply must be one JSON object and nothing else — " +
			"no explanation before it, no commentary after it. If you were about to " +
			"explain something, put the explanation in the \"reason\" field.\n")
	case errors.Is(err, ErrUnknownDecision):
		b.WriteString("The \"decision\" field takes exactly one of: call_tool, transition, " +
			"finish, fail, ask_human. It is not a state name and not a tool name — " +
			"a state goes in \"to_state\" with \"decision\": \"transition\", and a tool " +
			"goes in \"tool_name\" with \"decision\": \"call_tool\".\n")
	case errors.Is(err, ErrMissingToolName):
		b.WriteString("A call_tool decision needs \"tool_name\" set to the name of one of " +
			"the tools listed above.\n")
	case errors.Is(err, ErrMissingReason):
		b.WriteString("A fail decision needs \"reason\" set to what actually went wrong. " +
			"If you do not know, say that you do not know — do not invent a cause.\n")
	case errors.Is(err, ErrInvalidState):
		b.WriteString("A transition decision needs \"to_state\" set to one of: intake, " +
			"explore, decide, act, validate, done, failed.\n")
	case errors.Is(err, ErrInvalidToolArgs):
		b.WriteString("The tool arguments must be a valid JSON object.\n")
	default:
		b.WriteString("Reply with one well-formed JSON object in one of the documented " +
			"decision shapes, and nothing else.\n")
	}

	b.WriteString("\nReply now with only that JSON object.")
	return b.String()
}

// wrapExhausted reports the final failure in terms of both what happened and
// how hard the planner tried, so a caller reading a log can tell a model that
// answered wrongly once from one that would never have complied.
func wrapExhausted(err error, attempts int) error {
	return fmt.Errorf("%w after %d attempt(s): %w", ErrRepairExhausted, attempts, err)
}
