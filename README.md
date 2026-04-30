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

Config and models live under `%USERPROFILE%\.wt\` (Windows) or `~/.wt/` (Linux).

## GUI

`wt-gui` is a Fyne desktop app (Windows/Linux) and Android app:

* Drag and drop audio files, or pick via file dialog
* In app recording (Android) and live microphone transcription (desktop)
* Transcript history with search, replay, and re-export
* Export to JSON, RTF, or CSV
* Session log tab with live whisper.cpp / diarizer output
* System tray icon and CPU/RAM/GPU stats
* Settings UI for model, language, device, threads, diarization

## CLI Usage

```bash
wt recording.ogg                    # single file
wt file1.mp3 file2.wav              # multiple files
wt "recordings/*.ogg"               # glob pattern
wt -m medium -l en audio.wav        # specify model and language
wt --speakers 3 meeting.ogg         # hint number of speakers
wt --no-diarize audio.ogg           # skip speaker detection
wt --tdrz audio.ogg                 # whisper.cpp tinydiarize instead of NeMo
wt --live -l en                     # live microphone transcription
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
| `-m, --model-size` | `turbo` | `tiny`, `base`, `small`, `medium`, `large-v3`, `turbo` |
| `--model` | | Explicit path to a GGML model file. |
| `-t, --threads` | all cores | Thread count. |
| `--speakers` | 0 (auto) | Hint number of speakers. Forces sherpa backend when > 0. |
| `--tdrz` | off | Use whisper.cpp tinydiarize. |
| `--no-diarize` | off | Skip diarization entirely. |
| `--live` | off | Live microphone transcription. |
| `-V, --verbose` | off | Debug output. |

CLI flags override `config.yml` values.

## Models

| Model | Size | Notes |
|-------|------|-------|
| `tiny` / `tiny.en` | 75 MB | Fastest, low accuracy |
| `base` / `base.en` | 142 MB | General use |
| `small` / `small.en` | 466 MB | Better accuracy |
| `medium` / `medium.en` | 1.5 GB | High accuracy |
| `large-v3` | 3.1 GB | Best accuracy, slowest |
| `turbo` | 1.6 GB | Best speed/accuracy tradeoff (default) |

`.en` variants are English only and slightly faster. Models download to `~/.wt/models/` on first use.

## Config

`~/.wt/config.yml` (created on first run):

```yaml
model: turbo
language: ""           # empty = auto detect
device: auto           # auto, cuda, cpu
threads: 0             # 0 = all cores
speakers: 0            # 0 = auto
no_diarize: false
tdrz: false
cache_expiry_days: 30
```

## Diarization

* **NeMo Sortformer** (desktop CLI default): up to 4 speakers, GPU accelerated when CUDA is available, runs via the bundled Python environment. Public model, no HuggingFace token needed. Caches to `~/.cache/huggingface/`.
* **sherpa-onnx** (GUI default, Android only backend, CLI when `--speakers N` is set): pyannote-3.0 segmentation + NeMo TitaNet-Large embeddings. Pure ONNX runtime, no Python.
* `--tdrz`: whisper.cpp built in tinydiarize. Fast, less accurate.
* `--no-diarize`: every utterance gets `SPEAKER_01`.

## Logs

* `~/.wt/setup.log` — installer output (ffmpeg, CUDA, Python env, NeMo, model download)
* `%TEMP%\Setup Log *.txt` — Inno Setup file operations

## Build from Source

Requires Go 1.26+, GCC (MinGW on Windows), CMake, ffmpeg, [Task](https://taskfile.dev/).

```bash
git clone https://github.com/asolopovas/wt.git
cd wt
task build              # compile CLI + GUI (clones whisper.cpp on first run)
task build ONLY=cli     # CLI only
task install            # replace installed binaries locally
task test               # run test suite
task check              # verify toolchain
```

Android APK: `task install-android` (requires Android SDK/NDK + adb).

## License

MIT. Provided as is, no warranty. PRs welcome.
