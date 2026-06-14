package sandbox

import (
	"testing"
)

func TestOpenArgvBrowserEnv(t *testing.T) {
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

func TestOpenArgvWSL(t *testing.T) {
	t.Setenv("BROWSER", "")
	t.Setenv("WSL_DISTRO_NAME", "Ubuntu")
	argv, err := openArgv("https://example.com")
	if err != nil {
		t.Fatalf("openArgv: %v", err)
	}
	if len(argv) < 2 || argv[0] != "wslview" {
		t.Errorf("openArgv WSL = %v, want [wslview ...]", argv)
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
