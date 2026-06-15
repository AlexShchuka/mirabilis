package styles

import (
	"io"
	"os"
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/colorprofile"
)

func TestHuhThemeBuilds(t *testing.T) {
	for _, dark := range []bool{true, false} {
		if s := HuhTheme().Theme(dark); s == nil {
			t.Fatalf("HuhTheme().Theme(%v) = nil", dark)
		}
		if s := HuhThemeDanger().Theme(true); s == nil {
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

func TestStylesSourceNoBackgroundOutsideDanger(t *testing.T) {
	f, err := os.Open("styles.go")
	if err != nil {
		t.Fatalf("open styles.go: %v", err)
	}
	defer f.Close()
	raw, err := io.ReadAll(f)
	if err != nil {
		t.Fatalf("read styles.go: %v", err)
	}
	src := string(raw)
	inDanger := false
	for _, line := range strings.Split(src, "\n") {
		if strings.Contains(line, "func HuhThemeDanger") {
			inDanger = true
		}
		if inDanger {
			continue
		}
		if strings.Contains(line, ".Background(") {
			t.Errorf("styles.go line contains .Background() outside HuhThemeDanger (INV §3 host-bg): %q", strings.TrimSpace(line))
		}
	}
}

func TestMintColorDowngradesNoTTY(t *testing.T) {
	saved := termProfile
	t.Cleanup(func() { termProfile = saved })

	termProfile = colorprofile.NoTTY
	c := resolvedMint()
	if _, ok := c.(lipgloss.NoColor); !ok {
		t.Errorf("resolvedMint() under NoTTY = %T, want lipgloss.NoColor (INV §3 NO_COLOR degrade)", c)
	}

	termProfile = colorprofile.Ascii
	c = resolvedMint()
	if _, ok := c.(lipgloss.NoColor); !ok {
		t.Errorf("resolvedMint() under Ascii = %T, want lipgloss.NoColor (INV §3 NO_COLOR degrade)", c)
	}
}
