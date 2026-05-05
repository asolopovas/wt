# AGENTS.md — wt

Go CLI + GUI + Android APK wrapping sherpa-onnx for audio transcription with speaker diarization and optional LLM-based renaming.

Module: `github.com/asolopovas/wt` (Go 1.26).
Direct deps: `fyne.io/fyne/v2`, `pterm`, `urfave/cli/v3`, `gopkg.in/yaml.v3`, `fsnotify/fsnotify`, `xuri/excelize/v2`, `lumberjack.v2`.

## Commands (always via `task`, never bare `go build`/`go test`)

```
task build  [ONLY=cli|gui|android]                Build binaries (+ installer/.deb); android = APK
task install [TARGET=android] [QUICK=1]           Replace local install (or push APK + launch). QUICK = redeploy bins only.
task test    [SHORT=1|INTEGRATION=1]              Default = full; SHORT skips cgo; INTEGRATION = diarizer suite
task check   [ANDROID=1]                          Single quality gate (see "Quality gate" below)
task release [BUMP=1]                             Default = dev prerelease (no bump); BUMP=1 = bump patch + stable GH release
task clean   [DEEP=1]                             Clean dist/ (+ third_party builds)
task models  FETCH=samples|import                 Fetch diarization samples / import models
```

Sub-taskfiles: `Taskfile.android.yml` (APK build, ADB install, lint), `Taskfile.release.yml` (dev + stable publish).

## Quality gate — `task check`

Runs in order, must all pass before commit:

1. `go run ./scripts/clean-comments ./cmd ./internal` — strips comments from generated code.
2. `golangci-lint fmt` + `gofumpt -w` — formatting.
3. `golangci-lint run` — linters: errcheck, govet (enable-all minus fieldalignment/shadow), staticcheck, unused, ineffassign, gocritic, gosec, misspell, unconvert, bodyclose, errorlint, nilerr, nolintlint.
4. `deadcode -test ./cmd/wt/... ./cmd/wt-gui/... ./cmd/wt-test/...`.
5. `govulncheck ./cmd/... ./internal/...`.
6. Full `go test`.

`ANDROID=1` swaps step 1–6 for the NDK build-tag check (`task android:lint`). Tools auto-install into `$(go env GOPATH)/bin` via `ensure:tools` (golangci-lint v2, deadcode, govulncheck, gofumpt) pinned to current Go toolchain.

## Layout

```
cmd/
  wt/                              CLI entrypoint (urfave/cli v3)
  wt-gui/                          Fyne GUI entrypoint (+ AndroidManifest.xml.in)
  wt-test/                         On-device / dev CLI for engine + diarizer probes

internal/
  appinfo/                         Version + build-date rendering (DisplayVersion)
  config.go, default_config.yml    Shared config (yaml), embedded defaults, versioned
  config_android.go / _default.go  Per-platform model dir overrides
  download.go                      HTTP download helpers (model fetch)
  logfile.go                       lumberjack rotating log
  hidewindow_*.go                  Windows console hider stub

  diarizer/                        Speaker diarization
    nemo.go                        NeMo Sortformer subprocess (uv + diarize.py)
    sherpa.go                      sherpa-onnx offline diarization
    subproc.go, types.go           Shared subprocess + RTTM types
    integration_test.go            //go:build integration

  transcriber/                     ASR pipeline
    audio.go / audio_android.go    PCM read, ffmpeg invocation
    ffmpeg_android.go              Android-bundled ffmpeg path resolution
    engine.go                      Job + dispatcher (runASR)
    engine_chunk.go                Chunked driver (mandatory; never one-shot)
    engine_zipformer.go            sherpa-onnx engines (whisper-onnx/zipformer/parakeet/sensevoice/moonshine/canary/nemo-ctc; gigaam-v3-ru routes through nemo-ctc)
    model.go                       Model resolve, catalog → on-disk paths
    live.go                        Live (mic) mode
    csv.go, format.go, output.go   Coalescing + output formatting
    wav.go                         WAV header parsing
    job.go                         JobSpec
    cache/                         Result cache (probe + store)

  llm/                             llama-cli subprocess (rename, summarize)
    runner.go                      Subprocess runner
    dirs_*.go                      Per-platform binary discovery

  models/                          Model catalog + manager
    catalog.go                     Curated ASR / diarizer models (csukuangfj mirrors)
    manager.go, paths.go           Download / import / locate
    external_*.go                  Per-platform external dirs

  namer/                           Filename generation (autorename + LLM)
  progress/                        Smoothed progress reporter
  ui/                              Terminal spinners / pterm wrappers (CLI only)

  gui/                             Fyne GUI (desktop + Android)
    app.go / app_android.go        Entrypoint, lifecycle, platform init
    layouts.go / layouts_desktop.go
    components.go, aliases.go      Component re-exports (single source)
    tokens.go, theme.go, theme_desktop.go, rename_theme.go
    history.go, settings*.go       Tabs + persistence
    model_picker.go                Shared model selector (mirror pattern)
    timepicker.go, datepicker.go
    trunctext.go                   Truncating canvas text rows
    languages.go, device.go, dock.go, version_label.go
    decor/                         Buttons, dialogs, forms, notifications, progress, select, colors, tokens
    transcribe/                    Transcribe tab: panel, runner, share, export (rtf/bundle), drop area, recorder, AI rename, tray
    preview/                       Read-only text modals (ShowText)
    player/                        Audio playback (per-platform)
    waveform/                      Peaks + canvas widget
    sysstats/                      CPU/mem/affinity/priority probes (per-OS)
    platsvc/                       Android platform services: foreground service, wakelock, keep-screen-on, MediaStore, share intents, permissions, SDK probe
    assets/                        Bundled GUI resources

scripts/                           Build/install/release helpers, Inno Setup, diarize.py, diar_sweep.py, clean-comments tool, android-service Java
docs/                              Topic-scoped rules (load on demand)
third_party/sherpa-onnx            Cloned at build time (gitignored)
third_party/sherpa-onnx-cuda       Pre-built CUDA binaries downloaded at build time (gitignored)
third_party/llama.cpp              Built or downloaded at build time (gitignored)
samples/                           Test audio (+ samples/diarization/sherpa/ gitignored)
_tmp/                              Local scratch (gitignored)
```

