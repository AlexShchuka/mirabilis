package a11y

import "testing"

func TestReducedMotionGates(t *testing.T) {
	tests := []struct {
		name     string
		noColor  string
		noAnim   string
		access   string
		wantRM   bool
		wantAcc  bool
		wantNoCl bool
	}{
		{name: "all unset", wantRM: false},
		{name: "no_color set", noColor: "1", wantRM: true, wantNoCl: true},
		{name: "no_animate set", noAnim: "1", wantRM: true},
		{name: "accessible set", access: "1", wantRM: true, wantAcc: true},
		{name: "empty values are unset", noColor: "", noAnim: "", access: "", wantRM: false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("NO_COLOR", tc.noColor)
			t.Setenv("NO_ANIMATE", tc.noAnim)
			t.Setenv("ACCESSIBLE", tc.access)
			if got := ReducedMotion(); got != tc.wantRM {
				t.Errorf("ReducedMotion() = %v, want %v", got, tc.wantRM)
			}
			if got := Accessible(); got != tc.wantAcc {
				t.Errorf("Accessible() = %v, want %v", got, tc.wantAcc)
			}
			if got := NoColor(); got != tc.wantNoCl {
				t.Errorf("NoColor() = %v, want %v", got, tc.wantNoCl)
			}
		})
	}
}
