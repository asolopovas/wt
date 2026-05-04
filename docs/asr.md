# ASR engines & transcription

## Chunked driver — `internal/transcriber/engine_chunk.go`

- **Never** add a "process full buffer in one shot" path. Long inputs OOM-kill on Android arm64 (often reboots the device, no crash log).
- New engine: implement a `chunkProcessor(chunkSamples) → chunkSegments`. Driver handles timeline shift, partials, ctx cancel, progress.
- Chunk size override: env `WT_CHUNK_SEC` (5–60).

## Engine selection

Dispatched in `internal/transcriber/engine.go:Job.runASR` — **never** branch inside `runWhisper`. Values: `whisper` (default), `zipformer`, `moonshine`, `parakeet`, `sensevoice`. Set via `shared.Config.Engine` / env `WT_ENGINE` / `JobSpec.Engine`.

New sherpa engine: reuse helpers in `engine_zipformer.go` (`findSherpaASRBinary`, `writeTempWAV`, `runSherpaCmd`, `parseSherpaJSON`, `runSherpaEngineChunked`, `coalesceTokens`). GUI must call `models.EngineForActiveASR(mgr.Active(models.FamilyASR))` to set `spec.Engine`. Catalog entries need `Family: FamilyASR` and `Engine: shared.EngineX`.

## Engine quirks

- **Moonshine** — empty `text` for <12–15s input; pad-and-trim or fall back to zipformer.
- **Parakeet TDT** — needs `--model-type=nemo_transducer` (plain `transducer` reads `vocab_size` metadata TDT lacks).
- **SenseVoice** — single-file flag `--sense-voice-model=`, not encoder/decoder/joiner triplet.

## Token coalescing

Both whisper.cpp `Segment.Tokens` and `sherpa-onnx-offline` JSON emit BPE sub-word tokens. Word boundary = leading-space token; continuations have no leading space. **Always coalesce continuations onto the previous word before any `strings.Join(parts, " ")`** — otherwise contractions split (`I 'm`), multi-piece words split (`F ul ham`), gaps double.

- whisper: `coalesceWhisperTokens` in `internal/transcriber/output.go`
- sherpa: `coalesceTokens` in `internal/transcriber/engine_zipformer.go`

## Catalog policy — `internal/models/catalog.go`

- **Never add on paper-SOTA grounds.** Validate first: `wt-test -engine=<asr> -diarize -speakers=2 <audio>`. If reported `unique speakers` < expected, model fails regardless of benchmarks.
- Use `csukuangfj/*` HF mirrors for individual ONNX files; avoid `sherpa-onnx/releases/*.tar.bz2` (no tar/bz2 extraction in `FileSpec` downloader).

### Don't add

- **Picovoice Falcon** — closed-source.
- **Whisper Vulkan on Xclipse 940** — see `android.md`.
- **Distil-Whisper Large V3** — whisper-turbo already is the official multilingual distillation.
- **Reverb-v2** — failed (1 cluster) on a clip pyannote-3.0 handled (12 segs, 2 speakers).
- **Sortformer v2 / Qwen3-ASR** — wait for `csukuangfj/*` ONNX + benchmark first. Sortformer must beat pyannote-3.0 raw segment count on a 1–2min `--speakers=2` clip.

## Audio I/O performance

