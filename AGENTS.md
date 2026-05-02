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

## Diarization

Backend selection via `internal/diarizer/New(numSpeakers)` / `NewWithPreference(numSpeakers, preferSherpa)`:

| Caller              | Default        | Override                          |
|---------------------|----------------|-----------------------------------|
| CLI (`wt`)          | NeMo           | `--speakers N` (N>0) → sherpa     |
| GUI (Windows)       | sherpa+titanet | always sherpa                     |
| GUI / CLI (Android) | sherpa+titanet | only backend (no bundled CPython) |

**NeMo streaming Sortformer v2** (`nvidia/diar_streaming_sortformer_4spk-v2`, GPU, ≤4 speakers, auto-detects count). Don't downgrade to v1: its `model.diarize(audio=path)` loads the entire signal in one tensor (~12 min cap on 48 GB GPU; ~18 GB RSS for 22-min clip page-faults the desktop). v2 streaming uses chunk+FIFO+speaker-cache so memory scales with `chunk_len + fifo_len + spkcache_len` frames (80 ms each). Use the high-latency / best-accuracy preset (`chunk_len=340 chunk_right_context=40 fifo_len=40 spkcache_update_period=300 spkcache_len=188`) on `model.sortformer_modules` and call `_check_streaming_parameters()` before `model.diarize(...)`. Output format unchanged from v1.

**Sherpa-onnx tuned params** (winner of 11×8×3 sweep, mean DER 0.137 — do not change, do not re-run `scripts/diar_sweep.py`):

```
--segmentation.pyannote-model = pyannote-3.0
--embedding.model             = nemo_en_titanet_large.onnx   (~96 MB)
--clustering.cluster-threshold = 0.75   (replaced with --num-clusters=N when --speakers N)
--min-duration-on              = 0.3
--min-duration-off             = 0.5
```

## GUI design system (`internal/gui/`)

No raw pixel literals, hex colors, or `widget.NewButton` + manual styling.

- **Tokens** (`tokens.go`): spacing `spaceXS/SM/MD/LG/XL/XXL` (2/4/6/8/12/16); text `textCaption/Body/Label/Row/Heading` (10/11/12/13/14); `borderSubtle/Default/Strong/Accent`; `surfacePanel/Raised`; `actionPrimary/Danger`.
- **Components** (`components.go`): `newPrimaryButton` / `newSecondaryButton` / `newDangerButton`; wrap with `wrapAction` (one-shot) or `wrapGhost` (toggle). Layout: `newSectionHeader`, `newSectionDivider`, `newFormField`, `newCaptionText`, `newPanelBackground`. Modals: `showDialog(dialogConfig{...})` — never hand-roll `widget.NewModalPopUp`.
- **Notifications**: `showNotice` / `showError` / `showConfirm`. Never `dialog.ShowError/Information/Confirm` directly (file pickers `NewFileOpen/Save/FolderOpen` are the only exception).
- **Aliases** (`aliases.go`): single file for `decor`/`assets` re-exports, `validModels`, `attachLibrary`.
- **Widget reuse**: a Fyne widget can only have one parent. To show the same control in two tabs, add a mirror factory on the owning panel (see `settingsPanel.newModelSelectMirror`).

Never reintroduce `settingsField`/`sidebarHeader`/`sidebarDivider`/`borderedBtn`; never mix toggle borders.

## Platform splits

`//go:build android` / `//go:build !android` pairs: `config_*`, `app_*`, `transcribe_*`, `audio_*`. Windows uses `_windows.go` / `_other.go` (with `//go:build !windows`).

If a new package imports a desktop-only symbol from `internal/transcriber` (e.g. `FindFFmpeg`, behind `//go:build !android`), tag every file in the new package `//go:build !android` too — else `task vet-android` fails with `undefined: transcriber.<sym>`. Once an android-tagged implementation exists (see `ffmpeg_android.go`), drop the `!android` tag downstream.

## Android-bundled CLI binaries

