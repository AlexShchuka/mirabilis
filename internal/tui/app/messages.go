package app

import (
	tea "charm.land/bubbletea/v2"

	"github.com/AlexShchuka/mirabilis/internal/engine/pipeline"
	"github.com/AlexShchuka/mirabilis/internal/obs"
)

type statusMsg obs.Snapshot

type pipelineEventMsg struct {
	ev pipeline.Event
}

type pipelineDoneMsg struct {
	failed bool
}

type execDoneMsg struct {
	step string
	err  error
}

type vscodeDoneMsg struct {
	err error
}

type gatePacksDoneMsg struct {
	err error
}

type selfUpdateDoneMsg struct {
	err error
}

type openURLDoneMsg struct {
	err error
}

type copyDoneMsg struct {
	text string
	err  error
}

func watchStatus(ch <-chan obs.Snapshot) tea.Cmd {
	return func() tea.Msg {
		snap, ok := <-ch
		if !ok {
			return nil
		}
		return statusMsg(snap)
	}
}

func pumpEvents(ch <-chan pipeline.Event) tea.Cmd {
	return func() tea.Msg {
		ev, ok := <-ch
		if !ok {
			return pipelineDoneMsg{}
		}
		return pipelineEventMsg{ev: ev}
	}
}
