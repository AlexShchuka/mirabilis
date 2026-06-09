package plugins

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/AlexShchuka/mirabilis/internal/runner"
)

func TestCheckSkipsWhenSetsEqual(t *testing.T) {
	tests := []struct {
		name          string
		containerOut  string
		hostDisabled  []string
		wantSatisfied bool
	}{
		{
			name:          "both empty",
			containerOut:  "",
			hostDisabled:  nil,
			wantSatisfied: true,
		},
		{
			name:          "same single entry",
			containerOut:  "plugin-a\n",
			hostDisabled:  []string{"plugin-a"},
			wantSatisfied: true,
		},
		{
			name:          "same set different order",
			containerOut:  "plugin-b\nplugin-a\n",
			hostDisabled:  []string{"plugin-a", "plugin-b"},
			wantSatisfied: true,
		},
		{
			name:          "container has more",
			containerOut:  "plugin-a\nplugin-b\n",
			hostDisabled:  []string{"plugin-a"},
			wantSatisfied: false,
		},
		{
			name:          "host has more",
			containerOut:  "plugin-a\n",
			hostDisabled:  []string{"plugin-a", "plugin-b"},
			wantSatisfied: false,
		},
		{
			name:          "different entries",
			containerOut:  "plugin-a\n",
			hostDisabled:  []string{"plugin-b"},
			wantSatisfied: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			if len(tt.hostDisabled) > 0 {
				if err := os.MkdirAll(dir, 0o755); err != nil {
					t.Fatal(err)
				}
				csv := ""
				for i, p := range tt.hostDisabled {
					if i > 0 {
						csv += ","
					}
					csv += p
				}
				if err := os.WriteFile(filepath.Join(dir, ".env"),
					[]byte("PLUGINS_DISABLED="+csv+"\n"), 0o644); err != nil {
					t.Fatal(err)
				}
			}

			r := &runner.FakeRunner{
				RepoVal: dir,
				ContFunc: func(args []string) (string, error) {
					return tt.containerOut, nil
				},
			}

			s := step{}
			got, err := s.Check(context.Background(), r)
			if err != nil {
				t.Fatalf("Check returned error: %v", err)
			}
			if got != tt.wantSatisfied {
				t.Errorf("Check = %v, want %v", got, tt.wantSatisfied)
			}
		})
	}
}

func TestSetsEqual(t *testing.T) {
	tests := []struct {
		a, b []string
		want bool
	}{
		{nil, nil, true},
		{[]string{}, nil, true},
		{[]string{"a"}, []string{"a"}, true},
		{[]string{"a", "b"}, []string{"b", "a"}, true},
		{[]string{"a"}, []string{"b"}, false},
		{[]string{"a", "b"}, []string{"a"}, false},
	}
	for _, tt := range tests {
		if got := setsEqual(tt.a, tt.b); got != tt.want {
			t.Errorf("setsEqual(%v, %v) = %v, want %v", tt.a, tt.b, got, tt.want)
		}
	}
}
