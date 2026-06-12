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

	GlyphDone    = "✔"
	GlyphFailed  = "✖"
	GlyphPending = "·"
	GlyphSkipped = "−"

	StepDetailWaiting = "waiting"

	MenuActionLaunch   = "Launch"
	MenuActionHarness  = "Harness"
	MenuActionTelegram = "Telegram"
	MenuActionVSCode   = "VS Code"
	MenuActionReset    = "Reset"
	MenuActionQuit     = "Quit"

	MenuDescLaunch   = "setup pipeline + Claude in container"
	MenuDescHarness  = "neuro-matrix: on / off / reinstall"
	MenuDescTelegram = "bot notifications: token, optional"
	MenuDescVSCode   = "attach /workspace in VS Code"
	MenuDescReset    = "container, image and volumes — permanent"

	MenuHarnessOn      = "neuro-matrix: on"
	MenuHarnessOff     = "neuro-matrix: off"
	MenuHarnessMissing = "neuro-matrix: missing"
	MenuHarnessUnknown = "neuro-matrix: unknown"

	WelcomeHint = "select an action on the left"

	NoticeHarnessErr  = "harness: "
	NoticeVSCodeErr   = "VS Code: "
	NoticeTelegramErr = "telegram: "
	NoticeResetDone   = "everything removed — next launch will rebuild the sandbox"

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
	FormDescTelegramToken    = "Token from @BotFather. Stored in the host secret store, not in git, not in the image."

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
	NoticeBusy                = "an operation is already running — wait for it to finish"
	NoticeVSCodeDone          = "VS Code: opened"
	LogTelegramFailed         = "telegram setup failed"
	LogHarnessFailed          = "harness apply failed"
	LogVSCodeFailed           = "vscode open failed"
)
