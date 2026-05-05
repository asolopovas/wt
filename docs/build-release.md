# Build & Release

## Taskfile

- Internal tasks: call via `task: X` cmd entries, never `cmd: 'task X'`. Don't combine with `&&`; split into separate steps.
- `{{.VERSION}}` doesn't propagate through nested `task:` calls. Re-declare per leaf: `vars: { VERSION: { sh: awk -F"\x27" '/^  VERSION:/{print $2; exit}' Taskfile.yml } }`.
- `_release-stable` must call `installer-exe` / `linux-deb` explicitly.
- `task release` re-reads `VERSION` from `Taskfile.yml` post-bump.

## QUICK install

`task install QUICK=1` requires the full `.deb` installed once first on Linux. Error out if missing.

## Version rendering

- Use `appinfo.DisplayVersion(version, buildDate)`; never inline buildDate checks.
- Both `wt` and `wt-gui` need `-X main.BuildDate=$GIT_DATE`.

## Shell quirks (mvdan/sh)

- Never `grep | sed -E "s/.../\1/"` in `sh:` vars; use `awk -F"\x27" '/pattern/{print $2}'`.
- No fd≥10 redirects; use cp-retry loops on Windows.

## Windows

- DLL handles linger after `taskkill`; install must wait+retry `cp` (whisper.dll especially).

## Cross-compile sherpa-onnx for Android

- Always use `build-android-<abi>.sh` via msys2 bash, never direct cmake.
- Keep `SHERPA_ONNX_ANDROID_PLATFORM=android-21` with static onnxruntime.
- `BUILD_OUT` must match the script's output dir (`build-android-arm64-v8a/` or `-static/`).
