# WTranscribe

[![CI](https://github.com/asolopovas/wt/actions/workflows/ci.yml/badge.svg?branch=main)](https://github.com/asolopovas/wt/actions/workflows/ci.yml)

Audio transcription for Windows, powered by [whisper.cpp](https://github.com/ggml-org/whisper.cpp) with speaker diarization from [NVIDIA NeMo Sortformer](https://huggingface.co/nvidia/diar_sortformer_4spk-v1).

Models download automatically on first run.

## Features

- Batch transcription of any audio format (WAV native, everything else via ffmpeg)
- Speaker diarization (up to 4 speakers) — GPU-accelerated via NeMo
- 99 languages with auto-detection
- Live microphone transcription
- Structured JSON output with per-word timestamps, speakers, and confidence
- CUDA acceleration for both whisper.cpp and diarization

## Install

Download the latest installer from [Releases](https://github.com/asolopovas/wt/releases) and run it. The installer bundles `wt`, a Python environment, the NeMo diarizer, and a default Whisper model.

Silent install:

```
wt-setup.exe /VERYSILENT /SP-
```

Config and models are stored under `%USERPROFILE%\.wt\`.

## Usage

```bash
wt recording.ogg                    # single file
wt file1.mp3 file2.wav              # multiple files
wt "recordings/*.ogg"               # glob pattern
wt -m medium -l en audio.wav        # specify model and language
wt --speakers 3 meeting.ogg         # hint number of speakers
wt --no-diarize audio.ogg           # skip speaker detection
wt --tdrz audio.ogg                 # use whisper.cpp tinydiarize instead of NeMo
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

All timestamps are milliseconds.

## Flags

| Flag | Default | Description |
|------|---------|-------------|
| `-l, --lang` | auto | Language code (`en`, `ru`, ...). Omit to auto-detect. |
| `-m, --model-size` | `turbo` | `tiny`, `base`, `small`, `medium`, `large-v3`, `turbo` |
| `--model` | — | Explicit path to a GGML model file. |
| `-t, --threads` | all cores | Thread count. |
| `--speakers` | 0 (auto) | Hint number of speakers (NeMo Sortformer caps at 4). |
| `--tdrz` | off | Use whisper.cpp tinydiarize instead of NeMo. |
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

`.en` variants are English-only and slightly faster. Models download to `~/.wt/models/` on first use.

## Config

`~/.wt/config.yml` (created on first run):

```yaml
model: turbo
language: ""           # empty = auto-detect
device: auto           # auto, cuda, cpu
threads: 0             # 0 = all cores
speakers: 0            # 0 = auto
no_diarize: false
tdrz: false
cache_expiry_days: 30
```

## Diarization

Speaker detection uses [NVIDIA NeMo Sortformer](https://huggingface.co/nvidia/diar_sortformer_4spk-v1), running locally via the bundled Python environment. It identifies up to 4 speakers and is GPU-accelerated when CUDA is available.

The model is public — no HuggingFace token is required. It downloads automatically on first run to `~/.cache/huggingface/`.

Alternatives:

- `--tdrz` — whisper.cpp's built-in tinydiarize. Pure Go, no Python, less accurate.
- `--no-diarize` — skip speaker detection; every utterance gets `SPEAKER_01`.

## Logs

- `~/.wt/setup.log` — installer output (ffmpeg, CUDA, Python env, NeMo, model download)
- `%TEMP%\Setup Log *.txt` — Inno Setup file operations

## Build from Source

Requires: Go 1.26+, GCC (MinGW), CMake, ffmpeg, [Task](https://taskfile.dev/).

```bash
git clone https://github.com/asolopovas/wt.git
cd wt
task build              # compile binaries (clones whisper.cpp on first run)
task install            # replace installed binaries locally
task setup              # full silent reinstall via installer
task test               # run test suite
task check              # verify toolchain
```

## Disclaimer

This software is provided "as is", without warranty of any kind. Use it at your own risk. If you want a feature, please open a pull request.

## License

MIT
