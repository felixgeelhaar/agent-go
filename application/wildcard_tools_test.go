package application

import (
	"testing"

	"go.klarlabs.de/agent/domain/agent"
	"go.klarlabs.de/agent/domain/policy"
)

// AllowedTools returns the literal "*" for a wildcard state and its own doc says
// "callers should check for '*' to determine if all tools are permitted".
// Engine.step did not check: it passed ["*"] to buildToolDefs, which called
// registry.Get("*"), found nothing, and handed the planner ZERO tool definitions.
//
// That means NewDefaultToolEligibility — this package's own default, which uses
// "*" for explore/decide/act/validate — produced a planner that could see no tools
// at all. IsAllowed honours the wildcard, so execution would have permitted them;
// only the planner's view was empty.
//
// Expanding it against the live registry also makes a tool registered DURING a run
// visible on the next step, which is what a deferred/lazy tool surface needs.
func TestWildcardEligibilityExpandsToTheRegistry(t *testing.T) {
	elig := policy.NewToolEligibility().Allow(agent.StateAct, "*")
	if !elig.HasWildcard(agent.StateAct) {
		t.Fatal("test setup: act should carry the wildcard")
	}

	names := expandWildcard(elig.AllowedTools(agent.StateAct), []string{"search", "post", "read"})
	if len(names) != 3 {
		t.Fatalf("wildcard expanded to %v, want all three registered tools", names)
	}
	for _, n := range names {
		if n == "*" {
			t.Error("the literal \"*\" survived expansion — registry.Get(\"*\") finds nothing " +
				"and the planner is handed no tool definitions")
		}
	}
}

// A state with explicit names must be left exactly as it is: expanding it would
// silently widen what the planner may call in that state.
func TestExplicitToolListIsNotWidened(t *testing.T) {
	got := expandWildcard([]string{"search"}, []string{"search", "post", "read"})
	if len(got) != 1 || got[0] != "search" {
		t.Errorf("explicit list became %v — a named state must never be widened to the registry", got)
	}
}

// No eligibility for a state means no tools, wildcard machinery notwithstanding.
func TestEmptyStaysEmpty(t *testing.T) {
	if got := expandWildcard(nil, []string{"search"}); len(got) != 0 {
		t.Errorf("empty allow-list expanded to %v, want nothing", got)
	}
}
