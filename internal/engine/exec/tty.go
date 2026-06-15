//go:build darwin || linux

package exec

import (
	"io"
	"os"
	osexec "os/exec"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/creack/pty"
)

const (
	ttyWaitDelay  = 5 * time.Second
	ptyDrainGrace = time.Second
)

type TTY struct {
	stdin          io.Reader
	stdout, stderr io.Writer
	Env            []string
	Argv           []string
}

func (t *TTY) SetStdin(r io.Reader)  { t.stdin = r }
func (t *TTY) SetStdout(w io.Writer) { t.stdout = w }
func (t *TTY) SetStderr(w io.Writer) { t.stderr = w }

func (t *TTY) Run() error {
	cmd := osexec.Command(t.Argv[0], t.Argv[1:]...)
	cmd.Env = append(os.Environ(), t.Env...)
	cmd.Stdin = t.stdin
	cmd.Stdout = t.stdout
	cmd.Stderr = t.stderr
	cmd.WaitDelay = ttyWaitDelay
	if err := cmd.Start(); err != nil {
		return err
	}
	stopWinch := t.forwardWinch(cmd.Process)
	defer stopWinch()
	return cmd.Wait()
}

func (t *TTY) forwardWinch(proc *os.Process) func() {
	winch := make(chan os.Signal, 1)
	signal.Notify(winch, syscall.SIGWINCH)
	go func() {
		_ = proc.Signal(syscall.SIGWINCH)
		for range winch {
			_ = proc.Signal(syscall.SIGWINCH)
		}
	}()
	return func() {
		signal.Stop(winch)
		close(winch)
	}
}

type PTYTee struct {
	tee            io.Writer
	stdin          io.Reader
	stdout, stderr io.Writer
	Env            []string
	Argv           []string
}

func NewPTYTee(argv []string, tee io.Writer) *PTYTee {
	return &PTYTee{Argv: argv, tee: tee}
}

func (p *PTYTee) SetStdin(r io.Reader)  { p.stdin = r }
func (p *PTYTee) SetStdout(w io.Writer) { p.stdout = w }
func (p *PTYTee) SetStderr(w io.Writer) { p.stderr = w }

func (p *PTYTee) Run() error {
	cmd := osexec.Command(p.Argv[0], p.Argv[1:]...)
	cmd.Env = append(os.Environ(), p.Env...)
	cmd.WaitDelay = ttyWaitDelay

	master, err := p.start(cmd)
	if err != nil {
		return err
	}

	var closeOnce sync.Once
	closeMaster := func() { closeOnce.Do(func() { master.Close() }) }
	defer closeMaster()

	stopWinch := p.forwardWinch(master)
	defer stopWinch()

	var outDone sync.WaitGroup
	outDone.Add(1)
	go func() {
		defer outDone.Done()
		out := p.stdout
		if out == nil {
			out = io.Discard
		}
		dst := out
		if p.tee != nil {
			dst = io.MultiWriter(out, p.tee)
		}
		_, _ = io.Copy(dst, master)
	}()

	inDone := make(chan struct{})
	go p.pumpStdin(master, inDone)

	err = cmd.Wait()
	close(inDone)
	forceClose := time.AfterFunc(ptyDrainGrace, closeMaster)
	_ = master.SetReadDeadline(time.Now().Add(ptyDrainGrace))
	outDone.Wait()
	forceClose.Stop()
	closeMaster()
	return err
}

func (p *PTYTee) start(cmd *osexec.Cmd) (*os.File, error) {
	if f, ok := p.stdout.(*os.File); ok {
		if sz, err := pty.GetsizeFull(f); err == nil {
			return pty.StartWithSize(cmd, sz)
		}
	}
	return pty.Start(cmd)
}

func (p *PTYTee) forwardWinch(master *os.File) func() {
	winch := make(chan os.Signal, 1)
	signal.Notify(winch, syscall.SIGWINCH)
	go func() {
		for range winch {
			if f, ok := p.stdout.(*os.File); ok {
				if sz, err := pty.GetsizeFull(f); err == nil {
					_ = pty.Setsize(master, sz)
				}
			}
		}
	}()
	return func() {
		signal.Stop(winch)
		close(winch)
	}
}

func (p *PTYTee) pumpStdin(master *os.File, done <-chan struct{}) {
	if p.stdin == nil {
		return
	}
	f, ok := p.stdin.(*os.File)
	if !ok {
		copyDone := make(chan struct{})
		go func() {
			defer close(copyDone)
			_, _ = io.Copy(master, p.stdin)
		}()
		select {
		case <-done:
		case <-copyDone:
		}
		return
	}
	defer func() { _ = f.SetReadDeadline(time.Time{}) }()
	buf := make([]byte, 1024)
	for {
		select {
		case <-done:
			return
		default:
		}
		if err := f.SetReadDeadline(time.Now().Add(50 * time.Millisecond)); err != nil {
			_, _ = io.Copy(master, f)
			return
		}
		n, err := f.Read(buf)
		if n > 0 {
			if _, werr := master.Write(buf[:n]); werr != nil {
				return
			}
		}
		if err != nil {
			if os.IsTimeout(err) {
				continue
			}
			return
		}
	}
}
