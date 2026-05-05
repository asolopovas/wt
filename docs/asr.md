# ASR engines & transcription

## Chunked driver — `internal/transcriber/engine_chunk.go`

- Never add a "process full buffer in one shot" path (OOM-kills Android arm64).
- Chunk size override: env `WT_CHUNK_SEC` (5–60).

## Engine selection

Dispatched in `internal/transcriber/engine.go:Job.runASR`; never branch inside `runWhisper`. Values: `whisper` (default), `zipformer`, `moonshine`, `parakeet`, `sensevoice`. Set via `shared.Config.Engine` / env `WT_ENGINE` / `JobSpec.Engine`.

## Engine quirks

- **Moonshine**: empty `text` for <12–15s input; pad-and-trim or fall back to zipformer.
- **Parakeet TDT**: needs `--model-type=nemo_transducer`.
- **SenseVoice**: single-file flag `--sense-voice-model=`, not encoder/decoder/joiner triplet.

## Token coalescing

Both whisper.cpp tokens and sherpa JSON emit BPE sub-word tokens. Always coalesce continuations onto the previous word before any `strings.Join(parts, " ")`.

- whisper: `coalesceWhisperTokens` in `internal/transcriber/output.go`
- sherpa: `coalesceTokens` in `internal/transcriber/engine_zipformer.go`

## Catalog policy — `internal/models/catalog.go`

- Validate before adding: `wt-test -engine=<asr> -diarize -speakers=2 <audio>`. Reject if `unique speakers` < expected.
- Use `csukuangfj/*` HF mirrors for individual ONNX files; avoid `sherpa-onnx/releases/*.tar.bz2`.

### Don't add

- Picovoice Falcon (closed-source).
- Whisper Vulkan on Xclipse 940.
- Distil-Whisper Large V3 (whisper-turbo is the official distillation).
- Reverb-v2 (failed validation).
- Sortformer v2 / Qwen3-ASR (wait for `csukuangfj/*` ONNX + benchmark first).

## Audio I/O

- Skip full audio decode on raw cache hit; `Job.Run` only calls `LoadAudioSamples` when ASR will actually run or diarizer needs a freshly produced 16k mono WAV.
- `readPCM16WAV` streams 1 MiB blocks via `streamPCMToFloat32` into a single preallocated `[]float32`.
- `WritePCM16WAV` flushes int16 in 4 KiB batches through a 256 KiB `bufio.Writer`.
- `runWhisper`'s chunk processor wraps `wctx.Process` in `defer recover()` so CGo crashes surface as errors.

## Streaming ffmpeg pipe (whisper engine)

- `internal/transcriber/audio_stream.go:OpenAudioStream` spawns `ffmpeg -f s16le pipe:1`; `runChunkedStream` in `engine_chunk.go` runs decode and inference in lockstep.
- Tee'd WAV cache: when diarization is enabled and no cached `<audio-key>.wav` exists, `OpenAudioStream` is given `CacheWAVPath`; `teeWAVWriter` mirrors PCM blocks to disk and patches the data-size field at finalize.
- Streaming activates only when ALL of: `WT_STREAM != 0`, raw cache miss, no `<key>.partial.json` resume blob, ffmpeg available, engine is `whisper`, source extension is not `.wav`.
- `runChunkedStream` keeps the same partial-cache + offset semantics as `runChunked`.

## GUI model cache & idle unload — `internal/gui/transcribe/model_cache.go`

- `whisper.Model` lives on `Panel` (`cachedModel`, `cachedModelSize`, `modelMu`). Use `acquireModel(size)` + deferred `releaseModel()` (releaseModel only bumps `lastModelActivity`).
- Watcher goroutine (`ensureUnloadWatcher`) ticks every 30 s; drops the model when idle and `time.Since(lastModelActivity) >= unloadTimeout()`. Default 5 min, override `WT_MODEL_UNLOAD_SEC` (`0` disables).
- Watcher resets activity timestamp during runs. CLI doesn't need this; only `wt-gui` carries the cache.

## LLM auto-rename — `internal/llm/runner.go`

- Any post-transcription phase >1s must update both `p.setStatus(...)` and `platsvc.UpdateProgress(...)`.
- GBNF grammars: never write `(rule)? (rule)? …` chains. Use `{n,m}` repetition (e.g. `slugChar{5,60}`). Verify with `time llama-cli ... --grammar-file g.gbnf`.
- Per-call timeout: `llmTimeout()` (override `WT_LLM_TIMEOUT` seconds).
