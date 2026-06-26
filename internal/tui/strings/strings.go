package strings

const (
	AppName = "mirabilis"

	TitleBanner = "" +
		"╔╦╗ ╦ ╦═╗ ╔═╗ ╔╗  ╦ ╦   ╦ ╔═╗\n" +
		"║║║ ║ ╠╦╝ ╠═╣ ╠╩╗ ║ ║   ║ ╚═╗\n" +
		"╩ ╩ ╩ ╩╚═ ╩ ╩ ╚═╝ ╩ ╩═╝ ╩ ╚═╝"

	LogoLargeFrameA = "   ·   \n · ✧ · \n   ·   "
	LogoLargeFrameB = "   ╷   \n   ✦   \n   ╵   "
	LogoLargeFrameC = " ─   ─ \n ─ ✦ ─ \n ─   ─ "
	LogoLargeFrameD = " ╲ ╷ ╱ \n ─ ✦ ─ \n ╱ ╵ ╲ "
	LogoLargeFrameE = " ───── \n   ✦   \n ───── "
	LogoLargeFrameF = " ╲   ╱ \n   ✦   \n ╱   ╲ "
	LogoLargeFrameG = " ·   · \n · ✦ · \n ·   · "
	LogoLargeFrameH = "   ╷   \n ─ ✧ ─ \n   ╵   "
	LogoLargeStatic = LogoLargeFrameD

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

	VersionNode     = "version"
	OutdatedPrefix  = "update "
	OutdatedDefault = "available"
	UpToDateMark    = "✓"
	HealthSep       = " "
	HealthCountSep  = ""

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

	RolePickerTitle = "Choose party"
	RolePickerHint  = "←→↑↓ move · enter select · esc cancel"

	RoleEmojiSpark   = "⚡"
	RoleEmojiDrift   = "🧭"
	RoleEmojiOrbit   = "🔭"
	RoleEmojiForge   = "🔨"
	RoleEmojiNova    = "💥"
	RoleEmojiDefault = "•"

	RoleLabelSpark = "spark"
	RoleLabelDrift = "drift"
	RoleLabelOrbit = "orbit"
	RoleLabelForge = "forge"
	RoleLabelNova  = "nova"

	RoleFactEffort    = "effort "
	RoleFactFleet     = "fleet"
	RoleFactSolo      = "solo"
	RoleFactSep       = " · "
	RoleDefaultSuffix = " (default)"

	TuneTitle       = "Quick tune"
	TuneLead        = "tweak just this launch — or esc to keep the party defaults"
	TuneEffortLabel = "effort"
	TuneFleetLabel  = "fleet"
	TuneFleetOn     = "on"
	TuneFleetOff    = "off"
	TuneHint        = "↑↓ row · ←→ change · enter apply · esc defaults"

	EffortLow    = "low"
	EffortMedium = "medium"
	EffortHigh   = "high"
	EffortXHigh  = "xhigh"
	EffortMax    = "max"

	MenuActionLaunch = "Launch"
	MenuActionReview = "Review"
	MenuActionVSCode = "VS Code"
	MenuActionQuit   = "Quit"

	MenuDescLaunch = "setup pipeline + Claude in container"
	MenuDescReview = "harvest ecosystem changes for co-review"
	MenuDescVSCode = "open container root in VS Code"

	WelcomeHint = "pick an action ←"

	GateTitle          = "Update before launch?"
	GateUpToDate       = "up to date"
	GateOutdatedPrefix = "update "
	GateOutdatedSuffix = " available"
	GateCurrentPrefix  = "current "
	GateHint           = "↑↓ move · enter select · esc skip"
	GateOptionSkip     = "Skip"
	GateOptionSelf     = "Self"
	GateOptionPacks    = "Packs"
	GateOptionAll      = "All"
	GateDescSkip       = "launch as-is"
	GateDescSelf       = "rebuild mirabilis (applies next start)"
	GateDescPacks      = "refresh ecosystem repos + live harness"
	GateDescAll        = "self then packs"

	RestartWarnTitle    = "Restart running container?"
	RestartWarnLead     = "Launch will recreate the running container."
	RestartWarnLost     = "lost: live processes · /tmp"
	RestartWarnKept     = "kept: /workspace · ~/.claude · ~/.config/gh"
	RestartWarnConfirm  = "Restart"
	RestartWarnCancel   = "Cancel"
	RestartWarnConfirmD = "recreate and launch"
	RestartWarnCancelD  = "back to menu, change nothing"
	RestartWarnHint     = "↑↓ move · enter select · esc cancel"

	NoticeSelfUpdateRunning  = "rebuilding mirabilis…"
	NoticeSelfUpdateStaged   = "update staged — restart mirabilis to apply"
	NoticeSelfUpdateDegraded = "self-update failed — launching on current binary: "
	LogSelfUpdateFailed      = "self-update failed"

	TooSmall = "terminal too small"
	SizeSep  = " < "

	BusyElapsedSep = " "

	NoticeVSCodeErr = "VS Code: "

	NoticeLaunchFailed     = "launch failed"
	NoticeLaunchCanceled   = "launch canceled"
	NoticeLaunchErrPrefix  = "launch error: "
	NoticeLoadoutErrPrefix = "loadout error: "

	NoticeVSCodeOpening = "opening VS Code…"
	NoticeBusy          = "busy — wait"
	NoticeVSCodeDone    = "VS Code: opened"
	NoticeUpdateRunning = "updating ecosystem and harness…"
	LogVSCodeFailed     = "vscode open failed"
	LogUpdateFailed     = "ecosystem and harness update failed"
)
