# AGENTS.md — wt

Go CLI + GUI wrapping sherpa-onnx for audio transcription with speaker diarization.
Module: `github.com/asolopovas/wt`. Deps: `fyne.io/fyne/v2`, `pterm`, `urfave/cli/v3`, `yaml.v3`.

## Commands (always via `task`, never bare `go build`)

```
task build [ONLY=cli|gui|android]        Build binaries + installer; android = APK
task install [TARGET=android] [QUICK=1]  Replace local install (or push APK + launch). QUICK = redeploy binaries only.
task test [SHORT=1|INTEGRATION=1]        Default = full; SHORT skips CGo; INTEGRATION = diarizer
task check [ANDROID=1]                   Single quality gate: clean-comments + format + golangci-lint (errcheck/govet/staticcheck/unused/ineffassign) + deadcode + govulncheck + tests
task release [ROLLING=1]                 Default bumps + publishes; ROLLING updates `rolling` prerelease
task clean [DEEP=1]                      Clean dist/ (+ third_party builds)
task models FETCH=samples|import         Fetch diarization samples / import models
```

## Layout

```
cmd/{wt,wt-gui,wt-test}        CLI / Fyne GUI / Android test CLI
internal/gui/                  Fyne GUI
internal/transcriber/          Audio, model resolve, chunked driver, live mode
internal/diarizer/             NeMo subprocess + sherpa-onnx
internal/llm/                  llama-cli subprocess
internal/ui/                   Terminal spinners (CLI only)
scripts/                       Build helpers, Inno Setup, diarize.py, diar_sweep.py
third_party/sherpa-onnx       Cloned at build time (gitignored)
third_party/sherpa-onnx-cuda  Pre-built CUDA binaries downloaded at build time (gitignored)
docs/                          Topic-scoped rules (see below)
```

## Topic-scoped rules (read on demand)

- `docs/build-release.md` — Taskfile dispatch, QUICK install, version policy, mvdan/sh quirks, Windows DLL retry, sherpa cross-compile.
- `docs/gui.md` — Design tokens, components, modals, Android Entry, truncating rows, mirror init.
- `docs/android.md` — Java sources, native lib discovery, JNI, sherpa CLI bundling, model storage, NNAPI.
- `docs/asr.md` — Chunked driver, engine selection, catalog policy, watch/reject lists, token coalescing, LLM rename.
- `docs/testing.md` — Conventions, integration tests, cgo gotchas.

## Always (cross-cutting)

- Run `task check` before every commit (runs clean-comments, format, lint, deadcode, govulncheck, tests); fix every issue it reports.
- No comments in any generated code (Go, bash, YAML, Python, JS); shebangs stay. Put rationale in `docs/*.md` or commit messages.
- Remove dead code, imports, and tests for removed code on every change.
- Never write screenshots/logs/binary debug files into tracked dirs; use system tempdir or `_tmp/` (gitignored).
- GUI compile-checks only through `task build ONLY=gui`.