## Topic-scoped rules (read on demand)

- `docs/build-release.md` — Taskfile dispatch, mvdan/sh quirks, version propagation, QUICK install, Windows DLL retry, sherpa cross-compile, llama-cli host download.
- `docs/gui.md` — Design tokens, components (`decor/`), modals (`showDialog`/`preview.ShowText`), Android Entry rules, mirror init, truncating rows.
- `docs/android.md` — Java sources, native lib discovery via `/proc/self/maps`, JNI `loadClass` rule, sherpa CLI bundling as `lib*.so`, model storage, NNAPI, screenshot workflow.
- `docs/asr.md` — Chunked driver invariant, engine selection (`Job.runASR`), per-engine quirks, token coalescing, catalog policy + reject list, LLM auto-rename rules.
- `docs/testing.md` — Conventions, integration build tag, cgo gotchas, GUI compile-check.

## Always (cross-cutting invariants)

### Process
- Run `task check` before every commit; fix every issue it reports. CI mirrors it.
- Use `task` for all builds/tests/installs. Never bare `go build` / `go test` (cgo flags + asset embedding break).
- Touch only one taskfile per concern: host stuff in `Taskfile.yml`, Android in `Taskfile.android.yml`, publishing in `Taskfile.release.yml`. Don't duplicate.

### Code hygiene
- **No comments in generated Go code.** `clean-comments` strips them from `./cmd` and `./internal`. Same convention applies to bash / YAML / Python / JS we author (shebangs stay), enforced by review rather than tooling. Rationale belongs in `docs/*.md` or commit messages.
- Remove dead code, imports, and tests for removed code in the same change. `deadcode` will catch leftovers.
- No new direct dependencies without first checking if an existing one (fyne, pterm, urfave/cli, yaml.v3, fsnotify, excelize, lumberjack) covers the need.
- Errors: wrap with `%w`, never `errors.New(fmt.Sprintf(...))`. Respect `errorlint` / `nilerr`.
- Subprocesses: always pass `context.Context`, propagate cancellation, capture stderr, surface non-zero exits with the command line.
- File I/O: prefer `os.ReadFile` / `os.WriteFile`; close every `Open`/`Create` (`bodyclose` covers HTTP).
- Concurrency: protect shared state with mutexes or channels; never assume Fyne callbacks run on the main goroutine — wrap UI mutations in `fyne.Do(...)`.

