package router_test

import (
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/AlexShchuka/mirabilis/internal/bus"
	"github.com/AlexShchuka/mirabilis/internal/tui/router"
)

type fakeScreen struct {
	id     bus.NodeID
	got    []tea.Msg
	inited bool
}

func (f *fakeScreen) ID() bus.NodeID { return f.id }

func (f *fakeScreen) Init() tea.Cmd {
	f.inited = true
	return nil
}

func (f *fakeScreen) Update(msg tea.Msg) (router.Screen, tea.Cmd) {
	f.got = append(f.got, msg)
	return f, nil
}

func (f *fakeScreen) View() string { return string(f.id) }

type ping struct{}

func TestPushPop(t *testing.T) {
	root := &fakeScreen{id: "app/menu"}
	child := &fakeScreen{id: "app/launch"}
	m := router.New(root)
	m.Init()
	if !root.inited {
		t.Fatal("Init() must init the root screen")
	}

	m, _ = m.Update(bus.ScreenPush{Model: child})
	if m.Depth() != 2 || m.Top() != router.Screen(child) {
		t.Fatalf("after push: depth=%d top=%v", m.Depth(), m.Top().ID())
	}
	if !child.inited {
		t.Fatal("pushed screen must be inited")
	}
	if m.View() != "app/launch" {
		t.Fatalf("View() = %q, want top view", m.View())
	}

	m, _ = m.Update(bus.ScreenPop{})
	if m.Depth() != 1 || m.Top() != router.Screen(root) {
		t.Fatalf("after pop: depth=%d top=%v", m.Depth(), m.Top().ID())
	}

	m, _ = m.Update(bus.ScreenPop{})
	if m.Depth() != 1 {
		t.Fatalf("pop at root: depth=%d, want 1", m.Depth())
	}
}

func TestPushIgnoresNonScreen(t *testing.T) {
	m := router.New(&fakeScreen{id: "app/menu"})
	m, _ = m.Update(bus.ScreenPush{Model: 42})
	if m.Depth() != 1 {
		t.Fatalf("depth = %d, want 1", m.Depth())
	}
}

func TestKeysGoToTopOnly(t *testing.T) {
	root := &fakeScreen{id: "app/menu"}
	top := &fakeScreen{id: "app/launch"}
	m := router.New(root)
	m, _ = m.Update(bus.ScreenPush{Model: top})

	m, _ = m.Update(tea.KeyPressMsg{Code: 'q', Text: "q"})
	if len(top.got) != 1 {
		t.Fatalf("top got %d msgs, want 1", len(top.got))
	}
	if len(root.got) != 0 {
		t.Fatalf("root got %d msgs, want 0", len(root.got))
	}
	_ = m
}

func TestAddressing(t *testing.T) {
	tests := []struct {
		name     string
		to       bus.NodeID
		wantMenu int
		wantTop  int
	}{
		{name: "exact top", to: "app/launch", wantTop: 1},
		{name: "component under top", to: "app/launch/steplist", wantTop: 1},
		{name: "background screen", to: "app/menu", wantMenu: 1},
		{name: "no match dropped", to: "app/telegram", wantMenu: 0, wantTop: 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			menu := &fakeScreen{id: "app/menu"}
			top := &fakeScreen{id: "app/launch"}
			m := router.New(menu)
			m, _ = m.Update(bus.ScreenPush{Model: top})

			env := bus.Envelope{To: tt.to, Msg: ping{}}
			_, _ = m.Update(env)
			if len(menu.got) != tt.wantMenu {
				t.Errorf("menu got %d msgs, want %d", len(menu.got), tt.wantMenu)
			}
			if len(top.got) != tt.wantTop {
				t.Errorf("top got %d msgs, want %d", len(top.got), tt.wantTop)
			}
			if tt.wantTop == 1 {
				if got, ok := top.got[0].(bus.Envelope); !ok || got.To != tt.to {
					t.Errorf("top got %v, want the envelope", top.got[0])
				}
			}
		})
	}
}

func TestAddressedStopsAtFirstMatch(t *testing.T) {
	outer := &fakeScreen{id: "app"}
	inner := &fakeScreen{id: "app/launch"}
	m := router.New(outer)
	m, _ = m.Update(bus.ScreenPush{Model: inner})

	m, _ = m.Update(bus.Envelope{To: "app/launch", Msg: ping{}})
	if len(inner.got) != 1 {
		t.Fatalf("inner got %d msgs, want 1", len(inner.got))
	}
	if len(outer.got) != 0 {
		t.Fatalf("outer got %d msgs, want 0 (handled msg re-broadcast)", len(outer.got))
	}
	_ = m
}

func TestBroadcast(t *testing.T) {
	root := &fakeScreen{id: "app/menu"}
	top := &fakeScreen{id: "app/launch"}
	m := router.New(root)
	m, _ = m.Update(bus.ScreenPush{Model: top})

	m, _ = m.Update(bus.Envelope{Msg: ping{}})
	for _, s := range []*fakeScreen{root, top} {
		if len(s.got) != 1 {
			t.Fatalf("%s got %d msgs, want 1", s.id, len(s.got))
		}
		if _, ok := s.got[0].(ping); !ok {
			t.Fatalf("%s got %T, want unwrapped ping", s.id, s.got[0])
		}
	}

	_, _ = m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	for _, s := range []*fakeScreen{root, top} {
		if len(s.got) != 2 {
			t.Fatalf("%s got %d msgs, want window size broadcast", s.id, len(s.got))
		}
	}
}