APK ships CLI binaries (sherpa-diar, llama-cli, ffmpeg) renamed `lib<name>.so` under `lib/arm64-v8a/`. Android only grants exec permission to files matching `lib*.so` inside `nativeLibraryDir` — bundling as `assets/<name>` or `bin/<name>` does not work. Locate at runtime by globbing `nativeLibraryDir`; the dir-discovery pattern (env vars → `/proc/self/maps` → `/data/app/.../lib/arm64` glob) is duplicated across `internal/llm/dirs_android.go`, `internal/diarizer/sherpa.go`, `internal/transcriber/ffmpeg_android.go`.

Adding a new bundled binary: (1) `scripts/build-<name>-android.sh`, (2) `task <name>-android` depended on by `android-apk`, (3) env var + `zout.write` in `scripts/android-apk-patch.py`, (4) Go-side `Find<Name>()` globbing `nativeLibraryDir`.

## Cross-compile + Android Fyne/FFmpeg gotchas

- Autoconf projects (FFmpeg) → android-arm64 require **msys2 GNU make** (`pacman -S make`), not `mingw32-make.exe` (chokes on `include /c/...` paths). Use `--disable-asm` (configure asm probes flaky on Windows; perf irrelevant for cached one-shot peak extraction).
- From Taskfile shell blocks, invoke msys2 bash with Windows paths (`C:/msys64/usr/bin/bash.exe`), not msys-style — Task's `mvdan/sh` doesn't translate the latter.
- `-f s16le` (raw PCM) requires `--enable-muxer=pcm_s16le` in FFmpeg configure (not `s16le`). Same for `pcm_s16be`/`pcm_s24le`/`pcm_f32le`. Symptom: ffmpeg exits 234 with `Requested output format 's16le' is not known`.
- `container.Border` doesn't reflow when a hidden bottom child becomes visible; neither `child.Refresh()` nor `window.Content().Refresh()` suffices. Call `window.SetContent(...)` again with a fresh Border via `onVisibilityChange` callback (see `internal/gui/dock.go` + `app{,_android}.go`).
- Touchscreen taps on a widget implementing both `Tappable` and `Draggable` often dispatch as `Dragged(0,0)` → `DragEnd` instead of `Tapped`. Handle tap-to-X in `DragEnd` when total drag distance is zero.
- `Player.Stop()` should not synchronously fire `onStop` when the caller plans an immediate `StartRange` (seek-restart pattern). Both desktop (`stopping` flag) and android suppress the callback in explicit `Stop()` and rely on the watcher goroutine for natural end. Otherwise a seek hides the playhead and flips the play icon.

## llama-cli subprocess

- Don't use `shared.HideWindow` (CREATE_NO_WINDOW) on llama-cli — its console probes fail and generation hangs past any timeout. Use `internal/llm.hideLlamaWindow` (`SysProcAttr.HideWindow=true`, no CREATE_NO_WINDOW).
- Pass `--single-turn`. Newer builds (b8999+) silently auto-enable conversation mode with instruct templates and ignore `--no-conversation`/`--no-display-prompt`. Without `-st`, the process never exits and `cmd.Wait()` times out. Keep `cmd.Stdin = nil` (not `strings.NewReader("")`).
- llama-cli echoes the prompt to stdout in chat mode despite `--no-display-prompt`. Grammar-constrained JSON appears AFTER the echo, so when extracting model JSON, scan for the **last** balanced `{...}` block, not the first.

## Subprocess IPC

`internal/diarizer/subproc.go` is canonical for any backend spawning an external process — owns pipe setup, optional stderr line interceptor (return `true` to skip tail buffer), tail-on-error formatting via `wait(ctx)`. Both nemo and sherpa go through it. Don't reintroduce ad-hoc `cmd.StdoutPipe`/`cmd.StderrPipe` + `sync.Mutex` in new diarizer or transcribe-side code — extend `subproc`.

## Config env vars

`shared.Load()` applies `WT_*` overrides after YAML parse: `WT_MODEL`, `WT_LANGUAGE`, `WT_DEVICE`, `WT_THREADS`, `WT_SPEAKERS`, `WT_NO_DIARIZE`, `WT_TDRZ`, `WT_CACHE_EXPIRY_DAYS`. Booleans accept `1/true/yes/on` and `0/false/no/off` (case-insensitive). Invalid values are silently ignored.

