package forms

import (
	"reflect"
	"testing"
)

func TestSplitCSV(t *testing.T) {
	cases := []struct {
		in   string
		want []string
	}{
		{"", nil},
		{"   ", nil},
		{"a", []string{"a"}},
		{"a,b,c", []string{"a", "b", "c"}},
		{" a , b ,c ", []string{"a", "b", "c"}},
		{"a,,b,", []string{"a", "b"}},
		{",", []string{}},
	}
	for _, c := range cases {
		if got := splitCSV(c.in); !reflect.DeepEqual(got, c.want) {
			t.Errorf("splitCSV(%q) = %#v, want %#v", c.in, got, c.want)
		}
	}
}

func TestJoinCSVRoundTrip(t *testing.T) {
	items := []string{"a", "b", "c"}
	if got := joinCSV(items); got != "a,b,c" {
		t.Errorf("joinCSV(%#v) = %q, want %q", items, got, "a,b,c")
	}
	if got := splitCSV(joinCSV(items)); !reflect.DeepEqual(got, items) {
		t.Errorf("round trip = %#v, want %#v", got, items)
	}
}

func TestContains(t *testing.T) {
	haystack := []string{"go", "dotnet"}
	if !contains(haystack, "go") {
		t.Error("contains should find present element")
	}
	if contains(haystack, "python") {
		t.Error("contains should not find absent element")
	}
	if contains(nil, "go") {
		t.Error("contains on nil should be false")
	}
}
