// Package harness provides install actions and probe scripts for the Claude Code harness.
package harness

import (
	"crypto/sha256"
	"encoding/hex"
)

const (
	ProbeScript  = `claude plugin list 2>/dev/null | grep -q neuro-matrix`
	RelinkScript = `NM_DIR="$(printf '%s\n' "$HOME"/.claude/plugins/cache/*/neuro-matrix/*/ | sort -V | tail -n1)"; [ -d "$NM_DIR" ] && ln -sfn "${NM_DIR%/}" "$HOME/.neuro-matrix"; L='export CLAUDE_PLUGIN_ROOT="$HOME/.neuro-matrix"'; grep -qxF "$L" "$HOME/.bashrc" 2>/dev/null || printf '%s\n' "$L" >>"$HOME/.bashrc"`

	CreateMarkerName = ".mirabilis-provision-status"
	StartMarkerName  = ".mirabilis-start-marker"
	MarkerOK         = "ok"
)

type Action struct {
	Argv     []string
	Fallback []string
	WrapErr  string
}

func InstallActions() []Action {
	return []Action{
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
}

func StartMarkerHash(fingerprint, sessionKey string) string {
	sum := sha256.Sum256([]byte(fingerprint + sessionKey))
	return hex.EncodeToString(sum[:])
}
