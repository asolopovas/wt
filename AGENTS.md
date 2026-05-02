# AGENTS.md — wt

Go CLI + GUI wrapping whisper.cpp for audio transcription with speaker diarization.
Module: `github.com/asolopovas/wt` (Go 1.26). Deps: `fyne.io/fyne/v2`, `pterm`, `urfave/cli/v3`, `yaml.v3`.

@CLAUDE.local.md

## Commands (always via `task`, never bare `go build`)

```
task build [ONLY=cli|gui]   Build binaries → dist/bin (compile-only, never launches)
task install                Replace binaries at %LOCALAPPDATA%\wt
task install-android        Build APK, install, launch
task check | clean [DEEP=1] Verify toolchain | clean dist/ (+ whisper.cpp build)
task test | test-unit | test-integration   (-short skips CGo/model; integration needs build tag)
task fetch-samples          Stage diarization samples → samples/diarization/ (gitignored)
task lint | lint-android | vet | vet-android | fmt | clean-comments
task bump                   Auto-increment version (1.0.0→1.0.9→1.1.0), build, install, commit
task release                Bump → build installer + APK → push tag → GH release + upload
task release-latest         Build current HEAD, force-update `rolling` prerelease tag
```

GUI compile-checks **only** through `task build ONLY=gui` — `CGO_LDFLAGS` differs between MinGW (`-lwhisper`) and CUDA/MSVC (`whisper.lib`).

Release tasks create the GH release empty first, then upload artifacts in a 3-attempt retry loop (large APK/EXE uploads occasionally fail mid-stream). `task release` re-reads `VERSION` from `Taskfile.yml` post-bump because `{{.VERSION}}` captures at parse time.

## Layout

```
cmd/{wt,wt-gui,wt-test}        CLI / Fyne GUI / Android test CLI
internal/gui/                  Fyne GUI (see design system below)
internal/transcriber/          Audio, model, CSV, live mode
internal/diarizer/             NeMo subprocess + sherpa-onnx
internal/llm/                  llama-cli subprocess
internal/ui/                   Terminal spinners (CLI only)
bindings/go/                   Vendored whisper.cpp CGo bindings (own go.mod)
scripts/                       Build helpers, Inno Setup, diarize.py, diar_sweep.py
third_party/whisper.cpp        Cloned at build time (gitignored)
```

## GUI design system (`internal/gui/`)

No raw pixel literals, hex colors, or `widget.NewButton` + manual styling.

- **Tokens** (`tokens.go`): spacing `spaceXS/SM/MD/LG/XL/XXL` (2/4/6/8/12/16); text `textCaption/Body/Label/Row/Heading` (10/11/12/13/14); `borderSubtle/Default/Strong/Accent`; `surfacePanel/Raised`; `actionPrimary/Danger`.
- **Components** (`components.go`): `newPrimaryButton` / `newSecondaryButton` / `newDangerButton`; wrap with `wrapAction` (one-shot) or `wrapGhost` (toggle). Layout: `newSectionHeader`, `newSectionDivider`, `newFormField`, `newCaptionText`, `newPanelBackground`. Modals: `showDialog(dialogConfig{...})` — never hand-roll `widget.NewModalPopUp`.
- **Notifications**: `showNotice` / `showError` / `showConfirm`. Never `dialog.ShowError/Information/Confirm` directly (file pickers `NewFileOpen/Save/FolderOpen` are the only exception).
- **Aliases** (`aliases.go`): single file for `decor`/`assets` re-exports, `validModels`, `attachLibrary`.
- **Widget reuse**: a Fyne widget can only have one parent. To show the same control in two tabs, add a mirror factory on the owning panel (see `settingsPanel.newModelSelectMirror`).

## Testing

stdlib `testing` only. Names: `Test<Function>_<Scenario>`, table-driven preferred. Config tests: `t.TempDir()` + `t.Setenv("HOME", ...)`. CI runs `go vet`, `golangci-lint`, full `go test` on Linux. Diarization integration tests: `//go:build integration` (`task test-integration`); `getSherpaSample` lazy-downloads to `samples/diarization/sherpa/` (gitignored). Use `-short` to skip the download. Don't add skip-on-missing-model tests.


## Self-improvement

When you discover a non-obvious gotcha, footgun, or workflow rule that future sessions would benefit from, you **MUST** propose an AGENTS.md edit before ending the turn. Triggers: user corrects an approach ("don't do X"); a build/test/tooling failure has a non-obvious fix not documented here; you re-derive a project fact you've derived before; a command in this file is wrong. Keep additions terse and rule-shaped (Always / Ask / Never), not narrative. Don't add training-data-level advice. Don't commit the change unless instructed.
