package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	shared "github.com/asolopovas/wt/internal"
	"github.com/asolopovas/wt/internal/diarizer"
	"github.com/asolopovas/wt/internal/transcriber"
	whisper "github.com/ggerganov/whisper.cpp/bindings/go/pkg/whisper"
)

func main() {
	modelSize := flag.String("m", "", "model size (tiny/base/small/medium/turbo)")
	modelPath := flag.String("model", "", "explicit path to model file")
	lang := flag.String("lang", "auto", "language code (auto/en/ru/...)")
	threads := flag.Int("t", max(runtime.NumCPU()-2, 1), "threads")
	diarizeOnly := flag.Bool("diarize-only", false, "run only the diarizer and exit")
	diarize := flag.Bool("diarize", false, "after ASR, run diarizer + BuildTranscript and print utterances")
	speakers := flag.Int("speakers", 0, "force number of speakers (0=auto)")
	engine := flag.String("engine", "whisper", "ASR engine: whisper | whisper-onnx | parakeet | sensevoice | moonshine | zipformer")
	flag.Parse()

	if flag.NArg() < 1 {
		fmt.Fprintf(os.Stderr, "Usage: wt-test [flags] <audio-file>\n")
		flag.PrintDefaults()
		os.Exit(1)
	}

	audioPath := flag.Arg(0)

	fmt.Printf("=== wt test ===\n")
	fmt.Printf("PID: %d  GOARCH: %s  GOOS: %s  NumCPU: %d\n",
		os.Getpid(), runtime.GOARCH, runtime.GOOS, runtime.NumCPU())

	memReport("startup")

	if _, err := os.Stat(audioPath); err != nil {
		fatal("audio file not found: %v", err)
	}

	if *diarizeOnly {
		runDiarizeOnly(audioPath, *speakers)
		return
	}

	switch *engine {
	case shared.EngineParakeet, shared.EngineSenseVoice, shared.EngineMoonshine, shared.EngineZipformer, shared.EngineWhisperONNX:
		runSherpaEngine(*engine, audioPath, *modelSize, *lang, *threads, *diarize, *speakers)
		return
	}

	resolvedModel := resolveModel(*modelSize, *modelPath)
	fmt.Printf("Audio:  %s\n", audioPath)
	fmt.Printf("Model:  %s\n", resolvedModel)
	fmt.Printf("Lang:   %s\n", *lang)
	fmt.Printf("Threads: %d\n", *threads)

	fmt.Printf("\n--- Loading audio ---\n")
	t0 := time.Now()
	samples, err := transcriber.LoadAudioSamples(audioPath)
	if err != nil {
		fatal("loading audio: %v", err)
	}
	audioDur := float64(len(samples)) / float64(transcriber.WhisperSampleRate)
	fmt.Printf("OK: %d samples (%.1fs audio) in %.1fs\n", len(samples), audioDur, since(t0))

	fmt.Printf("\n--- Loading model ---\n")
	whisper.SetLogQuiet(true)
	t0 = time.Now()
	model, err := whisper.New(resolvedModel)
	if err != nil {
		fatal("loading model: %v", err)
	}
	defer func() { _ = model.Close() }()
	fmt.Printf("OK: loaded in %.1fs\n", since(t0))
	memReport("after model load")

	for _, dev := range whisper.BackendDevices() {
		fmt.Printf("Device: %s %s (free=%dMB total=%dMB)\n",
			dev.Type, dev.Description, dev.FreeMB, dev.TotalMB)
	}

	fmt.Printf("\n--- Transcribing ---\n")
	ctx, err := model.NewContext()
	if err != nil {
		fatal("creating context: %v", err)
	}

	transcriber.ConfigureContext(ctx, transcriber.ContextConfig{
		Threads: *threads,
		TDRZ:    true,
	})
	transcriber.SetLanguage(ctx, *lang)

	t0 = time.Now()
	lastPct := -1

	progressCb := func(pct int) {
		pct = min(pct, 100)
		if pct > lastPct && pct%10 == 0 {
			lastPct = pct
			fmt.Printf("  %d%% (%.0fs)\n", pct, since(t0))
		}
	}

	if err := ctx.Process(samples, func() bool { return true }, nil, whisper.ProgressCallback(progressCb)); err != nil {
		fatal("processing: %v", err)
	}

	elapsed := since(t0)
	fmt.Printf("Done in %.1fs (RTF=%.2f)\n", elapsed, audioDur/elapsed)

	if detected := ctx.DetectedLanguage(); detected != "" {
		fmt.Printf("Detected language: %s\n", detected)
	}

	fmt.Printf("\n--- Results ---\n")
	segCount := 0
	speaker := 1
	for {
		seg, err := ctx.NextSegment()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			fmt.Fprintf(os.Stderr, "ERROR reading segment: %v\n", err)
			break
		}
		segCount++
		fmt.Printf("[%s -> %s] SPEAKER_%02d %s\n", seg.Start, seg.End, speaker, seg.Text)
		if seg.SpeakerTurnNext {
			speaker = 3 - speaker
		}
	}

	fmt.Printf("\nSegments: %d\n", segCount)
	memReport("final")
	fmt.Printf("=== DONE ===\n")
}

