package bus

import "testing"

func TestNodeIDChild(t *testing.T) {
	tests := []struct {
		name  string
		n     NodeID
		child string
		want  NodeID
	}{
		{name: "root", n: "", child: "menu", want: "menu"},
		{name: "single", n: "app", child: "menu", want: "app/menu"},
		{name: "nested", n: "app/launch", child: "steplist", want: "app/launch/steplist"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.n.Child(tt.child); got != tt.want {
				t.Errorf("NodeID(%q).Child(%q) = %q, want %q", tt.n, tt.child, got, tt.want)
			}
		})
	}
}

func TestNodeIDContains(t *testing.T) {
	tests := []struct {
		name   string
		n      NodeID
		target NodeID
		want   bool
	}{
		{name: "root broadcast", n: "", target: "app/launch/steplist", want: true},
		{name: "root contains root", n: "", target: "", want: true},
		{name: "exact", n: "app/launch", target: "app/launch", want: true},
		{name: "descendant", n: "app", target: "app/launch/steplist", want: true},
		{name: "non-descendant", n: "app/launch", target: "app/telegram", want: false},
		{name: "prefix collision", n: "app/la", target: "app/launch", want: false},
		{name: "ancestor not contained", n: "app/launch", target: "app", want: false},
		{name: "non-root excludes root", n: "app", target: "", want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.n.Contains(tt.target); got != tt.want {
				t.Errorf("NodeID(%q).Contains(%q) = %v, want %v", tt.n, tt.target, got, tt.want)
			}
		})
	}
}
