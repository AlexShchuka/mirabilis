# TUI principles — mirabilis

Owner doctrine, dictated 2026-06-12. Binding on every TUI change; reviewed against this file. Mechanical homes where they exist are named per item; everything else is enforced at PR review against this document.

## Linter exceptions

`App.ctx` stores a `context.Context` in a struct field; this is the recognized Bubble Tea program-lifetime exception — set once in `New`, cancelled only on quit, analogous to `http.Request`.

## Principles

1. **Adaptive Layout.** Every dimension is a function of the terminal size (`WindowSizeMsg`), like responsive CSS: proportional splits, named breakpoints for narrow/short terminals, reflow on resize. Constant widths/heights are forbidden; only named minimums and breakpoint thresholds may be constants. The frame never renders taller than the viewport.
2. **Keyboard-Driven & Efficiency.** Mouse-free operation; the path to any action is the minimum number of keystrokes. Arrows and vim keys (j/k) navigate; Enter confirms; Esc steps back; q quits from the root.
3. **Visual Hierarchy & Contrast.** The eye lands on the selected element first; state colors (ok/degraded/off) are distinguishable; chrome (header/footer) recedes behind content.
4. **Feedback & Predictability.** Every keypress produces a visible response. Long operations show live progress (spinner/elapsed), never a frozen notice. The same key always does the same thing in the same context.
5. **Accessibility & Compatibility.** Honors `NO_COLOR` and low-capability terminals (`TERM=dumb` degrades, never corrupts); defined minimum terminal size with a graceful "terminal too small" state; restores the terminal exactly on exit and around child handoffs.

## Patterns

- **Матрёшка (State Isolation).** One screen — one task. Deep settings live deeper in the menu until needed.
- **Слои (Overlays & Modals).** Hints and forms render above the interface; the user never loses the context behind them.
- **Память истории (Smart Defaults).** Last choices are remembered and pre-filled; frequent values are anticipated.
- **Текстовый минимализм (Scan-not-Read).** Text is cut to keywords; a screen is scannable in one second.
- **Опережающий ввод (Typeahead/Buffering).** Keystrokes typed before the screen finishes drawing are queued, never dropped.
