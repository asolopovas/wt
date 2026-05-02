# CONTEXT.md — wt domain glossary

This file names the concepts the codebase is *about*. Architecture reviews
(see `.agents/skills/improve-codebase-architecture`) use these terms when
proposing seams; future agents should prefer them over ad-hoc names like
"handler," "service," or "manager."

Keep terse. Add a term when a refactor introduces or sharpens a concept.

---

## Audio & transcripts

**Source audio** — the user's input file (any format ffmpeg can read).
Identified by absolute path + size + mtime; the `(path, size, mtime)`
triple is the *audio identity* that keys every cache lookup.

**Samples** — `[]float32` at 16 kHz mono, the form whisper.cpp accepts.
Produced from source audio by ffmpeg-decode-and-cache.

**Transcript** — the structured output: `Transcript` struct in
`internal/transcriber/output.go`. Has utterances, words, detected language,
duration, diarizer name, device. The transcript is what callers ultimately
want; everything else is plumbing.

**Utterance** — one segment-level chunk of speech, with start/end ms,
speaker label, text. Whisper produces these.

**Word** — token-level entry under an utterance, with per-token
confidence. Whisper produces these when token timestamps are enabled.

**Speaker label** — `"SPEAKER_01"` form, assigned by mapping whisper
segment times into diarizer segments via `diarizer.SpeakerForTime`.

---

## Pipeline concepts

**Job** — a single audio file's worth of transcription work, end to end:
cache check → audio load → whisper transcribe (with optional resume) →
diarize → build transcript → cache store. Lives at `transcriber.Job`.
Parameterized by a `JobSpec` (model, language, threads, speakers,
diarization flags) and a `Hooks` struct (phase, progress, log, resume
callbacks). Returns a `Result`.

The Job is the deep module; CLI (`cmd/wt`) and GUI
(`internal/gui/transcribe`) are thin adapters over it. Both used to
reimplement the orchestration; that drift is what the Job exists to
prevent.

**JobSpec** — pure-value description of *what* to transcribe.

**Hooks** — pure-value struct of optional callbacks (`OnPhase`,
`OnProgress`, `OnLog`, `OnResume`). Zero value = silent run; ideal
test fixture.

**Phase** — coarse stage of a Job: `cache_check`, `loading_audio`,
`transcribing`, `diarizing`, `writing`. Drives status text.

**Resume prompt / choice** — when a partial transcript is on disk for
this audio identity, the Job asks the caller whether to resume from
the last cached segment, discard and start fresh, or abort. CLI always
chooses fresh; GUI shows a modal.

---

## Diarizer

**Diarizer Backend** — pluggable speaker-segmentation engine satisfying
`diarizer.Backend`. Two real adapters: **NeMo Sortformer** (Python
subprocess, GPU, desktop default) and **sherpa-onnx** (native binary,
Android default + Windows when forced speaker count). The Backend's
`Name()` is metadata that travels with the Transcript — never hardcode
the diarizer name at the call site.

**Diarizer Segment** — `(speaker int, startSec, endSec)`. The intermediate
form between backend output and speaker-labelled utterances.

**TDRZ fallback** — tinydiarize, a whisper-internal speaker-turn detector.
Used when no external Backend is available and `--no-diarize` wasn't set.

---

## Cache

**Transcript cache** — content-addressed store of completed Transcripts,
keyed by audio identity + `(model, language, speakers, no_diarize)`.
Lives at `internal/transcriber/cache/`. A cache hit short-circuits the
whole Job.

**Raw-segments cache** — content-addressed store of whisper segments
*before* diarization, keyed by audio identity + `(model, language)` only.
Lets the Job re-diarize with different speaker counts without re-running
whisper.

**Partial transcript** — incremental snapshot saved when a Job is
cancelled mid-whisper. The next Job for the same audio identity offers
to resume from it.

**Cache export** — copying a cached transcript out to a user-visible path
(e.g. CLI's `<filename>_<model>_<stamp>.json` next to the source).
Orchestration concern, not a Job concern.

---

## External processes

**Subprocess seam** — `internal/diarizer/subproc.go` owns `exec.Cmd`
lifecycle: stdout streaming, stderr tail buffer, optional stderr line
interceptor, `HideWindow`, context-cancel translation. Currently used
only by diarizer Backends. Candidate to lift package-level so
`internal/llm.Runner` can stop reimplementing it.

**FFmpeg facade** — does not exist yet. Today every caller looks up the
binary via `transcriber.FindFFmpeg()` and assembles its own `exec.Cmd`.
A future facade would expose the four real jobs (convert-to-PCM16-WAV,
probe-duration, extract-peaks, microphone-capture) as the seam.
