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

	// Diarizer-only: which file inside Files is the segmentation model
	// (pyannote-style ONNX) and which is the speaker embedding model.
	// internal/diarizer/sherpa.go uses these to build the CLI args.
	DiarSegRelPath string
	DiarEmbRelPath string

	// Languages the model can transcribe. Empty == "all / multilingual"
	// (whisper-style). Drives the LANGUAGE dropdown filtering on the
	// Transcode tab — picking Parakeet (en-only) collapses the dropdown
	// to ["en"]; picking Whisper restores the full 99-lang list.
	Languages []string
}

// LanguagesFor returns the language whitelist for the given model entry
// ID. Empty result means "unrestricted / multilingual" (whisper) and the
// caller should show the full languages list.
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
	{ID: "whisper-base", Family: FamilyWhisper, DisplayName: "Whisper base (multilingual)", URL: "https://huggingface.co/ggerganov/whisper.cpp/resolve/main/ggml-base.bin", RelPath: "ggml-base.bin", SizeBytes: 147_000_000, RAMHintMB: 300},
	{ID: "whisper-small", Family: FamilyWhisper, DisplayName: "Whisper small (multilingual)", URL: "https://huggingface.co/ggerganov/whisper.cpp/resolve/main/ggml-small.bin", RelPath: "ggml-small.bin", SizeBytes: 488_000_000, RAMHintMB: 700},
	{ID: "whisper-medium", Family: FamilyWhisper, DisplayName: "Whisper medium (multilingual)", URL: "https://huggingface.co/ggerganov/whisper.cpp/resolve/main/ggml-medium.bin", RelPath: "ggml-medium.bin", SizeBytes: 1_530_000_000, RAMHintMB: 2200},
	{ID: "whisper-large-v3", Family: FamilyWhisper, DisplayName: "Whisper large-v3 (multilingual)", URL: "https://huggingface.co/ggerganov/whisper.cpp/resolve/main/ggml-large-v3.bin", RelPath: "ggml-large-v3.bin", SizeBytes: 3_100_000_000, RAMHintMB: 4400},
	{ID: "whisper-turbo", Family: FamilyWhisper, DisplayName: "Whisper large-v3-turbo", URL: "https://huggingface.co/ggerganov/whisper.cpp/resolve/main/ggml-large-v3-turbo.bin", RelPath: "ggml-large-v3-turbo.bin", SizeBytes: 1_620_000_000, RAMHintMB: 2400, DefaultActive: true},
	{ID: "whisper-vad-silero", Family: FamilyWhisper, DisplayName: "Silero VAD v6.2.0", URL: "https://huggingface.co/ggml-org/whisper-vad/resolve/main/ggml-silero-v6.2.0.bin", RelPath: "ggml-silero-v6.2.0.bin", SizeBytes: 2_300_000, RAMHintMB: 50},
}

// Diarizer presets: each entry pairs a pyannote-style segmentation model
// with a speaker embedding model. Multiple entries may share the same
// segmentation or embedding file on disk; manager.Delete preserves shared
// files (don't orphan another installed entry).
//
// Embedding rankings come from scripts/diar_sweep_results.csv (3 fixtures,
// 72 configs each):
//
//	titanet_large    DER 0.190  (winner)
//	titanet_small    DER 0.191  (statistical tie at 1/2.5 the size!)
//	eres2net_en      DER 0.211
//	campplus_zh_en   DER 0.222  (multilingual leader)
//
var (
	diarSegPyannote30URL = "https://huggingface.co/csukuangfj/sherpa-onnx-pyannote-segmentation-3-0/resolve/main/model.onnx"
	diarSegPyannote30Rel = "sherpa-onnx-pyannote-segmentation-3-0/model.onnx"

	diarSegReverbV1URL = "https://huggingface.co/csukuangfj/sherpa-onnx-reverb-diarization-v1/resolve/main/model.onnx"
	diarSegReverbV1Rel = "sherpa-onnx-reverb-diarization-v1/model.onnx"

	diarSegReverbV2URL = "https://huggingface.co/csukuangfj/sherpa-onnx-reverb-diarization-v2/resolve/main/model.onnx"
	diarSegReverbV2Rel = "sherpa-onnx-reverb-diarization-v2/model.onnx"

	diarEmbBase = "https://github.com/k2-fsa/sherpa-onnx/releases/download/speaker-recongition-models/"
)

