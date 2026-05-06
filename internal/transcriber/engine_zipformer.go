package transcriber

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	shared "github.com/asolopovas/wt/internal"
	"github.com/asolopovas/wt/internal/diarizer"
)

func sherpaBinaryName() string {
	switch runtime.GOOS {
	case "windows":
		return "sherpa-onnx-offline.exe"
	case "android":
		return "libsherpa-asr.so"
	default:
		return "sherpa-onnx-offline"
	}
}

func SherpaASRBinaryAvailable() bool {
	_, err := findSherpaASRBinary()
	return err == nil
}

func appRuntimeRoots() []string {
	roots := []string{shared.Dir()}
	if runtime.GOOS == "windows" {
		if v := os.Getenv("LOCALAPPDATA"); v != "" {
			roots = append(roots, filepath.Join(v, "wt"))
		}
	}
	return roots
}

func sherpaInstallDirs() []string {
	var dirs []string
	if exe, err := os.Executable(); err == nil {
		dirs = append(dirs, filepath.Dir(exe))
	}
	dirs = append(dirs, appRuntimeRoots()...)
	return dirs
}

func sherpaCudaRuntimeDirs() []string {
	if runtime.GOOS != "windows" && runtime.GOOS != "linux" {
		return nil
	}
	var dirs []string
	if v := os.Getenv("WT_SHERPA_CUDA_DIR"); v != "" {
		dirs = append(dirs, filepath.Join(v, "bin"), v)
	}
	for _, base := range sherpaInstallDirs() {
		dirs = append(dirs, filepath.Join(base, "sherpa-cuda", "bin"))
	}
	return dirs
}

func findSherpaBinaryIn(dirs []string, name string) string {
	for _, d := range dirs {
		if d == "" {
			continue
		}
		c := filepath.Join(d, name)
		if fileExists(c) {
			return c
		}
	}
	return ""
}

func findSherpaASRBinary() (string, error) {
	name := sherpaBinaryName()

	if sherpaProvider() == "cuda" {
		if p := findSherpaBinaryIn(sherpaCudaRuntimeDirs(), name); p != "" {
			return p, nil
		}
	}

	if p := findSherpaBinaryIn(sherpaInstallDirs(), name); p != "" {
		return p, nil
	}

	if runtime.GOOS == "android" {
		if p := findSherpaBinaryIn(androidNativeLibDirs(), name); p != "" {
			return p, nil
		}
	}

	if p, err := exec.LookPath(name); err == nil {
		return p, nil
	}
	return "", fmt.Errorf("%s not found (build via task android-sherpa-bin or install sherpa-onnx)", name)
}

func fileExists(p string) bool {
	st, err := os.Stat(p)
	return err == nil && !st.IsDir()
}

func androidNativeLibDirs() []string {
	var dirs []string
	if v := os.Getenv("ANDROID_NATIVE_LIBS_DIR"); v != "" {
		dirs = append(dirs, v)
	}
	for _, env := range []string{"LD_LIBRARY_PATH", "LIB_DIR"} {
		for _, p := range strings.Split(os.Getenv(env), ":") {
			if p != "" {
				dirs = append(dirs, p)
			}
		}
	}
	if data, err := os.ReadFile("/proc/self/maps"); err == nil {
		seen := map[string]bool{}
		for _, line := range strings.Split(string(data), "\n") {
			idx := strings.Index(line, "/data/app/")
			if idx < 0 {
				continue
			}
			path := line[idx:]
			if !strings.HasSuffix(path, ".so") {
				continue
			}
			dir := filepath.Dir(path)
			if !seen[dir] {
				seen[dir] = true
				dirs = append(dirs, dir)
			}
		}
	}
	return dirs
}

func sherpaProvider() string {
	if v := os.Getenv("WT_ZIPFORMER_PROVIDER"); v != "" {
		return v
	}
	return "cpu"
}

func cudaProviderLibName() string {
	if runtime.GOOS == "windows" {
		return "onnxruntime_providers_cuda.dll"
	}
	return "libonnxruntime_providers_cuda.so"
}

func SherpaCUDAAvailable() bool {
	lib := cudaProviderLibName()
	for _, d := range sherpaCudaRuntimeDirs() {
		if fileExists(filepath.Join(d, lib)) {
			return true
		}
	}
	for _, d := range sherpaInstallDirs() {
		if fileExists(filepath.Join(d, lib)) {
			return true
		}
	}
	return false
}

