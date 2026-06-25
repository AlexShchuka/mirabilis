package screens

import (
	"testing"
)

func TestMenuNotice(t *testing.T) {
	m := NewMenu("app/menu").WithNotice("test notice")
	if m.Notice() != "test notice" {
		t.Errorf("Notice() = %q, want %q", m.Notice(), "test notice")
	}
}

func TestMenuNoticeEmpty(t *testing.T) {
	m := NewMenu("app/menu")
	if m.Notice() != "" {
		t.Errorf("Notice() = %q, want empty", m.Notice())
	}
}
