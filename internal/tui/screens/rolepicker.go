package screens

import (
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/AlexShchuka/mirabilis/internal/bus"
	"github.com/AlexShchuka/mirabilis/internal/tui/router"
	uistr "github.com/AlexShchuka/mirabilis/internal/tui/strings"
	"github.com/AlexShchuka/mirabilis/internal/tui/styles"
)

type RoleOption struct {
	Key     string
	Effort  string
	Harness bool
	Batch   bool
	Default bool
}

type RolePicker struct {
	id      bus.NodeID
	options []RoleOption
	cursor  int
}

var _ router.Screen = RolePicker{}

func NewRolePicker(id bus.NodeID, options []RoleOption) RolePicker {
	cursor := 0
	for i, o := range options {
		if o.Default {
			cursor = i
			break
		}
	}
	return RolePicker{id: id, options: options, cursor: cursor}
}

func (r RolePicker) ID() bus.NodeID { return r.id }

func (r RolePicker) Init() tea.Cmd { return nil }

func (r RolePicker) Update(msg tea.Msg) (router.Screen, tea.Cmd) {
	switch msg := msg.(type) {
	case bus.Envelope:
		return r.Update(msg.Msg)
	case tea.KeyPressMsg:
		switch msg.String() {
		case "up", "k":
			if r.cursor > 0 {
				r.cursor--
			}
			return r, nil
		case "down", "j":
			if r.cursor < len(r.options)-1 {
				r.cursor++
			}
			return r, nil
		case "enter":
			if len(r.options) == 0 {
				return r, nil
			}
			key := r.options[r.cursor].Key
			return r, func() tea.Msg { return bus.ScreenResult{Value: key} }
		case "esc":
			return r, func() tea.Msg { return bus.ScreenPop{} }
		}
	}
	return r, nil
}

func roleEmoji(key string) string {
	switch key {
	case uistr.RoleLabelGrind:
		return uistr.RoleEmojiGrind
	case uistr.RoleLabelRaid:
		return uistr.RoleEmojiRaid
	case uistr.RoleLabelPvP:
		return uistr.RoleEmojiPvP
	default:
		return uistr.RoleEmojiDefault
	}
}

func roleFacts(o RoleOption) string {
	var parts []string
	if o.Effort != "" {
		parts = append(parts, uistr.RoleFactEffort+o.Effort)
	}
	if o.Harness {
		parts = append(parts, uistr.RoleFactHarnessOn)
	} else {
		parts = append(parts, uistr.RoleFactHarnessOff)
	}
	if o.Batch {
		parts = append(parts, uistr.RoleFactBatch)
	}
	return strings.Join(parts, uistr.RoleFactSep)
}

func (r RolePicker) View() string {
	var b strings.Builder
	b.WriteString(" " + styles.Title.Render(uistr.RolePickerTitle) + "\n\n")
	for i, o := range r.options {
		label := roleEmoji(o.Key) + " " + o.Key
		if o.Default {
			label += uistr.RoleDefaultSuffix
		}
		titleStyle := styles.NormTitle
		if i == r.cursor {
			titleStyle = styles.SelTitle
		}
		b.WriteString(" " + titleStyle.Render(label) + "\n")
		b.WriteString("   " + styles.Hint.Render(roleFacts(o)) + "\n")
	}
	b.WriteString("\n " + styles.Hint.Render(uistr.RolePickerHint))
	return b.String()
}
