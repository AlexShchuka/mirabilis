package sandbox

import (
	"errors"
	"testing"
)

func TestOpenArgvBrowserEnv(t *testing.T) {
	t.Setenv("MIRABILIS_NO_BROWSER", "")
	t.Setenv("BROWSER", "/usr/bin/my-browser")
	t.Setenv("WSL_DISTRO_NAME", "")
	argv, err := openArgv("https://example.com")
	if err != nil {
		t.Fatalf("openArgv: %v", err)
	}
	if len(argv) != 2 || argv[0] != "/usr/bin/my-browser" || argv[1] != "https://example.com" {
		t.Errorf("openArgv = %v, want [/usr/bin/my-browser https://example.com]", argv)
	}
}

func TestOpenArgvEmptyBrowserDisables(t *testing.T) {
	t.Setenv("MIRABILIS_NO_BROWSER", "")
	t.Setenv("BROWSER", "")
	t.Setenv("WSL_DISTRO_NAME", "Ubuntu")
	_, err := openArgv("https://example.com")
	if !errors.Is(err, errOpenDisabled) {
		t.Errorf("openArgv with BROWSER='' = %v, want errOpenDisabled", err)
	}
}

func TestOpenArgvNoBrowserEnvDisables(t *testing.T) {
	t.Setenv("MIRABILIS_NO_BROWSER", "1")
	t.Setenv("WSL_DISTRO_NAME", "Ubuntu")
	_, err := openArgv("https://example.com")
	if !errors.Is(err, errOpenDisabled) {
		t.Errorf("openArgv with MIRABILIS_NO_BROWSER set = %v, want errOpenDisabled", err)
	}
}

func TestOpenURLNoBrowserIsNoop(t *testing.T) {
	t.Setenv("MIRABILIS_NO_BROWSER", "1")
	if err := OpenURL(t.Context(), nil, "https://example.com"); err != nil {
		t.Errorf("OpenURL with MIRABILIS_NO_BROWSER = %v, want nil (graceful no-op)", err)
	}
}

func TestIsWSLEnvVar(t *testing.T) {
	t.Setenv("WSL_DISTRO_NAME", "Ubuntu-22.04")
	if !isWSL() {
		t.Error("isWSL() = false, want true when WSL_DISTRO_NAME is set")
	}
}

func TestIsWSLNoEnvVar(t *testing.T) {
	t.Setenv("WSL_DISTRO_NAME", "")
	result := isWSL()
	t.Logf("isWSL() = %v (depends on /proc/version content)", result)
}
