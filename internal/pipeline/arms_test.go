package pipeline

import (
	"testing"
)

func TestCheckCmd_NilImpl_ReportsUnsatisfied(t *testing.T) {
	r := reg(StepMeta{Name: "noimpl"}, nil)
	p := trivialPipeline([]Registered{r})
	msg, ok := p.checkCmd(&r)().(CheckedMsg)
	if !ok {
		t.Fatal("checkCmd did not return a CheckedMsg")
	}
	if msg.Name != "noimpl" {
		t.Errorf("name = %q, want noimpl", msg.Name)
	}
	if msg.Satisfied {
		t.Error("a nil-Impl step must report Satisfied=false so it is treated as unmet")
	}
	if msg.Err != nil {
		t.Errorf("err = %v, want nil for a nil-Impl step", msg.Err)
	}
}

func TestOnChecked_UnknownName_Advances(t *testing.T) {
	p := trivialPipeline([]Registered{reg(StepMeta{Name: "s"}, funcStep{})})
	cmd := p.onChecked(CheckedMsg{Name: "ghost"})
	if cmd == nil {
		t.Error("onChecked for an unknown name should still advance the pipeline")
	}
	if p.failed {
		t.Error("an unknown CheckedMsg must not mark the pipeline failed")
	}
}

func TestOnRan_UnknownName_Advances(t *testing.T) {
	p := trivialPipeline([]Registered{reg(StepMeta{Name: "s"}, funcStep{})})
	cmd := p.onRan(RanMsg{Name: "ghost"})
	if cmd == nil {
		t.Error("onRan for an unknown name should still advance the pipeline")
	}
	if p.failed {
		t.Error("an unknown RanMsg must not mark the pipeline failed")
	}
}