- **Skip full audio decode on raw cache hit.** `Job.Run` only calls `LoadAudioSamples` when ASR will actually run _or_ when the diarizer needs a freshly produced 16k mono WAV (i.e. source isn't `.wav` and no cached `<key>.wav` exists in `CacheDir`). Otherwise duration comes from `ProbeDurationMs`. Saves ~4 bytes/sample of f32 RAM and the ffmpeg decode for re-runs.
- **Streamed PCM read.** `readPCM16WAV` no longer allocates the full `[]byte` PCM buffer + the `[]float32` together. It streams 1 MiB blocks via `streamPCMToFloat32` straight into a single preallocated `[]float32`. Peak memory drops from ~6 B/sample to ~4 B/sample — for a 2 h mono clip that's ≈230 MB saved.
- **Buffered batched WAV write.** `WritePCM16WAV` builds a fixed 44-byte header in one `Write` and flushes samples in 4 KiB int16 batches through a 256 KiB `bufio.Writer`. Replaces the previous one-`Write`-per-sample syscall storm; cache-WAV writes are ~50× faster for long files.
- **Whisper engine panic guard.** `runWhisper`'s chunk processor wraps `wctx.Process` in `defer recover()` so a CGo crash on one chunk surfaces as a normal `error` (saved partial cache, next chunk still attempted) instead of taking down the host process. Mirrors Handy's `catch_unwind` pattern around `WhisperEngine::transcribe`.

## Streaming ffmpeg pipe (whisper engine)

- **`internal/transcriber/audio_stream.go` — `OpenAudioStream`** spawns `ffmpeg -f s16le pipe:1`, hands back chunks of `chunkSec()*16000` samples directly from the pipe via `Next()`. Decode and inference run in lockstep through `runChunkedStream` in `engine_chunk.go`, so chunk N+1 is being decoded while whisper is still on chunk N. Peak resident audio: one chunk (≈4 MB at 30 s) instead of the whole file (≈230 MB for 2 h).
- **Tee'd WAV cache** — when diarization is enabled and no cached `<audio-key>.wav` exists, `OpenAudioStream` is given `CacheWAVPath`. A `teeWAVWriter` writes a placeholder header, mirrors every PCM block to disk through a `bufio.Writer`, and patches the data-size field at finalize time via `WriteAt`. Avoids the previous "decode twice" pattern (full ffmpeg pass for cache + later re-read for `[]float32`).
- **Path selection in `Job.Run`.** Streaming activates only when _all_ of: `WT_STREAM != 0`, raw cache miss, no `<key>.partial.json` resume blob, ffmpeg available, engine resolves to `whisper`, and source extension is not `.wav`. Resume / `.wav` / non-whisper engines fall back to the buffered path because their offset math depends on a single addressable `[]float32`.
- **`runChunkedStream`** keeps the same partial-cache + offset semantics as `runChunked`. Chunks before `resumeAtSec` are decoded by ffmpeg but **not** fed to the processor (cheap to skip; we still pay decode but save inference). Resume after a streamed crash is supported because each chunk's `endSec` is committed to the partial cache.
- **Verified**: `samples/test-audio-4-speakers.mp3` produces an identical word-level transcript MD5 in buffered vs streamed mode at RTF ≈34 on a 3070 (small.en).

## GUI model cache & idle unload (`internal/gui/transcribe/model_cache.go`)

- The `whisper.Model` lives on `Panel` (fields `cachedModel`, `cachedModelSize`, `modelMu`). A run calls `acquireModel(size)` instead of `loadModel(size)` and a deferred `releaseModel()` only bumps `lastModelActivity` — it never closes. Subsequent runs reuse the same model when the size matches. Size change unloads + reloads.
- A single watcher goroutine (`ensureUnloadWatcher`) ticks every 30 s and, when the panel is idle and `time.Since(lastModelActivity) >= unloadTimeout()`, drops the model. Default 5 min, override `WT_MODEL_UNLOAD_SEC=<seconds>` (set to `0` to disable). Mirrors Handy's `TranscriptionManager` idle thread.
- The watcher _resets_ the activity timestamp while a run is active so the model can never be unloaded mid-job. `StopUnloadWatcher()` is exposed for future graceful shutdown wiring; today the goroutine simply exits with the process.
- CLI does **not** need this — each `wt` invocation owns the model for its lifetime. Only `wt-gui` carries the cache.

## Structural backlog (still Handy-inspired)

- **Background model preload on GUI startup.** Handy's `initiate_model_load` warms the engine in a thread so the first transcription doesn't pay the load cost. wt-gui currently lazy-loads on first job — consider preloading when the user picks a file but hasn't hit Run yet, and unloading via the existing watcher if they never click.
- **Cache GPU device list once.** Wrap `whisper.BackendDevices()` behind a `sync.Once` like Handy's `OnceLock<Vec<GpuDeviceOption>>`, with an FMA3-style guard for known-bad CPU/driver combos to avoid SIGILL on enumeration. Currently `runner.go` enumerates twice per run.
- **Engine reuse across batch CLI files.** Already partially done (Model is loaded once per `wt` invocation). Verify `whisper.Context` is also reused across files in batch mode rather than `NewContext` per file — the KV cache alloc is non-trivial for `large-v3-turbo`.
- **Streaming for sherpa engines.** `runWhisperStream` only handles whisper; `runZipformer`/`runParakeet`/`runSenseVoice`/`runMoonshine` still go through `LoadAudioSamples`. Their chunk processors take `[]float32` too — plumbing the same `next()` source through is straightforward, but each engine writes its own temp WAV today and the tee semantics need to be checked first.

## LLM auto-rename — `internal/llm/runner.go`

- Any post-transcription phase >1s **must** update both `p.setStatus(...)` and `platsvc.UpdateProgress(...)`. Otherwise UI looks frozen on the previous phase's message.
- GBNF grammars: **never** write `(rule)? (rule)? …` chains — each `?` doubles state space and llama.cpp's grammar sampler is single-threaded. Use `{n,m}` repetition (e.g. `slugChar{5,60}`). Verify with `time llama-cli ... --grammar-file g.gbnf` before merging.
- Per-call timeout: `llmTimeout()` (override `WT_LLM_TIMEOUT` seconds).
