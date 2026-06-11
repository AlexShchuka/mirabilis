package app

import (
	"context"
	"errors"
	"os"
	"reflect"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/AlexShchuka/mirabilis/internal/runner"
	"github.com/AlexShchuka/mirabilis/internal/ui"
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

func TestFormScreenCompleted_Aborted(t *testing.T) {
	f := newResetForm(context.Background(), &runner.FakeRunner{}, 80, 24)
	if f.completed() {
		t.Error("freshly created form should not be completed")
	}
	if f.aborted() {
		t.Error("freshly created form should not be aborted")
	}
}

func TestFormScreenView_NonEmpty(t *testing.T) {
	f := newResetForm(context.Background(), &runner.FakeRunner{}, 80, 24)
	v := f.View()
	if v == "" {
		t.Error("formScreen.View() should return non-empty string")
	}
}

func TestFormScreenUpdate_CtrlCAborts(t *testing.T) {
	f := newResetForm(context.Background(), &runner.FakeRunner{}, 80, 24)
	f2, _ := f.Update(tea.KeyPressMsg{Code: 'c', Mod: tea.ModCtrl})
	if !f2.aborted() {
		t.Error("ctrl+c should set form to aborted state")
	}
}

func TestUpdateForm_AbortedEmitsBackToMenu(t *testing.T) {
	a := newTestApp()
	a.form = newResetForm(context.Background(), &runner.FakeRunner{}, 80, 24)
	a.phase = phaseForm

	a.form, _ = a.form.Update(tea.KeyPressMsg{Code: 'c', Mod: tea.ModCtrl})
	if !a.form.aborted() {
		t.Fatal("ctrl+c should abort the reset form")
	}
	_, cmd := a.updateForm(tea.KeyPressMsg{})
	if cmd == nil {
		t.Fatal("updateForm on aborted form should return non-nil cmd")
	}
	msg := cmd()
	if _, ok := msg.(backToMenuMsg); !ok {
		t.Errorf("updateForm on aborted emits %T, want backToMenuMsg", msg)
	}
}

func TestNewResetForm_Apply_NotConfirmed(t *testing.T) {
	f := newResetForm(context.Background(), &runner.FakeRunner{}, 80, 24)
	cmd := f.apply()
	if cmd == nil {
		t.Fatal("apply should return non-nil cmd")
	}
	msg := cmd()
	btm, ok := msg.(backToMenuMsg)
	if !ok {
		t.Fatalf("apply() when not confirmed emits %T, want backToMenuMsg", msg)
	}
	if btm.notice != "" {
		t.Errorf("apply() when not confirmed: notice = %q, want empty", btm.notice)
	}
}

func TestApplyHarness_Off(t *testing.T) {
	var gotArgs []string
	r := &runner.FakeRunner{
		ContFunc: func(args []string) (string, error) {
			gotArgs = args
			return "", nil
		},
	}
	if err := applyHarness(context.Background(), r, "off"); err != nil {
		t.Fatalf("applyHarness(off): %v", err)
	}
	if len(gotArgs) == 0 || !strings.Contains(strings.Join(gotArgs, " "), "skip") {
		t.Errorf("applyHarness(off) args = %v, want to write skip marker", gotArgs)
	}
}

func TestApplyHarness_On(t *testing.T) {
	var gotArgs []string
	r := &runner.FakeRunner{
		ContFunc: func(args []string) (string, error) {
			gotArgs = args
			return "", nil
		},
	}
	if err := applyHarness(context.Background(), r, "on"); err != nil {
		t.Fatalf("applyHarness(on): %v", err)
	}
	if len(gotArgs) == 0 || !strings.Contains(strings.Join(gotArgs, " "), "install") {
		t.Errorf("applyHarness(on) args = %v, want to write install marker", gotArgs)
	}
}

func TestApplyHarness_Unknown_NoError(t *testing.T) {
	r := &runner.FakeRunner{}
	if err := applyHarness(context.Background(), r, "badval"); err != nil {
		t.Errorf("applyHarness(unknown) = %v, want nil", err)
	}
}

func TestApplyHarness_Error(t *testing.T) {
	errBoom := errors.New("container error")
	r := &runner.FakeRunner{
		ContFunc: func(args []string) (string, error) {
			return "", errBoom
		},
	}
	if err := applyHarness(context.Background(), r, "on"); !errors.Is(err, errBoom) {
		t.Errorf("applyHarness(on) error = %v, want boom", err)
	}
}

func TestApplyHarness_Reinstall_TwoCalls(t *testing.T) {
	var calls []string
	r := &runner.FakeRunner{
		ContFunc: func(args []string) (string, error) {
			calls = append(calls, strings.Join(args, " "))
			return "", nil
		},
	}
	if err := applyHarness(context.Background(), r, "reinstall"); err != nil {
		t.Fatalf("applyHarness(reinstall): %v", err)
	}
	if len(calls) < 2 {
		t.Fatalf("applyHarness(reinstall) made %d container calls, want >= 2", len(calls))
	}
	if !strings.Contains(calls[0], "install") {
		t.Errorf("first container call = %q, want the install marker write", calls[0])
	}
}

func TestResetAllCmd_Success(t *testing.T) {
	dockerDir := t.TempDir()
	if err := os.WriteFile(dockerDir+"/docker", []byte("#!/bin/sh\nexit 0"), 0o755); err != nil {
		t.Fatal(err)
	}
	prependTestPath(t, dockerDir)

	r := &runner.FakeRunner{RepoVal: t.TempDir()}
	cmd := resetAllCmd(context.Background(), r)
	if cmd == nil {
		t.Fatal("resetAllCmd returned nil")
	}
	msg := cmd()
	btm, ok := msg.(backToMenuMsg)
	if !ok {
		t.Fatalf("resetAllCmd success emits %T, want backToMenuMsg", msg)
	}
	if btm.notice != ui.NoticeResetDone {
		t.Errorf("notice = %q, want %q", btm.notice, ui.NoticeResetDone)
	}
}

func TestResetAllCmd_Failure(t *testing.T) {
	dockerDir := t.TempDir()
	if err := os.WriteFile(dockerDir+"/docker", []byte("#!/bin/sh\nexit 1"), 0o755); err != nil {
		t.Fatal(err)
	}
	prependTestPath(t, dockerDir)

	r := &runner.FakeRunner{RepoVal: t.TempDir()}
	cmd := resetAllCmd(context.Background(), r)
	msg := cmd()
	btm, ok := msg.(backToMenuMsg)
	if !ok {
		t.Fatalf("resetAllCmd failure emits %T, want backToMenuMsg", msg)
	}
	if !strings.HasPrefix(btm.notice, ui.NoticeResetErr) {
		t.Errorf("notice = %q, want prefix %q", btm.notice, ui.NoticeResetErr)
	}
}

func TestDoVSCodeCmd_Success(t *testing.T) {
	codeDir := t.TempDir()
	if err := os.WriteFile(codeDir+"/code", []byte("#!/bin/sh\nexit 0"), 0o755); err != nil {
		t.Fatal(err)
	}
	prependTestPath(t, codeDir)

	r := &runner.FakeRunner{
		HostFunc: func(name string, args []string) (string, error) {
			return "true", nil
		},
	}
	cmd := doVSCodeCmd(context.Background(), r)
	if cmd == nil {
		t.Fatal("doVSCodeCmd returned nil")
	}
	msg := cmd()
	btm, ok := msg.(backToMenuMsg)
	if !ok {
		t.Fatalf("doVSCodeCmd success emits %T, want backToMenuMsg", msg)
	}
	if btm.notice != "" {
		t.Errorf("doVSCodeCmd success notice = %q, want empty", btm.notice)
	}
}

func TestDoVSCodeCmd_Failure(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	r := &runner.FakeRunner{
		HostFunc: func(name string, args []string) (string, error) {
			return "true", nil
		},
	}
	cmd := doVSCodeCmd(context.Background(), r)
	msg := cmd()
	btm, ok := msg.(backToMenuMsg)
	if !ok {
		t.Fatalf("doVSCodeCmd failure emits %T, want backToMenuMsg", msg)
	}
	if !strings.HasPrefix(btm.notice, ui.NoticeVSCodeErr) {
		t.Errorf("notice = %q, want prefix %q", btm.notice, ui.NoticeVSCodeErr)
	}
}

func TestNewHarnessForm_NotNil(t *testing.T) {
	r := &runner.FakeRunner{
		ContFunc: func(args []string) (string, error) {
			return "skip", nil
		},
	}
	f := newHarnessForm(context.Background(), r, 80, 24)
	if f == nil {
		t.Fatal("newHarnessForm returned nil")
	}
	if f.form == nil {
		t.Fatal("newHarnessForm form field is nil")
	}
}

func TestNewHarnessForm_Apply_Success(t *testing.T) {
	var callCount int
	r := &runner.FakeRunner{
		ContFunc: func(args []string) (string, error) {
			callCount++
			return "", nil
		},
	}
	f := newHarnessForm(context.Background(), r, 80, 24)
	if f == nil {
		t.Fatal("newHarnessForm returned nil")
	}
	cmd := f.apply()
	if cmd == nil {
		t.Fatal("apply returned nil")
	}
	msg := cmd()
	if _, ok := msg.(backToMenuMsg); !ok {
		t.Fatalf("newHarnessForm apply success emits %T, want backToMenuMsg", msg)
	}
}

func TestNewHarnessForm_Apply_Error(t *testing.T) {
	errBoom := errors.New("harness error")
	callCount := 0
	r := &runner.FakeRunner{
		ContFunc: func(args []string) (string, error) {
			callCount++
			if callCount == 1 {
				return "skip", nil
			}
			return "", errBoom
		},
	}
	f := newHarnessForm(context.Background(), r, 80, 24)
	cmd := f.apply()
	msg := cmd()
	btm, ok := msg.(backToMenuMsg)
	if !ok {
		t.Fatalf("newHarnessForm apply error emits %T, want backToMenuMsg", msg)
	}
	if !strings.HasPrefix(btm.notice, ui.NoticeHarnessErr) {
		t.Errorf("notice = %q, want prefix %q", btm.notice, ui.NoticeHarnessErr)
	}
}

func TestNewLaunchForm_WithCatalog_ApplyWrites(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(dir+"/config", 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(dir+"/config/stacks.txt", []byte("rust\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	r := &runner.FakeRunner{RepoVal: dir}
	f := newLaunchForm(r, 80, 24)
	if f == nil {
		t.Fatal("newLaunchForm with catalog should return non-nil")
	}
	cmd := f.apply()
	if cmd == nil {
		t.Fatal("apply returned nil cmd")
	}
	msg := cmd()
	if _, ok := msg.(launchReadyMsg); !ok {
		t.Fatalf("apply() emits %T, want launchReadyMsg", msg)
	}
	v, ok := readStacksFromEnv(t, dir)
	if !ok {
		t.Error("WriteStacks did not write .env")
	}
	_ = v
}

func readStacksFromEnv(t *testing.T, dir string) (string, bool) {
	t.Helper()
	data, err := os.ReadFile(dir + "/.env")
	if err != nil {
		return "", false
	}
	for _, line := range strings.Split(string(data), "\n") {
		if after, ok := strings.CutPrefix(line, "STACKS="); ok {
			return strings.TrimSpace(after), true
		}
	}
	return "", false
}

func TestNewLaunchForm_WithPluginsCatalog_ApplyWrites(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(dir+"/config", 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(dir+"/config/plugins.txt", []byte("plugin-a\nplugin-b\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	r := &runner.FakeRunner{RepoVal: dir}
	f := newLaunchForm(r, 80, 24)
	if f == nil {
		t.Fatal("newLaunchForm with plugins catalog should be non-nil")
	}
	cmd := f.apply()
	msg := cmd()
	if _, ok := msg.(launchReadyMsg); !ok {
		t.Fatalf("newLaunchForm apply() emits %T, want launchReadyMsg", msg)
	}
}

func TestNewLaunchForm_WithCatalog_ApplyStacksError(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(dir+"/config", 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(dir+"/config/stacks.txt", []byte("rust\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(dir, 0o555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chmod(dir, 0o755) })

	r := &runner.FakeRunner{RepoVal: dir}
	f := newLaunchForm(r, 80, 24)
	if f == nil {
		t.Fatal("newLaunchForm with catalog should return non-nil")
	}
	cmd := f.apply()
	if cmd == nil {
		t.Fatal("apply returned nil")
	}
	msg := cmd()
	btm, ok := msg.(backToMenuMsg)
	if !ok {
		t.Fatalf("apply() on error emits %T, want backToMenuMsg", msg)
	}
	if !strings.HasPrefix(btm.notice, ui.NoticeStacksErr) {
		t.Errorf("notice = %q, want prefix %q", btm.notice, ui.NoticeStacksErr)
	}
}

func TestNewLaunchForm_WithPluginsCatalog_ApplyError(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(dir+"/config", 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(dir+"/config/plugins.txt", []byte("plugin-a\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(dir, 0o555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chmod(dir, 0o755) })

	r := &runner.FakeRunner{RepoVal: dir}
	f := newLaunchForm(r, 80, 24)
	if f == nil {
		t.Fatal("newLaunchForm with plugins catalog should return non-nil")
	}
	cmd := f.apply()
	if cmd == nil {
		t.Fatal("apply returned nil")
	}
	msg := cmd()
	btm, ok := msg.(backToMenuMsg)
	if !ok {
		t.Fatalf("apply() on error emits %T, want backToMenuMsg", msg)
	}
	if !strings.HasPrefix(btm.notice, ui.NoticePluginsErr) {
		t.Errorf("notice = %q, want prefix %q", btm.notice, ui.NoticePluginsErr)
	}
}

func TestFormScreen_Init_NonNil(t *testing.T) {
	f := newResetForm(context.Background(), &runner.FakeRunner{}, 80, 24)
	if cmd := f.Init(); cmd == nil {
		t.Error("formScreen.Init() returned nil cmd")
	}
}

func TestFormScreen_Update_SizeMsg(t *testing.T) {
	f := newResetForm(context.Background(), &runner.FakeRunner{}, 80, 24)
	f2, _ := f.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	if f2 == nil {
		t.Error("Update should return non-nil formScreen")
	}
}

func TestNewLaunchForm_WithSkillsCatalog_ApplyWrites(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(dir+"/config", 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(dir+"/config/skills.txt", []byte("owner/skill-a\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	r := &runner.FakeRunner{RepoVal: dir}
	f := newLaunchForm(r, 80, 24)
	if f == nil {
		t.Fatal("newLaunchForm with skills catalog should return non-nil")
	}
	cmd := f.apply()
	if cmd == nil {
		t.Fatal("apply returned nil cmd")
	}
	msg := cmd()
	if _, ok := msg.(launchReadyMsg); !ok {
		t.Fatalf("apply() emits %T, want launchReadyMsg", msg)
	}
}

func TestNewLaunchForm_WithSkillsCatalog_ApplyError(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(dir+"/config", 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(dir+"/config/skills.txt", []byte("owner/skill-a\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(dir, 0o555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chmod(dir, 0o755) })

	r := &runner.FakeRunner{RepoVal: dir}
	f := newLaunchForm(r, 80, 24)
	if f == nil {
		t.Fatal("newLaunchForm with skills catalog should return non-nil")
	}
	cmd := f.apply()
	if cmd == nil {
		t.Fatal("apply returned nil")
	}
	msg := cmd()
	btm, ok := msg.(backToMenuMsg)
	if !ok {
		t.Fatalf("apply() on error emits %T, want backToMenuMsg", msg)
	}
	if !strings.HasPrefix(btm.notice, ui.NoticeSkillsErr) {
		t.Errorf("notice = %q, want prefix %q", btm.notice, ui.NoticeSkillsErr)
	}
}

func TestNewLaunchForm_AllCatalogsEmpty_Nil(t *testing.T) {
	dir := t.TempDir()
	r := &runner.FakeRunner{RepoVal: dir}
	f := newLaunchForm(r, 80, 24)
	if f != nil {
		t.Error("newLaunchForm with no catalogs should return nil")
	}
}

// ── Telegram form tests ────────────────────────────────────────────────────

func TestNewTelegramForm_NotNil(t *testing.T) {
	f := newTelegramForm(80, 24)
	if f == nil {
		t.Fatal("newTelegramForm returned nil")
	}
	if f.form == nil {
		t.Fatal("newTelegramForm form field is nil")
	}
}

func TestNewTelegramForm_View_NonEmpty(t *testing.T) {
	// Calling View exercises the form rendering path including WithHideFunc.
	f := newTelegramForm(80, 24)
	// Init is required before rendering; result is the initial Cmd (may be nil).
	_ = f.Init()
	v := f.View()
	if v == "" {
		t.Error("newTelegramForm.View() should return non-empty string")
	}
	// The options are rendered with ANSI codes; check the raw UTF-8 text is present.
	if !strings.Contains(v, "Пропустить") {
		t.Errorf("newTelegramForm.View() missing Пропустить option, got:\n%s", v)
	}
}

func TestNewTelegramForm_SkipPath_EmitsStartPipelineMsg(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	f := newTelegramForm(80, 24)
	// Default is "skip" (configure = false), so apply() without changing
	// anything should immediately emit startPipelineMsg.
	cmd := f.apply()
	if cmd == nil {
		t.Fatal("apply() on skip path returned nil cmd")
	}
	msg := cmd()
	if _, ok := msg.(telegramDoneMsg); !ok {
		t.Fatalf("skip path apply() emits %T, want telegramDoneMsg", msg)
	}
}

func TestNewTelegramForm_SkipPath_WritesNoFile(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	f := newTelegramForm(80, 24)
	cmd := f.apply()
	if cmd == nil {
		t.Fatal("apply() returned nil")
	}
	cmd()

	// The token file must NOT be created on the skip path.
	tokenFile := tmp + "/.claude/.mirabilis-telegram-token"
	if _, err := os.Stat(tokenFile); err == nil {
		t.Error("skip path must not write the token file")
	}
}

func TestNewTelegramForm_ConfigurePath_WritesTokenFile(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	// Simulate user choosing "configure" and entering a token by directly
	// setting the captured closure variables via the form's internal state.
	// We do this via a test-only wrapper that simulates the configure=true state.
	msg := applyTelegramConfigure(t, "bot999:TestToken")
	if _, ok := msg.(telegramDoneMsg); !ok {
		t.Fatalf("configure path apply() emits %T, want telegramDoneMsg", msg)
	}

	tokenFile := tmp + "/.claude/.mirabilis-telegram-token"
	data, err := os.ReadFile(tokenFile)
	if err != nil {
		t.Fatalf("token file not written: %v", err)
	}
	got := strings.TrimRight(string(data), "\r\n")
	if got != "bot999:TestToken" {
		t.Errorf("token file content = %q, want bot999:TestToken", got)
	}
}

func TestNewTelegramForm_ConfigurePath_FileHas0600(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	applyTelegramConfigure(t, "bot999:PermTest")

	tokenFile := tmp + "/.claude/.mirabilis-telegram-token"
	info, err := os.Stat(tokenFile)
	if err != nil {
		t.Fatalf("token file not found: %v", err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Errorf("token file mode = %04o, want 0600", info.Mode().Perm())
	}
}

func TestNewTelegramForm_ConfigurePath_EmptyToken_BackToMenu(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	// Directly test the defensive guard in apply() for empty token with
	// configure=true (this path is normally blocked by huh Validate, but the
	// apply() function guards against it defensively).
	msg := applyTelegramConfigure(t, "   ")
	btm, ok := msg.(backToMenuMsg)
	if !ok {
		t.Fatalf("empty-token configure path emits %T, want backToMenuMsg", msg)
	}
	if !strings.HasPrefix(btm.notice, ui.NoticeTelegramErr) {
		t.Errorf("notice = %q, want prefix %q", btm.notice, ui.NoticeTelegramErr)
	}
}

// applyTelegramConfigure exercises the configure=true branch of the Telegram
// form by calling the extracted applyTelegramToken function directly.
func applyTelegramConfigure(t *testing.T, token string) tea.Msg {
	t.Helper()
	return applyTelegramToken(true, token)()
}

// ── validateTelegramToken tests ───────────────────────────────────────────

func TestValidateTelegramToken_Empty_ReturnsError(t *testing.T) {
	if err := validateTelegramToken(""); err == nil {
		t.Error("validateTelegramToken('') = nil, want error")
	}
}

func TestValidateTelegramToken_Spaces_ReturnsError(t *testing.T) {
	if err := validateTelegramToken("   "); err == nil {
		t.Error("validateTelegramToken('   ') = nil, want error")
	}
}

func TestValidateTelegramToken_Valid_ReturnsNil(t *testing.T) {
	if err := validateTelegramToken("bot123:abc"); err != nil {
		t.Errorf("validateTelegramToken(valid) = %v, want nil", err)
	}
}

// ── applyTelegramToken tests ──────────────────────────────────────────────

func TestApplyTelegramToken_Skip_EmitsTelegramDone(t *testing.T) {
	msg := applyTelegramToken(false, "")()
	if _, ok := msg.(telegramDoneMsg); !ok {
		t.Fatalf("applyTelegramToken(skip) emits %T, want telegramDoneMsg", msg)
	}
}

func TestApplyTelegramToken_Configure_WritesAndEmitsTelegramDone(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	msg := applyTelegramToken(true, "bot456:hello")()
	if _, ok := msg.(telegramDoneMsg); !ok {
		t.Fatalf("applyTelegramToken(configure) = %T, want telegramDoneMsg", msg)
	}
}

func TestApplyTelegramToken_Configure_WriteError_BackToMenu(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("permission checks not effective as root")
	}
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	// Make ~/.claude a read-only dir so WriteTelegramToken fails.
	cd := tmp + "/.claude"
	if err := os.MkdirAll(cd, 0o555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(cd, 0o755) })

	msg := applyTelegramToken(true, "bot789:fail")()
	btm, ok := msg.(backToMenuMsg)
	if !ok {
		t.Fatalf("applyTelegramToken write-error = %T, want backToMenuMsg", msg)
	}
	if !strings.HasPrefix(btm.notice, ui.NoticeTelegramErr) {
		t.Errorf("notice = %q, want prefix %q", btm.notice, ui.NoticeTelegramErr)
	}
}

// ── newClaudeTokenForm / applyClaudeToken tests ───────────────────────────

func TestNewClaudeTokenForm_NonEmpty(t *testing.T) {
	r := &runner.FakeRunner{}
	f := newClaudeTokenForm(context.Background(), r, 80, 24)
	if f == nil {
		t.Fatal("newClaudeTokenForm returned nil")
	}
	if f.View() == "" {
		t.Error("newClaudeTokenForm View() is empty")
	}
}

func TestApplyClaudeToken_Skip_EmitsStartPipeline(t *testing.T) {
	r := &runner.FakeRunner{}
	msg := applyClaudeToken(context.Background(), r, false, "")()
	if _, ok := msg.(startPipelineMsg); !ok {
		t.Fatalf("applyClaudeToken(skip) emits %T, want startPipelineMsg", msg)
	}
}

func TestApplyClaudeToken_EmptyToken_BackToMenu(t *testing.T) {
	r := &runner.FakeRunner{}
	msg := applyClaudeToken(context.Background(), r, true, "   ")()
	btm, ok := msg.(backToMenuMsg)
	if !ok {
		t.Fatalf("applyClaudeToken empty token = %T, want backToMenuMsg", msg)
	}
	if !strings.HasPrefix(btm.notice, ui.NoticeClaudeErr) {
		t.Errorf("notice = %q, want prefix %q", btm.notice, ui.NoticeClaudeErr)
	}
}

func TestApplyClaudeToken_Configure_NoConflict_EmitsStartPipeline(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	r := &runner.FakeRunner{
		ContFunc: func(_ []string) (string, error) { return "no\n", nil },
	}
	msg := applyClaudeToken(context.Background(), r, true, "oauth-test-token")()
	if _, ok := msg.(startPipelineMsg); !ok {
		t.Fatalf("applyClaudeToken(configure, no conflict) = %T, want startPipelineMsg", msg)
	}
}

func TestApplyClaudeToken_Configure_Conflict_BackToMenu(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	r := &runner.FakeRunner{
		ContFunc: func(_ []string) (string, error) { return "yes\n", nil },
	}
	msg := applyClaudeToken(context.Background(), r, true, "oauth-test-token")()
	btm, ok := msg.(backToMenuMsg)
	if !ok {
		t.Fatalf("applyClaudeToken(configure, conflict) = %T, want backToMenuMsg", msg)
	}
	if btm.notice != ui.NoticeClaudeConflict {
		t.Errorf("notice = %q, want %q", btm.notice, ui.NoticeClaudeConflict)
	}
}

func TestApplyClaudeToken_Configure_WriteError_BackToMenu(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("permission checks not effective as root")
	}
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	cd := tmp + "/.claude"
	if err := os.MkdirAll(cd, 0o555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(cd, 0o755) })

	r := &runner.FakeRunner{}
	msg := applyClaudeToken(context.Background(), r, true, "oauth-fail")()
	btm, ok := msg.(backToMenuMsg)
	if !ok {
		t.Fatalf("applyClaudeToken write-error = %T, want backToMenuMsg", msg)
	}
	if !strings.HasPrefix(btm.notice, ui.NoticeClaudeErr) {
		t.Errorf("notice = %q, want prefix %q", btm.notice, ui.NoticeClaudeErr)
	}
}

func TestValidateClaudeToken_Empty_ReturnsError(t *testing.T) {
	if err := validateClaudeToken(""); err == nil {
		t.Error("validateClaudeToken('') = nil, want error")
	}
}

func TestValidateClaudeToken_Spaces_ReturnsError(t *testing.T) {
	if err := validateClaudeToken("   "); err == nil {
		t.Error("validateClaudeToken('   ') = nil, want error")
	}
}

func TestValidateClaudeToken_Valid_ReturnsNil(t *testing.T) {
	if err := validateClaudeToken("oauth-valid-token"); err != nil {
		t.Errorf("validateClaudeToken(valid) = %v, want nil", err)
	}
}

func TestApplyClaudeToken_ContainerError_EmitsStartPipeline(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	r := &runner.FakeRunner{
		ContFunc: func(_ []string) (string, error) {
			return "", errors.New("container not running")
		},
	}
	msg := applyClaudeToken(context.Background(), r, true, "oauth-ok-token")()
	if _, ok := msg.(startPipelineMsg); !ok {
		t.Fatalf("applyClaudeToken(container error) = %T, want startPipelineMsg", msg)
	}
}
