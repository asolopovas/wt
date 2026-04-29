# AGENTS.md — wt

Go CLI + GUI wrapping whisper.cpp for audio transcription with speaker diarization.
Desktop default: NeMo sortformer (`scripts/diarize.py`). Fallback / Android / GUI: sherpa-onnx
(pyannote-3.0 + NeMo TitaNet-Large, threshold 0.75).

## Commands

```bash
task build               # Both binaries + DLLs → dist/bin
task build ONLY=cli|gui  # Single binary (compile-only, never launches)
task install             # Replace binaries at %LOCALAPPDATA%\wt
task setup               # Full silent install + launch GUI (user-initiated only)
task install-android     # Build APK, install, launch
task check               # Verify toolchain
task clean               # dist/ + samples/*.json (DEEP=1 also clears whisper.cpp build)
task test                # go test -v ./...
task test-unit           # -short, no CGo/model
task lint                # golangci-lint (scoped to ./cmd/... ./internal/...)
task fmt                 # gofumpt -w
task clean-comments      # Strip non-directive comments
task bump                # Auto-increment version, build, install, verify, commit
```

Always compile-check GUI via `task build ONLY=gui`. A bare `go build ./cmd/wt-gui` will
fail to link: `CGO_LDFLAGS` differs between MinGW (`-lwhisper`) and CUDA/MSVC
(`whisper.lib`) whisper.cpp builds, and Taskfile picks the right flags.

Requires Go 1.26+, GCC/MinGW, CMake, ffmpeg, [Task](https://taskfile.dev/). CGo env is set by Taskfile.

## Boundaries

**Always:** run `task build ONLY=gui` for GUI compile checks; keep `Taskfile.yml VERSION` and `scripts/installer.iss MyAppVersion` in sync.

**Ask first:** anything that mutates user state outside the repo (installs, registry, version bumps).

**Never:**
- Bump the version unless the user types "bump" (use `task bump`, scheme `1.0.0 → 1.0.9 → 1.1.0`).
- Launch the GUI. If you do by accident, `taskkill /F /IM wt-gui.exe` immediately.
- Commit unless explicitly instructed.
- Re-add `-tags=android` to `.vscode/settings.json` gopls flags (NDK headers cascade fake `C.xxx` errors on Windows).
- Re-scope lint to `./...` (picks up vendored cgo bindings that won't compile standalone).
- Re-run `scripts/diar_sweep.py` to retune diarization (slow; winning params already chosen — see below).
- Add skip-on-missing-model tests.
- Write Go comments other than directives (`//go:build`, `//go:embed`, `//go:generate`, `//export`, `//line`, `// +build`, `// Code generated ... DO NOT EDIT.`, cgo preamble).

## Layout

```
cmd/wt/                  CLI (urfave/cli)
cmd/wt-gui/              GUI (Fyne)
cmd/wt-test/             Android test CLI
internal/gui/            Fyne GUI
internal/transcriber/    Audio, model, CSV, live mode
internal/diarizer/       NeMo subprocess + sherpa-onnx
internal/ui/             Terminal spinners (CLI only)
bindings/go/             Vendored whisper.cpp CGo bindings (own go.mod)
scripts/                 Build helpers, Inno Setup, diarize.py, diar_sweep.py
third_party/whisper.cpp  Cloned at build time (gitignored)
```

Module: `github.com/asolopovas/wt` (Go 1.26). Key deps: `fyne.io/fyne/v2`, `pterm`, `urfave/cli/v3`, `yaml.v3`.

## Platform splits

`//go:build android` / `//go:build !android` pairs:
`config_*`, `app_*`, `transcribe_*`, `audio_*`. Windows uses `_windows.go` / `_other.go` (with `//go:build !windows`).

## Testing

stdlib `testing` only. Names: `Test<Function>_<Scenario>`, prefer table-driven.
Config tests: `t.TempDir()` + `t.Setenv("HOME", ...)`. CI runs `go vet` only.

## Diarization

Backend selection via `internal/diarizer/New(numSpeakers)` / `NewWithPreference(numSpeakers, preferSherpa)`:

| Caller              | Default        | Override                              |
|---------------------|----------------|---------------------------------------|
| CLI (`wt`)          | NeMo           | `--speakers N` (any N>0) → sherpa     |
| GUI (Windows)       | sherpa+titanet | always sherpa                         |
| GUI / CLI (Android) | sherpa+titanet | only backend (no bundled CPython)     |

NeMo (`diar_sortformer_4spk-v1`, GPU): capped at 4 speakers, auto-detects count, ignores `--num-speakers`. Under-counts on quick interjections.

Sherpa-onnx tuned params (do not change — winner of 11×8×3 sweep, mean DER 0.137):

```
--segmentation.pyannote-model = pyannote-3.0
--embedding.model             = nemo_en_titanet_large.onnx   (~96 MB)
--clustering.cluster-threshold = 0.75
--min-duration-on              = 0.3
--min-duration-off             = 0.5
```

`--clustering.num-clusters=N` is never passed; `--speakers N` only hints backend selection.

## Self-improvement

When you discover a non-obvious gotcha, footgun, or workflow rule that future sessions would benefit from, propose an AGENTS.md edit before ending the turn. Triggers:

- The user corrects an approach you took ("don't do X", "stop Xing").
- A build/test/tooling failure has a non-obvious fix not documented here.
- You re-derive the same project fact you've derived before.
- A command in this file is wrong or outdated.

Keep additions terse and rule-shaped (Always / Ask / Never), not narrative. Do not add training-data-level advice. Do not commit the change unless instructed.
