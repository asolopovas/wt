# Build & Release

## Taskfile dispatch

- Internal tasks (`internal: true`) — call via `task: X` cmd entries. **Never** `cmd: 'task X'` (spawns new process, sees flag, exits 202). Same for compound shells: split `task X && go test ...` into separate `task:` + `cmd:` steps.
- Vars on `task: <name>` invocations don't propagate through nested `task:` calls in Task v3. Every leaf touching `{{.VERSION}}` needs its own `vars: { VERSION: { sh: awk -F"\x27" '/^  VERSION:/{print $2; exit}' Taskfile.yml } }`.
- `_release-stable` must call `installer-exe` / `linux-deb` explicitly. Otherwise `bump → install → build` leaves them as up-to-date no-ops.

## QUICK install (`task install QUICK=1`)

Linux QUICK requires the full `.deb` installed once first (postinst sets up `/opt/wt`, `/usr/bin` symlinks, per-user `wt-setup` venv). Error out clearly if missing — don't bootstrap from QUICK.

## Version rendering

- Centralize in `appinfo.DisplayVersion(version, buildDate)`. **Never inline** `if buildDate != "" {…}` in callers.
- Both wt CLI and wt-gui need `-X main.BuildDate=$GIT_DATE` linker flag.

## Release

`task release` re-reads `VERSION` from `Taskfile.yml` post-bump because `{{.VERSION}}` captures at parse time. Don't rely on the parse-time value in release tasks.

## Shell quirks (mvdan/sh)

- **Never** `grep ... | sed -E "s/.../\1/"` in Task `sh:` vars — mangles `\1`. Use `awk -F"\x27" '/pattern/{print $2}'`.
- mvdan/sh panics on fd≥10 redirects (`exec 9>>file`) — use cp-retry loop instead of fd-based lock probing on Windows.

## Windows

- PowerShell PATH usually omits `C:\Program Files\Git\usr\bin` → Task fails with `"awk": executable file not found`. Prepend Git's `usr\bin` to user PATH or run from Git Bash.
- DLL handles held briefly after `taskkill` — install must wait+retry `cp` (whisper.dll especially); a single sleep is insufficient.

## Cross-compile sherpa-onnx for Android

- **Never via direct cmake** — upstream `cmake/onnxruntime.cmake` only handles Linux/macOS/Windows hosts. Always invoke `build-android-<abi>.sh` via msys2 bash.
- With static onnxruntime archive (`BUILD_SHARED_LIBS=OFF`), keep `SHERPA_ONNX_ANDROID_PLATFORM=android-21`. API ≥27 pulls `nnapi_provider_factory.h` which static prebuilts lack. Binary remains forward-compatible with 28+.
- `build-android-arm64-v8a.sh` emits to `build-android-arm64-v8a/` (shared) or `-static/` — `BUILD_OUT` Taskfile var must match.
