package models

import (
	shared "github.com/asolopovas/wt/internal"
)

type Family string

const (
	FamilyWhisper  Family = "whisper"
	FamilyDiarizer Family = "diarizer"
	FamilyLLM      Family = "llm"
	FamilyASR      Family = "asr" // sherpa-onnx-backed ASR engines (Parakeet, etc.)
)

type Entry struct {
	ID            string
	Family        Family
	Engine        string // populated for FamilyASR entries (parakeet/moonshine/...)
	DisplayName   string
	URL           string
	RelPath       string
	SizeBytes     int64
	SHA256        string
	RAMHintMB     int
	DefaultActive bool
	Files         []FileSpec
	Description   string // one-line marketing/quality blurb
}

type FileSpec struct {
	URL       string
	RelPath   string
	SizeBytes int64
	SHA256    string
}

func (e Entry) FileSpecs() []FileSpec {
	if len(e.Files) > 0 {
		return e.Files
	}
	return []FileSpec{{URL: e.URL, RelPath: e.RelPath, SizeBytes: e.SizeBytes, SHA256: e.SHA256}}
}

func Catalog() []Entry {
	out := make([]Entry, 0, len(whisperEntries)+len(asrEntries)+len(diarizerEntries)+len(llmEntries))
	out = append(out, whisperEntries...)
	out = append(out, asrEntries...)
	out = append(out, diarizerEntries...)
	out = append(out, llmEntries...)
	return out
}

func ByID(id string) (Entry, bool) {
	for _, e := range Catalog() {
		if e.ID == id {
			return e, true
		}
	}
	return Entry{}, false
}

func ByFamily(f Family) []Entry {
	out := []Entry{}
	for _, e := range Catalog() {
		if e.Family == f {
			out = append(out, e)
		}
	}
	return out
}

var whisperEntries = []Entry{
	{ID: "whisper-tiny", Family: FamilyWhisper, DisplayName: "Whisper tiny (multilingual)", URL: "https://huggingface.co/ggerganov/whisper.cpp/resolve/main/ggml-tiny.bin", RelPath: "ggml-tiny.bin", SizeBytes: 77_700_000, RAMHintMB: 200, DefaultActive: false},
	{ID: "whisper-base", Family: FamilyWhisper, DisplayName: "Whisper base (multilingual)", URL: "https://huggingface.co/ggerganov/whisper.cpp/resolve/main/ggml-base.bin", RelPath: "ggml-base.bin", SizeBytes: 147_000_000, RAMHintMB: 300},
	{ID: "whisper-small", Family: FamilyWhisper, DisplayName: "Whisper small (multilingual)", URL: "https://huggingface.co/ggerganov/whisper.cpp/resolve/main/ggml-small.bin", RelPath: "ggml-small.bin", SizeBytes: 488_000_000, RAMHintMB: 700},
	{ID: "whisper-medium", Family: FamilyWhisper, DisplayName: "Whisper medium (multilingual)", URL: "https://huggingface.co/ggerganov/whisper.cpp/resolve/main/ggml-medium.bin", RelPath: "ggml-medium.bin", SizeBytes: 1_530_000_000, RAMHintMB: 2200},
	{ID: "whisper-large-v3", Family: FamilyWhisper, DisplayName: "Whisper large-v3 (multilingual)", URL: "https://huggingface.co/ggerganov/whisper.cpp/resolve/main/ggml-large-v3.bin", RelPath: "ggml-large-v3.bin", SizeBytes: 3_100_000_000, RAMHintMB: 4400},
	{ID: "whisper-turbo", Family: FamilyWhisper, DisplayName: "Whisper large-v3-turbo", URL: "https://huggingface.co/ggerganov/whisper.cpp/resolve/main/ggml-large-v3-turbo.bin", RelPath: "ggml-large-v3-turbo.bin", SizeBytes: 1_620_000_000, RAMHintMB: 2400, DefaultActive: true},
	{ID: "whisper-vad-silero", Family: FamilyWhisper, DisplayName: "Silero VAD v6.2.0", URL: "https://huggingface.co/ggml-org/whisper-vad/resolve/main/ggml-silero-v6.2.0.bin", RelPath: "ggml-silero-v6.2.0.bin", SizeBytes: 2_300_000, RAMHintMB: 50},
}

var diarizerEntries = []Entry{
	{
		ID:            "sherpa-diarizer",
		Family:        FamilyDiarizer,
		DisplayName:   "Sherpa diarizer (pyannote-3.0 + TitaNet-Large)",
		SizeBytes:     102_000_000,
		RAMHintMB:     350,
		DefaultActive: true,
		Files: []FileSpec{
			{URL: "https://huggingface.co/csukuangfj/sherpa-onnx-pyannote-segmentation-3-0/resolve/main/model.onnx", RelPath: "sherpa-onnx-pyannote-segmentation-3-0/model.onnx", SizeBytes: 6_000_000},
			{URL: "https://github.com/k2-fsa/sherpa-onnx/releases/download/speaker-recongition-models/nemo_en_titanet_large.onnx", RelPath: "titanet_large.onnx", SizeBytes: 96_000_000},
		},
	},
}

// EngineForActiveASR returns (engineID, modelID) for the currently selected
// transcription engine, picking the first installed entry from FamilyASR
// if any, else falling back to whisper.
func EngineForActiveASR(activeASR string) (engine, modelID string) {
	if activeASR != "" {
		if e, ok := ByID(activeASR); ok && e.Family == FamilyASR && e.Engine != "" {
			return e.Engine, e.ID
		}
	}
	return shared.EngineWhisper, ""
}

