package styles

import "testing"

func TestHuhThemeBuilds(t *testing.T) {
	for _, dark := range []bool{true, false} {
		if s := HuhTheme().Theme(dark); s == nil {
			t.Fatalf("HuhTheme().Theme(%v) = nil", dark)
		}
		if s := HuhThemeDanger().Theme(dark); s == nil {
			t.Fatalf("HuhThemeDanger().Theme(%v) = nil", dark)
		}
	}
}

func TestDangerThemeDiffersFromBase(t *testing.T) {
	base := HuhTheme().Theme(true)
	danger := HuhThemeDanger().Theme(true)
	if base.Focused.FocusedButton.GetBackground() == danger.Focused.FocusedButton.GetBackground() {
		t.Error("danger theme affirmative button shares background with the base theme; destructive action must read as dangerous")
	}
}
