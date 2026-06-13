package strings

const (
	AppName = "mirabilis"

	GHAuthTitle         = "GitHub sign-in"
	GHAuthLabelCode     = "  code:  "
	GHAuthLabelURL      = "  URL:   "
	GHAuthStatusWaiting = "requesting device code from GitHub…"

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

	MenuActionLaunch   = "Launch"
	MenuActionAttach   = "Attach"
	MenuActionHarness  = "Harness"
	MenuActionTelegram = "Telegram"
	MenuActionVSCode   = "VS Code"
	MenuActionReset    = "Reset"
	MenuActionQuit     = "Quit"

	MenuDescLaunch   = "setup pipeline + Claude in container"
	MenuDescAttach   = "spawn a new Claude in the container"
	MenuDescHarness  = "neuro-matrix: on / off / reinstall"
	MenuDescTelegram = "bot notifications: token, optional"
	MenuDescVSCode   = "attach /workspace in VS Code"
	MenuDescReset    = "container, image and volumes — permanent"

	NoticeSecondary = "secondary — sandbox owned by another tab"

	MenuHarnessOn      = "neuro-matrix: on"
	MenuHarnessOff     = "neuro-matrix: off"
	MenuHarnessMissing = "neuro-matrix: missing"
	MenuHarnessUnknown = "neuro-matrix: unknown"

	WelcomeHint = "pick an action ←"

	TooSmall = "terminal too small"
	SizeSep  = " < "

	BusyElapsedSep = " "

	NoticeHarnessErr    = "harness: "
	NoticeVSCodeErr     = "VS Code: "
	NoticeTelegramErr   = "telegram: "
	NoticeAttachErr     = "attach: "
	NoticeAttachOpening = "attaching…"
	NoticeResetDone     = "removed — relaunch to rebuild"

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

	NoticeTelegramConfiguring = "configuring telegram…"
	NoticeTelegramDone        = "telegram: notifications enabled"
	NoticeHarnessApplying     = "applying harness…"
	NoticeHarnessDone         = "harness: done"
	NoticeVSCodeOpening       = "opening VS Code…"
	NoticeBusy                = "busy — wait"
	NoticeVSCodeDone          = "VS Code: opened"
	LogTelegramFailed         = "telegram setup failed"
	LogHarnessFailed          = "harness apply failed"
	LogVSCodeFailed           = "vscode open failed"
	LogAttachFailed           = "attach failed"
)
