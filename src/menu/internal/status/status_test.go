package status

import (
	"encoding/json"
	"testing"
)

func TestUnmarshal(t *testing.T) {
	cases := []struct {
		name  string
		input string
		want  Status
	}{
		{"valid", `{"commitsBehind":3,"stale":true,"harness":"missing"}`, Status{CommitsBehind: 3, Stale: true, Harness: "missing"}},
		{"empty object", `{}`, Status{}},
		{"unknown harness", `{"harness":"unknown"}`, Status{Harness: "unknown"}},
		{"partial", `{"stale":true}`, Status{Stale: true}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			var s Status
			if err := json.Unmarshal([]byte(c.input), &s); err != nil {
				t.Fatalf("unmarshal %q: %v", c.input, err)
			}
			if s != c.want {
				t.Errorf("unmarshal %q = %+v, want %+v", c.input, s, c.want)
			}
		})
	}
}

func TestUnmarshalGarbage(t *testing.T) {
	for _, input := range []string{"", "not json", "[1,2,3]", "null"} {
		var s Status
		_ = json.Unmarshal([]byte(input), &s)
		if s != (Status{}) {
			t.Errorf("garbage %q left non-zero status %+v", input, s)
		}
	}
}