## Testing

stdlib `testing` only. Names: `Test<Function>_<Scenario>`, table-driven preferred. Config tests: `t.TempDir()` + `t.Setenv("HOME", ...)`. CI runs `go vet`, `golangci-lint`, full `go test` on Linux. Diarization integration tests: `//go:build integration` (`task test-integration`); `getSherpaSample` lazy-downloads to `samples/diarization/sherpa/` (gitignored). Use `-short` to skip the download. Don't add skip-on-missing-model tests.

## Boundaries

**Always:** before every commit run `task lint && task test && task vet` (CGo env). No `--no-verify`. `Taskfile.yml VERSION` is the single source of truth; `installer.iss` gets it via ISCC `/D`.

**Always:** when relocating a package and its dependencies form a cycle with the new parent, break the cycle via a package-level injection variable set from the parent's `init()` (e.g. `cache.ProbeDurationMsFn = transcriber.ProbeDurationMs` in `internal/transcriber/cache_probe.go`). Don't move helper functions speculatively to dodge the cycle — break the specific edge that closes it.

**Always:** before committing any change that adds or modifies `.go` files, run `task clean-comments` (after confirming `git status` is clean per the existing dirty-tree rule). The existing "Never write Go comments" rule is easy to miss when focused on logic; the task is the enforcement mechanism.

**Ask first:** anything mutating state outside the repo (installs, registry, version bumps).

**Never:**
- Use Unix text tools (`awk`/`grep`/`sed`/`head`/`cut`) in `Taskfile.yml` shell blocks unless that task already exports `PATH=...msys64/mingw64/bin;...` (build/install do; release/bump don't). Prefer `powershell -NoProfile -Command "..."`. Use `-CaseSensitive` on `Select-String` so `VERSION:` doesn't collide with `version: '3'`. When capturing PS output via `$(powershell ...)`, emit with `[Console]::Write(...)` — default formatter appends `\r\n`, bash strips `\n` but keeps `\r`, and `git tag` rejects the `\r` suffix.
- Commit, push, or launch the GUI unless explicitly instructed. If GUI launches: `taskkill /F /IM wt-gui.exe`.
- Bump version unless user types "bump".
- Add `-tags=android` to `.vscode/settings.json` gopls flags (cascades fake `C.xxx` errors). Use `wt-android.code-workspace` for android-tagged editing.
- Re-scope lint to `./...` (vendored cgo bindings won't compile standalone).
- Write Go comments other than directives (`//go:build`, `//go:embed`, `//go:generate`, `//export`, `//line`, `// +build`, `// Code generated ... DO NOT EDIT.`, cgo preamble).
- Add em dashes (`—`) to git commit messages. Plain hyphens fine.
- Mark a Taskfile task `internal: true` if invoked from a shell `cmd:` block (`task: Task "<name>" is internal`, exit 202). Use `- task: <name>` references in `cmds:` for internal tasks.
- Run `task clean-comments` over a dirty tree without checking `git status` first — it unconditionally rewrites every `.go` file and `gofumpt`s, silently stripping in-flight comments.
- Combine Taskfile `status:` with `sources:`/`generates:` on the same task. `status:` returning 1 forces always-run, bypassing source-checksum caching. Pick one per task. Each shell-`task X` callout costs ~300 ms (parent re-evaluates every `vars: sh:`) — prefer top-level source-tracking on `build` over many small per-binary sub-tasks.

## Self-improvement

When you discover a non-obvious gotcha, footgun, or workflow rule that future sessions would benefit from, you **MUST** propose an AGENTS.md edit before ending the turn. Triggers: user corrects an approach ("don't do X"); a build/test/tooling failure has a non-obvious fix not documented here; you re-derive a project fact you've derived before; a command in this file is wrong. Keep additions terse and rule-shaped (Always / Ask / Never), not narrative. Don't add training-data-level advice. Don't commit the change unless instructed.