func runSherpaEngine(engine, audioPath, modelID, lang string, threads int, withDiar bool, speakers int) {
	fmt.Printf("Audio:    %s\n", audioPath)
	fmt.Printf("Engine:   %s (sherpa-onnx)\n", engine)
	if modelID != "" {
		fmt.Printf("ModelID:  %s\n", modelID)
	}
	fmt.Printf("Lang:     %s\n", lang)
	fmt.Printf("Threads:  %d\n", threads)
	if p := os.Getenv("WT_ZIPFORMER_PROVIDER"); p != "" {
		fmt.Printf("Provider: %s (from WT_ZIPFORMER_PROVIDER)\n", p)
	} else {
		fmt.Printf("Provider: cpu (default)\n")
	}
	for _, ev := range []string{"WT_PARAKEET_DIR", "WT_SENSEVOICE_DIR", "WT_ZIPFORMER_DIR", "WT_MOONSHINE_DIR"} {
		if d := os.Getenv(ev); d != "" {
			fmt.Printf("ModelDir: %s (from %s)\n", d, ev)
		}
	}

	fmt.Printf("\n--- Loading audio ---\n")
	t0 := time.Now()
	samples, err := transcriber.LoadAudioSamples(audioPath)
	if err != nil {
		fatal("loading audio: %v", err)
	}
	audioDur := float64(len(samples)) / float64(transcriber.WhisperSampleRate)
	fmt.Printf("OK: %d samples (%.1fs audio) in %.1fs\n", len(samples), audioDur, since(t0))
	memReport("after audio")

	fmt.Printf("\n--- Running %s ---\n", engine)
	hooks := transcriber.Hooks{
		OnPhase:    func(p transcriber.Phase) { fmt.Printf("phase: %s\n", p) },
		OnProgress: func(p transcriber.Progress) { fmt.Printf("  %.0f%% (%s)\n", p.Pct, p.Phase) },
		OnLog:      func(level, msg string) { fmt.Printf("[%s] %s\n", level, msg) },
	}
	spec := transcriber.JobSpec{
		SourcePath: audioPath,
		Engine:     engine,
		ModelSize:  modelID,
		Language:   lang,
		Threads:    threads,
	}
	t0 = time.Now()
	var (
		segs         []diarizer.TranscriptSegment
		detectedLang string
		rtf          float64
	)
	switch engine {
	case shared.EngineParakeet:
		segs, detectedLang, rtf, err = transcriber.RunParakeet(
			context.Background(), spec, samples, audioDur, "", hooks)
	case shared.EngineSenseVoice:
		segs, detectedLang, rtf, err = transcriber.RunSenseVoice(
			context.Background(), spec, samples, audioDur, "", hooks)
	case shared.EngineZipformer:
		segs, detectedLang, rtf, err = transcriber.RunZipformer(
			context.Background(), spec, samples, audioDur, "", hooks)
	case shared.EngineMoonshine:
		segs, detectedLang, rtf, err = transcriber.RunMoonshine(
			context.Background(), spec, samples, audioDur, "", hooks)
	case shared.EngineWhisperONNX:
		segs, detectedLang, rtf, err = transcriber.RunWhisperONNX(
			context.Background(), spec, samples, audioDur, "", hooks)
	default:
		fatal("unknown sherpa sub-engine %q", engine)
	}
	if err != nil {
		fatal("%s: %v", engine, err)
	}
	elapsed := since(t0)
	fmt.Printf("\nDone in %.1fs (RTF=%.2f, lang=%s)\n", elapsed, rtf, detectedLang)

	fmt.Printf("\n--- Segments (%d) ---\n", len(segs))
	for i, s := range segs {
		ntoks := len(s.Tokens)
		fmt.Printf("[%d] %s -> %s tokens=%d: %s\n", i, s.Start, s.End, ntoks, s.Text)
	}

	if withDiar {
		fmt.Printf("\n--- Diarizing ---\n")
		dt0 := time.Now()
		backend, derr := diarizer.New(speakers)
		if derr != nil {
			fatal("init diarizer: %v", derr)
		}
		fmt.Printf("Backend: %s (init %.1fs)\n", backend.Name(), since(dt0))
		wavPath, cleanup, werr := transcriber.WriteTempWAVForTest(samples)
		if werr != nil {
			fatal("writing temp wav: %v", werr)
		}
		defer cleanup()
		dt0 = time.Now()
		diarSegs, derr := backend.Diarize(context.Background(), wavPath, speakers, audioDur, nil)
		if derr != nil {
			fatal("diarize: %v", derr)
		}
		fmt.Printf("Diarized in %.1fs, %d raw segments, %d unique speakers\n",
			since(dt0), len(diarSegs), countUniqueSpeakers(diarSegs))

		transcript := transcriber.BuildTranscript(segs, diarSegs, true, transcriber.TranscriptMeta{
			Language:   detectedLang,
			DurationMs: int64(audioDur * 1000),
			Diarizer:   backend.Name(),
		})
		fmt.Printf("\n--- Utterances (%d) ---\n", len(transcript.Utterances))
		for i, u := range transcript.Utterances {
			fmt.Printf("[%d] %5.2fs->%5.2fs %s: %s\n",
				i, float64(u.Start)/1000, float64(u.End)/1000, u.Speaker, u.Text)
		}
		fmt.Printf("\nSpeakersDetected: %d  Words: %d  Utterances: %d\n",
			transcript.SpeakersDetected, len(transcript.Words), len(transcript.Utterances))
	}

	memReport("final")
	fmt.Printf("=== DONE ===\n")
}