func sherpaThreads(spec JobSpec) int {
	if spec.Threads > 0 {
		return spec.Threads
	}
	return 4
}

func WriteTempWAVForTest(samples []float32) (string, func(), error) {
	return writeTempWAV(samples, "wt-pipeline")
}

func writeTempWAV(samples []float32, prefix string) (string, func(), error) {
	tmpDir, err := os.MkdirTemp("", prefix+"-*")
	if err != nil {
		return "", func() {}, fmt.Errorf("tempdir: %w", err)
	}
	wavPath := filepath.Join(tmpDir, "input.wav")
	if err := WritePCM16WAV(wavPath, samples, WhisperSampleRate); err != nil {
		_ = os.RemoveAll(tmpDir)
		return "", func() {}, fmt.Errorf("writing wav: %w", err)
	}
	return wavPath, func() { _ = os.RemoveAll(tmpDir) }, nil
}

func runSherpaCmd(ctx context.Context, bin string, args []string) (string, string, float64, error) {
	start := time.Now()
	cmd := exec.CommandContext(ctx, bin, args...)
	shared.HideWindow(cmd)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		if errors.Is(ctx.Err(), context.Canceled) {
			return "", "", 0, ErrAborted
		}
		return stdout.String(), stderr.String(), 0,
			fmt.Errorf("sherpa subprocess: %w (stderr: %s)",
				err, strings.TrimSpace(stderr.String()))
	}
	return stdout.String(), stderr.String(), time.Since(start).Seconds(), nil
}

func chunkSegmentsFromSherpa(r sherpaResult, chunkDurSec float64) []diarizer.TranscriptSegment {
	if len(r.Tokens) > 0 && len(r.Tokens) == len(r.Timestamps) {
		return coalesceTokens(r.Tokens, r.Timestamps, chunkDurSec)
	}
	text := strings.TrimSpace(r.Text)
	if text == "" {
		return nil
	}
	return []diarizer.TranscriptSegment{{
		Start: 0,
		End:   time.Duration(chunkDurSec * float64(time.Second)),
		Text:  text,
	}}
}

func runSherpaEngineChunked(
	ctx context.Context,
	engineName, bin string,
	argsForWAV func(wavPath string) []string,
	hooks Hooks,
	samples []float32,
	audioDurSec float64,
	rawKey string,
) ([]diarizer.TranscriptSegment, sherpaResult, float64, error) {
	var firstResult sherpaResult
	tempPrefix := "wt-" + engineName

	process := func(ctx context.Context, chunkSamples []float32, chunkDurSec float64) ([]diarizer.TranscriptSegment, error) {
		wavPath, cleanup, werr := writeTempWAV(chunkSamples, tempPrefix)
		if werr != nil {
			return nil, werr
		}
		defer cleanup()

		stdout, stderr, _, runErr := runSherpaCmd(ctx, bin, argsForWAV(wavPath))
		if runErr != nil {
			return nil, runErr
		}
		parsed, perr := parseSherpaJSON(stdout)
		if perr != nil {

			hooks.log("debug", fmt.Sprintf("%s: empty chunk (%v); stderr=%s",
				engineName, perr, truncate(stderr, 120)))
			return nil, nil
		}
		if firstResult.Text == "" && firstResult.Lang == "" {
			firstResult = parsed
		}
		return chunkSegmentsFromSherpa(parsed, chunkDurSec), nil
	}

	segs, rtf, err := runChunked(ctx, engineName, hooks, samples, audioDurSec, rawKey, process)
	return segs, firstResult, rtf, err
}

type sherpaResult struct {
	Text       string    `json:"text"`
	Tokens     []string  `json:"tokens"`
	Timestamps []float64 `json:"timestamps"`

	Lang    string `json:"lang,omitempty"`
	Emotion string `json:"emotion,omitempty"`
	Event   string `json:"event,omitempty"`
}

func parseSherpaJSON(stdout string) (sherpaResult, error) {
	for _, line := range strings.Split(stdout, "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "{") || !strings.Contains(line, "\"text\"") {
			continue
		}
		var r sherpaResult
		if err := json.Unmarshal([]byte(line), &r); err != nil {
			continue
		}
		if strings.TrimSpace(r.Text) == "" {
			return r, fmt.Errorf("empty transcript")
		}
		return r, nil
	}
	return sherpaResult{}, fmt.Errorf("no JSON result line found in subprocess output")
}

