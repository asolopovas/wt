# Android

## Adding a Java class — `scripts/android-service/com/asolopovas/wtranscribe/`

Three coordinated edits required:

1. Drop `.java` next to existing ones.
2. Extend `javac` AND `d8` arg lists in `Taskfile.yml:android-apk` (d8 takes `.class`, never `.java`).
3. Declare `<service>` / `<provider>` / `<receiver>` in `cmd/wt-gui/AndroidManifest.xml.in`.

ContentProviders sharing files to other apps need `android:grantUriPermissions="true"` on `<provider>` and `FLAG_GRANT_READ_URI_PERMISSION` on the Intent.

## Native lib discovery

**Never glob `/data/app/*/...` from inside the app process** — EPERM. Read `/proc/self/maps` and extract dirs containing already-loaded `.so` files. Helpers: `internal/diarizer/sherpa.go:androidNativeLibDirs`, mirror in `internal/transcriber/engine_zipformer.go:androidNativeLibDirs`. Any new sherpa / llama-cli launcher must use these — not `filepath.Glob`.

## JNI

**Never `FindClass` for app classes from a JNI thread** — boot ClassLoader can't see them. Use the `loadClass` pattern via `activity.getClassLoader()` (see `wt_fp_load_app_class` / `wts_load_app_class`).

## Sherpa CLI binaries

Bundled as `lib*.so` so the packager installs them under `/data/app/<pkg>/lib/arm64/`. New launchers must check that path **before** `exec.LookPath` (see `findSherpaBinary` / `findSherpaASRBinary`).

## Model storage

Public sideload path resolved by `internal/config_android.go:platformModelsDirOverride`. **Never copy from `/sdcard` back to private storage** — doubles 4+ GB cost. Backup/transfer is plain `adb pull /sdcard/Documents/WTranscribe/Models`; don't add custom tasks for it.

## Workflows

- Screenshot: `adb shell "screencap -p /sdcard/s.png" && MSYS_NO_PATHCONV=1 adb pull /sdcard/s.png _tmp/s.png`. Quote the remote cmd or adb eats `-p`.
- On-device prototyping: `task android-test -- <wt-test-args>`. On Windows/msys prefix with `MSYS_NO_PATHCONV=1` when forwarding `/data/local/tmp/...` paths.
- Env vars don't propagate through hardcoded `adb shell` — use `adb shell 'cd /data/local/tmp && FOO=bar ./wt-test ...'` directly.

## ASR engine selection on Android

**Use the sherpa-onnx engines, not whisper.cpp.** On the same hardware, the ONNX Runtime path is dramatically faster:

| Engine                        | Model                              | Galaxy S24+ (Exynos 2400) RTF | vs whisper.cpp turbo                                |
| ----------------------------- | ---------------------------------- | ----------------------------- | --------------------------------------------------- |
| whisper.cpp                   | turbo (1.6 GB ggml)                | 0.27                          | 1× (baseline)                                       |
| **sherpa-onnx whisper-turbo** | sherpa-whisper-turbo (1.0 GB)      | **1.99**                      | **7.3× faster, identical accuracy**                 |
| sherpa-onnx parakeet-tdt-v2   | parakeet-tdt-0.6b-v2-int8          | 5.65                          | 21× faster, ~turbo accuracy                         |
| sherpa-onnx whisper-tiny.en   | sherpa-whisper-tiny.en             | 9.19                          | 34× faster, lower acc                               |
| sherpa-onnx moonshine-tiny    | sherpa-onnx-moonshine-tiny-en-int8 | 10.25                         | 38× faster, lower acc                               |
| whisper.cpp                   | tiny                               | 0.50                          | 0.5× (slower than tiny via ONNX **and** worse text) |

Measured 2026-05-04 on `audio.wav` (120 s English speech). The whisper.cpp tiny output is strictly worse than sherpa whisper-tiny.en — it makes more name and homophone errors. Recommendation:

- **Default Android ASR**: `sherpa-whisper-turbo` (catalog ID) for best quality
- **Fast Android ASR**: `parakeet-tdt-0.6b-v2-int8` for near-turbo accuracy at 3× the speed
- **Budget / streaming**: `moonshine-tiny-en-int8` for very fast English-only
- **Multilingual**: `sherpa-whisper-tiny` (99 langs) or `sense-voice-zh-en-ja-ko-yue-int8` (5 Asian langs)

The new engine `whisper-onnx` (`internal/transcriber/engine_zipformer.go:RunWhisperONNX`) routes a Whisper model through `sherpa-onnx-offline --whisper-encoder=...`. Sherpa-whisper has a hardcoded 30 s per-call limit; the existing `runChunked` driver already chunks at 30 s so it composes naturally. Override the model directory with `WT_WHISPER_ONNX_DIR` for tests.

## NPU / NNAPI

- Don't ship `libonnxruntime.so` by default. NNAPI was slower than CPU for SenseVoice on Exynos 2400; static APK wins.
- Whisper Vulkan path (Xclipse 940) is dead — hard-checks for desktop-AMD `VK_AMD_shader_core_properties`. Don't try to revive.
- For NNAPI experiments: rebuild sherpa-onnx with `BUILD_SHARED_LIBS=ON`. Verify `<bin> --help | grep provider` lists `nnapi` (silently accepted but rejected at session-create otherwise).