var diarizerEntries = []Entry{
	// 1. Default — best DER in the sweep, English-tuned.
	{
		ID:            "diar-titanet-large",
		Family:        FamilyDiarizer,
		DisplayName:   "Standard (pyannote-3.0 + TitaNet-Large)",
		Description:   "Best DER in our sweep (0.190). Recommended default for English. ~107 MB.",
		SizeBytes:     107_000_000,
		RAMHintMB:     350,
		DefaultActive: true,
		DiarSegRelPath: diarSegPyannote30Rel,
		DiarEmbRelPath: "titanet_large.onnx",
		Files: []FileSpec{
			{URL: diarSegPyannote30URL, RelPath: diarSegPyannote30Rel, SizeBytes: 5_992_913},
			{URL: diarEmbBase + "nemo_en_titanet_large.onnx", RelPath: "titanet_large.onnx", SizeBytes: 101_405_493},
		},
	},
	// 2. Quality-first — Rev.ai's 2024 release, conversational/telephone SOTA.
	{
		ID:            "diar-reverb-v2",
		Family:        FamilyDiarizer,
		DisplayName:   "High-quality (Reverb-v2 + TitaNet-Large)",
		Description:   "Rev.ai 2024 release. Best quality on conversational/telephone audio. Heavier (~494 MB).",
		SizeBytes:     494_000_000,
		RAMHintMB:     900,
		DiarSegRelPath: diarSegReverbV2Rel,
		DiarEmbRelPath: "titanet_large.onnx",
		Files: []FileSpec{
			{URL: diarSegReverbV2URL, RelPath: diarSegReverbV2Rel, SizeBytes: 392_921_403},
			{URL: diarEmbBase + "nemo_en_titanet_large.onnx", RelPath: "titanet_large.onnx", SizeBytes: 101_405_493},
		},
	},
	// 3. Multilingual — zh+en, very compact.
	{
		ID:            "diar-multilingual",
		Family:        FamilyDiarizer,
		DisplayName:   "Multilingual (pyannote-3.0 + CAM++ zh+en)",
		Description:   "3D-Speaker CAM++ zh+en advanced. Sweep DER 0.222. Best multilingual + small (~34 MB).",
		SizeBytes:     34_000_000,
		RAMHintMB:     200,
		DiarSegRelPath: diarSegPyannote30Rel,
		DiarEmbRelPath: "3dspeaker_campplus_zh_en_advanced.onnx",
		Files: []FileSpec{
			{URL: diarSegPyannote30URL, RelPath: diarSegPyannote30Rel, SizeBytes: 5_992_913},
			{URL: diarEmbBase + "3dspeaker_speech_campplus_sv_zh_en_16k-common_advanced.onnx", RelPath: "3dspeaker_campplus_zh_en_advanced.onnx", SizeBytes: 28_281_164},
		},
	},
	// 4. Mobile-light — TitaNet-Small ties Large in our sweep at 1/2.5 the size.
	{
		ID:            "diar-mobile-light",
		Family:        FamilyDiarizer,
		DisplayName:   "Mobile-light (pyannote-3.0 + TitaNet-Small)",
		Description:   "TitaNet-Small. Sweep DER 0.191 (statistically tied with Large) at 1/2.5 the size. ~46 MB total.",
		SizeBytes:     46_000_000,
		RAMHintMB:     200,
		DiarSegRelPath: diarSegPyannote30Rel,
		DiarEmbRelPath: "titanet_small.onnx",
		Files: []FileSpec{
			{URL: diarSegPyannote30URL, RelPath: diarSegPyannote30Rel, SizeBytes: 5_992_913},
			{URL: diarEmbBase + "nemo_en_titanet_small.onnx", RelPath: "titanet_small.onnx", SizeBytes: 40_257_283},
		},
	},
	// 5. Newest architecture — ERes2NetV2 (3D-Speaker 2024) + Reverb-v1 segmenter.
	{
		ID:            "diar-3dspeaker-v2",
		Family:        FamilyDiarizer,
		DisplayName:   "3D-Speaker SOTA (Reverb-v1 + ERes2NetV2)",
		Description:   "Newest 2024 architectures on both ends. Untested in our sweep but theoretically strongest. ~81 MB.",
		SizeBytes:     81_000_000,
		RAMHintMB:     400,
		DiarSegRelPath: diarSegReverbV1Rel,
		DiarEmbRelPath: "3dspeaker_eres2netv2.onnx",
		Files: []FileSpec{
			{URL: diarSegReverbV1URL, RelPath: diarSegReverbV1Rel, SizeBytes: 9_512_223},
			{URL: diarEmbBase + "3dspeaker_speech_eres2netv2_sv_zh-cn_16k-common.onnx", RelPath: "3dspeaker_eres2netv2.onnx", SizeBytes: 71_441_526},
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
	"sherpa-pyannote-segmentation-3.0": "diar-titanet-large",
	"sherpa-titanet-large":             "diar-titanet-large",
	"sherpa-diarizer":                  "diar-titanet-large",
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
