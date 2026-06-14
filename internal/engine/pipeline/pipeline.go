package pipeline

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"
)

type stepState int

const (
	stateDone stepState = iota
	stateSkipped
	stateFailed
)

type Pipeline struct {
	log    *slog.Logger
	steps  []Command
	events chan Event
	done   <-chan struct{}

	mu      sync.Mutex
	resume  chan Result
	waiting string
	started bool
}

func New(log *slog.Logger, steps ...Command) (*Pipeline, error) {
	if log == nil {
		log = slog.New(slog.DiscardHandler)
	}
	all := make(map[string]bool, len(steps))
	for _, s := range steps {
		name := s.Meta().Name
		if name == "" {
			return nil, errors.New("pipeline: step with empty name")
		}
		if all[name] {
			return nil, fmt.Errorf("pipeline: duplicate step %q", name)
		}
		all[name] = true
	}
	earlier := make(map[string]bool, len(steps))
	for _, s := range steps {
		m := s.Meta()
		for _, dep := range m.Deps {
			if !all[dep] {
				return nil, fmt.Errorf("pipeline: step %q has unknown dependency %q", m.Name, dep)
			}
			if !earlier[dep] {
				return nil, fmt.Errorf("pipeline: dependency cycle: step %q requires %q before it runs", m.Name, dep)
			}
		}
		earlier[m.Name] = true
	}
	return &Pipeline{
		log:    log,
		steps:  steps,
		events: make(chan Event, 64),
	}, nil
}

func (p *Pipeline) Events() <-chan Event { return p.events }

func (p *Pipeline) Run(ctx context.Context) error {
	p.mu.Lock()
	if p.started {
		p.mu.Unlock()
		return errors.New("pipeline: already started")
	}
	p.started = true
	p.done = ctx.Done()
	p.mu.Unlock()
	defer close(p.events)

	states := make(map[string]stepState, len(p.steps))
	var firstErr error
	for _, s := range p.steps {
		if ctx.Err() != nil {
			p.emit(Event{Kind: EvPipelineDone, Failed: true})
			return ctx.Err()
		}
		m := s.Meta()
		if dep, blocked := failedDep(m, states); blocked {
			states[m.Name] = stateSkipped
			p.log.Info("step skipped", "step", m.Name, "failed_dep", dep)
			p.emit(Event{Kind: EvSkipped, Step: m.Name, Line: "dependency failed: " + dep})
			continue
		}
		state, err := p.runStep(ctx, s, m)
		states[m.Name] = state
		if state == stateFailed && firstErr == nil {
			firstErr = fmt.Errorf("pipeline: step %q: %w", m.Name, err)
		}
		if ctx.Err() != nil {
			p.emit(Event{Kind: EvPipelineDone, Failed: true})
			return ctx.Err()
		}
	}
	p.emit(Event{Kind: EvPipelineDone, Failed: firstErr != nil})
	p.log.Info("pipeline done", "failed", firstErr != nil)
	return firstErr
}

func (p *Pipeline) Resume(step string, r Result) error {
	p.mu.Lock()
	if p.waiting != step || p.resume == nil {
		p.mu.Unlock()
		return fmt.Errorf("pipeline: step %q is not waiting", step)
	}
	ch := p.resume
	p.waiting = ""
	p.resume = nil
	p.mu.Unlock()
	ch <- r
	return nil
}

func failedDep(m Meta, states map[string]stepState) (string, bool) {
	for _, dep := range m.Deps {
		if states[dep] == stateFailed {
			return dep, true
		}
	}
	return "", false
}

func (p *Pipeline) runStep(ctx context.Context, s Command, m Meta) (stepState, error) {
	satisfied, err := p.check(ctx, s, m)
	switch {
	case err != nil && m.Optional:
		p.log.Warn("optional step check failed", "step", m.Name, "err", err)
		p.emit(Event{Kind: EvSkipped, Step: m.Name, Err: err})
		return stateSkipped, nil
	case err != nil:
		p.log.Error("step check failed", "step", m.Name, "err", err)
		p.emit(Event{Kind: EvFailed, Step: m.Name, Err: err})
		return stateFailed, err
	case satisfied:
		p.log.Info("step satisfied", "step", m.Name)
		p.emit(Event{Kind: EvDone, Step: m.Name, Line: LineSatisfied})
		return stateDone, nil
	}
	p.log.Info("step started", "step", m.Name)
	p.emit(Event{Kind: EvStepStarted, Step: m.Name})
	if err := p.runAttempts(ctx, s, m); err != nil {
		if m.Optional {
			p.log.Warn("optional step failed", "step", m.Name, "err", err)
			p.emit(Event{Kind: EvSkipped, Step: m.Name, Err: err})
			return stateSkipped, nil
		}
		p.log.Error("step failed", "step", m.Name, "err", err)
		p.emit(Event{Kind: EvFailed, Step: m.Name, Err: err})
		return stateFailed, err
	}
	p.log.Info("step done", "step", m.Name)
	p.emit(Event{Kind: EvDone, Step: m.Name})
	return stateDone, nil
}

