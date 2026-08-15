package agentgo

import "testing"

func TestVersion(t *testing.T) {
	t.Parallel()

	if Version == "" {
		t.Error("Version should not be empty")
	}

	// Verify version matches the current release line
	if Version != "0.15.0" {
		t.Errorf("Version is %s, expected 0.15.0", Version)
	}
}

func TestGetVersion(t *testing.T) {
	t.Parallel()

	v := GetVersion()
	if v == "" {
		t.Error("GetVersion() should not return empty string")
	}

	if v != Version {
		t.Errorf("GetVersion() = %s, want %s", v, Version)
	}
}