### Performance
- ASR must stay chunked — see `docs/asr.md`. Never reintroduce a "process whole file" path.
- Reuse buffers / `sync.Pool` for hot audio paths; don't allocate per-frame.
- Coalesce BPE sub-word tokens before any `strings.Join(parts, " ")` (`coalesceTokens` in `engine_zipformer.go`).
- Cache model resolution + result lookups via `internal/transcriber/cache`; don't re-stat or re-hash on every job.

### Layout discipline
- Platform splits use build tags + `_android.go` / `_other.go` / `_default.go` / `_linux.go` / `_windows.go` filenames. (`_other.go` and `_default.go` both rely on explicit `//go:build` tags — they are not Go-recognized GOOS suffixes.) Don't gate with `runtime.GOOS` checks inside one file when a build tag works.
- GUI widgets, styling, modals, and notifications go through `internal/gui/decor` + `tokens.go` — see `docs/gui.md` for the component list.

### Filesystem
- Never write screenshots, logs, or binary debug artifacts into tracked dirs. Use `os.TempDir()` or `_tmp/` (gitignored).
- **Runtime log for troubleshooting** (`wt.log`, rotated by `internal/logfile.go` via lumberjack):
  - Android: `/storage/emulated/0/Documents/WTranscribe/wt.log` — pull with `adb -s <serial> shell cat /storage/emulated/0/Documents/WTranscribe/wt.log` (and `config.yml` lives next to it). Also surfaced in Settings → VIEW LOG.
  - Desktop: `<os.UserCacheDir>/wt/imports/wt.log` (Linux: `~/.cache/wt/imports/wt.log`, macOS: `~/Library/Caches/wt/imports/wt.log`, Windows: `%LOCALAPPDATA%\wt\imports\wt.log`).
  - `adb logcat` only captures Android system + native loader output; for app-level errors (engine resolve failures, model paths, runner phases) always read `wt.log` first.
- Model storage paths come from `internal/models/paths.go` + `external_*.go`, with `internal/config_android.go` owning the `/storage/emulated/0/Documents/WTranscribe` override on Android. Never hard-code `/sdcard/...` or `~/.cache/...` in other Go files. (Doc/adb examples are fine.)
- Android in-process code never globs `/data/app/*/...` — see `docs/android.md` for the `/proc/self/maps` discovery pattern.

### Build / release
- Both `wt` and `wt-gui` need `-X main.BuildDate=$GIT_DATE` ldflags; render via `appinfo.DisplayVersion`.
- GUI compile-checks **only** through `task build ONLY=gui` (CGO_LDFLAGS differ between MinGW and CUDA/MSVC).
- New Java class → drop in `scripts/android-service/com/asolopovas/wtranscribe/`, extend the `javac` + `d8` invocations in `scripts/build-apk.sh`, declare in `cmd/wt-gui/AndroidManifest.xml.in`.
- New sherpa / llama-cli launcher must check Android `lib*.so` path before `exec.LookPath`.

### Windows builds via the local VM (`~/os/windows-vm`)
The Windows installer (`dist/wt-setup-*.exe`) is produced inside a Tiny11
Docker VM — see `~/os/windows-vm/AGENTS.md` for the full runbook. From
the wt repo you only need:

- `ssh windows-vm` — SSH alias resolves to `localhost:2222`, user `andrius`,
  default shell is Git-Bash. Configured in `~/.ssh/config`.
- `cd ~/os/windows-vm && make wt-build` — clones/pulls wt into `C:\wt`
  inside the VM, runs `task build:windows`, scps the resulting
  `wt-setup-*.exe` back to `~/os/windows-vm/shared/`.
- `task release` (this repo) calls into `windows-vm` automatically when
  it needs the Windows artifact; SSH must be reachable.

If `ssh windows-vm` fails:
1. `cd ~/os/windows-vm && make status` — is the container up?
2. `make ssh-key` — sync `~/.ssh/id_rsa.pub` into `shared/authorized_keys`.
3. `make ssh-enable` — prints the one-time elevated-PowerShell command
   to bootstrap sshd on an existing install (OEM `install.bat` only fires
   on a fresh install). The VNC-bridge automation for that step is
   documented in `~/os/windows-vm/AGENTS.md`.

Never hard-code Windows paths or `localhost:2222` into Taskfiles — always
go through the `windows-vm` SSH alias so the connection details live in
one place (`~/.ssh/config` + `~/os/windows-vm/`).