func coalesceTokens(tokens []string, timestamps []float64, audioDurSec float64) []diarizer.TranscriptSegment {
	if len(tokens) == 0 {
		return nil
	}
	type word struct {
		text       string
		start, end float64
	}
	words := make([]word, 0, len(tokens)/2+1)
	for i, tok := range tokens {
		if tok == "" {
			continue
		}
		isBoundary := i == 0 || strings.HasPrefix(tok, " ")
		piece := strings.TrimPrefix(tok, " ")
		if isBoundary || len(words) == 0 {
			words = append(words, word{text: piece, start: timestamps[i]})
			continue
		}
		words[len(words)-1].text += piece
	}
	if len(words) == 0 {
		return nil
	}

	for i := range words {
		if i+1 < len(words) {
			words[i].end = words[i+1].start
		} else {
			words[i].end = audioDurSec
		}
	}

	parts := make([]string, len(words))
	toks := make([]diarizer.TokenData, len(words))
	for i, w := range words {
		parts[i] = w.text
		toks[i] = diarizer.TokenData{
			Text:  w.text,
			Start: time.Duration(w.start * float64(time.Second)),
			End:   time.Duration(w.end * float64(time.Second)),
		}
	}
	return []diarizer.TranscriptSegment{{
		Start:  time.Duration(words[0].start * float64(time.Second)),
		End:    time.Duration(words[len(words)-1].end * float64(time.Second)),
		Text:   strings.Join(parts, " "),
		Tokens: toks,
	}}
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}

const zipformerBundleName = "sherpa-onnx-zipformer-en-2023-04-01"

func zipformerModelDir() string {
	if v := os.Getenv("WT_ZIPFORMER_DIR"); v != "" {
		return v
	}
	name := zipformerBundleName
	if v := os.Getenv("WT_ZIPFORMER_BUNDLE"); v != "" {
		name = v
	}
	return filepath.Join(shared.ModelsDir(), "zipformer", name)
}

type zipformerModelPaths struct {
	Encoder, Decoder, Joiner, Tokens string
}

func resolveZipformerModels() (zipformerModelPaths, error) {
	dir := zipformerModelDir()
	p := zipformerModelPaths{
		Encoder: filepath.Join(dir, "encoder-epoch-99-avg-1.int8.onnx"),
		Decoder: filepath.Join(dir, "decoder-epoch-99-avg-1.onnx"),
		Joiner:  filepath.Join(dir, "joiner-epoch-99-avg-1.int8.onnx"),
		Tokens:  filepath.Join(dir, "tokens.txt"),
	}
	missing := []string{}
	for _, f := range []string{p.Encoder, p.Decoder, p.Joiner, p.Tokens} {
		if _, err := os.Stat(f); err != nil {
			missing = append(missing, filepath.Base(f))
		}
	}
	if len(missing) > 0 {
		return p, fmt.Errorf("zipformer models missing in %s: %s", dir, strings.Join(missing, ", "))
	}
	return p, nil
}

func (j *Job) runZipformer(ctx context.Context, spec JobSpec, samples []float32, audioDurSec float64, rawKey string) ([]diarizer.TranscriptSegment, string, float64, error) {
	return RunZipformer(ctx, spec, samples, audioDurSec, rawKey, j.Hooks)
}

func RunZipformer(ctx context.Context, spec JobSpec, samples []float32, audioDurSec float64, rawKey string, hooks Hooks) ([]diarizer.TranscriptSegment, string, float64, error) {
	bin, err := findSherpaASRBinary()
	if err != nil {
		return nil, "", 0, fmt.Errorf("zipformer engine: %w", err)
	}
	models, err := resolveZipformerModels()
	if err != nil {
		return nil, "", 0, fmt.Errorf("zipformer engine: %w", err)
	}
	argsForWAV := func(wavPath string) []string {
		return []string{
			"--tokens=" + models.Tokens,
			"--encoder=" + models.Encoder,
			"--decoder=" + models.Decoder,
			"--joiner=" + models.Joiner,
			fmt.Sprintf("--num-threads=%d", sherpaThreads(spec)),
			"--decoding-method=greedy_search",
			"--provider=" + sherpaProvider(),
			wavPath,
		}
	}
	segs, _, rtf, err := runSherpaEngineChunked(ctx, "zipformer", bin, argsForWAV, hooks, samples, audioDurSec, rawKey)
	if err != nil {
		return nil, "", 0, err
	}
	return segs, "en", rtf, nil
}