func (p *Pipeline) check(ctx context.Context, s Command, m Meta) (bool, error) {
	if m.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, m.Timeout)
		defer cancel()
	}
	return s.Check(ctx)
}

func (p *Pipeline) runAttempts(ctx context.Context, s Command, m Meta) error {
	attempts := max(m.Retry.Attempts, 1)
	var err error
	for i := 0; i < attempts; i++ {
		if i > 0 {
			if !p.sleep(ctx, m.Retry.Delay) {
				return ctx.Err()
			}
			if satisfied, cerr := p.check(ctx, s, m); cerr == nil && satisfied {
				p.log.Info("step satisfied after retry check", "step", m.Name)
				return nil
			}
			p.log.Info("step retrying", "step", m.Name, "attempt", i+1)
		}
		if err = p.attempt(ctx, s, m); err == nil {
			return nil
		}
		if ctx.Err() != nil {
			return err
		}
	}
	return err
}

func (p *Pipeline) sleep(ctx context.Context, d time.Duration) bool {
	if d <= 0 {
		return ctx.Err() == nil
	}
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-t.C:
		return true
	case <-ctx.Done():
		return false
	}
}

func (p *Pipeline) attempt(ctx context.Context, s Command, m Meta) error {
	runCtx, cancel := context.WithCancelCause(ctx)
	defer cancel(nil)

	out := make(chan Event)
	in := make(chan Result)
	resume := make(chan Result, 1)
	runErr := make(chan error, 1)
	go func() { runErr <- s.Run(runCtx, out, in) }()

	var timer *time.Timer
	var timerC <-chan time.Time
	remaining := m.Timeout
	var resumedAt time.Time
	if m.Timeout > 0 {
		timer = time.NewTimer(remaining)
		defer timer.Stop()
		timerC = timer.C
		resumedAt = time.Now()
	}
	done := ctx.Done()
	cancelled := false
	timedOut := false
	for {
		select {
		case ev := <-out:
			if ev.Kind != EvWaiting {
				p.forward(m.Name, ev)
				continue
			}
			if timerC != nil {
				timer.Stop()
				remaining -= time.Since(resumedAt)
				timerC = nil
			}
			if cancelled {
				p.forward(m.Name, ev)
				if exited, err := p.deliver(m.Name, in, out, runErr, Result{Cancelled: true}); exited {
					return p.settle(m, err, timedOut)
				}
				continue
			}
			p.setWaiting(m.Name, resume)
			p.log.Info("step waiting", "step", m.Name)
			p.forward(m.Name, ev)
		case r := <-resume:
			p.log.Info("step resumed", "step", m.Name, "cancelled", r.Cancelled)
			if timer != nil && !cancelled {
				if remaining <= 0 {
					timedOut = true
					cancel(context.DeadlineExceeded)
				} else {
					timer.Reset(remaining)
					timerC = timer.C
					resumedAt = time.Now()
				}
			}
			if exited, err := p.deliver(m.Name, in, out, runErr, r); exited {
				return p.settle(m, err, timedOut)
			}
		case <-timerC:
			timerC = nil
			timedOut = true
			cancel(context.DeadlineExceeded)
		case <-done:
			done = nil
			cancelled = true
			if p.takeWaiting(m.Name) {
				if exited, err := p.deliver(m.Name, in, out, runErr, Result{Cancelled: true}); exited {
					return p.settle(m, err, timedOut)
				}
			}
		case err := <-runErr:
			return p.settle(m, err, timedOut)
		}
	}
}

func (p *Pipeline) deliver(step string, in chan<- Result, out <-chan Event, runErr <-chan error, r Result) (bool, error) {
	for {
		select {
		case in <- r:
			return false, nil
		case ev := <-out:
			p.forward(step, ev)
		case err := <-runErr:
			return true, err
		}
	}
}

func (p *Pipeline) settle(m Meta, err error, timedOut bool) error {
	p.takeWaiting(m.Name)
	if err != nil && timedOut {
		return fmt.Errorf("step %q timed out after %v: %w", m.Name, m.Timeout, context.DeadlineExceeded)
	}
	return err
}

func (p *Pipeline) setWaiting(step string, ch chan Result) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.waiting = step
	p.resume = ch
}

func (p *Pipeline) takeWaiting(step string) bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.waiting != step {
		return false
	}
	p.waiting = ""
	p.resume = nil
	return true
}

func (p *Pipeline) forward(step string, ev Event) {
	ev.Step = step
	p.emit(ev)
}

func (p *Pipeline) emit(ev Event) {
	select {
	case p.events <- ev:
		return
	default:
	}
	select {
	case p.events <- ev:
	case <-p.done:
	}
}
