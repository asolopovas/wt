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

When building sherpa-onnx for Android with the **static** onnxruntime archive (`BUILD_SHARED_LIBS=OFF`), keep `SHERPA_ONNX_ANDROID_PLATFORM=android-21` (sherpa upstream default). API ≥ 27 unconditionally pulls in `nnapi_provider_factory.h` from `session.cc`, but the static prebuilts ship without that header (only the JNI/AAR distribution includes it). Binary remains forward-compatible with API 28+ devices.

Release tasks create the GH release empty first, then upload artifacts in a 3-attempt retry loop (large APK/EXE uploads occasionally fail mid-stream). `task release` re-reads `VERSION` from `Taskfile.yml` post-bump because `{{.VERSION}}` captures at parse time. Vars passed via `vars:` on a `task: <name>` invocation do **not** propagate through nested `task:` calls in Task v3 — every leaf that touches `{{.VERSION}}` (`_build-host`, `installer-exe`, `linux-deb`, `_install-host`, `_install-android`, `android-apk`) needs its own task-local `vars: { VERSION: { sh: awk -F\"\\x27\" '/^  VERSION:/{print $2; exit}' Taskfile.yml } }` so the post-bump value is used. The `_release-stable` task must also call `installer-exe`/`linux-deb` explicitly because `bump`'s `task: install` runs `_build-host` with `ONLY=host` (skipping the installer step), and the subsequent `task: build` then sees `_build-host` as up-to-date and never reaches the installer.

Never use `grep ... | sed -E "s/.../\1/"` inside a Task `sh:` var — mvdan/sh mangles the `\1` backref. Use `awk -F"\x27" '/pattern/{print $2}'` instead. Same shell also panics on fd>=10 redirects (`exec 9>>file`) — use a cp-retry loop instead of fd-based lock probing on Windows.

Windows holds DLL handles briefly after `taskkill` returns. `_install-host` must wait + retry `cp` (whisper.dll especially) — a single sleep is insufficient under load.

Android Java sources live in `scripts/android-service/com/asolopovas/wtranscribe/`. Adding a class requires three coordinated edits: (1) drop the `.java` next to existing ones, (2) extend the `javac` argument list **and** the `d8` argument list in `Taskfile.yml` `android-apk` (d8 takes `.class` files — never `.java`), (3) declare any `<service>` / `<provider>` / `<receiver>` in `cmd/wt-gui/AndroidManifest.xml.in`. ContentProviders sharing files to other apps need `android:grantUriPermissions="true"` on the `<provider>` and `FLAG_GRANT_READ_URI_PERMISSION` on the `Intent`. Verify registration after install with `adb shell dumpsys package <id> | grep -A1 -i provider`.

Never call `FindClass` for app classes from a JNI thread — the boot ClassLoader can't see them. Use the `loadClass` pattern from `wt_fp_load_app_class` / `wts_load_app_class` (load via `activity.getClassLoader()`).

LLM auto-rename uses `llama-cli` as a subprocess; mobile CPUs are slow. Per-invocation timeout is `llmTimeout()` in `internal/llm/runner.go` (10 min Android, 2 min desktop, override `WT_LLM_TIMEOUT` seconds). Always update the live status (`p.setStatus`) and foreground notification (`platsvc.UpdateProgress`) for any post-transcription phase that can run >1 s, otherwise the UI looks frozen on the previous phase’s message.

GBNF grammars: never write `(rule)? (rule)? …` chains — each `?` doubles the state space and llama.cpp's sampler degrades to effectively single-threaded grammar evaluation. A 55-`?` chain made auto-rename hang for 4+ minutes on a phone (single 100% CPU thread despite `-t 6`). Use the `{n,m}` repetition operator (e.g. `slugChar{5,60}`); same expressiveness, ~100× faster. Verify with `time llama-cli ... --grammar-file g.gbnf` before merging grammar changes.

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


## Scratch artifacts

Never write screenshots, logs, or other ad-hoc binary debug files (`*.png`, `*.jpg`, capture dumps, etc.) into the repo root or any tracked directory. Use the system tempdir (`/tmp/...` on msys, `$TMPDIR`) or a `_tmp/` subdir at the repo root (gitignored) when a tool can't read outside the project. Read tool can't access `C:\tmp` directly — copy/move into `_tmp/` then read. Clean up afterwards.

## Self-improvement

When you discover a non-obvious gotcha, footgun, or workflow rule that future sessions would benefit from, you **MUST** propose an AGENTS.md edit before ending the turn. Triggers: user corrects an approach ("don't do X"); a build/test/tooling failure has a non-obvious fix not documented here; you re-derive a project fact you've derived before; a command in this file is wrong. Keep additions terse and rule-shaped (Always / Ask / Never), not narrative. Don't add training-data-level advice. Don't commit the change unless instructed.
