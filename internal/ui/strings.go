package ui

const (
	PipelineTitle  = "mirabilis — launch"
	LabelDone      = "done"
	HintAnyKeyMenu = "any key — back to menu"
	HintEscCancel  = "esc — cancel"

	GHAuthTitle         = "GitHub sign-in"
	GHAuthLabelCode     = "  code:  "
	GHAuthLabelURL      = "  URL:   "
	GHAuthStatusWaiting = "requesting device code from GitHub…"
	GHAuthStatusOpened  = "browser opened on host — confirm sign-in and enter the code"
	GHAuthStatusNoOpen  = "could not open browser — open the URL manually"

	MenuHint = "enter · q quit"

	MenuActionLaunch  = "Launch"
	MenuActionHarness = "Harness"
	MenuActionVSCode  = "Open in VS Code"
	MenuActionReset   = "Reset"
	MenuActionQuit    = "Quit"

	MenuDescLaunch       = "setup pipeline + Claude in container"
	MenuDescHarness      = "neuro-matrix: on / off / reinstall"
	MenuDescVSCode       = "attach /workspace in VS Code"
	MenuDescReset        = "container, image and volumes — permanent"
	MenuDescContainerOff = "container not running — Launch first"

	MenuHarnessOff     = "neuro-matrix: off"
	MenuHarnessMissing = "neuro-matrix: missing"
	MenuHarnessUnknown = "neuro-matrix: unknown"
	MenuStale          = "workspace: stale (rebuild on launch)"

	NoticeLaunchCanceled = "launch canceled"
	NoticePluginsErr     = "plugins: "
	NoticeHarnessErr     = "harness: "
	NoticeResetErr       = "reset: "
	NoticeSkillsErr      = "skills: "
	NoticeStacksErr      = "stacks: "
	NoticeVSCodeErr      = "VS Code: "
	NoticeResetDone      = "everything removed — next launch will rebuild the sandbox"

	FormTitlePlugins  = "Plugins (space — toggle, Enter — ok)"
	FormTitleHarness  = "neuro-matrix harness"
	FormOptHarnessOn  = "Enable"
	FormOptHarnessOff = "Disable"
	FormOptHarnessRe  = "Reinstall"
	FormTitleReset    = "Delete everything?"
	FormDescReset     = "Container, image and volumes (/workspace code, ~/.claude memory and auth, gh) will be permanently erased."
	FormConfirmReset  = "Delete everything"
	FormCancelReset   = "Cancel"
	FormTitleSkills   = "Optional skills (space — toggle, Enter — ok)"
	FormTitleStacks   = "Optional stacks (node + python + go already in base)"

	FormTitleTelegram        = "Telegram"
	FormOptTelegramConfigure = "Настроить"
	FormOptTelegramSkip      = "Пропустить"
	FormTitleTelegramToken   = "Токен бота"
	FormDescTelegramToken    = "Токен от @BotFather. Хранится в файле-секрете, не в git, не в образе."
	NoticeTelegramErr        = "telegram: "
)
