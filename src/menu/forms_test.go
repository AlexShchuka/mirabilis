package main

import (
	"reflect"
	"testing"
)

func TestSplitCSV(t *testing.T) {
	tests := []struct {
		give string
		want []string
	}{
		{give: "", want: nil},
		{give: "   ", want: nil},
		{give: "a", want: []string{"a"}},
		{give: "a,b,c", want: []string{"a", "b", "c"}},
		{give: " a , b ,c ", want: []string{"a", "b", "c"}},
		{give: "a,,b,", want: []string{"a", "b"}},
		{give: ",", want: []string{}},
	}
	for _, tt := range tests {
		if got := splitCSV(tt.give); !reflect.DeepEqual(got, tt.want) {
			t.Errorf("splitCSV(%q) = %#v, want %#v", tt.give, got, tt.want)
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
