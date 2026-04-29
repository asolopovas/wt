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
	speakers := flag.Int("speakers", 0, "force number of speakers (0=auto)")
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
	if path != "" {
		if _, err := os.Stat(path); err != nil {
			fatal("model not found: %v", err)
		}
		return path
	}

	modelsDir := shared.ModelsDir()

	if size != "" {
		filename, ok := transcriber.ModelFiles[size]
		if !ok {
			fatal("unknown model size %q", size)
		}
		p := filepath.Join(modelsDir, filename)
		if _, err := os.Stat(p); err != nil {
			fatal("model %s not found at %s", size, p)
		}
		return p
	}

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
