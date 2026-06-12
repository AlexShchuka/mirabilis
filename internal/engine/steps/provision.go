package steps

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"time"

	"github.com/AlexShchuka/mirabilis/internal/engine/exec"
	"github.com/AlexShchuka/mirabilis/internal/engine/pipeline"
	"github.com/AlexShchuka/mirabilis/internal/engine/sandbox"
)

const provisionCheckTimeout = 30 * time.Second

const (
	phaseCreate = "create"
	phaseStart  = "start"

	createMarkerPath = "/home/node/.claude/.mirabilis-provision-status"
	startMarkerPath  = "/home/node/.claude/.mirabilis-start-marker"
	createMarkerOK   = "ok"
)

func startMarkerHash(fingerprint, sessionKey string) string {
	sum := sha256.Sum256([]byte(fingerprint + sessionKey))
	return hex.EncodeToString(sum[:])
}

type provisionStep struct {
	d     Deps
	phase string
}

func newProvision(d Deps, phase string) *provisionStep {
	return &provisionStep{d: d, phase: phase}
}

func (s *provisionStep) name() string { return "provision-" + s.phase }

func (s *provisionStep) Meta() pipeline.Meta {
	dep := "container"
	if s.phase == phaseStart {
		dep = "provision-create"
	}
	return pipeline.Meta{
		Name:    s.name(),
		Title:   "Provision (" + s.phase + ")",
		Deps:    []string{dep},
		Kind:    pipeline.Auto,
		Timeout: 5 * time.Minute,
		Retry:   pipeline.RetryPolicy{Attempts: 2, Delay: 2 * time.Second},
	}
}

func (s *provisionStep) Check(ctx context.Context) (bool, error) {
	checkCtx, cancel := context.WithTimeout(ctx, provisionCheckTimeout)
	defer cancel()
	path := createMarkerPath
	if s.phase == phaseStart {
		path = startMarkerPath
	}
	out, err := exec.Run(checkCtx, s.d.Runner, exec.Spec{Argv: containerArgv("cat", path)})
	if err != nil {
		return false, nil
	}
	want := createMarkerOK
	if s.phase == phaseStart {
		want = startMarkerHash(s.d.Sandbox.Desired(ctx), s.d.SessionKey())
	}
	return strings.TrimSpace(out) == want, nil
}

func (s *provisionStep) Run(ctx context.Context, out chan<- pipeline.Event, _ <-chan pipeline.Result) error {
	spec := exec.Spec{
		Argv: []string{
			"docker", "exec", "-i", sandbox.ContainerName,
			"mirabilis", "provision", "--phase", s.phase, "--proxy-addr", s.d.ProxyAddr(),
		},
		Stdin: strings.NewReader(s.d.SessionKey()),
	}
	return stream(s.name(), out, s.d.Runner.Stream(ctx, spec))
}
