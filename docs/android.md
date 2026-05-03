# Android

## Java sources

Live at `scripts/android-service/com/asolopovas/wtranscribe/`. Adding a class needs three coordinated edits:

1. Drop `.java` next to existing ones.
2. Extend `javac` AND `d8` argument lists in `Taskfile.yml` `android-apk` (d8 takes `.class` files, never `.java`).
3. Declare any `<service>` / `<provider>` / `<receiver>` in `cmd/wt-gui/AndroidManifest.xml.in`.

ContentProviders sharing files to other apps need `android:grantUriPermissions="true"` on `<provider>` and `FLAG_GRANT_READ_URI_PERMISSION` on Intent. Verify after install: `adb shell dumpsys package <id> | grep -A1 -i provider`.

## Native lib discovery

Never glob `/data/app/*/...` from inside an app process — app user gets EPERM. Read `/proc/self/maps` and extract dirs containing already-loaded `.so` files: those are the real `nativeLibraryDir` paths Android resolved at load time, randomised UUID subdirs included. Helpers: `internal/diarizer/sherpa.go:androidNativeLibDirs`, mirrored in `internal/transcriber/engine_zipformer.go:androidNativeLibDirs`. Any new sherpa/llama-cli launcher must use this, not `filepath.Glob`.

## JNI

Never call `FindClass` for app classes from a JNI thread — boot ClassLoader can't see them. Use `loadClass` pattern from `wt_fp_load_app_class` / `wts_load_app_class` (load via `activity.getClassLoader()`).

## Sherpa CLI binaries

Bundled as `lib*.so` (e.g. `libsherpa-diar.so`, `libsherpa-asr.so`) so packager installs under `/data/app/<pkg>/lib/arm64/`. Binary discovery must check that path **before** `exec.LookPath` — see `internal/diarizer/sherpa.go:findSherpaBinary` and `internal/transcriber/engine_zipformer.go:findSherpaASRBinary`.

`android-sherpa-bin` task already produces `sherpa-onnx-offline` (not just diarization CLI) — `SHERPA_ONNX_ENABLE_BINARY=ON` builds all CLIs. Reuse for ASR engines unless you want NNAPI.

## Model storage

`/storage/emulated/0/Documents/WTranscribe/Models/` (resolved via `shared.platformModelsDirOverride()` in `internal/config_android.go`). Idiomatic sideload: survives uninstall AND "Clear Data", visible in Files app, USB drop-in. Mirrors `MediaDir()`. Requires `MANAGE_EXTERNAL_STORAGE` (already in manifest). Falls back to private dir (`Dir()+"models"`) on write-test failure.

Never copy from /sdcard back to private internal storage — override returns public path directly. Doubling 4+ GB cost is unacceptable.

For backup/transfer: user runs `adb pull /sdcard/Documents/WTranscribe/Models` then push to fresh device. Don't add model-pull/push tasks — plain `adb pull` is the right interface.

## Workflows

- Screenshot: `adb shell "screencap -p /sdcard/s.png" && MSYS_NO_PATHCONV=1 adb pull /sdcard/s.png _tmp/s.png`. Quote the remote cmd or adb eats `-p`; `MSYS_NO_PATHCONV=1` or msys rewrites `/sdcard/...`.
- On-device prototyping: `task android-test -- <wt-test-args>` (cross-builds, pushes binary + libc++/libomp to `/data/local/tmp/`, runs via `adb shell`). Prefix with `MSYS_NO_PATHCONV=1` when forwarding `/data/local/tmp/...` paths through `--` on Windows/msys. Env vars don't propagate through hardcoded `adb shell` — invoke `adb shell 'cd /data/local/tmp && FOO=bar ./wt-test ...'` directly when overriding.

## NPU / NNAPI

Exynos 2400 (s5e9945, S24/S24+ EU) exposes NPU via NNAPI HALs 1.0–1.3 + Samsung ENN driver. onnxruntime / sherpa-onnx accept `--provider=nnapi` and route int8 ops to NPU automatically. Xclipse 940 Vulkan path in whisper.cpp is dead — hard-checks for desktop-AMD `VK_AMD_shader_core_properties` Samsung's driver doesn't expose.

Static onnxruntime prebuilt only supports `cpu/cuda/coreml`. For NNAPI, rebuild sherpa-onnx with `BUILD_SHARED_LIBS=ON` (pulls AAR-style ORT with `nnapi_provider_factory.h`). Verify with `<bin> --help | grep provider` — `nnapi` flag accepts syntactically but ORT rejects at session-create otherwise.

NNAPI investigation result (2026-05): **slower than CPU** for SenseVoice (1.21s vs 0.80s on 32s clip + 6.8s one-time graph compile). Many int8 ops fall back to CPU; round-trip overhead exceeds NPU benefit. Production APK ships **static** build. `-nnapi` task kept for multi-file batch experiments where NPU warmup amortizes. Don't ship `libonnxruntime.so` in APK by default.
