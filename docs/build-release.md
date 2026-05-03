# Build & Release

## Taskfile dispatch

- Internal tasks (`internal: true`) — call via `task: X` cmd entries, never `cmd: 'task X'` (spawns new process, sees flag, exits 202). Same for compound shells (`task X && go test ...`) — split into `task:` + `cmd:` steps.
- Vars on `task: <name>` invocations don't propagate through nested `task:` calls in Task v3. Every leaf touching `{{.VERSION}}` needs its own `vars: { VERSION: { sh: awk -F"\x27" '/^  VERSION:/{print $2; exit}' Taskfile.yml } }`.
- `_release-stable` must call `installer-exe`/`linux-deb` explicitly — `bump`'s `task: install` runs `_build-host` with `ONLY=host` (skips installer), then later `task: build` sees `_build-host` up-to-date and skips installer too.

## QUICK install (`task install QUICK=1`)

Dev-loop install (~7s Linux). Skips `linux-deb`/`installer-exe`, drops fresh `wt`/`wt-gui` (+ rebuilt `lib*.so*`, preserve symlinks via `cp -Pf`) into existing install dir: `/opt/wt/` (sudo install) or `$LOCALAPPDATA\wt\` (taskkill+retry-cp).

Linux QUICK requires full `.deb` installed once first (postinst sets up `/opt/wt`, `/usr/bin` symlinks, per-user `wt-setup` venv). Error out clearly if missing — don't bootstrap from QUICK.

## Version rendering

From-source builds (HEAD not on a release tag) show latest commit date `YYYY-MM-DD`; tagged builds show version. Wired via `GIT_DATE` Taskfile var → `-X main.BuildDate=$GIT_DATE` on **both** wt CLI and wt-gui. Centralize in `appinfo.DisplayVersion(version, buildDate)` — never inline `if buildDate != "" {…}` in callers. wt-gui's `versionLabel` adds `v` prefix only on tagged path; date-shaped labels and `"dev"` returned as-is.

## Release

GH release created empty first, artifacts uploaded in 3-attempt retry loop (large APK/EXE uploads fail mid-stream). `task release` re-reads `VERSION` from `Taskfile.yml` post-bump because `{{.VERSION}}` captures at parse time.

## Shell quirks (mvdan/sh)

- Never `grep ... | sed -E "s/.../\1/"` in Task `sh:` var — mangles `\1`. Use `awk -F"\x27" '/pattern/{print $2}'`.
- Panics on fd>=10 redirects (`exec 9>>file`) — use cp-retry loop instead of fd-based lock probing on Windows.

## Windows

- PowerShell PATH usually omits `C:\Program Files\Git\usr\bin` → Task fails with `"awk": executable file not found`. Fix: prepend Git's `usr\bin` to user PATH, or run from Git Bash.
- DLL handles held briefly after `taskkill` — `_install-host` must wait+retry `cp` (whisper.dll esp.); single sleep insufficient.

## Cross-compile sherpa-onnx for Android

Never via direct cmake — upstream `cmake/onnxruntime.cmake` only handles Linux/macOS/Windows hosts. Always invoke `build-android-<abi>.sh` via msys2 bash. With static onnxruntime archive (`BUILD_SHARED_LIBS=OFF`), keep `SHERPA_ONNX_ANDROID_PLATFORM=android-21` — API ≥27 pulls `nnapi_provider_factory.h` which static prebuilts lack. Binary remains forward-compatible with 28+.

`build-android-arm64-v8a.sh` always emits to `build-android-arm64-v8a/` (shared) or `-static/`. `BUILD_OUT` Taskfile var must match.
