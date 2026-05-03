# ASR engines & transcription

## Chunked, resumable driver

All ASR engines (Whisper + sherpa-onnx variants) run through unified chunked driver in `internal/transcriber/engine_chunk.go`. Default 30s windows (env `WT_CHUNK_SEC`, range 5–60). Partial cache (`internal/transcriber/cache.Partial`) persisted **after every chunk**, engine-agnostic format → resume works across engines, ≤1 chunk lost on OOM/reboot/kill.

Never reintroduce a "process full sample buffer in one shot" path — long inputs (>5min) on Android arm64 reliably trigger kernel OOM killer, often reboots the device with no userland crash log (persistent `wt.log` appears cut off because GUI is killed before next `AppendLogLine` flushes).

When adding a new ASR engine: write a `chunkProcessor` taking chunk-local samples → returning chunk-local segments. Driver shifts timestamps onto absolute timeline, saves partials, handles ctx cancellation, emits uniform progress.

## Engine selection

Pluggable via `shared.Config.Engine` / `WT_ENGINE` / `JobSpec.Engine`. Values:

- `whisper` (default) — 99-lang fallback
- `zipformer` — sherpa transducer, uppercase output
- `moonshine` — sherpa, cased+punctuated, ~10× RTF on Exynos 2400 CPU

Dispatch in `internal/transcriber/engine.go` `Job.runASR`. New sherpa-backed engine: reuse helpers in `engine_zipformer.go` (`findSherpaASRBinary`, `writeTempWAV`, `invokeSherpaCLI`, `finalizeSherpaRun`, `coalesceTokens`) and add a case to `runASR` — do **not** branch inside `runWhisper`.

GUI must set `spec.Engine = models.EngineForActiveASR(mgr.Active(models.FamilyASR))` (or fall back to whisper). Catalog entries need `Family: FamilyASR` and `Engine: shared.EngineX`.

## Engine quirks

- Moonshine: ~12–15s minimum effective input; shorter inputs may produce empty `text`. Pad-and-trim or fall back to Zipformer for <15s.
- Vanilla Zipformer transducer: accepts arbitrarily short inputs.
- Parakeet TDT: requires `--model-type=nemo_transducer` (NOT `transducer`). Plain transducer path looks for `vocab_size` metadata TDT models don't populate.
- SenseVoice: `--sense-voice-model=` (single-file), NOT encoder/decoder/joiner triplet.

## Token coalescing

Whisper.cpp emits BPE sub-word tokens (`Segment.Tokens`), not words. Word boundaries = tokens whose `Text` starts with space (e.g. `" Good"`, `"'s"`). Continuation pieces have no leading space. Any code path consuming per-token output for word-level speaker mapping MUST coalesce continuation pieces onto previous word before downstream `joinWords` runs `strings.Join(parts, " ")` — otherwise contractions split (`"I 'm"`), multi-piece words split (`"F ul ham"`), every word gap doubles. Use `coalesceWhisperTokens` in `internal/transcriber/output.go`.

`sherpa-onnx-offline` emits one JSON line per WAV: `{"text", "tokens", "timestamps"}`. Tokens are BPE sub-word; word boundaries are space-leading tokens. Same coalesce: glue non-space-leading tokens onto previous before emitting word-level segments. Mirror in `engine_zipformer.go:coalesceTokens`.

## Catalog policy (`internal/models/catalog.go`)

Curated, not exhaustive. Each entry must be best-in-class for its niche or be removed. Never add on theoretical/paper-SOTA grounds — validate on real device with real audio fixture first.

Right test for diarization: `wt-test -engine=<asr> -diarize -speakers=2 <audio>` then check `Diarized in Xs, N raw segments, M unique speakers`. If M < expected speakers, model failed regardless of paper benchmarks.

Current top-tier picks:

- **Parakeet TDT 0.6B v2 int8** — English-only, ~9× RTF, native cased+punct
- **SenseVoice int8** — multilingual zh/en/ja/ko/yue, ~16× RTF, native cased+punct, word timestamps
- **Whisper-turbo** — 99-lang fallback
- **Qwen3 0.6B Q4_K_M** — auto-rename namer default; 1.7B kept as quality option

Use `csukuangfj/*` HF mirrors for individual ONNX files instead of `sherpa-onnx/releases/*.tar.bz2` archives so existing FileSpec downloader works without tar/bz2 extraction.

### Watch list

Wait for `csukuangfj/*` or k2-fsa to publish ONNX + sherpa-onnx integration, benchmark on real fixture before adding:

- **Sortformer v2** (NVIDIA NeMo) — potential pyannote-3.0 replacement; test against `diar-titanet-large` on 1–2min conversational clip with `--speakers=2`; reject if fewer raw segments than pyannote-3.0 (Reverb-v2 lesson).
- **Qwen3-ASR 0.6B** — potential SOTA multilingual (52 langs, Apache 2.0); no Android port yet. Would slot alongside Parakeet (English) and SenseVoice (Asian).

### Reject list

- **Picovoice Falcon** — closed-source commercial; incompatible with on-device privacy positioning even on free tier.
- **Whisper Vulkan on Xclipse 940** — dead end (see android.md).
- **Distil-Whisper Large V3** — redundant; whisper-turbo IS official OpenAI distillation of large-v3 (multilingual; distil-large-v3 is English-only).
- **Moonshine in catalog** — 12–15s min input unfit for voice notes/live captions. Code remains for env-bundle benchmarking only.
- **Reverb-v2** — Rev.ai 2024 "conversational SOTA" failed catastrophically on 1-min conversational interview (1 cluster, 1 speaker, 91.7s) where pyannote-3.0 succeeded (12 segs, 2 speakers, 16.4s).
- **Moonshine/Zipformer/Paraformer/CT-Transformer in catalog** — engines remain available via env-var bundles for benchmarking but dominated by curated picks for end users.

## LLM auto-rename

`llama-cli` subprocess; mobile CPUs slow. Per-invocation timeout: `llmTimeout()` in `internal/llm/runner.go` (10min Android, 2min desktop, override `WT_LLM_TIMEOUT` seconds). Always update live status (`p.setStatus`) and foreground notification (`platsvc.UpdateProgress`) for any post-transcription phase that can run >1s — UI looks frozen on previous phase's message otherwise.

GBNF grammars: never write `(rule)? (rule)? …` chains — each `?` doubles state space, llama.cpp's sampler degrades to effectively single-threaded grammar evaluation. 55-`?` chain made auto-rename hang 4+ min on phone (single 100% CPU thread despite `-t 6`). Use `{n,m}` repetition (e.g. `slugChar{5,60}`); same expressiveness, ~100× faster. Verify with `time llama-cli ... --grammar-file g.gbnf` before merging grammar changes.
