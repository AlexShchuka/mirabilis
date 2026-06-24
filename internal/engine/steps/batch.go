package steps

import (
	"context"
	"fmt"
	"sort"
	"sync"

	"golang.org/x/sync/errgroup"

	"github.com/AlexShchuka/mirabilis/internal/engine/pipeline"
)

type batchStep struct {
	name  string
	title string
	deps  []string
	cmds  []pipeline.Command
}

func newBatch(name, title string, cmds []pipeline.Command) *batchStep {
	return &batchStep{
		name:  name,
		title: title,
		deps:  outerDeps(cmds),
		cmds:  cmds,
	}
}

func (b *batchStep) Meta() pipeline.Meta {
	return pipeline.Meta{
		Name:  b.name,
		Title: b.title,
		Deps:  b.deps,
		Kind:  pipeline.Auto,
	}
}

func (b *batchStep) Check(ctx context.Context) (bool, error) {
	for _, c := range b.cmds {
		ok, err := c.Check(ctx)
		if err != nil {
			if c.Meta().Optional {
				continue
			}
			return false, fmt.Errorf("batch %s: %s check: %w", b.name, c.Meta().Name, err)
		}
		if !ok {
			return false, nil
		}
	}
	return true, nil
}

func (b *batchStep) Run(ctx context.Context, out chan<- pipeline.Event, _ <-chan pipeline.Result) error {
	var outMu sync.Mutex
	forward := func(ev pipeline.Event) {
		outMu.Lock()
		out <- ev
		outMu.Unlock()
	}

	waves, err := b.waves()
	if err != nil {
		return err
	}
	for _, wave := range waves {
		grp, gctx := errgroup.WithContext(ctx)
		for _, c := range wave {
			grp.Go(func() error {
				return b.runOne(gctx, c, forward)
			})
		}
		if err := grp.Wait(); err != nil {
			return err
		}
		if err := ctx.Err(); err != nil {
			return err
		}
	}
	return nil
}

func (b *batchStep) runOne(ctx context.Context, c pipeline.Command, forward func(pipeline.Event)) error {
	m := c.Meta()
	if ok, err := c.Check(ctx); err != nil {
		if m.Optional {
			forward(pipeline.Event{Kind: pipeline.EvSkipped, Step: m.Name, Err: err})
			return nil
		}
		return fmt.Errorf("batch %s: %s check: %w", b.name, m.Name, err)
	} else if ok {
		forward(pipeline.Event{Kind: pipeline.EvDone, Step: m.Name, Line: pipeline.LineSatisfied})
		return nil
	}

	forward(pipeline.Event{Kind: pipeline.EvStepStarted, Step: m.Name})
	stepOut := make(chan pipeline.Event, 64)
	errCh := make(chan error, 1)
	go func() {
		errCh <- c.Run(ctx, stepOut, nil)
		close(stepOut)
	}()
	for ev := range stepOut {
		forward(ev)
	}
	if err := <-errCh; err != nil {
		if m.Optional {
			forward(pipeline.Event{Kind: pipeline.EvSkipped, Step: m.Name, Err: err})
			return nil
		}
		forward(pipeline.Event{Kind: pipeline.EvFailed, Step: m.Name, Err: err})
		return fmt.Errorf("batch %s: %s: %w", b.name, m.Name, err)
	}
	forward(pipeline.Event{Kind: pipeline.EvDone, Step: m.Name})
	return nil
}

func (b *batchStep) waves() ([][]pipeline.Command, error) {
	inner := make(map[string]bool, len(b.cmds))
	for _, c := range b.cmds {
		inner[c.Meta().Name] = true
	}
	done := make(map[string]bool, len(b.cmds))
	remaining := make([]pipeline.Command, len(b.cmds))
	copy(remaining, b.cmds)

	var waves [][]pipeline.Command
	for len(remaining) > 0 {
		var wave, blocked []pipeline.Command
		for _, c := range remaining {
			if innerDepsSatisfied(c.Meta(), inner, done) {
				wave = append(wave, c)
			} else {
				blocked = append(blocked, c)
			}
		}
		if len(wave) == 0 {
			return nil, fmt.Errorf("batch %s: %d steps have an unsatisfiable inner dependency cycle", b.name, len(blocked))
		}
		for _, c := range wave {
			done[c.Meta().Name] = true
		}
		waves = append(waves, wave)
		remaining = blocked
	}
	return waves, nil
}

func innerDepsSatisfied(m pipeline.Meta, inner, done map[string]bool) bool {
	for _, dep := range m.Deps {
		if inner[dep] && !done[dep] {
			return false
		}
	}
	return true
}

func outerDeps(cmds []pipeline.Command) []string {
	inner := make(map[string]bool, len(cmds))
	for _, c := range cmds {
		inner[c.Meta().Name] = true
	}
	seen := make(map[string]bool)
	var deps []string
	for _, c := range cmds {
		for _, dep := range c.Meta().Deps {
			if inner[dep] || seen[dep] {
				continue
			}
			seen[dep] = true
			deps = append(deps, dep)
		}
	}
	sort.Strings(deps)
	return deps
}
