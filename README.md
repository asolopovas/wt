# WTranscribe

[![CI](https://github.com/asolopovas/wt/actions/workflows/ci.yml/badge.svg?branch=main)](https://github.com/asolopovas/wt/actions/workflows/ci.yml)

Audio transcription for Windows, Linux, and Android. Powered by [whisper.cpp](https://github.com/ggml-org/whisper.cpp) with speaker diarization via [NVIDIA NeMo Sortformer](https://huggingface.co/nvidia/diar_sortformer_4spk-v1) (desktop CLI) or sherpa-onnx with pyannote + TitaNet (GUI and Android).

Ships as a CLI (`wt`), a desktop GUI (`wt-gui`), and an Android APK. Models download automatically on first run.

## Download

* [Latest release](https://github.com/asolopovas/wt/releases/latest) (stable, versioned)
* [Rolling release](https://github.com/asolopovas/wt/releases/tag/rolling) (current `main`, prerelease)

Each release includes:

* `wt-setup-<v>.exe` — Windows installer (Inno Setup)
* `wt_<v>_amd64.deb` — Debian/Ubuntu package
* `wt-<v>.apk` — Android app

Silent Windows install:

```
wt-setup.exe /VERYSILENT /SP-
```

Config and models directory:

* **Windows:** `%APPDATA%\wt\` (i.e. `C:\Users\<you>\AppData\Roaming\wt\`)
* **Linux:** `$XDG_CONFIG_HOME/wt/` or `~/.config/wt/`
* **Android:** app private storage

## GUI

`wt-gui` is a Fyne desktop app (Windows/Linux) and Android app:

* Drag and drop audio files, or pick via file dialog
* In-app recording (Android) and live microphone transcription (desktop)
* Transcript history with search, replay, and re-export
* Export to JSON, CSV, or plain text
* Session log tab with live whisper.cpp / diarizer output
* System tray icon and CPU/RAM/GPU stats
* Settings UI for model, language, device, threads, diarization
* Optional auto-rename of audio + transcript via local LLM (`llama-cli`)

## CLI Usage

```bash
wt recording.ogg                    # single file
wt file1.mp3 file2.wav              # multiple files
wt "recordings/*.ogg"               # glob pattern
wt -m medium -l en audio.wav        # specify model and language
wt --speakers 3 meeting.ogg         # hint number of speakers
wt --no-diarize audio.ogg           # skip speaker detection
wt --tdrz audio.ogg                 # whisper.cpp tinydiarize (small.en-tdrz model)
wt --live -l en                     # live microphone transcription
wt --no-rename audio.ogg            # skip LLM-based auto-rename
wt models                           # list / manage downloaded models
```

Output is written next to the input as `<name>_<model>_<timestamp>.json`.

## Output Format

```json
{
  "model": "turbo",
  "language": "en",
  "duration_ms": 9800,
  "diarizer": "nemo-sortformer",
  "device": "cuda",
  "speakers_detected": 2,
  "utterances": [
    {"start": 0, "end": 5200, "speaker": "SPEAKER_01", "text": "Good morning, let's begin."},
    {"start": 5200, "end": 9800, "speaker": "SPEAKER_02", "text": "Thanks, I have a few items to discuss."}
  ],
  "words": [
    {"text": "Good", "start": 0, "end": 320, "speaker": "SPEAKER_01", "confidence": 0.99}
  ]
}
```

All timestamps are in milliseconds.

## Flags

| Flag | Default | Description |
|------|---------|-------------|
| `-l, --lang` | auto | Language code (`en`, `ru`, ...). Omit to auto detect. |
| `-m, --model-size` | `turbo` | See **Models** table below. |
| `--model` | | Explicit path to a GGML model file. |
| `-t, --threads` | all cores | Thread count. |
| `--speakers` | `0` (auto) | Hint number of speakers. Forces sherpa backend when > 0. |
| `--tdrz` | off | Use whisper.cpp tinydiarize (requires `small.en-tdrz` model). |
| `--no-diarize` | off | Skip diarization entirely; everything maps to `SPEAKER_01`. |
| `--no-rename` | off | Skip LLM-based auto-rename of audio + transcript. |
| `--live` | off | Live microphone transcription. |
| `-V, --verbose` | off | Debug output. |

CLI flags override `config.yml` values. Subcommand: `wt models` for model management.

## Models

| Model | Size | Notes |
|-------|------|-------|
| `tiny` / `tiny.en` | 75 MB | Fastest, low accuracy |
| `base` / `base.en` | 142 MB | General use |
| `small` / `small.en` | 466 MB | Better accuracy |
| `medium` / `medium.en` | 1.5 GB | High accuracy |
| `large-v1` / `large-v2` / `large-v3` (alias `large`) | 3.1 GB | Best accuracy, slowest |
| `turbo` (alias `large-v3-turbo`) | 1.6 GB | Best speed/accuracy tradeoff (default) |
| `distil-small.en` / `distil-medium.en` | 0.3 / 0.8 GB | Distilled, faster |
| `distil-large-v2` / `distil-large-v3` | 1.5 GB | Distilled large |

`.en` variants are English-only and slightly faster. Models download to `<config-dir>/models/` on first use.

## Config

`<config-dir>/config.yml` (created on first run; see paths above):

```yaml
version: 1
model: turbo
language: ""           # empty = auto detect
device: auto           # auto, cuda, cpu
threads: 0             # 0 = all cores
speakers: 0            # 0 = auto
no_diarize: false
tdrz: false
cache_expiry_days: 30
```

Cache (transcripts, imports) lives at `<config-dir>/cache/`.

## Diarization

* **NeMo Sortformer** (desktop CLI default): up to 4 speakers, GPU-accelerated when CUDA is available, runs via the bundled Python environment (`uv` + `scripts/diarize.py`). Public model, no HuggingFace token needed. Caches to `~/.cache/huggingface/`.
* **sherpa-onnx** (GUI default, only Android backend, CLI when `--speakers N` is set): pyannote-3.0 segmentation + NeMo TitaNet-Large embeddings. Pure ONNX runtime, no Python.
* `--tdrz`: whisper.cpp built-in tinydiarize. Fast, less accurate, requires the `small.en-tdrz` model.
* `--no-diarize`: every utterance gets `SPEAKER_01`.

## Logs

* `%APPDATA%\wt\setup.log` (Windows) — installer output (ffmpeg, CUDA, Python env, NeMo, model download)
* `%TEMP%\Setup Log *.txt` — Inno Setup file operations
* GUI: **Session log** tab streams live whisper.cpp / diarizer output

## Build from Source

Requires Go 1.26+, GCC (MinGW on Windows), CMake, ffmpeg, [Task](https://taskfile.dev/). On first run `task build` clones whisper.cpp into `third_party/`.

```bash
git clone https://github.com/asolopovas/wt.git
cd wt

task build                       # CLI + GUI + installer (host platform)
task build ONLY=cli              # CLI only (skip installer)
task build ONLY=gui              # GUI only (skip installer)
task build ONLY=android          # Android APK
task install                     # build + replace local install
task install TARGET=android      # build APK + adb install + launch
task test                        # full test suite (rebuilds whisper lib)
task test SHORT=1                # skip CGo / model tests
task test INTEGRATION=1          # diarization integration tests
task lint [FIX=1] [ANDROID=1]    # golangci-lint (+gofumpt with FIX)
task models FETCH=samples        # fetch diarization test samples
task models FETCH=import         # import models from Windows mounts (WSL)
task release [ROLLING=1]         # bump + GH release (or update rolling prerelease)
task clean [DEEP=1]              # clean dist/ (+ whisper.cpp build with DEEP)
```

Android build requires Android SDK + NDK 27.2.x + msys2 (for ffmpeg cross-compile). See `AGENTS.md` for the full task reference.

## Project Layout

```
cmd/{wt,wt-gui,wt-test}    CLI / Fyne GUI / Android test CLI
internal/transcriber/      Audio, model resolution, CSV, live mode
internal/diarizer/         NeMo subprocess + sherpa-onnx
internal/llm/              llama-cli subprocess (auto-rename)
internal/gui/              Fyne GUI (token-based design system)
internal/ui/               Terminal spinners (CLI only)
bindings/go/               Vendored whisper.cpp CGo bindings
scripts/                   Build helpers, Inno Setup, diarize.py
```

## License

MIT. Provided as-is, no warranty. PRs welcome.