func countUniqueSpeakers(segs []diarizer.Segment) int {
	seen := map[int]struct{}{}
	for _, s := range segs {
		seen[s.Speaker] = struct{}{}
	}
	return len(seen)
}

func runDiarizeOnly(audioPath string, speakers int) {
	if !strings.HasSuffix(strings.ToLower(audioPath), ".wav") {
		fatal("--diarize-only requires a 16kHz mono WAV; got %q", audioPath)
	}
	st, err := os.Stat(audioPath)
	if err != nil {
		fatal("audio: %v", err)
	}
	audioDur := float64(st.Size()) / (16000.0 * 2.0)
	fmt.Printf("Audio: %s (~%.1fs by size)\n", audioPath, audioDur)
	wavPath := audioPath

	fmt.Printf("\n--- Initializing diarizer ---\n")
	t0 := time.Now()
	backend, err := diarizer.New(speakers)
	if err != nil {
		fatal("init diarizer: %v", err)
	}
	fmt.Printf("Backend: %s (init %.1fs)\n", backend.Name(), since(t0))

	fmt.Printf("\n--- Diarizing ---\n")
	t0 = time.Now()
	lastPct := -1.0
	progressCb := func(pct float64) {
		if pct-lastPct >= 10 {
			lastPct = pct
			fmt.Printf("  %.0f%% (%.0fs)\n", pct, since(t0))
		}
	}
	segs, err := backend.Diarize(context.Background(), wavPath, speakers, audioDur, progressCb)
	if err != nil {
		fatal("diarize: %v", err)
	}
	elapsed := since(t0)
	fmt.Printf("Done in %.1fs (RTF=%.2f), %d segments\n", elapsed, elapsed/audioDur, len(segs))

	speakerSet := map[int]int{}
	for _, s := range segs {
		speakerSet[s.Speaker]++
	}
	fmt.Printf("Speakers: %d\n", len(speakerSet))
	for spk, n := range speakerSet {
		fmt.Printf("  speaker_%d: %d segments\n", spk, n)
	}
	for i, s := range segs {
		if i >= 30 {
			fmt.Printf("  ... (%d more)\n", len(segs)-i)
			break
		}
		fmt.Printf("  %.2fs -- %.2fs speaker_%d\n", s.StartSec, s.EndSec, s.Speaker)
	}
	memReport("final")
	fmt.Printf("=== DONE ===\n")
}

func resolveModel(size, path string) string {
	if path != "" || size != "" {
		p, err := transcriber.ResolveModelPathLocal(size, path)
		if err != nil {
			fatal("%v", err)
		}
		return p
	}

	modelsDir := shared.ModelsDir()
	entries, _ := os.ReadDir(modelsDir)
	fmt.Printf("ModelsDir: %s\n", modelsDir)
	for _, e := range entries {
		if info, err := e.Info(); err == nil {
			fmt.Printf("  %s (%d MB)\n", e.Name(), info.Size()/1024/1024)
		}
	}

	for _, candidate := range []string{"ggml-small.bin", "ggml-medium.bin", "ggml-large-v3-turbo.bin"} {
		p := filepath.Join(modelsDir, candidate)
		if _, err := os.Stat(p); err == nil {
			name := strings.TrimPrefix(strings.TrimSuffix(candidate, ".bin"), "ggml-")
			fmt.Printf("Auto-selected: %s\n", name)
			return p
		}
	}

	fatal("no model found in %s — push one with: adb push model.bin /data/local/tmp/", modelsDir)
	return ""
}

func memReport(label string) {
	var ms runtime.MemStats
	runtime.ReadMemStats(&ms)
	fmt.Printf("Mem [%s]: Sys=%dMB Alloc=%dMB\n", label, ms.Sys/1024/1024, ms.Alloc/1024/1024)
}

func since(t time.Time) float64 { return time.Since(t).Seconds() }

func fatal(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "ERROR: "+format+"\n", args...)
	os.Exit(1)
}
