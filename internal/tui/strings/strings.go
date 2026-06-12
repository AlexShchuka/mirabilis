package strings

const (
	AppName = "mirabilis"

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

	FooterHints    = "enter select · esc back · tab log · q quit"
	DegradedPrefix = "degraded: "
	StatusSep      = " · "

	CmdlogTitle  = "commands"
	CmdlogPrefix = "+ "

	GlyphDone    = "✔"
	GlyphRunning = "⠋"
	GlyphFailed  = "✖"
	GlyphPending = "·"
	GlyphSkipped = "−"

	StepDetailWaiting = "waiting"

	MenuHint = "enter · q quit"

	MenuActionLaunch   = "Launch"
	MenuActionHarness  = "Harness"
	MenuActionTelegram = "Telegram"
	MenuActionVSCode   = "VS Code"
	MenuActionReset    = "Reset"
	MenuActionQuit     = "Quit"

	MenuDescLaunch       = "setup pipeline + Claude in container"
	MenuDescHarness      = "neuro-matrix: on / off / reinstall"
	MenuDescTelegram     = "bot notifications: token, optional"
	MenuDescVSCode       = "attach /workspace in VS Code"
	MenuDescReset        = "container, image and volumes — permanent"
	MenuDescContainerOff = "container not running — Launch first"

	MenuHarnessOff     = "neuro-matrix: off"
	MenuHarnessMissing = "neuro-matrix: missing"
	MenuHarnessUnknown = "neuro-matrix: unknown"
	MenuStale          = "workspace: stale (rebuild on launch)"

	WelcomeHint = "select an action on the left"

	NoticeLaunchCanceled = "launch canceled"
	NoticePluginsErr     = "plugins: "
	NoticeHarnessErr     = "harness: "
	NoticeResetErr       = "reset: "
	NoticeSkillsErr      = "skills: "
	NoticeStacksErr      = "stacks: "
	NoticeVSCodeErr      = "VS Code: "
	NoticeTelegramErr    = "telegram: "
	NoticeClaudeErr      = "claude: "
	NoticeResetDone      = "everything removed — next launch will rebuild the sandbox"
	NoticeClaudeConflict = "claude: ~/.claude/.credentials.json found in the volume — it overrides the host token; delete it or avoid /login inside the container"

	FormTitlePlugins       = "Plugins (space — toggle, Enter — ok)"
	FormTitleHarness       = "neuro-matrix harness"
	FormOptHarnessOn       = "Enable"
	FormOptHarnessOff      = "Disable"
	FormOptHarnessRe       = "Reinstall"
	FormTitleResetMemory   = "Memory (~/.claude/memory)"
	FormOptResetPreserve   = "Preserve (copy out before wipe, restore on next launch)"
	FormOptResetDestroyAll = "Destroy (wipe everything)"
	FormTitleReset         = "Delete everything?"
	FormDescReset          = "Container, image and volumes (/workspace code, ~/.claude memory and auth, gh) will be permanently erased."
	FormConfirmReset       = "Delete everything"
	FormCancelReset        = "Cancel"
	FormTitleSkills        = "Optional skills (space — toggle, Enter — ok)"
	FormTitleStacks        = "Optional stacks (node + python + go already in base)"

	FormTitleTelegram        = "Telegram"
	FormOptTelegramConfigure = "Configure"
	FormOptTelegramSkip      = "Skip"
	FormTitleTelegramToken   = "Bot token"
	FormDescTelegramToken    = "Token from @BotFather. Stored in the host secret store, not in git, not in the image."

	FormTitleClaude        = "Claude"
	FormOptClaudeConfigure = "Configure"
	FormOptClaudeSkip      = "Skip"
	FormTitleClaudeToken   = "OAuth token"
	FormDescClaudeToken    = "Run `claude setup-token` on the host and paste the token. Stored in the keychain / secret file."

	ErrTokenEmpty = "token cannot be empty"

	NoticeLaunchFailed    = "launch failed"
	NoticeLaunchErrPrefix = "launch error: "
	NoticeResetFailed     = "reset failed"
	LogResetFailed        = "reset failed"
)
