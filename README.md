# WTranscribe

[![CI](https://github.com/asolopovas/wt/actions/workflows/ci.yml/badge.svg?branch=main)](https://github.com/asolopovas/wt/actions/workflows/ci.yml)

Audio transcription for Windows, Linux, and Android. Powered by [sherpa-onnx](https://github.com/k2-fsa/sherpa-onnx) (Whisper / Parakeet / SenseVoice / Moonshine / Canary / NeMo CTC) with speaker diarization via [NVIDIA NeMo Sortformer](https://huggingface.co/nvidia/diar_sortformer_4spk-v1) (desktop CLI) or sherpa-onnx pyannote-3.0 segmentation + TitaNet-Large embeddings (GUI and Android).

Ships as a CLI (`wt`), a desktop GUI (`wt-gui`), and an Android APK. Models download automatically on first run.

## Download

- [Latest release](https://github.com/asolopovas/wt/releases/latest) (stable, versioned)
- [Dev release](https://github.com/asolopovas/wt/releases/tag/dev) (current `main`, prerelease, version `dev-YYYY-MM-DD-HH-MM-SS`)

Each release includes:

- `wt-setup-<v>.exe` — Windows installer (Inno Setup)
- `wt_<v>_amd64.deb` — Debian/Ubuntu package
- `wt-<v>.apk` — Android app

Silent Windows install:

```
wt-setup.exe /VERYSILENT /SP-
```

Config and models directory:

- **Windows:** `%APPDATA%\wt\` (i.e. `C:\Users\<you>\AppData\Roaming\wt\`)
- **Linux:** `$XDG_CONFIG_HOME/wt/` or `~/.config/wt/`
- **Android:** app private storage; user-visible models live in `/sdcard/Documents/WTranscribe/Models/`

## GUI

`wt-gui` is a Fyne desktop app (Windows/Linux) and Android app:

- Drag and drop audio files, or pick via file dialog
- In-app recording (Android) and live microphone transcription (desktop)
- Transcript history with search, replay, and re-export
- Export to JSON, CSV, plain text, or RTF bundle
- Session log tab with live engine / diarizer output
- System tray icon and CPU/RAM/GPU stats
- Settings UI for model, language, device, threads, diarization
- Optional auto-rename of audio + transcript via local LLM (`llama-cli`)

## CLI Usage

```bash
wt recording.ogg                    # single file
wt file1.mp3 file2.wav              # multiple files
wt "recordings/*.ogg"               # glob pattern
wt -l en audio.wav                  # specify language
wt --speakers 3 meeting.ogg         # hint number of speakers
wt --no-diarize audio.ogg           # skip speaker detection
wt --live -l en                     # live microphone transcription
wt --no-rename audio.ogg            # skip LLM-based auto-rename
wt models                           # list / manage downloaded models
```

Output is written next to the input as `<name>_<timestamp>.json`.

## Output Format

```json
{
  "model": "sherpa-whisper-turbo",
  "language": "en",
  "duration_ms": 9800,
  "diarizer": "nemo-sortformer",
  "device": "cuda",
  "speakers_detected": 2,
  "utterances": [
    {
      "start": 0,
      "end": 5200,
      "speaker": "SPEAKER_01",
      "text": "Good morning, let's begin."
    },
    {
      "start": 5200,
      "end": 9800,
      "speaker": "SPEAKER_02",
      "text": "Thanks, I have a few items to discuss."
    }
  ],
  "words": [
    {
      "text": "Good",
      "start": 0,
      "end": 320,
      "speaker": "SPEAKER_01",
      "confidence": 0.99
    }
  ]
}
```

All timestamps are in milliseconds.

## Flags

| Flag             | Default    | Description                                                  |
| ---------------- | ---------- | ------------------------------------------------------------ |
| `-l, --lang`     | auto       | Language code (`en`, `ru`, ...). Omit to auto detect.        |
| `--model`        |            | Catalog ID or directory of an active ASR model.              |
| `-t, --threads`  | all cores  | Thread count.                                                |
| `--speakers`     | `0` (auto) | Hint number of speakers. Forces sherpa backend when > 0.     |
| `--no-diarize`   | off        | Skip diarization entirely; everything maps to `SPEAKER_01`.  |
| `--no-rename`    | off        | Skip LLM-based auto-rename of audio + transcript.            |
| `--live`         | off        | Live microphone transcription.                               |
| `-V, --verbose`  | off        | Debug output.                                                |

CLI flags override `config.yml` values. Subcommand: `wt models` for model management.

## Models

The catalog is defined in `internal/default_config.yml`; pick one with `wt models` or in the GUI Settings tab. Highlights:

| Catalog ID                           | Family            | Notes                                            |
| ------------------------------------ | ----------------- | ------------------------------------------------ |
| `sherpa-whisper-turbo`               | Whisper turbo     | Best accuracy, 99 langs, recommended Android default |
| `parakeet-tdt-0.6b-v2-int8`          | Parakeet TDT      | ~3× faster than turbo, near-turbo accuracy      |
| `sense-voice-zh-en-ja-ko-yue-int8`   | SenseVoice        | 5 Asian langs (ZH/EN/JA/KO/YUE)                  |
| `moonshine-tiny-en-int8`             | Moonshine tiny    | Fastest English-only, ~250 MB RAM                |
| `gigaam-v3-ru`                       | NeMo CTC (GigaAM) | Russian                                          |

Models download to `<config-dir>/models/` (or the public Documents path on Android) on first use.

## Config

`<config-dir>/config.yml` (created on first run; see paths above):

```yaml
version: 1
language: ""        # empty = auto detect
device: auto        # auto, cuda, cpu
engine: whisper-onnx
threads: 4
speakers: 0         # 0 = auto
no_diarize: false
cache_expiry_days: 30
log_retention_days: 1
model: ""           # empty = use catalog default
diarizer: ""
llm: ""
```

Cache (transcripts, imports) lives at `<config-dir>/cache/`.

## Diarization

- **NeMo Sortformer** (desktop CLI default): up to 4 speakers, GPU-accelerated when CUDA is available, runs via the bundled Python environment (`uv` + `scripts/diarize.py`). Public model, no HuggingFace token needed. Caches to `~/.cache/huggingface/`.
- **sherpa-onnx** (GUI default, only Android backend, CLI when `--speakers N` is set): pyannote-3.0 segmentation + NeMo TitaNet-Large embeddings. Pure ONNX runtime, no Python.
- `--no-diarize`: every utterance gets `SPEAKER_01`.

## Logs

- `%APPDATA%\wt\setup.log` (Windows) — installer output (ffmpeg, CUDA, Python env, NeMo, model download)
- `%TEMP%\Setup Log *.txt` — Inno Setup file operations
- GUI: **Session log** tab streams live engine / diarizer output

## Build from Source

Requires Go 1.26+, GCC (MinGW on Windows), CMake, ffmpeg, [Task](https://taskfile.dev/). On first run `task build` clones sherpa-onnx into `third_party/`.

```bash
git clone https://github.com/asolopovas/wt.git
cd wt

task build                       # CLI + GUI + installer (host platform)
task build ONLY=cli              # CLI only (skip installer)
task build ONLY=gui              # GUI only (skip installer)
task build ONLY=android          # Android APK
task install                     # build + replace local install
task install TARGET=android      # build APK + adb install + launch
task test                        # full test suite
task test SHORT=1                # skip cgo / model tests
task test INTEGRATION=1          # diarization integration tests
task check [ANDROID=1]           # single quality gate (run before every commit): format + golangci-lint + deadcode + govulncheck + tests
task models FETCH=samples        # fetch diarization test samples
task models FETCH=import         # import sherpa models from Windows mounts (WSL)
task release [BUMP=1]            # default = dev prerelease (no version bump); BUMP=1 = bump patch + stable GH release
task clean [DEEP=1]              # clean dist/ (+ third_party builds with DEEP)
```

Android build requires Android SDK + NDK 27.2.x + msys2 (for ffmpeg cross-compile). See `AGENTS.md` for the full task reference.

## Project Layout

```
cmd/{wt,wt-gui,wt-test}    CLI / Fyne GUI / Android test CLI
internal/transcriber/      Audio, model resolution, chunked driver, sherpa engines, live mode
internal/diarizer/         NeMo subprocess + sherpa-onnx
internal/llm/              llama-cli subprocess (auto-rename)
internal/models/           Catalog + manager
internal/gui/              Fyne GUI (token-based design system)
internal/ui/               Terminal spinners (CLI only)
scripts/                   Build helpers, Inno Setup, diarize.py
third_party/sherpa-onnx    Cloned at build time (gitignored)
third_party/llama.cpp      Built or downloaded at build time (gitignored)
```

## License

MIT. Provided as-is, no warranty. PRs welcome.
