// Package strings holds UI copy constants: notices, log messages, and action labels.
package strings

const (
	AppName = "mirabilis"

	LogoSmallFrames = "⊕⊛※◈◉◎⊗⊘"

	LogoLargeFrameA = "   │   \n ──○── \n   │   "
	LogoLargeFrameB = " ╲   ╱ \n   ○   \n ╱   ╲ "
	LogoLargeFrameC = "   ╷   \n   ○   \n   ╵   "
	LogoLargeFrameD = " ·   · \n   ○   \n ·   · "
	LogoLargeFrameE = "  ─── \n  ○  \n  ─── "
	LogoLargeFrameF = " ╱   ╲ \n   ○   \n ╲   ╱ "
	LogoLargeFrameG = "   ·   \n ─ ○ ─ \n   ·   "
	LogoLargeFrameH = " ─   ─ \n   ○   \n ─   ─ "
	LogoLargeStatic = LogoLargeFrameA

	GHAuthTitle         = "GitHub sign-in"
	GHAuthLabelCode     = "  code:  "
	GHAuthLabelURL      = "  URL:   "
	GHAuthStatusWaiting = "requesting device code from GitHub…"
	GHAuthCopyHint      = "  press c to copy code"
	GHAuthCopied        = "code copied (also shown above)"
	GHAuthCopyFailed    = "copy failed — enter the code shown above manually"

	FooterHints    = "enter select · esc back · tab log · q quit"
	DegradedPrefix = "degraded: "
	StatusSep      = " · "

	CmdlogTitle  = "commands"
	CmdlogPrefix = "+ "

	GlyphDone          = "✔"
	GlyphFailed        = "✖"
	GlyphPending       = "·"
	GlyphSkipped       = "−"
	GlyphWaiting       = "?"
	GlyphRunningStatic = "▸"

	GlyphStatusOK       = "●"
	GlyphStatusDegraded = "⊘"
	GlyphStatusOff      = "○"
	GlyphStatusUnknown  = "?"
	GlyphError          = "⊘"
	GlyphDanger         = "⚠"

	ErrorDismissHint = "press x to dismiss"

	StepDetailWaiting = "waiting"

	StepOverflowPrefix = "+"
	StepOverflowSuffix = " more"

	ProgressSep = "/"

	MenuActionLaunch  = "Launch"
	MenuActionHarness = "Harness"
	MenuActionVSCode  = "VS Code"
	MenuActionReset   = "Reset"
	MenuActionQuit    = "Quit"

	MenuDescLaunch  = "setup pipeline + Claude in container"
	MenuDescHarness = "neuro-matrix: on / off / reinstall"
	MenuDescVSCode  = "open container root in VS Code"
	MenuDescReset   = "container, image and volumes — permanent"

	MenuHarnessOn      = "neuro-matrix: on"
	MenuHarnessOff     = "neuro-matrix: off"
	MenuHarnessMissing = "neuro-matrix: missing"
	MenuHarnessUnknown = "neuro-matrix: unknown"

	WelcomeHint = "pick an action ←"

	TooSmall = "terminal too small"
	SizeSep  = " < "

	BusyElapsedSep = " "

	NoticeHarnessErr = "harness: "
	NoticeVSCodeErr  = "VS Code: "
	NoticeResetDone  = "removed — relaunch to rebuild"

	FormTitleHarness  = "neuro-matrix harness"
	FormOptHarnessOn  = "Enable"
	FormOptHarnessOff = "Disable"
	FormOptHarnessRe  = "Reinstall"
	FormTitleReset    = "Delete everything?"
	FormConfirmReset  = "Delete everything"
	FormCancelReset   = "Cancel"

	FormTitleTelegram        = "Telegram"
	FormOptTelegramConfigure = "Configure"
	FormOptTelegramSkip      = "Skip"
	FormTitleTelegramToken   = "Bot token"
	FormDescTelegramToken    = "From @BotFather. Stored host-side, never in git/image."

	NoticeLaunchFailed    = "launch failed"
	NoticeLaunchCanceled  = "launch canceled"
	NoticeLaunchErrPrefix = "launch error: "
	NoticeResetFailed     = "reset failed"
	LogResetFailed        = "reset failed"

	NoticeHarnessApplying = "applying harness…"
	NoticeHarnessDone     = "harness: done"
	NoticeVSCodeOpening   = "opening VS Code…"
	NoticeBusy            = "busy — wait"
	NoticeVSCodeDone      = "VS Code: opened"
	LogHarnessFailed      = "harness apply failed"
	LogVSCodeFailed       = "vscode open failed"
)
