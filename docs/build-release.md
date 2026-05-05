# Build & Release

## Taskfile dispatch

- Internal tasks: call via `task: X` cmd entries, never `cmd: 'task X'` — the latter spawns a new `task` process that re-sees the parent flag and exits 202. Don't combine with `&&`; split into separate `task:` + `cmd:` steps.
- `{{.VERSION}}` doesn't propagate through nested `task:` calls. Re-declare per leaf: `vars: { VERSION: { sh: awk -F"\x27" '/^  VERSION:/{print $2; exit}' Taskfile.yml } }`.
- `_release-stable` must call `installer-exe` / `linux-deb` explicitly.
- `task release` re-reads `VERSION` from `Taskfile.yml` post-bump.

## QUICK install

`task install QUICK=1` requires the full `.deb` installed once first on Linux. Error out if missing.

## Version rendering

- Use `appinfo.DisplayVersion(version, buildDate)`; never inline buildDate checks.
- Both `wt` and `wt-gui` need `-X main.BuildDate=$GIT_DATE`.

## Shell quirks (mvdan/sh)

- Never `grep | sed -E "s/.../\1/"` in `sh:` vars — mvdan/sh mangles `\1`. Use `awk -F"\x27" '/pattern/{print $2}'`.
- mvdan/sh panics on fd≥10 redirects (`exec 9>>file`); don't use fd-based locking.

## Windows

- DLL handles linger briefly after `taskkill` (whisper.dll especially); install must wait+retry `cp`. A single sleep is insufficient — use a retry loop.

## llama-cli host download

`Taskfile.yml:llama-cli-host` downloads the latest `llama-cli` release from `ggml-org/llama.cpp` into `dist/llama/` (Windows .zip via `gh release download`, Linux equivalent). Required by `installer-exe` and `linux-deb`. Bumps automatically on tag changes; no version pin.

## Cross-compile sherpa-onnx for Android

- Always use `build-android-<abi>.sh` via msys2 bash, never direct cmake.
- Keep `SHERPA_ONNX_ANDROID_PLATFORM=android-21` with static onnxruntime.
- `BUILD_OUT` must match the script's output dir (`build-android-arm64-v8a/` or `-static/`).
