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
	"github.com/asolopovas/wt/internal/transcriber/cache"
)

// Sherpa-onnx-offline backed engines (Zipformer, Moonshine, ...).
//
// These engines all shell out to the same `sherpa-onnx-offline` CLI, which
// the existing android-sherpa-bin Taskfile target already produces. They
// differ only in the model files passed via flags. Shared infrastructure
// (binary discovery, WAV write, JSON parsing, sub-word coalescing) lives in
// this file.

// ---------------------------------------------------------------------------
// Shared infrastructure
// ---------------------------------------------------------------------------

// sherpaBinaryName is the on-disk name of sherpa-onnx-offline per platform.
// On Android we ship it as a `lib*.so` so the Android packager installs it
// under /data/app/<pkg>/lib/arm64/. Same convention as libsherpa-diar.so.
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

// findSherpaASRBinary looks up the sherpa-onnx-offline binary using the same
// strategy as the diarizer's findSherpaBinary.
func findSherpaASRBinary() (string, error) {
	name := sherpaBinaryName()

	if exe, err := os.Executable(); err == nil {
		c := filepath.Join(filepath.Dir(exe), name)
		if fileExists(c) {
			return c, nil
		}
	}

	if runtime.GOOS == "android" {
		for _, dir := range androidNativeLibDirs() {
			c := filepath.Join(dir, name)
			if fileExists(c) {
				return c, nil
			}
		}
	}

	if c := filepath.Join(shared.Dir(), name); fileExists(c) {
		return c, nil
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

// androidNativeLibDirs mirrors internal/diarizer.androidNativeLibDirs.
// On Android the app user can't read /data/app/, so filepath.Glob over
// that tree returns nothing. Instead, read /proc/self/maps which lists
// every .so currently mapped into the process — the dirs containing
// those are the real nativeLibraryDir paths Android resolved at load
// time. This works even with randomised UUID-style /data/app/ subdirs.
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

// sherpaProvider returns the ONNX Runtime provider. WT_ZIPFORMER_PROVIDER is
// the (legacy) env name; applies to all sherpa-backed engines.
//
// The static onnxruntime prebuilt used by our existing android-sherpa-bin
// task only supports {cpu, cuda, coreml}. NNAPI requires a different build
// (BUILD_SHARED_LIBS=ON / AAR-style onnxruntime). Until then, default = cpu.
func sherpaProvider() string {
	if v := os.Getenv("WT_ZIPFORMER_PROVIDER"); v != "" {
		return v
	}
	return "cpu"
}

func sherpaThreads(spec JobSpec) int {
	if spec.Threads > 0 {
		return spec.Threads
	}
	return 4
}

// writeTempWAV writes samples to a fresh temp file and returns
// (wavPath, cleanup, err). cleanup must be deferred by caller.
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

// invokeSherpaCLI runs sherpa-onnx-offline with the given args, returning
// stdout, stderr, elapsed seconds, and error.
func invokeSherpaCLI(ctx context.Context, bin string, args []string, hooks Hooks, engineName string) (string, string, float64, error) {
	hooks.phase(PhaseTranscribing)
	hooks.log("debug", fmt.Sprintf("%s: %s %s", engineName, bin, strings.Join(args, " ")))
	hooks.progress(PhaseTranscribing, 0)

	start := time.Now()
	cmd := exec.CommandContext(ctx, bin, args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		if errors.Is(ctx.Err(), context.Canceled) {
			return "", "", 0, ErrAborted
		}
		return stdout.String(), stderr.String(), 0,
			fmt.Errorf("%s subprocess: %w (stderr: %s)",
				engineName, err, strings.TrimSpace(stderr.String()))
	}
	return stdout.String(), stderr.String(), time.Since(start).Seconds(), nil
}

// sherpaResult mirrors the JSON sherpa-onnx-offline emits per input file.
// Only the fields we consume are listed; sherpa adds more (words, ...)
// that vary by model family.
type sherpaResult struct {
	Text       string    `json:"text"`
	Tokens     []string  `json:"tokens"`
	Timestamps []float64 `json:"timestamps"`
	// Lang/Emotion/Event are populated by SenseVoice (and similar
	// multi-task models). Format is a tag like "<|en|>", "<|HAPPY|>",
	// "<|Speech|>". Empty string for vanilla ASR models.
	Lang    string `json:"lang,omitempty"`
	Emotion string `json:"emotion,omitempty"`
	Event   string `json:"event,omitempty"`
}

// stripSherpaTag unwraps a "<|TAG|>" SenseVoice tag string into its inner
// value (lowercased). Returns empty string for empty/unknown/missing tags.
func stripSherpaTag(tag string) string {
	tag = strings.TrimSpace(tag)
	tag = strings.TrimPrefix(tag, "<|")
	tag = strings.TrimSuffix(tag, "|>")
	tag = strings.ToLower(tag)
	switch tag {
	case "", "emo_unknown", "unknown", "event_unknown":
		return ""
	}
	return tag
}

// parseSherpaJSON scans stdout for the first JSON object line containing a
// "text" field and returns the parsed result. As of sherpa-onnx 1.10+ the
// CLI prints one such JSON object per input WAV.
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

// finalizeSherpaRun parses sherpa output and produces transcript segments.
// If token-level timestamps are present and aligned, sub-word BPE pieces are
// coalesced into word-level segments (a leading space marks a word
// boundary). Otherwise emits a single segment spanning the file.
func finalizeSherpaRun(stdout, stderr string, elapsed, audioDurSec float64, hooks Hooks, engineName string) ([]diarizer.TranscriptSegment, float64, error) {
	parsed, perr := parseSherpaJSON(stdout)
	if perr != nil {
		return nil, 0, fmt.Errorf("%s: %w (stdout: %q, stderr: %q)",
			engineName, perr, truncate(stdout, 200), truncate(stderr, 200))
	}
	hooks.progress(PhaseTranscribing, 100)

	var segs []diarizer.TranscriptSegment
	if len(parsed.Tokens) > 0 && len(parsed.Tokens) == len(parsed.Timestamps) {
		segs = coalesceTokens(parsed.Tokens, parsed.Timestamps, audioDurSec)
	} else {
		segs = []diarizer.TranscriptSegment{{
			Start: 0,
			End:   time.Duration(audioDurSec * float64(time.Second)),
			Text:  strings.TrimSpace(parsed.Text),
		}}
	}

	rtf := 0.0
	if elapsed > 0 {
		rtf = audioDurSec / elapsed
	}
	hooks.log("info", fmt.Sprintf("%s transcribed in %.1fs RTF=%.2f", engineName, elapsed, rtf))
	return segs, rtf, nil
}

// coalesceTokens merges BPE sub-word pieces into word-level segments. Each
// token's leading space (decoded from BPE's '▁') marks a word boundary.
func coalesceTokens(tokens []string, timestamps []float64, audioDurSec float64) []diarizer.TranscriptSegment {
	if len(tokens) == 0 {
		return nil
	}
	type word struct {
		text  string
		start float64
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
	segs := make([]diarizer.TranscriptSegment, 0, len(words))
	for i, w := range words {
		end := audioDurSec
		if i+1 < len(words) {
			end = words[i+1].start
		}
		segs = append(segs, diarizer.TranscriptSegment{
			Start: time.Duration(w.start * float64(time.Second)),
			End:   time.Duration(end * float64(time.Second)),
			Text:  w.text,
		})
	}
	return segs
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}

// ---------------------------------------------------------------------------
// Zipformer engine (transducer)
// ---------------------------------------------------------------------------

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

// RunZipformer is the standalone entrypoint (used by wt-test).
func RunZipformer(ctx context.Context, spec JobSpec, samples []float32, audioDurSec float64, rawKey string, hooks Hooks) ([]diarizer.TranscriptSegment, string, float64, error) {
	bin, err := findSherpaASRBinary()
	if err != nil {
		return nil, "", 0, fmt.Errorf("zipformer engine: %w", err)
	}
	models, err := resolveZipformerModels()
	if err != nil {
		return nil, "", 0, fmt.Errorf("zipformer engine: %w", err)
	}
	wavPath, cleanup, err := writeTempWAV(samples, "wt-zipformer")
	if err != nil {
		return nil, "", 0, fmt.Errorf("zipformer engine: %w", err)
	}
	defer cleanup()

	args := []string{
		"--tokens=" + models.Tokens,
		"--encoder=" + models.Encoder,
		"--decoder=" + models.Decoder,
		"--joiner=" + models.Joiner,
		fmt.Sprintf("--num-threads=%d", sherpaThreads(spec)),
		"--decoding-method=greedy_search",
		"--provider=" + sherpaProvider(),
		wavPath,
	}
	stdout, stderr, elapsed, err := invokeSherpaCLI(ctx, bin, args, hooks, "zipformer")
	if err != nil {
		return nil, "", 0, err
	}
	segs, rtf, err := finalizeSherpaRun(stdout, stderr, elapsed, audioDurSec, hooks, "zipformer")
	if err != nil {
		return nil, "", 0, err
	}
	if rawKey != "" {
		_ = cache.SaveRawSegments(rawKey, segs)
	}
	return segs, "en", rtf, nil
}

// ---------------------------------------------------------------------------
// Parakeet TDT 0.6B v2 engine (NeMo transducer, English, cased+punct)
// ---------------------------------------------------------------------------
//
// Sherpa-onnx treats Parakeet as a regular transducer (--encoder/--decoder/
// --joiner). Only difference vs Zipformer is filenames inside the bundle
// dir. Output is naturally cased+punctuated thanks to the NeMo training
// data, so no post-process is needed.

const parakeetBundleName = "sherpa-onnx-nemo-parakeet-tdt-0.6b-v2-int8"

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

// RunParakeet runs NeMo Parakeet TDT 0.6B v2 via sherpa-onnx-offline.
// Output is naturally cased + punctuated.
func RunParakeet(ctx context.Context, spec JobSpec, samples []float32, audioDurSec float64, rawKey string, hooks Hooks) ([]diarizer.TranscriptSegment, string, float64, error) {
	bin, err := findSherpaASRBinary()
	if err != nil {
		return nil, "", 0, fmt.Errorf("parakeet engine: %w", err)
	}
	models, err := resolveParakeetModels()
	if err != nil {
		return nil, "", 0, fmt.Errorf("parakeet engine: %w", err)
	}
	wavPath, cleanup, err := writeTempWAV(samples, "wt-parakeet")
	if err != nil {
		return nil, "", 0, fmt.Errorf("parakeet engine: %w", err)
	}
	defer cleanup()

	args := []string{
		"--tokens=" + models.Tokens,
		"--encoder=" + models.Encoder,
		"--decoder=" + models.Decoder,
		"--joiner=" + models.Joiner,
		fmt.Sprintf("--num-threads=%d", sherpaThreads(spec)),
		"--decoding-method=greedy_search",
		// Parakeet TDT uses NeMo's transducer variant (Token-and-Duration
		// Transducer); regular --model-type=transducer fails on metadata
		// lookup. nemo_transducer routes to offline-recognizer-transducer-
		// nemo-impl.h which knows about TDT.
		"--model-type=nemo_transducer",
		"--provider=" + sherpaProvider(),
		wavPath,
	}
	stdout, stderr, elapsed, err := invokeSherpaCLI(ctx, bin, args, hooks, "parakeet")
	if err != nil {
		return nil, "", 0, err
	}
	segs, rtf, err := finalizeSherpaRun(stdout, stderr, elapsed, audioDurSec, hooks, "parakeet")
	if err != nil {
		return nil, "", 0, err
	}
	if rawKey != "" {
		_ = cache.SaveRawSegments(rawKey, segs)
	}
	return segs, "en", rtf, nil
}

// ---------------------------------------------------------------------------
// SenseVoice engine (Alibaba multilingual: zh/en/ja/ko/yue, single ONNX file)
// ---------------------------------------------------------------------------
//
// SenseVoice is a single-model ASR (no separate encoder/decoder/joiner).
// Native cased + punctuated output. Language can be auto-detected or forced
// via spec.Language (zh/en/ja/ko/yue). Includes emotion/event tags by
// default which we currently ignore (they're in the JSON `emotion`/`event`
// fields, not `text`).

const senseVoiceBundleName = "sherpa-onnx-sense-voice-zh-en-ja-ko-yue-2024-07-17"

func senseVoiceModelDir() string {
	if v := os.Getenv("WT_SENSEVOICE_DIR"); v != "" {
		return v
	}
	name := senseVoiceBundleName
	if v := os.Getenv("WT_SENSEVOICE_BUNDLE"); v != "" {
		name = v
	}
	return filepath.Join(shared.ModelsDir(), name)
}

type senseVoiceModelPaths struct{ Model, Tokens string }

func resolveSenseVoiceModels() (senseVoiceModelPaths, error) {
	dir := senseVoiceModelDir()
	p := senseVoiceModelPaths{
		Model:  filepath.Join(dir, "model.int8.onnx"),
		Tokens: filepath.Join(dir, "tokens.txt"),
	}
	missing := []string{}
	for _, f := range []string{p.Model, p.Tokens} {
		if _, err := os.Stat(f); err != nil {
			missing = append(missing, filepath.Base(f))
		}
	}
	if len(missing) > 0 {
		return p, fmt.Errorf("sensevoice models missing in %s: %s", dir, strings.Join(missing, ", "))
	}
	return p, nil
}

func (j *Job) runSenseVoice(ctx context.Context, spec JobSpec, samples []float32, audioDurSec float64, rawKey string) ([]diarizer.TranscriptSegment, string, float64, error) {
	return RunSenseVoice(ctx, spec, samples, audioDurSec, rawKey, j.Hooks)
}

// RunSenseVoice runs Alibaba SenseVoice via sherpa-onnx-offline. Output is
// natively cased + punctuated. Set spec.Language to one of
// ""/auto/zh/en/ja/ko/yue.
func RunSenseVoice(ctx context.Context, spec JobSpec, samples []float32, audioDurSec float64, rawKey string, hooks Hooks) ([]diarizer.TranscriptSegment, string, float64, error) {
	bin, err := findSherpaASRBinary()
	if err != nil {
		return nil, "", 0, fmt.Errorf("sensevoice engine: %w", err)
	}
	models, err := resolveSenseVoiceModels()
	if err != nil {
		return nil, "", 0, fmt.Errorf("sensevoice engine: %w", err)
	}
	wavPath, cleanup, err := writeTempWAV(samples, "wt-sensevoice")
	if err != nil {
		return nil, "", 0, fmt.Errorf("sensevoice engine: %w", err)
	}
	defer cleanup()

	lang := strings.ToLower(strings.TrimSpace(spec.Language))
	switch lang {
	case "", "auto", "zh", "en", "ja", "ko", "yue":
		if lang == "" {
			lang = "auto"
		}
	default:
		hooks.log("warn", fmt.Sprintf("sensevoice: unsupported language %q (using auto)", spec.Language))
		lang = "auto"
	}

	args := []string{
		"--tokens=" + models.Tokens,
		"--sense-voice-model=" + models.Model,
		"--sense-voice-language=" + lang,
		"--sense-voice-use-itn=true",
		fmt.Sprintf("--num-threads=%d", sherpaThreads(spec)),
		"--provider=" + sherpaProvider(),
		wavPath,
	}
	stdout, stderr, elapsed, err := invokeSherpaCLI(ctx, bin, args, hooks, "sensevoice")
	if err != nil {
		return nil, "", 0, err
	}
	segs, rtf, err := finalizeSherpaRun(stdout, stderr, elapsed, audioDurSec, hooks, "sensevoice")
	if err != nil {
		return nil, "", 0, err
	}
	if rawKey != "" {
		_ = cache.SaveRawSegments(rawKey, segs)
	}
	detected := lang
	if detected == "auto" {
		detected = ""
	}
	// Surface SenseVoice's detected language tag if we asked for auto and
	// the model returned one. Emotion/event tags are parsed but currently
	// not surfaced in the segment metadata — future enhancement (e.g.
	// per-utterance emotion overlay in the GUI).
	if detected == "" {
		if r, _ := parseSherpaJSON(stdout); r.Lang != "" {
			if tag := stripSherpaTag(r.Lang); tag != "" {
				detected = tag
			}
		}
	}
	return segs, detected, rtf, nil
}

// ---------------------------------------------------------------------------
// Moonshine engine (preprocessor + encoder + cached/uncached decoders)
// ---------------------------------------------------------------------------

const moonshineBundleName = "sherpa-onnx-moonshine-base-en-int8"

func moonshineModelDir() string {
	if v := os.Getenv("WT_MOONSHINE_DIR"); v != "" {
		return v
	}
	name := moonshineBundleName
	if v := os.Getenv("WT_MOONSHINE_BUNDLE"); v != "" {
		name = v
	}
	return filepath.Join(shared.ModelsDir(), "moonshine", name)
}

type moonshineModelPaths struct {
	Preprocessor, Encoder, UncachedDecoder, CachedDecoder, Tokens string
}

func resolveMoonshineModels() (moonshineModelPaths, error) {
	dir := moonshineModelDir()
	p := moonshineModelPaths{
		Preprocessor:    filepath.Join(dir, "preprocess.onnx"),
		Encoder:         filepath.Join(dir, "encode.int8.onnx"),
		UncachedDecoder: filepath.Join(dir, "uncached_decode.int8.onnx"),
		CachedDecoder:   filepath.Join(dir, "cached_decode.int8.onnx"),
		Tokens:          filepath.Join(dir, "tokens.txt"),
	}
	missing := []string{}
	for _, f := range []string{p.Preprocessor, p.Encoder, p.UncachedDecoder, p.CachedDecoder, p.Tokens} {
		if _, err := os.Stat(f); err != nil {
			missing = append(missing, filepath.Base(f))
		}
	}
	if len(missing) > 0 {
		return p, fmt.Errorf("moonshine models missing in %s: %s", dir, strings.Join(missing, ", "))
	}
	return p, nil
}

func (j *Job) runMoonshine(ctx context.Context, spec JobSpec, samples []float32, audioDurSec float64, rawKey string) ([]diarizer.TranscriptSegment, string, float64, error) {
	return RunMoonshine(ctx, spec, samples, audioDurSec, rawKey, j.Hooks)
}

// RunMoonshine is the standalone entrypoint (used by wt-test).
//
// Moonshine outputs cased + punctuated text natively, so no post-process is
// needed for those. It does not output token-level timestamps in our static
// build, so we currently emit a single segment spanning the file (TODO:
// upstream sherpa added per-token timestamps for Moonshine in v1.11+).
func RunMoonshine(ctx context.Context, spec JobSpec, samples []float32, audioDurSec float64, rawKey string, hooks Hooks) ([]diarizer.TranscriptSegment, string, float64, error) {
	bin, err := findSherpaASRBinary()
	if err != nil {
		return nil, "", 0, fmt.Errorf("moonshine engine: %w", err)
	}
	models, err := resolveMoonshineModels()
	if err != nil {
		return nil, "", 0, fmt.Errorf("moonshine engine: %w", err)
	}
	wavPath, cleanup, err := writeTempWAV(samples, "wt-moonshine")
	if err != nil {
		return nil, "", 0, fmt.Errorf("moonshine engine: %w", err)
	}
	defer cleanup()

	args := []string{
		"--moonshine-preprocessor=" + models.Preprocessor,
		"--moonshine-encoder=" + models.Encoder,
		"--moonshine-uncached-decoder=" + models.UncachedDecoder,
		"--moonshine-cached-decoder=" + models.CachedDecoder,
		"--tokens=" + models.Tokens,
		fmt.Sprintf("--num-threads=%d", sherpaThreads(spec)),
		"--provider=" + sherpaProvider(),
		wavPath,
	}
	stdout, stderr, elapsed, err := invokeSherpaCLI(ctx, bin, args, hooks, "moonshine")
	if err != nil {
		return nil, "", 0, err
	}
	segs, rtf, err := finalizeSherpaRun(stdout, stderr, elapsed, audioDurSec, hooks, "moonshine")
	if err != nil {
		return nil, "", 0, err
	}
	if rawKey != "" {
		_ = cache.SaveRawSegments(rawKey, segs)
	}
	return segs, "en", rtf, nil
}
