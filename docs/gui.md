# GUI design system — `internal/gui/`

Use tokens and components below. Never raw pixel literals, hex colors, or `widget.NewButton` + manual styling.

## Tokens — `tokens.go`

`spaceXS/SM/MD/LG/XL/XXL` (2/4/6/8/12/16) · `textCaption/Body/Label/Row/Heading` (10/11/12/13/14) · `borderSubtle/Default/Strong/Accent` · `surfacePanel/Raised` · `actionPrimary/Danger`.

## Components — `components.go`

- Buttons: `newPrimaryButton` / `newSecondaryButton` / `newDangerButton`. Wrap with `wrapAction` (one-shot) or `wrapGhost` (toggle).
- Layout: `newSectionHeader`, `newSectionDivider`, `newFormField`, `newCaptionText`, `newPanelBackground`.
- Modals: `showDialog(dialogConfig{...})`. Never hand-roll `widget.NewModalPopUp`.
- Notifications: `showNotice` / `showError` / `showConfirm`. Never use `dialog.ShowError/Information/Confirm` directly. Exception: file pickers (`NewFileOpen/Save/FolderOpen`).
- Read-only text modals: `preview.ShowText(...)`. Never `widget.NewMultiLineEntry().Disable()`.
- Re-exports: single `aliases.go`.

## Modals with text inputs on Android

Set `AnchorTop: true`. `widget.NewModalPopUp` re-centers in full canvas size and ignores `Move()`; the Android mobile driver doesn't shrink `Canvas.Size()` when the soft keyboard opens (only `Canvas.InteractiveArea()` does), so a centered modal sits half-behind the IME.

## Widget reuse across tabs

A Fyne widget can only have one parent. To show the same control in two tabs, add a mirror factory on the owning panel (see `settingsPanel.newModelSelectMirror`).

## Truncating text rows

Use `newTruncText(s, color, size, style)` from `trunctext.go`. Never mix `widget.Label` truncation (`Truncation = fyne.TextTruncateEllipsis`) with `canvas.Text` in the same column — Label wraps in `theme.InnerPadding()`, `canvas.Text` has zero padding, so you get a ~4 px x-offset mismatch.

## Mirror init order

Mirror factories are called by `app.go` / `app_android.go` *after* panel-level state mutations (e.g. `refreshLanguageOptions`) have run. Always seed a new mirror from the master's already-filtered `Options` (and copy `Disabled()` state), not the raw global slice. `LimitSelect.Tapped` reads `Inner.Options` per tap, so stale unfiltered options surface otherwise.

## Entry on Android

Single-tap = cursor, long-press = cut/copy/paste, no drag-to-select. For rename/overwrite flows: focus entry then `entry.TypedShortcut(&fyne.ShortcutSelectAll{})` after dialog opens, wrapped in `fyne.Do(...)`. For filename rename, strip extension before display and re-append on save.
