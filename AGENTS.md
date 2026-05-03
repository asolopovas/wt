# AGENTS.md — wt

Go CLI + GUI wrapping whisper.cpp for audio transcription with speaker diarization.
Module: `github.com/asolopovas/wt`. Deps: `fyne.io/fyne/v2`, `pterm`, `urfave/cli/v3`, `yaml.v3`.

## Commands (always via `task`, never bare `go build`)

```
task build [ONLY=cli|gui|android]        Build binaries + installer; android = APK
task install [TARGET=android] [QUICK=1]  Replace local install (or push APK + launch). QUICK = redeploy binaries only.
task test [SHORT=1|INTEGRATION=1]        Default = full; SHORT skips CGo; INTEGRATION = diarizer
task lint [FIX=1] [ANDROID=1]            golangci-lint (+gofumpt with FIX); ANDROID = NDK toolchain
task release [ROLLING=1]                 Default bumps + publishes; ROLLING updates `rolling` prerelease
task clean [DEEP=1]                      Clean dist/ (+ whisper.cpp build)
task models FETCH=samples|import         Fetch diarization samples / import models
```

## Layout

```
cmd/{wt,wt-gui,wt-test}        CLI / Fyne GUI / Android test CLI
internal/gui/                  Fyne GUI
internal/transcriber/          Audio, model, CSV, live mode
internal/diarizer/             NeMo subprocess + sherpa-onnx
internal/llm/                  llama-cli subprocess
internal/ui/                   Terminal spinners (CLI only)
bindings/go/                   Vendored whisper.cpp CGo bindings (own go.mod)
scripts/                       Build helpers, Inno Setup, diarize.py, diar_sweep.py
third_party/whisper.cpp        Cloned at build time (gitignored)
docs/                          Topic-scoped rules (see below)
```

## Topic-scoped rules (read on demand)

- `docs/build-release.md` — Taskfile dispatch, QUICK install, version policy, mvdan/sh quirks, Windows DLL retry, sherpa cross-compile.
- `docs/gui.md` — Design tokens, components, modals, Android Entry, truncating rows, mirror init.
- `docs/android.md` — Java sources, native lib discovery, JNI, sherpa CLI bundling, model storage, NNAPI.
- `docs/asr.md` — Chunked driver, engine selection, catalog policy, watch/reject lists, token coalescing, LLM rename.
- `docs/testing.md` — Conventions, integration tests, cgo gotchas.

## Always (cross-cutting)

- Run `go run ./scripts/clean-comments ./cmd ./internal ./bindings && gofmt -w ./cmd/ ./internal/` before every commit. Repo style is comment-free Go — rules live in `docs/*.md`, not source.
- Never write screenshots/logs/binary debug files into the repo or any tracked dir. Use system tempdir or `_tmp/` (gitignored). The Read tool can't access `C:\tmp` — copy into `_tmp/` first. Clean up afterwards.
- GUI compile-checks **only** through `task build ONLY=gui` (`CGO_LDFLAGS` differs across toolchains).
