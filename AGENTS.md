# AGENTS.md — wt

Go CLI + GUI wrapping whisper.cpp for audio transcription with speaker diarization.
Module: `github.com/asolopovas/wt`. Deps: `fyne.io/fyne/v2`, `pterm`, `urfave/cli/v3`, `yaml.v3`.

@CLAUDE.local.md

## Commands (always via `task`, never bare `go build`)

User-facing tasks are flag-driven; internals are hidden (`internal: true`).

```
task build [ONLY=cli|gui|android]        Build binaries + installer; android = APK
task install [TARGET=android]            Replace local install (or push APK + launch)
task test [SHORT=1|INTEGRATION=1]        Default = full; SHORT skips CGo; INTEGRATION = diarizer
task lint [FIX=1] [ANDROID=1]            golangci-lint (+gofumpt with FIX); ANDROID = NDK toolchain
task release [ROLLING=1]                 Default bumps + publishes; ROLLING updates `rolling` prerelease
task clean [DEEP=1]                       Clean dist/ (+ whisper.cpp build)
task models FETCH=samples|import          Fetch diarization samples / import models from Windows mounts
```

Internal tasks (callable but hidden): `whisper-lib`, `fetch-deps`, `installer-exe`, `linux-deb`, `bump`, `clean-comments`, `android-apk`, `android-whisper-lib`, `android-vulkan-headers`, `ffmpeg-android`, `llama-cli-host`, `android-llama-cli`, `android-test`. The `_build-host`, `_install-host`, `_install-android`, `_lint-android`, `_fetch-samples`, `_models-import`, `_release-stable`, `_release-rolling` tasks are dispatch helpers — invoke via the public flag-driven entrypoints.

GUI compile-checks **only** through `task build ONLY=gui` — `CGO_LDFLAGS` differs between MinGW (`-lwhisper`) and CUDA/MSVC (`whisper.lib`).

Never dispatch to an `internal: true` task via `cmd: 'task X'` — that spawns a new task process which sees the internal flag and exits 202. Always use `task: X` cmd entries (supports `vars:` and `if:` in Task v3.30+). Same applies to compound shell calls like `task whisper-lib && go test ...` — split into a `task:` step plus a `cmd:` step.

Never cross-compile sherpa-onnx for Android via direct cmake — their `cmake/onnxruntime.cmake` only handles Linux/macOS/Windows hosts and aborts with `Only support Linux, macOS, and Windows at present`. Always invoke their upstream `build-android-<abi>.sh` via msys2 bash; it pre-fetches onnxruntime android libs from `csukuangfj/onnxruntime-libs` and sets `SHERPA_ONNXRUNTIME_{LIB,INCLUDE}_DIR` correctly. Same pattern as `ffmpeg-android`.

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
