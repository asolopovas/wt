package models

import (
	shared "github.com/asolopovas/wt/internal"
)

type Family string

const (
	FamilyWhisper  Family = "whisper"
	FamilyDiarizer Family = "diarizer"
	FamilyLLM      Family = "llm"
	FamilyASR      Family = "asr"
)

type Entry struct {
	ID            string
	Family        Family
	Engine        string
	DisplayName   string
	URL           string
	RelPath       string
	SizeBytes     int64
	SHA256        string
	RAMHintMB     int
	DefaultActive bool
	Files         []FileSpec
	Description   string

	DiarSegRelPath string
	DiarEmbRelPath string

	Languages []string
}

func LanguagesFor(id string) []string {
	e, ok := ByID(id)
	if !ok {
		return nil
	}
	return e.Languages
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
	{ID: "whisper-small", Family: FamilyWhisper, DisplayName: "Whisper small (multilingual)", URL: "https://huggingface.co/ggerganov/whisper.cpp/resolve/main/ggml-small.bin", RelPath: "ggml-small.bin", SizeBytes: 488_000_000, RAMHintMB: 700},
	{ID: "whisper-turbo", Family: FamilyWhisper, DisplayName: "Whisper large-v3-turbo", URL: "https://huggingface.co/ggerganov/whisper.cpp/resolve/main/ggml-large-v3-turbo.bin", RelPath: "ggml-large-v3-turbo.bin", SizeBytes: 1_620_000_000, RAMHintMB: 2400, DefaultActive: true},
	{ID: "whisper-vad-silero", Family: FamilyWhisper, DisplayName: "Silero VAD v6.2.0", URL: "https://huggingface.co/ggml-org/whisper-vad/resolve/main/ggml-silero-v6.2.0.bin", RelPath: "ggml-silero-v6.2.0.bin", SizeBytes: 2_300_000, RAMHintMB: 50},
}

var (
	diarSegPyannote30URL = "https://huggingface.co/csukuangfj/sherpa-onnx-pyannote-segmentation-3-0/resolve/main/model.onnx"
	diarSegPyannote30Rel = "sherpa-onnx-pyannote-segmentation-3-0/model.onnx"

	diarEmbBase = "https://github.com/k2-fsa/sherpa-onnx/releases/download/speaker-recongition-models/"
)

var diarizerEntries = []Entry{

	{
		ID:             "diar-titanet-large",
		Family:         FamilyDiarizer,
		DisplayName:    "Standard (pyannote-3.0 + TitaNet-Large)",
		Description:    "Best DER in our sweep (0.190). Recommended default for English. ~107 MB.",
		SizeBytes:      107_000_000,
		RAMHintMB:      350,
		DefaultActive:  true,
		DiarSegRelPath: diarSegPyannote30Rel,
		DiarEmbRelPath: "titanet_large.onnx",
		Files: []FileSpec{
			{URL: diarSegPyannote30URL, RelPath: diarSegPyannote30Rel, SizeBytes: 5_992_913},
			{URL: diarEmbBase + "nemo_en_titanet_large.onnx", RelPath: "titanet_large.onnx", SizeBytes: 101_405_493},
		},
	},

	{
		ID:             "diar-multilingual",
		Family:         FamilyDiarizer,
		DisplayName:    "Multilingual (pyannote-3.0 + CAM++ zh+en)",
		Description:    "3D-Speaker CAM++ zh+en advanced. Sweep DER 0.222. Best multilingual + small (~34 MB).",
		SizeBytes:      34_000_000,
		RAMHintMB:      200,
		DiarSegRelPath: diarSegPyannote30Rel,
		DiarEmbRelPath: "3dspeaker_campplus_zh_en_advanced.onnx",
		Files: []FileSpec{
			{URL: diarSegPyannote30URL, RelPath: diarSegPyannote30Rel, SizeBytes: 5_992_913},
			{URL: diarEmbBase + "3dspeaker_speech_campplus_sv_zh_en_16k-common_advanced.onnx", RelPath: "3dspeaker_campplus_zh_en_advanced.onnx", SizeBytes: 28_281_164},
		},
	},
}

func EngineForActiveASR(activeASR string) (engine, modelID string) {
	if activeASR != "" {
		if e, ok := ByID(activeASR); ok && e.Family == FamilyASR && e.Engine != "" {
			return e.Engine, e.ID
		}
	}
	return shared.EngineWhisper, ""
}

var legacyDiarizerIDs = map[string]string{
	"sherpa-pyannote-segmentation-3.0": "diar-titanet-large",
	"sherpa-titanet-large":             "diar-titanet-large",
	"sherpa-diarizer":                  "diar-titanet-large",
	"diar-mobile-light":                "diar-titanet-large",
	"diar-3dspeaker-v2":                "diar-titanet-large",
	"diar-reverb-v2":                   "diar-titanet-large",
}

var asrEntries = []Entry{
	{
		ID:          "parakeet-tdt-0.6b-v2-int8",
		Family:      FamilyASR,
		Engine:      shared.EngineParakeet,
		Languages:   []string{"en"},
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
		Languages:   []string{"auto", "zh", "en", "ja", "ko", "yue"},
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
