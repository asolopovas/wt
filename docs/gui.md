# GUI design system (`internal/gui/`)

No raw pixel literals, hex colors, or `widget.NewButton` + manual styling.

## Tokens (`tokens.go`)

- Spacing: `spaceXS/SM/MD/LG/XL/XXL` (2/4/6/8/12/16)
- Text: `textCaption/Body/Label/Row/Heading` (10/11/12/13/14)
- Borders: `borderSubtle/Default/Strong/Accent`
- Surfaces: `surfacePanel/Raised`
- Actions: `actionPrimary/Danger`

## Components (`components.go`)

- Buttons: `newPrimaryButton` / `newSecondaryButton` / `newDangerButton`. Wrap with `wrapAction` (one-shot) or `wrapGhost` (toggle).
- Layout: `newSectionHeader`, `newSectionDivider`, `newFormField`, `newCaptionText`, `newPanelBackground`.
- Modals: `showDialog(dialogConfig{...})` — never hand-roll `widget.NewModalPopUp`.
- Notifications: `showNotice` / `showError` / `showConfirm`. Never `dialog.ShowError/Information/Confirm` directly. Exception: file pickers `NewFileOpen/Save/FolderOpen`.
- Read-only text modals: `preview.ShowText(...)`. Never `widget.NewMultiLineEntry().Disable()` — pale-gray on Android dark theme, unreadable.
- Aliases: single `aliases.go` for `decor`/`assets` re-exports, `validModels`, `attachLibrary`.

## Modals with text inputs on Android

Set `AnchorTop: true`. Fyne's `widget.NewModalPopUp` re-centers in full canvas size and ignores `Move()`; Android mobile driver doesn't shrink `Canvas.Size()` when soft keyboard opens (only `Canvas.InteractiveArea()` shrinks via `InsetBottomPx`). Centered modal sits half-behind IME. `AnchorTop` switches to non-modal `widget.NewPopUp` at top of `InteractiveArea()` + hand-rolled dim `canvas.Rectangle` overlay.

## Widget reuse

A Fyne widget can only have one parent. To show same control in two tabs, add a mirror factory on the owning panel (see `settingsPanel.newModelSelectMirror`).

## Entry on Android

Mobile driver has limited selection — single-tap places cursor, long-press opens cut/copy/paste, no drag-to-select. For rename/overwrite flows: focus entry and `entry.TypedShortcut(&fyne.ShortcutSelectAll{})` after dialog opens (wrap in `fyne.Do(...)` so it runs after popup is shown). For filename rename, strip extension before display and re-append on save (mirrors Android Files app).

## Truncating text rows

Never `widget.Label` with `Truncation = fyne.TextTruncateEllipsis` next to `canvas.Text` row in same column — Label wraps in `theme.InnerPadding()`, canvas.Text has zero padding → mismatched x-offsets (~4px indent). Use `newTruncText(s, color, size, style)` from `trunctext.go`: wraps `canvas.Text`, `MinSize.Width=0` so row never forces horizontal scroll, truncates with `…` via `fyne.MeasureText` binary search.

## Mirror init order

Panel-level state mutations (e.g. `refreshLanguageOptions` filtering master select's `Options`) run during `settingsPanel.build()` via `defer`. Mirror factories called later by `app.go`/`app_android.go`. Always seed new mirror from master's already-filtered `Options` (and copy `Disabled()` state) — not raw global slice. `LimitSelect.Tapped` reads `Inner.Options` per tap, so stale unfiltered options surface otherwise.
