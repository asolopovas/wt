# ASR engines & transcription

## Chunked driver — `internal/transcriber/engine_chunk.go`

- Never add a "process full buffer in one shot" path — long inputs OOM-kill on Android arm64 (often reboots the device with no crash log).
- Chunk size override: env `WT_CHUNK_SEC` (5–60).

## Engine selection

Dispatched in `internal/transcriber/engine.go:Job.runASR`; never branch inside an engine function. Values: `whisper-onnx` (default; `whisper` accepted as legacy alias), `zipformer`, `parakeet`, `sensevoice`, `canary`, `nemo-ctc`. Set via `shared.Config.Engine` / env `WT_ENGINE` / `JobSpec.Engine`.

Note: `gigaam-v3-ru` is a model served by the `nemo-ctc` engine, not an engine.

## Engine quirks

- **Parakeet TDT**: needs `--model-type=nemo_transducer` (plain `transducer` reads `vocab_size` metadata TDT lacks).
- **SenseVoice**: single-file flag `--sense-voice-model=`, not encoder/decoder/joiner triplet.
- **Whisper-ONNX**: sherpa-whisper has a hardcoded 30 s per-call limit; existing `runChunked` driver already chunks at 30 s. Override model dir with `WT_WHISPER_ONNX_DIR`.

## Token coalescing

`sherpa-onnx-offline` JSON emits BPE sub-word tokens. Word boundary = leading-space token; continuations have no leading space. Always coalesce continuations onto the previous word before any `strings.Join(parts, " ")` — otherwise contractions split (`I 'm`), multi-piece words split (`F ul ham`), gaps double.

- `coalesceTokens` in `internal/transcriber/engine_zipformer.go`

## Catalog policy — `internal/models/catalog.go`

- Validate before adding: `wt-test -engine=<asr> -diarize -speakers=2 <audio>`. Reject if `unique speakers` < expected, regardless of paper benchmarks.
- Use `csukuangfj/*` HF mirrors for individual ONNX files; avoid `sherpa-onnx/releases/*.tar.bz2` (no tar/bz2 extraction in `FileSpec` downloader).

### Don't add

- Picovoice Falcon — closed-source.
- Whisper Vulkan on Xclipse 940 — see `android.md`.
- Distil-Whisper Large V3 — whisper-turbo is the official multilingual distillation.
- Reverb-v2 — failed validation (1 cluster on a clip pyannote-3.0 handled at 12 segs / 2 speakers).
- Sortformer v2 / Qwen3-ASR — wait for `csukuangfj/*` ONNX + benchmark first. Sortformer must beat pyannote-3.0 raw segment count on a 1–2 min `--speakers=2` clip.

## Audio I/O

- Skip full audio decode on raw cache hit; `Job.Run` only calls `LoadAudioSamples` when ASR will actually run, or when the diarizer needs a freshly produced 16 k mono WAV (source isn't `.wav` and no cached `<key>.wav` exists). Otherwise duration comes from `ProbeDurationMs`.
- `readPCM16WAV` streams 1 MiB blocks via `streamPCMToFloat32` into a single preallocated `[]float32`. Peak memory ~4 B/sample (saves ≈230 MB on a 2 h mono clip vs the buffered-then-converted path).
- `WritePCM16WAV` flushes int16 in 4 KiB batches through a 256 KiB `bufio.Writer` (~50× faster than per-sample `Write` for long files).

## LLM auto-rename — `internal/llm/runner.go`

- Any post-transcription phase >1 s must update both `p.setStatus(...)` and `platsvc.UpdateProgress(...)`; otherwise UI looks frozen on the previous phase.
- GBNF grammars: never write `(rule)? (rule)? …` chains — each `?` doubles state space and llama.cpp's grammar sampler is single-threaded. Use `{n,m}` repetition (e.g. `slugChar{5,60}`). Verify with `time llama-cli ... --grammar-file g.gbnf`.
- Per-call timeout: `llmTimeout()` (override `WT_LLM_TIMEOUT` seconds).