const parakeetBundleName = "sherpa-onnx-nemo-parakeet-tdt-0.6b-v3-int8"

func parakeetModelDir() string {
	if v := os.Getenv("WT_PARAKEET_DIR"); v != "" {
		return v
	}
	name := parakeetBundleName
	if v := os.Getenv("WT_PARAKEET_BUNDLE"); v != "" {
		name = v
	}
	return filepath.Join(shared.ModelsDir(), name)
}

func resolveParakeetModels() (zipformerModelPaths, error) {
	dir := parakeetModelDir()
	p := zipformerModelPaths{
		Encoder: filepath.Join(dir, "encoder.int8.onnx"),
		Decoder: filepath.Join(dir, "decoder.int8.onnx"),
		Joiner:  filepath.Join(dir, "joiner.int8.onnx"),
		Tokens:  filepath.Join(dir, "tokens.txt"),
	}
	missing := []string{}
	for _, f := range []string{p.Encoder, p.Decoder, p.Joiner, p.Tokens} {
		if _, err := os.Stat(f); err != nil {
			missing = append(missing, filepath.Base(f))
		}
	}
	if len(missing) > 0 {
		return p, fmt.Errorf("parakeet models missing in %s: %s", dir, strings.Join(missing, ", "))
	}
	return p, nil
}

func (j *Job) runParakeet(ctx context.Context, spec JobSpec, samples []float32, audioDurSec float64, rawKey string) ([]diarizer.TranscriptSegment, string, float64, error) {
	return RunParakeet(ctx, spec, samples, audioDurSec, rawKey, j.Hooks)
}

func RunParakeet(ctx context.Context, spec JobSpec, samples []float32, audioDurSec float64, rawKey string, hooks Hooks) ([]diarizer.TranscriptSegment, string, float64, error) {
	bin, err := findSherpaASRBinary()
	if err != nil {
		return nil, "", 0, fmt.Errorf("parakeet engine: %w", err)
	}
	models, err := resolveParakeetModels()
	if err != nil {
		return nil, "", 0, fmt.Errorf("parakeet engine: %w", err)
	}
	argsForWAV := func(wavPath string) []string {
		return []string{
			"--tokens=" + models.Tokens,
			"--encoder=" + models.Encoder,
			"--decoder=" + models.Decoder,
			"--joiner=" + models.Joiner,
			fmt.Sprintf("--num-threads=%d", sherpaThreads(spec)),
			"--decoding-method=greedy_search",

			"--model-type=nemo_transducer",
			"--provider=" + sherpaProvider(),
			wavPath,
		}
	}
	segs, _, rtf, err := runSherpaEngineChunked(ctx, "parakeet", bin, argsForWAV, hooks, samples, audioDurSec, rawKey)
	if err != nil {
		return nil, "", 0, err
	}
	return segs, "en", rtf, nil
}

type whisperONNXModelPaths struct{ Encoder, Decoder, Tokens string }

type canaryModelPaths struct{ Encoder, Decoder, Tokens string }

func canaryModelDir(modelID string) string {
	if v := os.Getenv("WT_CANARY_DIR"); v != "" {
		return v
	}
	name := modelID
	if name == "" {
		name = "sherpa-onnx-nemo-canary-180m-flash-en-es-de-fr-int8"
	}
	return filepath.Join(shared.ModelsDir(), name)
}

func resolveCanaryModels(modelID string) (canaryModelPaths, error) {
	dir := canaryModelDir(modelID)
	candidates := []canaryModelPaths{
		{Encoder: filepath.Join(dir, "encoder.int8.onnx"), Decoder: filepath.Join(dir, "decoder.int8.onnx"), Tokens: filepath.Join(dir, "tokens.txt")},
		{Encoder: filepath.Join(dir, "encoder.onnx"), Decoder: filepath.Join(dir, "decoder.onnx"), Tokens: filepath.Join(dir, "tokens.txt")},
	}
	for _, p := range candidates {
		if fileExists(p.Encoder) && fileExists(p.Decoder) && fileExists(p.Tokens) {
			return p, nil
		}
	}
	return canaryModelPaths{}, fmt.Errorf("canary models missing in %s", dir)
}

