package strings

const (
	AppName = "mirabilis"

	TitleBanner = "" +
		"┌┬┐┬┬─┐┌─┐┌┐ ┬ ┬  ┬┌─┐\n" +
		"│││││├┬┘├─┤├┴┐││  │└─┐\n" +
		"┴ ┴┴┴└─┴ ┴└─┘┴─┘  ┴└─┘"

	LogoLargeFrameA = "   │   \n ──○── \n   │   "
	LogoLargeFrameB = " ╲   ╱ \n   ○   \n ╱   ╲ "
	LogoLargeFrameC = "   ╷   \n   ○   \n   ╵   "
	LogoLargeFrameD = " ·   · \n   ○   \n ·   · "
	LogoLargeFrameE = " ───── \n   ○   \n ───── "
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

	RolePickerTitle = "Choose loadout"
	RolePickerHint  = "↑↓ move · enter select · esc cancel"

	RoleEmojiGrind   = "⚡"
	RoleEmojiRaid    = "🎯"
	RoleEmojiPvP     = "🏆"
	RoleEmojiDefault = "•"

	RoleLabelGrind = "grind"
	RoleLabelRaid  = "raid"
	RoleLabelPvP   = "pvp"

	RoleFactEffort     = "effort "
	RoleFactHarnessOn  = "harness on"
	RoleFactHarnessOff = "harness off"
	RoleFactBatch      = "batch"
	RoleFactSep        = " · "
	RoleDefaultSuffix  = " (default)"

	MenuActionLaunch = "Launch"
	MenuActionVSCode = "VS Code"
	MenuActionUpdate = "Update"
	MenuActionQuit   = "Quit"

	MenuDescLaunch = "setup pipeline + Claude in container"
	MenuDescVSCode = "open container root in VS Code"
	MenuDescUpdate = "re-hydrate the ecosystem repos (clone or pull)"

	WelcomeHint = "pick an action ←"

	TooSmall = "terminal too small"
	SizeSep  = " < "

	BusyElapsedSep = " "

	NoticeVSCodeErr = "VS Code: "
	NoticeUpdateErr = "update: "

	NoticeLaunchFailed     = "launch failed"
	NoticeLaunchCanceled   = "launch canceled"
	NoticeLaunchErrPrefix  = "launch error: "
	NoticeLoadoutErrPrefix = "loadout error: "

	NoticeVSCodeOpening = "opening VS Code…"
	NoticeBusy          = "busy — wait"
	NoticeVSCodeDone    = "VS Code: opened"
	NoticeUpdateRunning = "updating ecosystem repos…"
	NoticeUpdateDone    = "ecosystem repos updated"
	LogVSCodeFailed     = "vscode open failed"
	LogUpdateFailed     = "ecosystem update failed"
)
