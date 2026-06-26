package screens

import (
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/AlexShchuka/mirabilis/internal/bus"
	"github.com/AlexShchuka/mirabilis/internal/tui/router"
	uistr "github.com/AlexShchuka/mirabilis/internal/tui/strings"
	"github.com/AlexShchuka/mirabilis/internal/tui/styles"
)

const (
	tileInnerWidth = 24
	tileOuterWidth = tileInnerWidth + 4
	tileGap        = 1
)

type RoleOption struct {
	Key     string
	Effort  string
	Batch   bool
	Default bool
}

type RolePicker struct {
	id      bus.NodeID
	options []RoleOption
	cursor  int
	winW    int
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

func (r RolePicker) columns() int {
	if r.winW <= 0 {
		return 1
	}
	cols := (r.winW + tileGap) / (tileOuterWidth + tileGap)
	if cols < 1 {
		return 1
	}
	if cols > len(r.options) && len(r.options) > 0 {
		return len(r.options)
	}
	return cols
}

func (r RolePicker) move(delta int) RolePicker {
	target := r.cursor + delta
	if target >= 0 && target < len(r.options) {
		r.cursor = target
	}
	return r
}

func (r RolePicker) Update(msg tea.Msg) (router.Screen, tea.Cmd) {
	switch msg := msg.(type) {
	case bus.Envelope:
		return r.Update(msg.Msg)
	case tea.WindowSizeMsg:
		r.winW = msg.Width
		return r, nil
	case tea.KeyPressMsg:
		switch msg.String() {
		case "left", "h":
			return r.move(-1), nil
		case "right", "l":
			return r.move(1), nil
		case "up", "k":
			return r.move(-r.columns()), nil
		case "down", "j":
			return r.move(r.columns()), nil
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
	case uistr.RoleLabelSpark:
		return uistr.RoleEmojiSpark
	case uistr.RoleLabelDrift:
		return uistr.RoleEmojiDrift
	case uistr.RoleLabelOrbit:
		return uistr.RoleEmojiOrbit
	case uistr.RoleLabelForge:
		return uistr.RoleEmojiForge
	case uistr.RoleLabelNova:
		return uistr.RoleEmojiNova
	default:
		return uistr.RoleEmojiDefault
	}
}

func roleFacts(o RoleOption) string {
	var parts []string
	if o.Effort != "" {
		parts = append(parts, uistr.RoleFactEffort+o.Effort)
	}
	if o.Batch {
		parts = append(parts, uistr.RoleFactFleet)
	} else {
		parts = append(parts, uistr.RoleFactSolo)
	}
	return strings.Join(parts, uistr.RoleFactSep)
}

func (r RolePicker) tile(i int, o RoleOption) string {
	name := roleEmoji(o.Key) + " " + o.Key
	if o.Default {
		name += uistr.RoleDefaultSuffix
	}
	titleStyle := styles.NormTitle
	border := styles.Tile
	if i == r.cursor {
		titleStyle = styles.SelTitle
		border = styles.TileActive
	}
	body := titleStyle.Render(name) + "\n" + styles.Hint.Render(roleFacts(o))
	return border.Width(tileInnerWidth).Render(body)
}

func (r RolePicker) grid() string {
	cols := r.columns()
	var rows []string
	for start := 0; start < len(r.options); start += cols {
		end := start + cols
		if end > len(r.options) {
			end = len(r.options)
		}
		cells := make([]string, 0, end-start)
		for i := start; i < end; i++ {
			cells = append(cells, r.tile(i, r.options[i]))
		}
		rows = append(rows, joinRow(cells))
	}
	return strings.Join(rows, "\n")
}

func joinRow(cells []string) string {
	if len(cells) == 0 {
		return ""
	}
	gap := lipgloss.NewStyle().Width(tileGap).Render("")
	joined := cells[0]
	for _, c := range cells[1:] {
		joined = lipgloss.JoinHorizontal(lipgloss.Top, joined, gap, c)
	}
	return joined
}

func (r RolePicker) View() string {
	var b strings.Builder
	b.WriteString(" " + styles.Title.Render(uistr.RolePickerTitle) + "\n\n")
	b.WriteString(r.grid())
	b.WriteString("\n\n " + styles.Hint.Render(uistr.RolePickerHint))
	return b.String()
}
