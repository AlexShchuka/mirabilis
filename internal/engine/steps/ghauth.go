package steps

import (
	"context"
	"regexp"
	"strings"
	"time"

	"github.com/AlexShchuka/mirabilis/internal/engine/exec"
	"github.com/AlexShchuka/mirabilis/internal/engine/pipeline"
	"github.com/AlexShchuka/mirabilis/internal/engine/sandbox"
)

var (
	ghUserCode  = regexp.MustCompile(`\b[A-Z0-9]{4}-[A-Z0-9]{4}\b`)
	ghDeviceURL = regexp.MustCompile(`https://\S*?/login/device\S*`)
)

func ghLoginArgv() []string {
	return []string{
		"docker", "exec", "-i", sandbox.ContainerName,
		"env", "BROWSER=true",
		"gh", "auth", "login", "--hostname", "github.com",
		"--git-protocol", "https", "--web", "--scopes", "workflow",
		"--insecure-storage",
	}
}

type ghAuthStep struct {
	d Deps
}

func (s *ghAuthStep) Meta() pipeline.Meta {
	return pipeline.Meta{
		Name:  "gh-auth",
		Title: "GitHub sign-in",
		Deps:  []string{"container"},
		Kind:  pipeline.Interactive,
	}
}

func (s *ghAuthStep) Check(ctx context.Context) (bool, error) {
	checkCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	_, err := exec.Run(checkCtx, s.d.Runner, exec.Spec{Argv: containerArgv("gh", "auth", "status")})
	return err == nil, nil
}

func (s *ghAuthStep) Run(ctx context.Context, out chan<- pipeline.Event, in <-chan pipeline.Result) error {
	runCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	events := s.d.Runner.Stream(runCtx, exec.Spec{Argv: ghLoginArgv(), Stdin: strings.NewReader("\n")})
	var code, url string
	var exitErr error
	emitted := false
	cancelled := false
	for {
		select {
		case ev, ok := <-events:
			if !ok {
				if cancelled {
					return pipeline.ErrCancelled
				}
				return exitErr
			}
			if ev.Kind == exec.KindExited {
				exitErr = ev.Err
			}
			pipeline.Forward("gh-auth", out, ev)
			if ev.Kind != exec.KindStdout && ev.Kind != exec.KindStderr {
				continue
			}
			if c := ghUserCode.FindString(ev.Line); c != "" {
				code = c
			}
			if u := strings.TrimRight(ghDeviceURL.FindString(ev.Line), ".,)"); u != "" {
				url = u
			}
			if !emitted && code != "" && url != "" {
				emitted = true
				out <- pipeline.Event{Kind: pipeline.EvWaiting, Step: "gh-auth", Payload: GHAuth{Code: code, URL: url}}
			}
		case r := <-in:
			if r.Cancelled && !cancelled {
				cancelled = true
				cancel()
			}
		}
	}
}
