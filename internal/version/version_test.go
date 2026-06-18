package version

import "testing"

func TestString_Default(t *testing.T) {
	s := String()
	if s == "" {
		t.Error("String() returned empty")
	}
	// Default build has Version="dev", Commit="unknown", BuildDate="unknown".
	if Version != "dev" {
		t.Errorf("Version = %q, want %q", Version, "dev")
	}
}

func TestShort_Default(t *testing.T) {
	s := Short()
	if s != Version {
		t.Errorf("Short() = %q, want %q", s, Version)
	}
}

func TestString_Custom(t *testing.T) {
	// Save and restore original values.
	origVer, origCommit, origDate := Version, Commit, BuildDate
	defer func() {
		Version, Commit, BuildDate = origVer, origCommit, origDate
	}()

	Version = "v1.2.3"
	Commit = "abc1234"
	BuildDate = "2026-06-18T00:00:00Z"

	s := String()
	want := "web-search-mcp v1.2.3 (commit abc1234, built 2026-06-18T00:00:00Z)"
	if s != want {
		t.Errorf("String() = %q, want %q", s, want)
	}
}

func TestShort_Custom(t *testing.T) {
	origVer := Version
	defer func() { Version = origVer }()

	Version = "v4.5.6"
	s := Short()
	if s != "v4.5.6" {
		t.Errorf("Short() = %q, want %q", s, "v4.5.6")
	}
}
