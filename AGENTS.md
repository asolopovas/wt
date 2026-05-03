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

Running `task` from PowerShell (not Git Bash) fails with `"awk": executable file not found in $PATH` because PowerShell's PATH usually omits `C:\Program Files\Git\usr\bin`. Task's mvdan/sh interpreter still needs external Unix tools (awk/grep/sed/find) on disk. Fix: prepend Git's `usr\bin` to the user PATH (`[Environment]::SetEnvironmentVariable('Path','C:\Program Files\Git\usr\bin;'+[Environment]::GetEnvironmentVariable('Path','User'),'User')`), or run task from Git Bash. Don't try to rewrite the awk calls — see the next rule.

Never use `grep ... | sed -E "s/.../\1/"` inside a Task `sh:` var — mvdan/sh mangles the `\1` backref. Use `awk -F"\x27" '/pattern/{print $2}'` instead. Same shell also panics on fd>=10 redirects (`exec 9>>file`) — use a cp-retry loop instead of fd-based lock probing on Windows.

Windows holds DLL handles briefly after `taskkill` returns. `_install-host` must wait + retry `cp` (whisper.dll especially) — a single sleep is insufficient under load.

Android Java sources live in `scripts/android-service/com/asolopovas/wtranscribe/`. Adding a class requires three coordinated edits: (1) drop the `.java` next to existing ones, (2) extend the `javac` argument list **and** the `d8` argument list in `Taskfile.yml` `android-apk` (d8 takes `.class` files — never `.java`), (3) declare any `<service>` / `<provider>` / `<receiver>` in `cmd/wt-gui/AndroidManifest.xml.in`. ContentProviders sharing files to other apps need `android:grantUriPermissions="true"` on the `<provider>` and `FLAG_GRANT_READ_URI_PERMISSION` on the `Intent`. Verify registration after install with `adb shell dumpsys package <id> | grep -A1 -i provider`.

