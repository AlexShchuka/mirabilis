package sandbox

import (
	"context"
	"errors"
	"sync"
)

type inspectStub struct {
	err error
	c   Container
}

type FakeEvents struct {
	events chan ContainerEvent
	errs   chan error
}

func NewFakeEvents() *FakeEvents {
	return &FakeEvents{
		events: make(chan ContainerEvent, eventBuffer),
		errs:   make(chan error, 1),
	}
}

func (s *FakeEvents) Emit(action string) { s.events <- ContainerEvent{Action: action} }

func (s *FakeEvents) Fail(err error) { s.errs <- err }

func (s *FakeEvents) Close() { close(s.events) }

type FakeDocker struct {
	mu           sync.Mutex
	inspects     []inspectStub
	sessions     []*FakeEvents
	inspectCalls int
	eventsCalls  int
}

var _ Docker = (*FakeDocker)(nil)

func NewFakeDocker() *FakeDocker { return &FakeDocker{} }

func (f *FakeDocker) StubInspect(c Container, err error) *FakeDocker {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.inspects = append(f.inspects, inspectStub{c: c, err: err})
	return f
}

func (f *FakeDocker) StubEvents(s *FakeEvents) *FakeDocker {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.sessions = append(f.sessions, s)
	return f
}

func (f *FakeDocker) InspectCalls() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.inspectCalls
}

func (f *FakeDocker) EventsCalls() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.eventsCalls
}

func (f *FakeDocker) Inspect(_ context.Context) (Container, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.inspectCalls++
	if len(f.inspects) == 0 {
		return Container{}, errors.New("sandbox: fake docker: no inspect stub")
	}
	stub := f.inspects[0]
	if len(f.inspects) > 1 {
		f.inspects = f.inspects[1:]
	}
	return stub.c, stub.err
}

func (f *FakeDocker) Events(_ context.Context) (<-chan ContainerEvent, <-chan error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.eventsCalls++
	if len(f.sessions) == 0 {
		s := NewFakeEvents()
		return s.events, s.errs
	}
	s := f.sessions[0]
	if len(f.sessions) > 1 {
		f.sessions = f.sessions[1:]
	}
	return s.events, s.errs
}
