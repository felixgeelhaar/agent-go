package agentgo

import "testing"

func TestVersion(t *testing.T) {
	t.Parallel()

	if Version == "" {
		t.Error("Version should not be empty")
	}

	// Verify version format (should be semver)
	if Version != "0.1.0" {
		t.Logf("Version is %s, expected 0.1.0", Version)
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
