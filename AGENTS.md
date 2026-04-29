# AGENTS.md — wt

Go CLI + GUI wrapping [whisper.cpp](https://github.com/ggml-org/whisper.cpp) for audio transcription with speaker diarization. Desktop default runs the bundled NeMo sortformer pipeline via `scripts/diarize.py`; sherpa-onnx (pyannote-3.0 segmentation + NeMo TitaNet-Large embedding, cluster-threshold=0.75) is the fallback and is also used on Android via `libsherpa-diar.so` (no bundled CPython).

## Versioning

**Never bump the version unless the user types "bump".**
Version lives in both `Taskfile.yml` (`VERSION:`) and `scripts/installer.iss` (`#define MyAppVersion`). Keep them in sync. Use `task bump` (it auto-increments, builds, installs, verifies, commits). Scheme: `1.0.0 → 1.0.9 → 1.1.0`.

## Layout

```
cmd/wt/         CLI (urfave/cli)
cmd/wt-gui/     GUI (Fyne)
cmd/wt-test/    Android test CLI
internal/               Shared: config, window-hiding
internal/gui/           Fyne GUI (drag-drop, settings, theme)
internal/transcriber/   Audio, model, CSV, live mode
internal/diarizer/      NeMo Python subprocess + sherpa-onnx (Android)
internal/ui/            Terminal spinners/progress (CLI only)
bindings/go/            Vendored whisper.cpp CGo bindings
scripts/                Build helpers, Inno Setup, diarize.py
third_party/whisper.cpp Cloned at build time (gitignored)
```

## Build & Run

Requires: Go 1.26+, GCC/MinGW, CMake, ffmpeg, [Task](https://taskfile.dev/). CGo env (`CGO_ENABLED`, `CC`, `CFLAGS`, `LDFLAGS`) is set by the Taskfile.

```bash
task build               # Both binaries + DLLs into dist/bin (clones whisper.cpp first run)
task build ONLY=cli      # CLI only
task build ONLY=gui      # GUI only (does NOT launch)
task install             # Replace binaries at %LOCALAPPDATA%\wt (skips full installer)
task setup               # Full silent install via installer + launch GUI
task install-android     # Build APK, install on device, launch
task check               # Verify toolchain
task clean               # dist/ + samples/*.json (DEEP=1 also clears whisper.cpp build)
```

**Agent rule — never launch the GUI.**
No `task` target launches the GUI automatically anymore (`task build` is compile-only; `task setup` launches on purpose). For compile-only verification of GUI changes, use `task build ONLY=gui` or build directly:

```bash
PATH="C:/Users/asolo/src/wt/third_party/whisper.cpp/build/bin:/c/msys64/mingw64/bin:$PATH" \
  go build -o /dev/null ./cmd/wt-gui
```

If you ever launch the GUI, kill it with `taskkill /F /IM wt-gui.exe` before moving on.

## Testing

```bash
task test           # go test -v ./...
task test-unit      # go test -v -short ./... (no CGo/model)
go test -v -run TestName ./internal/pkg/
```

Framework: stdlib `testing` only. Tests named `Test<Function>_<Scenario>`; prefer table-driven. Use `t.TempDir()` + `t.Setenv("HOME", ...)` for config tests. No skip-on-missing-model tests — if a test can't run without external resources, don't add it.

## Lint & Format

```bash
task lint           # golangci-lint (falls back to go vet)
task fmt            # gofumpt -w (falls back to gofmt)
```

CI runs `go vet` only.

## Code Style
- No comments unless explicitly requested. The only comments allowed in
  Go sources are compiler/runtime directives: `//go:build`, `//go:embed`,
  `//go:generate`, `//export ...`, `//line ...`, `// +build`, the
  `// Code generated ... DO NOT EDIT.` marker, and the cgo `/* ... */`
  preamble immediately preceding `import "C"`. Run
  `task clean-comments` to strip everything else.
- No commits unless explicitly instructed.

## Platform Build Tags

Split platform-specific code with `//go:build android` / `//go:build !android`:
- `config_android.go` / `config_default.go`
- `app_android.go` / `app.go`
- `transcribe_android.go` / `transcribe.go`
- `audio_android.go` / `audio.go`

Don't put `-tags=android` in `.vscode/settings.json` gopls flags on Windows: NDK
cgo headers (`<media/Ndk*.h>`) can't resolve on the host and gopls cascades into
fake "undefined: C.xxx" errors across every cgo file. Leave gopls on the default
build; android files just show a benign "No packages found" notice when opened.

Lint runs only over `./cmd/... ./internal/...` (the Taskfile scopes it). Do not
re-add `./...` — it picks up the vendored cgo whisper.cpp bindings under
`bindings/go/` which can't compile without the whisper.cpp build headers.

Windows-specific: `_windows.go` suffix (auto-selected) with `_other.go` + `//go:build !windows` stubs.

## Module

Main module `github.com/asolopovas/wt` (Go 1.26). Vendored bindings have their own `go.mod` with:

```
replace github.com/ggerganov/whisper.cpp/bindings/go => ./bindings/go
```

Key deps: `fyne.io/fyne/v2` (GUI), `github.com/pterm/pterm` (CLI UI), `github.com/urfave/cli/v3` (CLI flags), `gopkg.in/yaml.v3` (config).

## Diarization

Two backends, selected by `internal/diarizer/New(numSpeakers)` /
`NewWithPreference(numSpeakers, preferSherpa)`:

- **NeMo Sortformer** (`scripts/diarize.py`, NVIDIA `diar_sortformer_4spk-v1`,
  GPU when available). Capped at 4 speakers; auto-detects count; ignores
  `--num-speakers`. Fast and accurate when speaker count is small and
  speakers don't overlap; under-counts on quick interjections.
- **sherpa-onnx** (`sherpa-onnx-offline-speaker-diarization`, pyannote-3.0
  segmentation + speaker embedding clustering, CPU). Sole backend on
  Android; fallback / quality-preferred path on desktop.

### Tuned sherpa-onnx settings (do not change without re-running the sweep)

```
--segmentation.pyannote-model = pyannote-3.0
--embedding.model             = nemo_en_titanet_large.onnx   (~96 MB)
--clustering.cluster-threshold = 0.75
--min-duration-on              = 0.3
--min-duration-off             = 0.5
```

These were chosen by sweeping 11 embedding models × 8 thresholds × 3 min-on
values across three reference clips (`2_speakers_russian.m4a`,
`3_speakers_english.m4a`, `3 speakers sample 2.mp4`) with frame-level DER +
speaker-count penalty as the objective. Winner: titanet_large + thr 0.75 +
min-on 0.3 (mean DER 0.137, 0/0/0 speaker-count error). The previous default
embedding (`3dspeaker_eres2net_zh-cn`) was Chinese-trained and badly mismatched
English audio.

`--clustering.num-clusters=N` is **never** passed: in the sweep, threshold
mode beat forced-count mode on every clip, so the `--speakers N` flag now
only acts as a hint to select the sherpa backend.

### Backend selection rules

| Caller         | Default backend  | Override                         |
|----------------|------------------|----------------------------------|
| CLI (`wt`)     | NeMo             | `--speakers N` (any N>0) → sherpa|
| GUI (Windows)  | sherpa+titanet   | (always sherpa via `NewWithPreference(_, true)`) |
| GUI / CLI (Android) | sherpa+titanet | (only backend; CPython unavailable) |

The sweep harness lives at `scripts/diar_sweep.py` for future reference; the
parameters above are the chosen winner — no need to re-run it unless the test
set or backend changes.

## CI

GitHub Actions on push/PR to `master`: build whisper.cpp → `go vet` → `go build` → `go test`. Release workflow on `v*` tags.
