# AGENTS.md — wt

Go CLI + GUI wrapping [whisper.cpp](https://github.com/ggml-org/whisper.cpp) for audio transcription with speaker diarization via [senko](https://github.com/narcotic-sh/senko).

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
internal/diarizer/      senko Python subprocess wrapper
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
- No comments unless explicitly requested.
- No commits unless explicitly instructed.

## Platform Build Tags

Split platform-specific code with `//go:build android` / `//go:build !android`:
- `config_android.go` / `config_default.go`
- `app_android.go` / `app.go`
- `transcribe_android.go` / `transcribe.go`
- `audio_android.go` / `audio.go`

Windows-specific: `_windows.go` suffix (auto-selected) with `_other.go` + `//go:build !windows` stubs.

## Module

Main module `github.com/asolopovas/wt` (Go 1.26). Vendored bindings have their own `go.mod` with:

```
replace github.com/ggerganov/whisper.cpp/bindings/go => ./bindings/go
```

Key deps: `fyne.io/fyne/v2` (GUI), `github.com/pterm/pterm` (CLI UI), `github.com/urfave/cli/v3` (CLI flags), `gopkg.in/yaml.v3` (config).

## CI

GitHub Actions on push/PR to `master`: build whisper.cpp → `go vet` → `go build` → `go test`. Release workflow on `v*` tags.
