package models

type Family string

const (
	FamilyWhisper  Family = "whisper"
	FamilyDiarizer Family = "diarizer"
	FamilyLLM      Family = "llm"
)

type Entry struct {
	ID            string
	Family        Family
	DisplayName   string
	URL           string
	RelPath       string
	SizeBytes     int64
	SHA256        string
	RAMHintMB     int
	DefaultActive bool
	Files         []FileSpec
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
	out := make([]Entry, 0, len(whisperEntries)+len(diarizerEntries)+len(llmEntries))
	out = append(out, whisperEntries...)
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

var legacyDiarizerIDs = map[string]string{
	"sherpa-pyannote-segmentation-3.0": "sherpa-diarizer",
	"sherpa-titanet-large":             "sherpa-diarizer",
}

var llmEntries = []Entry{
	{ID: "qwen2.5-0.5b-instruct-q4km", Family: FamilyLLM, DisplayName: "Qwen2.5 0.5B Instruct (Q4_K_M, namer)", URL: "https://huggingface.co/Qwen/Qwen2.5-0.5B-Instruct-GGUF/resolve/main/qwen2.5-0.5b-instruct-q4_k_m.gguf", RelPath: "qwen2.5-0.5b-instruct-q4km.gguf", SizeBytes: 400_000_000, RAMHintMB: 700},
	{ID: "qwen3-1.7b-q4km", Family: FamilyLLM, DisplayName: "Qwen3 1.7B (Q4_K_M, namer)", URL: "https://huggingface.co/Qwen/Qwen3-1.7B-GGUF/resolve/main/Qwen3-1.7B-Q4_K_M.gguf", RelPath: "qwen3-1.7b-q4km.gguf", SizeBytes: 1_100_000_000, RAMHintMB: 1800, DefaultActive: true},
}