Never glob `/data/app/*/...` from inside an app process to locate native libs — the app user gets EPERM listing `/data/app/`. Instead read `/proc/self/maps` and extract dirs containing already-loaded `.so` files: those are the real `nativeLibraryDir` paths Android resolved at load time, randomised UUID subdir names included. Pattern lives in `internal/diarizer/sherpa.go:androidNativeLibDirs` and mirrored in `internal/transcriber/engine_zipformer.go`. Any new sherpa/llama-cli launcher must use this helper, not `filepath.Glob`.

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
- **Components** (`components.go`): `newPrimaryButton` / `newSecondaryButton` / `newDangerButton`; wrap with `wrapAction` (one-shot) or `wrapGhost` (toggle). Layout: `newSectionHeader`, `newSectionDivider`, `newFormField`, `newCaptionText`, `newPanelBackground`. Modals: `showDialog(dialogConfig{...})` — never hand-roll `widget.NewModalPopUp`. For dialogs containing **text inputs** on Android, set `AnchorTop: true` — Fyne's `widget.NewModalPopUp` always re-centers in the full canvas size and ignores `Move()`, but Android's mobile driver does **not** shrink `Canvas.Size()` when the soft keyboard opens (only `Canvas.InteractiveArea()` shrinks via `InsetBottomPx`). So a centered modal sits half-behind the IME with no way to lift it. `AnchorTop` switches to a non-modal `widget.NewPopUp` positioned at the top of `InteractiveArea()` plus a hand-rolled dim `canvas.Rectangle` overlay for the modal feel.
- **Notifications**: `showNotice` / `showError` / `showConfirm`. Never `dialog.ShowError/Information/Confirm` directly (file pickers `NewFileOpen/Save/FolderOpen` are the only exception).
- **Read-only text modals**: use `preview.ShowText(preview.TextViewerOpts{...})` for log viewer / about / debug dumps / any monospace text display. It matches the transcription preview chrome (surfaceRaised bg, canvas.Text rows, copy-to-clipboard button, CLOSE). Never `widget.NewMultiLineEntry().Disable()` — disabled entries render pale-gray on Android dark theme and are unreadable.
- **Aliases** (`aliases.go`): single file for `decor`/`assets` re-exports, `validModels`, `attachLibrary`.
- **Widget reuse**: a Fyne widget can only have one parent. To show the same control in two tabs, add a mirror factory on the owning panel (see `settingsPanel.newModelSelectMirror`).
- **Entry on Android**: Fyne `widget.Entry` on the mobile driver has very limited selection — single-tap places the cursor, long-press opens cut/copy/paste, but there's no drag-to-select within a partially-selected range. For rename / overwrite-style flows, focus the entry and `entry.TypedShortcut(&fyne.ShortcutSelectAll{})` after the dialog opens (wrap in `fyne.Do(...)` so it runs after the popup is shown) so typing replaces the whole value. For filename rename specifically, also strip the extension before display and re-append on save — mirrors the Android Files app UX and avoids users accidentally clobbering the extension.
- **Truncating text rows**: never use `widget.Label` with `Truncation = fyne.TextTruncateEllipsis` next to a `canvas.Text` row in the same column — `widget.Label` wraps its text in `theme.InnerPadding()` while `canvas.Text` has zero padding, so the two rows render with mismatched x-offsets (visible as a ~4 px indent on the label row). Use `newTruncText(s, color, size, style)` from `trunctext.go` instead: it wraps `canvas.Text`, reports `MinSize.Width=0` so the row never forces horizontal scroll, and truncates with `…` via `fyne.MeasureText` binary search on Layout. Same widget for any list row whose filename/title can exceed available width.
- **Mirror initialisation order**: panel-level state mutations (e.g. `refreshLanguageOptions` filtering the master select's `Options` based on active engine) run during `settingsPanel.build()` via `defer`. Mirror factories like `newLangSelectMirror`/`newModelSelectMirror` are called *later* by `app.go`/`app_android.go` when the transcode tab is constructed. Always seed a new mirror from the master widget's already-filtered `Options` (and copy `Disabled()` state) — not the raw global slice. Otherwise the mirror surfaces stale unfiltered options at tap time (LimitSelect.Tapped reads Inner.Options on each tap).

## Android model storage

Models live at `/storage/emulated/0/Documents/WTranscribe/Models/` on Android (resolved via `shared.platformModelsDirOverride()` in `internal/config_android.go`). This is the idiomatic sideload location — survives uninstall AND "Clear Data", visible to the user in the Files app, lets them drop new model files in via USB without using the app. Mirrors the existing `MediaDir()` pattern. Requires `MANAGE_EXTERNAL_STORAGE` (already in manifest). Falls back to private dir (`Dir()+"models"`) if the write-test fails (permission revoked).

Never copy from /sdcard back to private internal storage — the override returns the public path directly so models are read in place. Doubling the storage cost (4+ GB) is unacceptable on phones.

For backup/transfer between devices, the user can `adb pull /sdcard/Documents/WTranscribe/Models` to a PC then push to a fresh device. Don't add model-pull/push tasks to the Taskfile — plain `adb pull` is the right interface.

## Testing

stdlib `testing` only. Names: `Test<Function>_<Scenario>`, table-driven preferred. Config tests: `t.TempDir()` + `t.Setenv("HOME", ...)`. CI runs `go vet`, `golangci-lint`, full `go test` on Linux. Diarization integration tests: `//go:build integration` (`task test-integration`); `getSherpaSample` lazy-downloads to `samples/diarization/sherpa/` (gitignored). Use `-short` to skip the download. Don't add skip-on-missing-model tests.


## Chunked, resumable ASR

All ASR engines (Whisper + sherpa-onnx variants) run through the unified chunked driver in `internal/transcriber/engine_chunk.go`. Default 30 s windows (env `WT_CHUNK_SEC`, range 5–60). Partial cache (`internal/transcriber/cache.Partial`) is persisted **after every chunk**, so a kernel-level OOM / thermal reboot / force-kill loses ≤1 chunk of work; the same partial format is shared across engines so resume is engine-agnostic. Never reintroduce a "process the full sample buffer in one shot" path — long inputs (>5 min) on Android arm64 reliably trigger the kernel OOM killer, which on phones often reboots the entire device with no userland crash log (the persistent `wt.log` will appear "cut off" mid-run because the GUI process is killed before the next `AppendLogLine` flushes). When adding a new ASR engine, write a `chunkProcessor` that takes chunk-local samples and returns chunk-local segments — the driver shifts timestamps onto the absolute timeline, saves partials, handles ctx cancellation, and emits uniform progress.

## ASR engine selection

Transcription engine is pluggable via `shared.Config.Engine` / `WT_ENGINE` / `JobSpec.Engine`. Values: `whisper` (default), `zipformer` (sherpa transducer, uppercase output), `moonshine` (sherpa, cased+punctuated, ~10× RTF on Exynos 2400 CPU). Dispatch lives in `internal/transcriber/engine.go`'s `Job.runASR`. When adding a new sherpa-backed engine, reuse the helpers in `engine_zipformer.go` (`findSherpaASRBinary`, `writeTempWAV`, `invokeSherpaCLI`, `finalizeSherpaRun`, `coalesceTokens`) and add a case to `runASR` — do **not** branch inside `runWhisper`.

Moonshine has a minimum effective input length somewhere around 12–15 s; shorter inputs may produce empty `text`. Vanilla Zipformer transducer accepts arbitrarily short inputs. If supporting <15 s clips with Moonshine, pad-and-trim or fall back to Zipformer.

Keep the model catalog in `internal/models/catalog.go` curated, not exhaustive. Each entry must be best-in-class for its niche or be removed. Never add a catalog entry on theoretical/paper-SOTA grounds alone — validate on a real device with a real audio fixture first.

Upstream watch list (do not port ourselves — wait for `csukuangfj/*` or k2-fsa to publish ONNX + a sherpa-onnx integration, then benchmark against the current default on a real fixture before adding):
  * **Sortformer v2** (NVIDIA NeMo) — potential pyannote-3.0 replacement; open weights, ONNX-exportable, claimed competitive with DiariZen at lower compute. Test against `diar-titanet-large` on a 1–2-min conversational clip with `--speakers=2`; reject if it produces fewer raw segments than pyannote-3.0 (Reverb-v2 lesson).
  * **Qwen3-ASR 0.6B** — potential SOTA multilingual ASR (52 languages, Apache 2.0). Currently no Android port. Watch for sherpa-onnx integration; would slot in alongside Parakeet (English-only) and SenseVoice (Asian-language).

Reject list (do not add even if asked):
  * **Picovoice Falcon** — closed-source commercial SDK; incompatible with privacy-focused on-device positioning even on the free tier. We already match pyannote-3.0 with fully open weights.
  * **Whisper Vulkan backend on Xclipse 940** — dead end (see existing rule below).
  * **Distil-Whisper Large V3** — redundant; whisper-turbo IS the official OpenAI distillation of large-v3 (multilingual, vs distil-large-v3 which is English-only).
  * **Moonshine in the catalog** — ~12–15 s minimum effective input length makes it unfit for voice notes and live captions. Code remains for env-bundle benchmarking only.
 Counter-example: Reverb-v2 diarizer (Rev.ai 2024 "conversational SOTA") catastrophically failed on a 1-min conversational interview (1 cluster, 1 speaker, 91.7 s) where pyannote-3.0 succeeded (12 segs, 2 speakers, 16.4 s). The right test for diarization is `wt-test -engine=<asr> -diarize -speakers=2 <audio>` followed by checking `Diarized in Xs, N raw segments, M unique speakers` — if M < expected speakers the model has failed regardless of paper benchmarks. Current top-tier picks: **Parakeet TDT 0.6B v2 int8** (English-only, ~9× RTF, native cased+punct), **SenseVoice int8** (multilingual zh/en/ja/ko/yue, ~16× RTF, native cased+punct, word timestamps), **Whisper-turbo** (99-lang fallback), **Qwen3 0.6B Q4_K_M** (auto-rename namer default; 1.7B kept as quality option). Do not add Moonshine/Zipformer/Paraformer/CT-Transformer to the catalog — the engines remain available via env-var bundles for benchmarking but are dominated by the curated picks for end users. Use `csukuangfj/*` HF mirrors for individual ONNX files instead of `sherpa-onnx/releases/*.tar.bz2` archives so the existing FileSpec downloader works without adding tar/bz2 extraction logic.

Parakeet TDT models require `--model-type=nemo_transducer` (NOT `transducer`). The plain transducer code path looks for `vocab_size` metadata at a location TDT models don't populate, failing at decoder init with `'vocab_size' does not exist in the metadata`. SenseVoice uses `--sense-voice-model=` (single-file model), NOT the encoder/decoder/joiner triplet.

NNAPI investigation (2026-05): the `android-sherpa-bin-nnapi` task builds sherpa-onnx with `BUILD_SHARED_LIBS=ON` + `ANDROID_PLATFORM=android-27` to enable the NNAPI provider. Verified runtime acceptance on Exynos 2400 (`Use nnapi` log line). Net result: **NNAPI is slower than CPU** for SenseVoice (1.21s vs 0.80s on a 32s clip + 6.8s one-time graph compile), because many int8 ops fall back to CPU and round-trip overhead exceeds NPU benefit. The shared ORT itself is ~2× faster than static when called directly, but slower when wrapped in `wt-test` due to dynamic linker overhead per subprocess. Production APK ships the **static** build (smaller, fewer files). Keep the `-nnapi` task for users who want to experiment with multi-file batches where the NPU warmup cost amortizes. Don't ship `libonnxruntime.so` in the APK by default.

The sherpa-onnx upstream `build-android-arm64-v8a.sh` always emits to `build-android-arm64-v8a/` for shared builds and `build-android-arm64-v8a-static/` for static. The `BUILD_OUT` Taskfile var must match — don't try to override the suffix.

When adding an ASR-family catalog entry, set `Family: FamilyASR` and `Engine: shared.EngineX`. Job.Run dispatches on `JobSpec.Engine` only; the GUI must set `spec.Engine = models.EngineForActiveASR(mgr.Active(models.FamilyASR))` (or fall back to whisper).

Never pure-`go test ./internal/transcriber/...` from the shell — whisper.cpp cgo bindings need the prebuilt lib. Use `task test SHORT=1` (skips cgo for unrelated packages, still builds transcriber via the task's prep step).

Android GUI screenshot: `adb shell "screencap -p /sdcard/s.png" && MSYS_NO_PATHCONV=1 adb pull /sdcard/s.png _tmp/s.png`. Quote the remote cmd or adb eats `-p`; set `MSYS_NO_PATHCONV=1` or msys rewrites `/sdcard/...`.

For on-device prototyping use `task android-test -- <wt-test-args>` (cross-builds wt-test, pushes binary + libc++/libomp to `/data/local/tmp/`, runs via `adb shell`). Always prefix with `MSYS_NO_PATHCONV=1` when forwarding `/data/local/tmp/...` paths through `--` on Windows/msys, otherwise paths get mangled to `C:/Program Files/Git/data/local/tmp/...`. Env vars don't propagate through the task's hardcoded `adb shell` command — invoke `adb shell 'cd /data/local/tmp && FOO=bar ./wt-test ...'` directly when you need env overrides.

On Android, sherpa-onnx CLIs are bundled as `lib*.so` (e.g. `libsherpa-diar.so`, `libsherpa-asr.so`) so the Android packager installs them under `/data/app/<pkg>/lib/arm64/`. Binary discovery must check that path *before* `exec.LookPath` — see `findSherpaBinary` / `findZipformerBinary`.

The existing `android-sherpa-bin` task already produces `sherpa-onnx-offline` (not just the diarization CLI) under `third_party/sherpa-onnx/build-android-arm64-v8a-static/install/bin/` because `SHERPA_ONNX_ENABLE_BINARY=ON` builds all CLIs. Reuse it for ASR engines — no separate build flow needed unless you want NNAPI (see next).

The static onnxruntime prebuilt sherpa-onnx ships only supports `cpu`, `cuda`, `coreml` providers. To get NNAPI/NPU acceleration on Android (Exynos 2400 NPU, Hexagon, etc.) you must rebuild sherpa-onnx with `BUILD_SHARED_LIBS=ON` so it pulls the AAR-style onnxruntime which includes `nnapi_provider_factory.h`. Verify provider list with `<bin> --help | grep provider` before assuming NNAPI works — the flag accepts `nnapi` syntactically but the ORT runtime will reject it at session-create.

Whisper.cpp emits BPE sub-word tokens (`Segment.Tokens`), not words. Word boundaries are tokens whose `Text` starts with a space (e.g. `" Good"`, `"'s"`, `" F"`, `"ul"`, `"ham"` for "Fulham"). Continuation pieces have no leading space. Any code path that consumes per-token output for word-level speaker mapping MUST coalesce continuation pieces onto the previous word before downstream `joinWords` runs `strings.Join(parts, " ")` — otherwise contractions split (`"I 'm"`, `"O 'D on nell"`), multi-piece words split (`"F ul ham"`), and every word gap doubles (`"Good  morning"` because each word-initial token already carries its own leading space). Use `coalesceWhisperTokens` in `internal/transcriber/output.go`; mirror of `coalesceTokens` in `engine_zipformer.go` for sherpa engines.

`sherpa-onnx-offline` emits one JSON line per input WAV with `{"text": ..., "tokens": [...], "timestamps": [...]}`. Tokens are BPE sub-word pieces; word boundaries are tokens whose first char is a space. Coalesce by gluing non-space-leading tokens onto the previous one before emitting word-level segments.

Exynos 2400 (s5e9945, S24/S24+ EU) exposes its NPU via NNAPI HALs 1.0–1.3 + Samsung ENN driver (`libenn_*`, `libnpu_compiler.so`). onnxruntime / sherpa-onnx accept `--provider=nnapi` and will route int8 ops to the NPU automatically. Xclipse 940 Vulkan path in whisper.cpp is a dead end — it hard-checks for desktop-AMD `VK_AMD_shader_core_properties` which Samsung's driver doesn't expose. Don't waste cycles on Xclipse-direct GPU acceleration.

## Scratch artifacts

Never write screenshots, logs, or other ad-hoc binary debug files (`*.png`, `*.jpg`, capture dumps, etc.) into the repo root or any tracked directory. Use the system tempdir (`/tmp/...` on msys, `$TMPDIR`) or a `_tmp/` subdir at the repo root (gitignored) when a tool can't read outside the project. Read tool can't access `C:\tmp` directly — copy/move into `_tmp/` then read. Clean up afterwards.

Always run `go run ./scripts/clean-comments ./cmd ./internal ./bindings && gofmt -w ./cmd/ ./internal/` before every commit. The repo style is comment-free Go — no inline narration, no rule-shaped explanations in source. Rules belong in AGENTS.md, not in `.go` files. The clean-comments script is the canonical stripper; don't hand-curate comments hoping they survive.

## Self-improvement

When you discover a non-obvious gotcha, footgun, or workflow rule that future sessions would benefit from, you **MUST** propose an AGENTS.md edit before ending the turn. Triggers: user corrects an approach ("don't do X"); a build/test/tooling failure has a non-obvious fix not documented here; you re-derive a project fact you've derived before; a command in this file is wrong. Keep additions terse and rule-shaped (Always / Ask / Never), not narrative. Don't add training-data-level advice. Don't commit the change unless instructed.
