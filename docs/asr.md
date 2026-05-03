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

## LLM auto-rename — `internal/llm/runner.go`

- Any post-transcription phase >1s **must** update both `p.setStatus(...)` and `platsvc.UpdateProgress(...)`. Otherwise UI looks frozen on the previous phase's message.
- GBNF grammars: **never** write `(rule)? (rule)? …` chains — each `?` doubles state space and llama.cpp's grammar sampler is single-threaded. Use `{n,m}` repetition (e.g. `slugChar{5,60}`). Verify with `time llama-cli ... --grammar-file g.gbnf` before merging.
- Per-call timeout: `llmTimeout()` (override `WT_LLM_TIMEOUT` seconds).
