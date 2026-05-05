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

## Rules

- Modals with text inputs on Android: set `AnchorTop: true`.
- Widget reuse across tabs: add a mirror factory on the owning panel (see `settingsPanel.newModelSelectMirror`).
- Truncating text rows: use `newTruncText(s, color, size, style)` from `trunctext.go`. Never mix `widget.Label` truncation with `canvas.Text` in the same column.
- Mirror init: seed new mirrors from the master's already-filtered `Options` (and copy `Disabled()` state), not the raw global slice.

## Entry on Android

Single-tap = cursor, long-press = cut/copy/paste, no drag-to-select. For rename/overwrite flows: focus entry then `entry.TypedShortcut(&fyne.ShortcutSelectAll{})` after dialog opens, wrapped in `fyne.Do(...)`. For filename rename, strip extension before display and re-append on save.
