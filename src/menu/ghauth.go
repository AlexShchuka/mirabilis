package main

import (
	"bufio"
	"context"
	"os"
	"os/exec"
	"regexp"
	"runtime"
	"strings"

	"charm.land/bubbles/v2/spinner"
	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
)

type ghLineMsg string

type ghExitMsg struct{ err error }

type browserMsg struct{ err error }

type ghAuthModel struct {
	ctx  context.Context
	r    Runner
	w, h int

	spin spinner.Model
	vp   viewport.Model

	code   string
	url    string
	opened bool
	status string

	finished bool
	err      error

	lines   []string
	linesCh chan string
	doneCh  chan error
}

func newGHAuth(ctx context.Context, r Runner, w, h int) *ghAuthModel {
	vp := viewport.New()
	vp.SetWidth(max(1, w-4))
	vp.SetHeight(max(3, h-9))
	return &ghAuthModel{
		ctx:    ctx,
		r:      r,
		w:      w,
		h:      h,
		spin:   spinner.New(spinner.WithSpinner(spinner.MiniDot)),
		vp:     vp,
		status: "запрашиваю код устройства у GitHub…",
	}
}

func (g *ghAuthModel) Init() tea.Cmd {
	g.linesCh = make(chan string)
	g.doneCh = make(chan error, 1)
	return tea.Batch(g.spin.Tick, g.launch())
}

func (g *ghAuthModel) launch() tea.Cmd {
	return func() tea.Msg {
		go g.run()
		return g.readNext()
	}
}

func (g *ghAuthModel) readNext() tea.Msg {
	line, ok := <-g.linesCh
	if !ok {
		return ghExitMsg{err: <-g.doneCh}
	}
	return ghLineMsg(line)
}

func (g *ghAuthModel) run() {
	cmd := containerCmd(g.r, "env", "BROWSER=true",
		"gh", "auth", "login", "--hostname", "github.com", "--git-protocol", "https", "--web")
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
	sc := bufio.NewScanner(pr)
	for sc.Scan() {
		g.linesCh <- sc.Text()
	}
	pr.Close()
	g.doneCh <- cmd.Wait()
	close(g.linesCh)
}

func (g *ghAuthModel) waitLine() tea.Cmd {
	return func() tea.Msg { return g.readNext() }
}

func (g *ghAuthModel) Update(msg tea.Msg) (*ghAuthModel, tea.Cmd) {
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
	case ghLineMsg:
		return g, g.onLine(string(m))
	case browserMsg:
		if m.err != nil {
			g.status = "не смог открыть браузер — открой URL вручную"
		}
		return g, nil
	case ghExitMsg:
		g.finished = true
		g.err = m.err
		return g, emit(ghDoneMsg{err: m.err})
	}
	return g, nil
}

func (g *ghAuthModel) onLine(line string) tea.Cmd {
	g.lines = append(g.lines, line)
	g.vp.SetContent(strings.Join(g.lines, "\n"))
	g.vp.GotoBottom()
	cmds := []tea.Cmd{g.waitLine()}
	if c := parseUserCode(line); c != "" {
		g.code = c
	}
	if u := parseDeviceURL(line); u != "" {
		g.url = u
	}
	if g.code != "" && g.url != "" && !g.opened {
		g.opened = true
		g.status = "браузер открыт на хосте — подтверди вход, введи код"
		cmds = append(cmds, openBrowserCmd(g.url))
	}
	return tea.Batch(cmds...)
}

func (g *ghAuthModel) View() string {
	var b strings.Builder
	b.WriteString(selTitle.Render("GitHub sign-in") + "\n\n")
	if g.code != "" {
		b.WriteString("  код:  " + selTitle.Render(g.code) + "\n")
		b.WriteString("  URL:  " + g.url + "\n\n")
	}
	b.WriteString(g.spin.View() + " " + g.status + "\n\n")
	b.WriteString(offStyle.Render(g.vp.View()))
	return b.String()
}

var (
	reUserCode  = regexp.MustCompile(`\b[A-Z0-9]{4}-[A-Z0-9]{4}\b`)
	reDeviceURL = regexp.MustCompile(`https://\S*?/login/device\S*`)
)

func parseUserCode(line string) string {
	return reUserCode.FindString(line)
}

func parseDeviceURL(line string) string {
	return strings.TrimRight(reDeviceURL.FindString(line), ".,)")
}

func openBrowserCmd(url string) tea.Cmd {
	return func() tea.Msg {
		return browserMsg{err: openHostBrowser(url)}
	}
}

func openHostBrowser(url string) error {
	name := "xdg-open"
	if runtime.GOOS == "darwin" {
		name = "open"
	}
	return exec.Command(name, url).Start()
}
