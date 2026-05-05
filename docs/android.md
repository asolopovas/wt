# Android

## Adding a Java class — `scripts/android-service/com/asolopovas/wtranscribe/`

1. Drop `.java` next to existing ones.
2. Extend `javac` AND `d8` arg lists in `Taskfile.yml:android-apk` (d8 takes `.class`, never `.java`).
3. Declare `<service>` / `<provider>` / `<receiver>` in `cmd/wt-gui/AndroidManifest.xml.in`.

ContentProviders sharing files to other apps need `android:grantUriPermissions="true"` on `<provider>` and `FLAG_GRANT_READ_URI_PERMISSION` on the Intent.

## Native lib discovery

Never glob `/data/app/*/...` from inside the app process. Read `/proc/self/maps` and extract dirs containing already-loaded `.so` files. Use `androidNativeLibDirs` (in `internal/diarizer/sherpa.go` and `internal/transcriber/engine_zipformer.go`). Any new sherpa / llama-cli launcher must use these.

## JNI

Never `FindClass` for app classes from a JNI thread. Use `loadClass` via `activity.getClassLoader()` (see `wt_fp_load_app_class` / `wts_load_app_class`).

## Sherpa CLI binaries

Bundled as `lib*.so` under `/data/app/<pkg>/lib/arm64/`. New launchers must check that path before `exec.LookPath` (see `findSherpaBinary` / `findSherpaASRBinary`).

## Model storage

Public sideload path: `internal/config_android.go:platformModelsDirOverride`. Never copy from `/sdcard` back to private storage. Backup/transfer is `adb pull /sdcard/Documents/WTranscribe/Models`.

## Workflows

- Screenshot: `adb shell "screencap -p /sdcard/s.png" && MSYS_NO_PATHCONV=1 adb pull /sdcard/s.png _tmp/s.png`. Quote the remote cmd.
- On-device prototyping: `task android-test -- <wt-test-args>`. Prefix with `MSYS_NO_PATHCONV=1` on Windows/msys when forwarding `/data/local/tmp/...` paths.
- Env vars: use `adb shell 'cd /data/local/tmp && FOO=bar ./wt-test ...'` directly.

## ASR engine selection

Use sherpa-onnx engines, not whisper.cpp (ONNX path is ~7× faster on Android).

- Default Android ASR: `sherpa-whisper-turbo`.
- Fast: `parakeet-tdt-0.6b-v2-int8`.
- Budget / streaming: `moonshine-tiny-en-int8`.
- Multilingual: `sherpa-whisper-tiny` or `sense-voice-zh-en-ja-ko-yue-int8`.

`whisper-onnx` engine (`internal/transcriber/engine_zipformer.go:RunWhisperONNX`) routes Whisper through `sherpa-onnx-offline --whisper-encoder=...`. Sherpa-whisper has a hardcoded 30 s per-call limit; existing `runChunked` driver already chunks at 30 s. Override model dir with `WT_WHISPER_ONNX_DIR`.

## NPU / NNAPI

- Don't ship `libonnxruntime.so` by default; static APK wins.
- Whisper Vulkan path (Xclipse 940) is dead; don't revive.
- For NNAPI experiments: rebuild sherpa-onnx with `BUILD_SHARED_LIBS=ON`. Verify `<bin> --help | grep provider` lists `nnapi`.
