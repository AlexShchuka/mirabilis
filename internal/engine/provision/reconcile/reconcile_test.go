package reconcile

import (
	"errors"
	"testing"
)

func TestMissingInstallsOnlyAbsent(t *testing.T) {
	t.Parallel()
	var got []int
	err := Missing([]int{1, 2, 3}, map[int]bool{2: true}, func(n int) error {
		got = append(got, n)
		return nil
	})
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if len(got) != 2 || got[0] != 1 || got[1] != 3 {
		t.Fatalf("installed %v, want [1 3] (2 already present)", got)
	}
}

func TestMissingJoinsErrors(t *testing.T) {
	t.Parallel()
	boom := errors.New("boom")
	err := Missing([]string{"a", "b"}, nil, func(string) error { return boom })
	if err == nil {
		t.Fatal("want joined error, got nil")
	}
}
