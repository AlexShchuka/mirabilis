package steps

import (
	"context"
	"fmt"
	"sync"

	"github.com/AlexShchuka/mirabilis/internal/engine/pipeline"
)

type batchStep struct {
	name  string
	title string
	deps  []string
	cmds  []pipeline.Command
}

func newBatch(name, title string, outerDeps []string, cmds []pipeline.Command) *batchStep {
	return &batchStep{name: name, title: title, deps: outerDeps, cmds: cmds}
}

type batchResult struct {
	name     string
	optional bool
	err      error
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
			return false, fmt.Errorf("batch %s: step %s check: %w", b.name, c.Meta().Name, err)
		}
		if !ok {
			return false, nil
		}
	}
	return true, nil
}

func (b *batchStep) innerName(name string) bool {
	for _, c := range b.cmds {
		if c.Meta().Name == name {
			return true
		}
	}
	return false
}

func (b *batchStep) runOne(ctx context.Context, c pipeline.Command, fanIn chan<- pipeline.Event, results chan<- batchResult) {
	m := c.Meta()
	stepOut := make(chan pipeline.Event, 64)
	errCh := make(chan error, 1)
	go func() {
		err := c.Run(ctx, stepOut, nil)
		close(stepOut)
		errCh <- err
	}()
	for ev := range stepOut {
		fanIn <- ev
	}
	results <- batchResult{name: m.Name, optional: m.Optional, err: <-errCh}
}

func (b *batchStep) Run(ctx context.Context, out chan<- pipeline.Event, _ <-chan pipeline.Result) error {
	doneSet := make(map[string]bool, len(b.cmds))
	var dmu sync.Mutex

	isDone := func(name string) bool {
		dmu.Lock()
		defer dmu.Unlock()
		return doneSet[name]
	}
	markDone := func(name string) {
		dmu.Lock()
		defer dmu.Unlock()
		doneSet[name] = true
	}

	var outMu sync.Mutex
	safeSend := func(ev pipeline.Event) {
		outMu.Lock()
		out <- ev
		outMu.Unlock()
	}

	remaining := make([]pipeline.Command, len(b.cmds))
	copy(remaining, b.cmds)

	for len(remaining) > 0 {
		if ctx.Err() != nil {
			return ctx.Err()
		}

		var ready, notReady []pipeline.Command
		for _, c := range remaining {
			m := c.Meta()
			ok := true
			for _, dep := range m.Deps {
				if !isDone(dep) && b.innerName(dep) {
					ok = false
					break
				}
			}
			if ok {
				ready = append(ready, c)
			} else {
				notReady = append(notReady, c)
			}
		}

		if len(ready) == 0 {
			if len(notReady) > 0 {
				return fmt.Errorf("batch %s: %d steps have unsatisfied deps within batch", b.name, len(notReady))
			}
			break
		}

		fanIn := make(chan pipeline.Event, 64)
		results := make(chan batchResult, len(ready))

		for _, c := range ready {
			c := c
			go b.runOne(ctx, c, fanIn, results)
		}

		fanDone := make(chan struct{})
		go func() {
			defer close(fanDone)
			for ev := range fanIn {
				safeSend(ev)
			}
		}()

		var firstErr error
		for range ready {
			r := <-results
			markDone(r.name)
			if r.err != nil && !r.optional && firstErr == nil {
				firstErr = fmt.Errorf("batch %s: step %s: %w", b.name, r.name, r.err)
			}
		}
		close(fanIn)
		<-fanDone

		if firstErr != nil {
			return firstErr
		}

		remaining = notReady
	}
	return nil
}
