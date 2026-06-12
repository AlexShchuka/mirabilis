//go:build darwin || linux

package exec

import (
	"bufio"
	"context"
	"errors"
	"io"
	"os"
	"os/exec"
	"sync"
	"syscall"
	"time"
)

const (
	scanLineMax = 1 << 20
	scanLineCap = 64 << 10
	chanBuffer  = 16
	waitDelay   = 5 * time.Second
)

type Host struct{}

var _ Runner = (*Host)(nil)

func NewHost() *Host { return &Host{} }

func (h *Host) Stream(ctx context.Context, spec Spec) <-chan Event {
	ch := make(chan Event, chanBuffer)
	go func() {
		defer close(ch)
		ch <- Event{Kind: KindStarted, Argv: spec.Argv}

		if len(spec.Argv) == 0 {
			ch <- Event{Kind: KindExited, Code: -1, Err: errors.New("exec: empty argv")}
			return
		}

		cmd := exec.CommandContext(ctx, spec.Argv[0], spec.Argv[1:]...)
		cmd.Dir = spec.Dir
		cmd.Stdin = spec.Stdin
		if len(spec.Env) > 0 {
			cmd.Env = append(os.Environ(), spec.Env...)
		}
		cmd.WaitDelay = waitDelay
		cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
		cmd.Cancel = func() error {
			if cmd.Process == nil {
				return nil
			}
			if err := syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL); err != nil && !errors.Is(err, syscall.ESRCH) {
				return err
			}
			return os.ErrProcessDone
		}

		stdout, err := cmd.StdoutPipe()
		if err != nil {
			ch <- Event{Kind: KindExited, Code: -1, Err: err}
			return
		}
		stderr, err := cmd.StderrPipe()
		if err != nil {
			ch <- Event{Kind: KindExited, Code: -1, Err: err}
			return
		}

		if err := cmd.Start(); err != nil {
			ch <- Event{Kind: KindExited, Code: -1, Err: err}
			return
		}

		var wg sync.WaitGroup
		wg.Add(2)
		go scanPipe(stdout, KindStdout, ch, &wg)
		go scanPipe(stderr, KindStderr, ch, &wg)
		wg.Wait()

		err = cmd.Wait()
		ch <- Event{Kind: KindExited, Code: exitCode(err), Err: err}
	}()
	return ch
}

func scanPipe(r io.Reader, kind EventKind, ch chan<- Event, wg *sync.WaitGroup) {
	defer wg.Done()
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 0, scanLineCap), scanLineMax)
	for scanner.Scan() {
		ch <- Event{Kind: kind, Line: scanner.Text()}
	}
}

func exitCode(err error) int {
	if err == nil {
		return 0
	}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		if code := exitErr.ExitCode(); code >= 0 {
			return code
		}
	}
	return -1
}