var legacyDiarizerIDs = map[string]string{
	"sherpa-pyannote-segmentation-3.0": "sherpa-diarizer",
	"sherpa-titanet-large":             "sherpa-diarizer",
}

// Top-tier ASR models (sherpa-onnx). Curated rather than exhaustive: each
// listed model is best-in-class for its niche. We deliberately do not ship
// Moonshine, SenseVoice, Paraformer, or LibriSpeech-trained Zipformer
// variants — Parakeet outperforms them on quality, and Whisper-turbo
// covers multilingual.
//
// Files for each entry come from csukuangfj/* HF mirrors (the same files
// that ship inside k2-fsa's sherpa-onnx-*.tar.bz2 release archives, but
// downloadable individually so we don't need tar/bz2 extraction).
var asrEntries = []Entry{
	{
		ID:          "parakeet-tdt-0.6b-v2-int8",
		Family:      FamilyASR,
		Engine:      shared.EngineParakeet,
		DisplayName: "Parakeet TDT 0.6B v2 (English)",
		Description: "#1 English ASR on Open ASR Leaderboard (~1.9% LibriSpeech WER). Native casing + punctuation. Best for English-only audio.",
		SizeBytes:   635_000_000,
		RAMHintMB:   1100,
		Files: []FileSpec{
			{URL: "https://huggingface.co/csukuangfj/sherpa-onnx-nemo-parakeet-tdt-0.6b-v2-int8/resolve/main/encoder.int8.onnx", RelPath: "sherpa-onnx-nemo-parakeet-tdt-0.6b-v2-int8/encoder.int8.onnx", SizeBytes: 622_000_000},
			{URL: "https://huggingface.co/csukuangfj/sherpa-onnx-nemo-parakeet-tdt-0.6b-v2-int8/resolve/main/decoder.int8.onnx", RelPath: "sherpa-onnx-nemo-parakeet-tdt-0.6b-v2-int8/decoder.int8.onnx", SizeBytes: 7_000_000},
			{URL: "https://huggingface.co/csukuangfj/sherpa-onnx-nemo-parakeet-tdt-0.6b-v2-int8/resolve/main/joiner.int8.onnx", RelPath: "sherpa-onnx-nemo-parakeet-tdt-0.6b-v2-int8/joiner.int8.onnx", SizeBytes: 2_500_000},
			{URL: "https://huggingface.co/csukuangfj/sherpa-onnx-nemo-parakeet-tdt-0.6b-v2-int8/resolve/main/tokens.txt", RelPath: "sherpa-onnx-nemo-parakeet-tdt-0.6b-v2-int8/tokens.txt", SizeBytes: 50_000},
		},
	},
	{
		ID:          "sense-voice-zh-en-ja-ko-yue-int8",
		Family:      FamilyASR,
		Engine:      shared.EngineSenseVoice,
		DisplayName: "SenseVoice (zh/en/ja/ko/yue)",
		Description: "Alibaba FunAudio — fast multilingual ASR for Chinese/English/Japanese/Korean/Cantonese. Native casing + punctuation. Single 228 MB model. Best for Asian-language audio.",
		SizeBytes:   228_000_000,
		RAMHintMB:   500,
		Files: []FileSpec{
			{URL: "https://huggingface.co/csukuangfj/sherpa-onnx-sense-voice-zh-en-ja-ko-yue-2024-07-17/resolve/main/model.int8.onnx", RelPath: "sherpa-onnx-sense-voice-zh-en-ja-ko-yue-2024-07-17/model.int8.onnx", SizeBytes: 228_000_000},
			{URL: "https://huggingface.co/csukuangfj/sherpa-onnx-sense-voice-zh-en-ja-ko-yue-2024-07-17/resolve/main/tokens.txt", RelPath: "sherpa-onnx-sense-voice-zh-en-ja-ko-yue-2024-07-17/tokens.txt", SizeBytes: 320_000},
		},
	},
}

// LLMs for the auto-rename namer. Curated for naming-task fitness, not
// general capability. Qwen3-0.6B is fast enough for live-feel rename on
// phone CPU; 1.7B kept as quality option.
var llmEntries = []Entry{
	{
		ID: "qwen3-0.6b-q4km", Family: FamilyLLM,
		DisplayName:   "Qwen3 0.6B (Q4_K_M, namer)",
		Description:   "3× faster than Qwen3-1.7B for filename naming with comparable output. Recommended default on phone.",
		URL:           "https://huggingface.co/unsloth/Qwen3-0.6B-GGUF/resolve/main/Qwen3-0.6B-Q4_K_M.gguf",
		RelPath:       "qwen3-0.6b-q4km.gguf",
		SizeBytes:     396_000_000,
		RAMHintMB:     800,
		DefaultActive: true,
	},
	{
		ID: "qwen3-1.7b-q4km", Family: FamilyLLM,
		DisplayName: "Qwen3 1.7B (Q4_K_M, namer)",
		Description: "Larger, slightly higher-quality naming. Slower on phone.",
		URL:         "https://huggingface.co/Qwen/Qwen3-1.7B-GGUF/resolve/main/Qwen3-1.7B-Q4_K_M.gguf",
		RelPath:     "qwen3-1.7b-q4km.gguf",
		SizeBytes:   1_100_000_000,
		RAMHintMB:   1800,
	},
}