func canaryLang(spec JobSpec) string {
	lang := strings.ToLower(strings.TrimSpace(spec.Language))
	switch lang {
	case "en", "de", "es", "fr":
		return lang
	default:
		return "en"
	}
}

func (j *Job) runCanary(ctx context.Context, spec JobSpec, samples []float32, audioDurSec float64, rawKey string) ([]diarizer.TranscriptSegment, string, float64, error) {
	return RunCanary(ctx, spec, samples, audioDurSec, rawKey, j.Hooks)
}

func RunCanary(ctx context.Context, spec JobSpec, samples []float32, audioDurSec float64, rawKey string, hooks Hooks) ([]diarizer.TranscriptSegment, string, float64, error) {
	bin, err := findSherpaASRBinary()
	if err != nil {
		return nil, "", 0, fmt.Errorf("canary engine: %w", err)
	}
	modelID := spec.ModelSize
	models, err := resolveCanaryModels(modelID)
	if err != nil {
		return nil, "", 0, fmt.Errorf("canary engine: %w", err)
	}
	lang := canaryLang(spec)
	argsForWAV := func(wavPath string) []string {
		return []string{
			"--canary-encoder=" + models.Encoder,
			"--canary-decoder=" + models.Decoder,
			"--tokens=" + models.Tokens,
			"--canary-src-lang=" + lang,
			"--canary-tgt-lang=" + lang,
			"--canary-use-pnc=true",
			fmt.Sprintf("--num-threads=%d", sherpaThreads(spec)),
			"--provider=" + sherpaProvider(),
			wavPath,
		}
	}
	segs, _, rtf, err := runSherpaEngineChunked(ctx, "canary", bin, argsForWAV, hooks, samples, audioDurSec, rawKey)
	if err != nil {
		return nil, "", 0, err
	}
	return segs, lang, rtf, nil
}

type nemoCTCModelPaths struct{ Model, Tokens string }

func nemoCTCModelDir(modelID string) string {
	if v := os.Getenv("WT_NEMO_CTC_DIR"); v != "" {
		return v
	}
	name := modelID
	if name == "" {
		name = "sherpa-onnx-nemo-ctc-giga-am-v3-russian-2025-12-16"
	}
	return filepath.Join(shared.ModelsDir(), name)
}

func resolveNemoCTCModels(modelID string) (nemoCTCModelPaths, error) {
	dir := nemoCTCModelDir(modelID)
	candidates := []nemoCTCModelPaths{
		{Model: filepath.Join(dir, "model.int8.onnx"), Tokens: filepath.Join(dir, "tokens.txt")},
		{Model: filepath.Join(dir, "model.onnx"), Tokens: filepath.Join(dir, "tokens.txt")},
	}
	for _, p := range candidates {
		if fileExists(p.Model) && fileExists(p.Tokens) {
			return p, nil
		}
	}
	return nemoCTCModelPaths{}, fmt.Errorf("nemo-ctc models missing in %s", dir)
}

func (j *Job) runNemoCTC(ctx context.Context, spec JobSpec, samples []float32, audioDurSec float64, rawKey string) ([]diarizer.TranscriptSegment, string, float64, error) {
	return RunNemoCTC(ctx, spec, samples, audioDurSec, rawKey, j.Hooks)
}

func RunNemoCTC(ctx context.Context, spec JobSpec, samples []float32, audioDurSec float64, rawKey string, hooks Hooks) ([]diarizer.TranscriptSegment, string, float64, error) {
	bin, err := findSherpaASRBinary()
	if err != nil {
		return nil, "", 0, fmt.Errorf("nemo-ctc engine: %w", err)
	}
	modelID := spec.ModelSize
	models, err := resolveNemoCTCModels(modelID)
	if err != nil {
		return nil, "", 0, fmt.Errorf("nemo-ctc engine: %w", err)
	}
	argsForWAV := func(wavPath string) []string {
		return []string{
			"--nemo-ctc-model=" + models.Model,
			"--tokens=" + models.Tokens,
			fmt.Sprintf("--num-threads=%d", sherpaThreads(spec)),
			"--provider=" + sherpaProvider(),
			"--model-type=nemo_ctc",
			wavPath,
		}
	}
	segs, _, rtf, err := runSherpaEngineChunked(ctx, "nemo-ctc", bin, argsForWAV, hooks, samples, audioDurSec, rawKey)
	if err != nil {
		return nil, "", 0, err
	}
	lang := strings.ToLower(strings.TrimSpace(spec.Language))
	if lang == "" || lang == "auto" {
		lang = ""
	}
	return segs, lang, rtf, nil
}

