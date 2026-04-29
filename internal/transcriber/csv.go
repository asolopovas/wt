package transcriber

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	shared "github.com/asolopovas/wt/internal"
	"github.com/asolopovas/wt/internal/diarizer"
	"github.com/asolopovas/wt/internal/ui"
	whisper "github.com/ggerganov/whisper.cpp/bindings/go/pkg/whisper"
)

func TranscribeToJSON(model *Model, path, outputFilename, modelSize, language string, threads, speakers int, tdrz, noDiarize bool) error {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return fmt.Errorf("resolving path: %w", err)
	}

	if _, err := os.Stat(absPath); err != nil {
		return fmt.Errorf("file not found: %s", absPath)
	}

	baseDir := filepath.Dir(absPath)
	outputPath := filepath.Join(baseDir, outputFilename)

	samples, err := loadAndReport(absPath)
	if err != nil {
		return err
	}

	audioDurSec := float64(len(samples)) / WhisperSampleRate

	var diarSegs []diarizer.Segment
	diarOK := false
	diarName := ""
	if !noDiarize && diarizer.SupportsExternalBackend() {
		wavPath := ResolveWAVPath(absPath)
		diarSegs, diarName, diarOK = runDiarization(wavPath, speakers, audioDurSec)
	}
	usedTDRZ := UseTDRZ(tdrz, diarOK, noDiarize)

	ctx, err := configureContext(model, threads, language, tdrz, diarOK, noDiarize, diarName)
	if err != nil {
		return err
	}

	transcriptSegs, err := transcribe(ctx, samples)
	if err != nil {
		return err
	}

	if !diarOK && usedTDRZ {
		diarSegs = diarizer.SpeakerTurnSegments(transcriptSegs)
		diarOK = len(diarSegs) > 0
		if diarOK {
			diarName = "tinydiarize"
		}
	}

	detected := ctx.DetectedLanguage()
	if detected == "" {
		detected = language
	}

	device := "cpu"
	for _, dev := range whisper.BackendDevices() {
		if dev.Type == "GPU" || dev.Type == "iGPU" {
			device = dev.Description
			break
		}
	}

	audioDurMs := int64(audioDurSec * 1000)
	transcript := BuildTranscript(transcriptSegs, diarSegs, diarOK, TranscriptMeta{
		Model:      modelSize,
		Language:   detected,
		DurationMs: audioDurMs,
		Diarizer:   diarName,
		Device:     device,
	})

	actualPath, err := WriteJSON(outputPath, transcript)
	if err != nil {
		return err
	}

	ui.Done(fmt.Sprintf("Output: %s (%d segments)", actualPath, len(transcript.Utterances)))
	return nil
}

func loadAndReport(absPath string) ([]float32, error) {
	spinner := ui.Spinner("Loading audio...")
	convertStart := time.Now()
	samples, err := LoadAudioSamples(absPath)
	if err != nil {
		_ = spinner.Stop()
		ui.Cross("Audio loading failed")
		return nil, fmt.Errorf("loading audio: %w", err)
	}
	duration := time.Duration(float64(len(samples)) / WhisperSampleRate * float64(time.Second))
	_ = spinner.Stop()
	ui.Tickf("Audio loaded (%s)", FormatHMS(duration))
	ui.Debug("Conversion time", fmt.Sprintf("%.1fs", time.Since(convertStart).Seconds()))
	return samples, nil
}

func runDiarization(wavPath string, speakers int, audioDurSec float64) ([]diarizer.Segment, string, bool) {
	dia, diarErr := diarizer.New(speakers)
	if diarErr != nil {
		ui.Crossf("Diarization unavailable: %v", diarErr)
		ui.Debug("Diarization", "TDRZ fallback")
		return nil, "", false
	}
	ui.Stage(fmt.Sprintf("Diarizing [%s]...", dia.Name()))

	diarStart := time.Now()
	lastPct := 0.0

	progress := func(pct float64) {
		if pct <= lastPct {
			return
		}
		lastPct = pct
		ui.ProgressLine(int(pct), time.Since(diarStart).Seconds())
	}

	diarSegs, diarErr := dia.Diarize(context.Background(), wavPath, speakers, audioDurSec, progress)
	ui.ClearProgress()

	if diarErr != nil {
		ui.Crossf("Diarization failed: %v", diarErr)
		return nil, "", false
	}

	seen := make(map[int]struct{})
	for _, s := range diarSegs {
		seen[s.Speaker] = struct{}{}
	}
	ui.Tickf("Diarized (%d speakers, %d segments, %.0fs)",
		len(seen), len(diarSegs), time.Since(diarStart).Seconds())
	return diarSegs, dia.Name(), true
}

func configureContext(model *Model, threads int, language string, tdrz, diarOK, noDiarize bool, diarName string) (whisper.Context, error) {
	ctx, err := model.NewContext()
	if err != nil {
		return nil, fmt.Errorf("creating context: %w", err)
	}

	ConfigureContext(ctx, ContextConfig{
		Threads: threads,
		TDRZ:    UseTDRZ(tdrz, diarOK, noDiarize),
	})

	hasVAD := ConfigureVAD(ctx)
	SetLanguage(ctx, language)

	ui.Debug("Language", language)
	ui.Debug("Beam size", "5")
	ui.Debug("Temperature", "0.0 / fallback 0.2")
	if hasVAD {
		ui.Debug("VAD", "Silero v6.2.0")
	}

	if noDiarize {
		ui.Debug("Diarization", "disabled")
	} else if UseTDRZ(tdrz, diarOK, noDiarize) {
		ui.Debug("Diarization", "TDRZ fallback")
	} else {
		ui.Debug("Diarization", diarName)
	}

	return ctx, nil
}

func transcribe(ctx whisper.Context, samples []float32) ([]diarizer.TranscriptSegment, error) {
	ui.Stage("Transcribing...")

	processStart := time.Now()
	lastProgress := -1

	progressCb := func(progress int) {
		if progress > 100 {
			progress = 100
		}
		if progress > lastProgress {
			lastProgress = progress
			ui.ProgressLine(progress, time.Since(processStart).Seconds())
		}
	}

	if err := ctx.Process(samples, nil, nil, whisper.ProgressCallback(progressCb)); err != nil {
		ui.ClearProgress()
		return nil, fmt.Errorf("processing audio: %w", err)
	}

	ui.ClearProgress()

	elapsed := time.Since(processStart).Seconds()
	ui.Tickf("Transcribed (%.0fs)", elapsed)

	if detected := ctx.DetectedLanguage(); detected != "" {
		ui.Debug("Detected language", detected)
	}

	segs := ExtractSegments(ctx)
	return DeduplicateSegments(segs), nil
}

func DeduplicateSegments(segs []diarizer.TranscriptSegment) []diarizer.TranscriptSegment {
	if len(segs) < 2 {
		return segs
	}
	out := make([]diarizer.TranscriptSegment, 0, len(segs))
	out = append(out, segs[0])
	for i := 1; i < len(segs); i++ {
		prev := out[len(out)-1]
		cur := segs[i]
		if strings.TrimSpace(cur.Text) == strings.TrimSpace(prev.Text) {
			out[len(out)-1].End = cur.End
			continue
		}
		out = append(out, cur)
	}
	return out
}

func ResolveWAVPath(absPath string) string {
	cacheFile, err := AudioCacheKey(absPath)
	if err != nil {
		return absPath
	}
	cachePath := filepath.Join(shared.CacheDir(), cacheFile)
	if _, err := os.Stat(cachePath); err == nil {
		return cachePath
	}
	return absPath
}
