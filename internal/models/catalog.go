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
	{ID: "sherpa-pyannote-segmentation-3.0", Family: FamilyDiarizer, DisplayName: "Sherpa pyannote-3.0 segmentation", URL: "https://huggingface.co/csukuangfj/sherpa-onnx-pyannote-segmentation-3-0/resolve/main/model.onnx", RelPath: "sherpa-onnx-pyannote-segmentation-3-0/model.onnx", SizeBytes: 6_000_000, RAMHintMB: 100, DefaultActive: true},
	{ID: "sherpa-titanet-large", Family: FamilyDiarizer, DisplayName: "NeMo TitaNet-Large embeddings", URL: "https://github.com/k2-fsa/sherpa-onnx/releases/download/speaker-recongition-models/nemo_en_titanet_large.onnx", RelPath: "titanet_large.onnx", SizeBytes: 96_000_000, RAMHintMB: 250, DefaultActive: true},
}

var llmEntries = []Entry{
	{ID: "qwen3-4b-instruct-q4km", Family: FamilyLLM, DisplayName: "Qwen3 4B Instruct (Q4_K_M)", URL: "https://huggingface.co/bartowski/Qwen_Qwen3-4B-Instruct-2507-GGUF/resolve/main/Qwen_Qwen3-4B-Instruct-2507-Q4_K_M.gguf", RelPath: "qwen3-4b-instruct-q4km.gguf", SizeBytes: 2_500_000_000, RAMHintMB: 3500, DefaultActive: true},
	{ID: "gemma-3-4b-it-q4km", Family: FamilyLLM, DisplayName: "Gemma 3 4B IT (Q4_K_M)", URL: "https://huggingface.co/lmstudio-community/gemma-3-4b-it-GGUF/resolve/main/gemma-3-4b-it-Q4_K_M.gguf", RelPath: "gemma-3-4b-it-q4km.gguf", SizeBytes: 2_500_000_000, RAMHintMB: 3600},
	{ID: "gemma-3n-e2b-it-q4km", Family: FamilyLLM, DisplayName: "Gemma 3n E2B IT (Q4_K_M)", URL: "https://huggingface.co/lmstudio-community/gemma-3n-E2B-it-GGUF/resolve/main/gemma-3n-E2B-it-Q4_K_M.gguf", RelPath: "gemma-3n-e2b-it-q4km.gguf", SizeBytes: 1_550_000_000, RAMHintMB: 2200},
	{ID: "phi-4-mini-instruct-q4km", Family: FamilyLLM, DisplayName: "Phi-4-mini Instruct (Q4_K_M)", URL: "https://huggingface.co/bartowski/microsoft_Phi-4-mini-instruct-GGUF/resolve/main/microsoft_Phi-4-mini-instruct-Q4_K_M.gguf", RelPath: "phi-4-mini-q4km.gguf", SizeBytes: 2_350_000_000, RAMHintMB: 3300},
	{ID: "smollm3-3b-q4km", Family: FamilyLLM, DisplayName: "SmolLM3 3B (Q4_K_M)", URL: "https://huggingface.co/bartowski/HuggingFaceTB_SmolLM3-3B-GGUF/resolve/main/HuggingFaceTB_SmolLM3-3B-Q4_K_M.gguf", RelPath: "smollm3-3b-q4km.gguf", SizeBytes: 1_900_000_000, RAMHintMB: 2700},
	{ID: "llama-3.2-3b-instruct-q4km", Family: FamilyLLM, DisplayName: "Llama 3.2 3B Instruct (Q4_K_M)", URL: "https://huggingface.co/bartowski/Llama-3.2-3B-Instruct-GGUF/resolve/main/Llama-3.2-3B-Instruct-Q4_K_M.gguf", RelPath: "llama-3.2-3b-instruct-q4km.gguf", SizeBytes: 2_020_000_000, RAMHintMB: 2900},
	{ID: "qwen3-1.7b-q4km", Family: FamilyLLM, DisplayName: "Qwen3 1.7B (Q4_K_M, fast pick)", URL: "https://huggingface.co/bartowski/Qwen_Qwen3-1.7B-GGUF/resolve/main/Qwen_Qwen3-1.7B-Q4_K_M.gguf", RelPath: "qwen3-1.7b-q4km.gguf", SizeBytes: 1_080_000_000, RAMHintMB: 1500},
}