func whisperONNXModelDir(modelID string) string {
	if v := os.Getenv("WT_WHISPER_ONNX_DIR"); v != "" {
		return v
	}
	name := modelID
	if name == "" {
		name = "sherpa-whisper-tiny.en"
	}
	return filepath.Join(shared.ModelsDir(), name)
}

func resolveWhisperONNXModels(modelID string) (whisperONNXModelPaths, error) {
	dir := whisperONNXModelDir(modelID)
	prefix := strings.TrimPrefix(modelID, "sherpa-whisper-")
	if prefix == "" || prefix == modelID {
		prefix = "tiny.en"
	}
	candidates := []whisperONNXModelPaths{
		{
			Encoder: filepath.Join(dir, prefix+"-encoder.int8.onnx"),
			Decoder: filepath.Join(dir, prefix+"-decoder.int8.onnx"),
			Tokens:  filepath.Join(dir, prefix+"-tokens.txt"),
		},
		{
			Encoder: filepath.Join(dir, "encoder.int8.onnx"),
			Decoder: filepath.Join(dir, "decoder.int8.onnx"),
			Tokens:  filepath.Join(dir, "tokens.txt"),
		},
	}
	for _, p := range candidates {
		if fileExists(p.Encoder) && fileExists(p.Decoder) && fileExists(p.Tokens) {
			return p, nil
		}
	}
	return whisperONNXModelPaths{}, fmt.Errorf("whisper-onnx models missing in %s (looked for %s-encoder.int8.onnx and encoder.int8.onnx)", dir, prefix)
}

func whisperONNXLanguage(spec JobSpec, modelID string) string {
	if strings.HasSuffix(modelID, ".en") || strings.HasSuffix(modelID, "-en") {
		return "en"
	}
	lang := strings.TrimSpace(strings.ToLower(spec.Language))
	if lang == "" || lang == "auto" {
		return ""
	}
	return lang
}

func (j *Job) runWhisperONNX(ctx context.Context, spec JobSpec, samples []float32, audioDurSec float64, rawKey string) ([]diarizer.TranscriptSegment, string, float64, error) {
	return RunWhisperONNX(ctx, spec, samples, audioDurSec, rawKey, j.Hooks)
}

func RunWhisperONNX(ctx context.Context, spec JobSpec, samples []float32, audioDurSec float64, rawKey string, hooks Hooks) ([]diarizer.TranscriptSegment, string, float64, error) {
	bin, err := findSherpaASRBinary()
	if err != nil {
		return nil, "", 0, fmt.Errorf("whisper-onnx engine: %w", err)
	}
	modelID := spec.ModelSize
	models, err := resolveWhisperONNXModels(modelID)
	if err != nil {
		return nil, "", 0, fmt.Errorf("whisper-onnx engine: %w", err)
	}
	lang := whisperONNXLanguage(spec, modelID)
	argsForWAV := func(wavPath string) []string {
		a := []string{
			"--whisper-encoder=" + models.Encoder,
			"--whisper-decoder=" + models.Decoder,
			"--tokens=" + models.Tokens,
			fmt.Sprintf("--num-threads=%d", sherpaThreads(spec)),
			"--provider=" + sherpaProvider(),
			"--model-type=whisper",
		}
		if lang != "" {
			a = append(a, "--whisper-language="+lang)
		}
		a = append(a, wavPath)
		return a
	}
	segs, _, rtf, err := runSherpaEngineChunked(ctx, "whisper-onnx", bin, argsForWAV, hooks, samples, audioDurSec, rawKey)
	if err != nil {
		return nil, "", 0, err
	}
	detected := lang
	if detected == "" {
		detected = spec.Language
	}
	return segs, detected, rtf, nil
}
