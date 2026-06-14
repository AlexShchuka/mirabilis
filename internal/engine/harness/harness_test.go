package harness

import (
	"slices"
	"testing"
)

func TestInstallActionsGolden(t *testing.T) {
	t.Parallel()
	got := InstallActions()
	want := []Action{
		{
			Argv:     []string{"claude", "plugin", "marketplace", "add", "AlexShchuka/neuro-matrix"},
			Fallback: []string{"claude", "plugin", "marketplace", "update", "neuro-matrix"},
			WrapErr:  "marketplace add/update neuro-matrix",
		},
		{
			Argv:    []string{"claude", "plugin", "install", "neuro-matrix@neuro-matrix", "--scope", "user"},
			WrapErr: "plugin install neuro-matrix",
		},
		{
			Argv:    []string{"claude", "plugin", "update", "neuro-matrix@neuro-matrix"},
			WrapErr: "plugin update neuro-matrix",
		},
	}
	if len(got) != len(want) {
		t.Fatalf("InstallActions length = %d, want %d", len(got), len(want))
	}
	for i := range want {
		if !slices.Equal(got[i].Argv, want[i].Argv) {
			t.Errorf("action %d argv = %v, want %v", i, got[i].Argv, want[i].Argv)
		}
		if !slices.Equal(got[i].Fallback, want[i].Fallback) {
			t.Errorf("action %d fallback = %v, want %v", i, got[i].Fallback, want[i].Fallback)
		}
		if got[i].WrapErr != want[i].WrapErr {
			t.Errorf("action %d wraperr = %q, want %q", i, got[i].WrapErr, want[i].WrapErr)
		}
	}
}

func TestStartMarkerHashStable(t *testing.T) {
	t.Parallel()
	const golden = "2476eea9945737aec76cb5c18e3acf117384f883949438952343fbd4e428953f"
	if got := StartMarkerHash("v1.2.3", "session"); got != golden {
		t.Errorf("StartMarkerHash(v1.2.3, session) = %s, want %s", got, golden)
	}
	if StartMarkerHash("v1.2.3", "other") == golden {
		t.Error("hash must change with the session key")
	}
	if StartMarkerHash("v9.9.9", "session") == golden {
		t.Error("hash must change with the fingerprint")
	}
}
