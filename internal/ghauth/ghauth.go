package ghauth

import (
	"bufio"
	"context"
	"io"
	"os"
	"os/exec"
	"regexp"
	gort "runtime"
	"strings"

	"charm.land/bubbles/v2/spinner"
	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"

	"github.com/AlexShchuka/mirabilis/internal/runner"
	"github.com/AlexShchuka/mirabilis/internal/runtime"
	"github.com/AlexShchuka/mirabilis/internal/ui"
)

type LineMsg string

type ExitMsg struct{ Err error }

type browserMsg struct{ err error }

type DoneMsg struct{ Err error }

type Model struct {
	ctx      context.Context
	r        runner.Runner
	err      error
	doneCh   chan error
	linesCh  chan string
	status   string
	code     string
	url      string
	lines    []string
	vp       viewport.Model
	spin     spinner.Model
	h        int
	w        int
	opened   bool
	finished bool
}

func New(ctx context.Context, r runner.Runner, w, h int) *Model {
	vp := viewport.New()
	vp.SetWidth(max(1, w-4))
	vp.SetHeight(max(3, h-9))
	return &Model{
		ctx:    ctx,
		r:      r,
		w:      w,
		h:      h,
		spin:   spinner.New(spinner.WithSpinner(spinner.MiniDot)),
		vp:     vp,
		status: ui.GHAuthStatusWaiting,
	}
}

func (g *Model) Init() tea.Cmd {
	g.linesCh = make(chan string)
	g.doneCh = make(chan error, 1)
	return tea.Batch(g.spin.Tick, g.launch())
}

func (g *Model) launch() tea.Cmd {
	return func() tea.Msg {
		go g.run()
		return g.readNext()
	}
}

func (g *Model) readNext() tea.Msg {
	line, ok := <-g.linesCh
	if !ok {
		return ExitMsg{Err: <-g.doneCh}
	}
	return LineMsg(line)
}

func loginArgs() []string {
	return []string{"env", "BROWSER=true",
		"gh", "auth", "login", "--hostname", "github.com",
		"--git-protocol", "https", "--web", "--scopes", "workflow",
		"--insecure-storage"}
}

func (g *Model) run() {
	cmd := runtime.ContainerCmd(g.ctx, g.r, loginArgs()...)
	cmd.Stdin = strings.NewReader("\n")
	pr, pw, err := os.Pipe()
	if err != nil {
		g.doneCh <- err
		close(g.linesCh)
		return
	}
	cmd.Stdout, cmd.Stderr = pw, pw
	if startErr := cmd.Start(); startErr != nil {
		pw.Close()
		pr.Close()
		g.doneCh <- startErr
		close(g.linesCh)
		return
	}
	pw.Close()
	g.pump(pr, cmd.Wait)
}

func (g *Model) pump(r io.ReadCloser, wait func() error) {
	sc := bufio.NewScanner(r)
	for sc.Scan() {
		select {
		case <-g.ctx.Done():
			r.Close()
			_ = wait()
			g.doneCh <- g.ctx.Err()
			close(g.linesCh)
			return
		case g.linesCh <- sc.Text():
		}
	}
	r.Close()
	g.doneCh <- wait()
	close(g.linesCh)
}

func (g *Model) waitLine() tea.Cmd {
	return func() tea.Msg { return g.readNext() }
}

func (g *Model) Update(msg tea.Msg) (*Model, tea.Cmd) {
	switch m := msg.(type) {
	case tea.WindowSizeMsg:
		g.w, g.h = m.Width, m.Height
		g.vp.SetWidth(max(1, m.Width-4))
		g.vp.SetHeight(max(3, m.Height-9))
		return g, nil
	case spinner.TickMsg:
		var cmd tea.Cmd
		g.spin, cmd = g.spin.Update(m)
		return g, cmd
	case LineMsg:
		return g, g.onLine(string(m))
	case browserMsg:
		if m.err != nil {
			g.status = ui.GHAuthStatusNoOpen
		}
		return g, nil
	case ExitMsg:
		g.finished = true
		g.err = m.Err
		return g, emit(DoneMsg(m))
	}
	return g, nil
}

func (g *Model) onLine(line string) tea.Cmd {
	g.lines = append(g.lines, line)
	g.vp.SetContent(strings.Join(g.lines, "\n"))
	g.vp.GotoBottom()
	cmds := []tea.Cmd{g.waitLine()}
	if c := ParseUserCode(line); c != "" {
		g.code = c
	}
	if u := ParseDeviceURL(line); u != "" {
		g.url = u
	}
	if g.code != "" && g.url != "" && !g.opened {
		g.opened = true
		g.status = ui.GHAuthStatusOpened
		cmds = append(cmds, openBrowserCmd(g.url))
	}
	return tea.Batch(cmds...)
}

func (g *Model) View() string {
	var b strings.Builder
	b.WriteString(ui.SelTitleStyle.Render(ui.GHAuthTitle) + "\n\n")
	if g.code != "" {
		b.WriteString(ui.GHAuthLabelCode + ui.SelTitleStyle.Render(g.code) + "\n")
		b.WriteString(ui.GHAuthLabelURL + g.url + "\n\n")
	}
	b.WriteString(g.spin.View() + " " + g.status + "\n\n")
	b.WriteString(ui.OffStyle.Render(g.vp.View()))
	return b.String()
}

var (
	reUserCode  = regexp.MustCompile(`\b[A-Z0-9]{4}-[A-Z0-9]{4}\b`)
	reDeviceURL = regexp.MustCompile(`https://\S*?/login/device\S*`)
)

func ParseUserCode(line string) string {
	return reUserCode.FindString(line)
}

func ParseDeviceURL(line string) string {
	return strings.TrimRight(reDeviceURL.FindString(line), ".,)")
}

func openBrowserCmd(url string) tea.Cmd {
	return func() tea.Msg {
		return browserMsg{err: openHostBrowser(url)}
	}
}

func openHostBrowser(url string) error {
	name := "xdg-open"
	if gort.GOOS == "darwin" {
		name = "open"
	}
	if _, err := exec.LookPath(name); err != nil {
		if _, werr := exec.LookPath("wslview"); werr == nil {
			name = "wslview"
		}
	}
	return exec.Command(name, url).Start()
}

func emit(msg tea.Msg) tea.Cmd { return func() tea.Msg { return msg } }
