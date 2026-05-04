package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/pterm/pterm"
	"github.com/urfave/cli/v3"

	shared "github.com/asolopovas/wt/internal"
	"github.com/asolopovas/wt/internal/appinfo"
	"github.com/asolopovas/wt/internal/models"
	"github.com/asolopovas/wt/internal/transcriber"
	"github.com/asolopovas/wt/internal/transcriber/cache"
	"github.com/asolopovas/wt/internal/ui"
)

var (
	Version   = "dev"
	BuildDate = ""
)

func main() {
	cfg, err := shared.Load()
	if err != nil {
		pterm.Warning.Printf("Config: %v (using defaults)\n", err)
		cfg = shared.Defaults()
	}

	models := transcriber.ValidModelNames()

	app := &cli.Command{
		Name:      "wt",
		Usage:     "Transcribe audio files using sherpa-onnx (whisper, parakeet, moonshine, sensevoice, canary, gigaam)",
		Version:   appinfo.DisplayVersion(Version, BuildDate),
		ArgsUsage: "<audio files...>",
		Description: fmt.Sprintf(
			"Config: %s\nSupported models: %s",
			shared.FilePath(),
			strings.Join(models, ", "),
		),
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:    "lang",
				Aliases: []string{"l"},
				Usage:   "language code (e.g. en, ru); auto-detected if omitted",
				Value:   cfg.Language,
			},
			&cli.StringFlag{
				Name:    "model-size",
				Aliases: []string{"m"},
				Usage:   "model size: tiny, base, small, medium, large, turbo",
				Value:   cfg.Model,
				Validator: func(s string) error {
					if !slices.Contains(models, s) {
						return fmt.Errorf("unknown model %q; valid: %s", s, strings.Join(models, ", "))
					}
					return nil
				},
			},
			&cli.StringFlag{
				Name:  "model",
				Usage: "path to GGML model file; downloads to ~/.wt/models/ if empty",
			},
			&cli.IntFlag{
				Name:    "threads",
				Aliases: []string{"t"},
				Usage:   "number of threads to use",
				Value:   cfg.Threads,
			},
			&cli.IntFlag{
				Name:  "speakers",
				Usage: "number of speakers for diarization (0 = auto-detect)",
				Value: 0,
			},
			&cli.BoolFlag{
				Name:  "no-diarize",
				Usage: "skip speaker diarization entirely",
			},
			&cli.BoolFlag{
				Name:    "verbose",
				Aliases: []string{"V"},
				Usage:   "show detailed debug information",
			},
			&cli.BoolFlag{
				Name:  "live",
				Usage: "enable live microphone transcription",
			},
			&cli.BoolFlag{
				Name:  "no-rename",
				Usage: "skip auto-renaming source + transcript via active LLM",
			},
		},
		Commands: []*cli.Command{modelsCommand()},
		Action: func(_ context.Context, cmd *cli.Command) error {
			ui.Verbose = cmd.Bool("verbose")
			return run(
				cmd.String("lang"),
				cmd.String("model-size"),
				cmd.String("model"),
				cmd.Int("threads"),
				cmd.Int("speakers"),
				cmd.Bool("no-diarize"),
				cmd.Bool("live"),
				cmd.Bool("no-rename"),
				cmd.Args().Slice(),
			)
		},
	}

	if err := app.Run(context.Background(), os.Args); err != nil {
		ui.Cross(err.Error())
		os.Exit(1)
	}
}

func run(lang, modelSize, modelPath string, threads, speakers int, noDiarize, live, noRename bool, args []string) error {
	if live {
		return transcriber.Live(lang, modelSize, modelPath, threads)
	}

	if len(args) == 0 {
		return nil
	}

	paths, err := expandFiles(args)
	if err != nil {
		return err
	}
	if len(paths) == 0 {
		return fmt.Errorf("no valid audio files found")
	}

	runStart := time.Now()
	shared.LogProcessStart(fmt.Sprintf("transcription (%d file(s))", len(paths)))
	shared.LogInfo(fmt.Sprintf("log: %s", shared.LogFilePath()))
	shared.LogInfo(fmt.Sprintf("args: model-size=%s model=%q lang=%q threads=%d speakers=%d no-diarize=%v no-rename=%v verbose=%v",
		modelSize, modelPath, lang, threads, speakers, noDiarize, noRename, ui.Verbose))
	for i, p := range paths {
		shared.LogInfo(fmt.Sprintf("input[%d/%d]: %s", i+1, len(paths), p))
	}

	ui.Banner(appinfo.DisplayVersion(Version, BuildDate), modelSize)

	if resolvedModel, mErr := transcriber.ResolveModelPathLocal(modelSize, modelPath); mErr == nil {
		shared.LogInfo("model dir: " + resolvedModel)
	} else {
		shared.LogDebug("model not yet resolved locally: " + mErr.Error())
	}

	ui.Debug("Files", fmt.Sprintf("%d", len(paths)))

	errCount := 0
	for i, path := range paths {
		filename := filepath.Base(path)
		ui.FileHeader(i+1, len(paths), filename)
		jobStart := time.Now()
		absPath, jsonPath, err := runJob(path, modelSize, lang, threads, speakers, noDiarize)
		if err != nil {
			shared.LogError(fmt.Sprintf("file failed: %s — %v (after %.1fs)", filename, err, time.Since(jobStart).Seconds()))
			ui.Errorf("%v", err)
			errCount++
			continue
		}
		shared.LogInfo(fmt.Sprintf("file ok: src=%s out=%s (%.1fs)", absPath, jsonPath, time.Since(jobStart).Seconds()))
		if !noRename {
			autoRename(absPath, jsonPath)
		}
	}

	outcome := "ok"
	if errCount > 0 {
		outcome = "failed"
	}
	shared.LogProcessEnd("transcription", outcome,
		fmt.Sprintf("%d/%d ok, %d failed in %.1fs", len(paths)-errCount, len(paths), errCount, time.Since(runStart).Seconds()))

	if errCount > 0 {
		msg := fmt.Sprintf("Done: %d/%d transcribed, %d failed.", len(paths)-errCount, len(paths), errCount)
		pterm.Warning.Println("\n  " + msg)
		shared.LogWarn(msg)
	} else if len(paths) > 1 {
		ui.Done(fmt.Sprintf("All %d files transcribed.", len(paths)))
	}
	return nil
}

func expandFiles(patterns []string) ([]string, error) {
	var paths []string
	for _, pattern := range patterns {
		matches, err := filepath.Glob(pattern)
		if err != nil {
			return nil, fmt.Errorf("invalid glob pattern %q: %w", pattern, err)
		}
		if len(matches) > 0 {
			paths = append(paths, matches...)
		} else if info, err := os.Stat(pattern); err == nil && !info.IsDir() {
			paths = append(paths, pattern)
		} else {
			ui.Warn(fmt.Sprintf("'%s' not found, skipping.", pattern))
		}
	}

	for i, p := range paths {
		abs, err := filepath.Abs(p)
		if err != nil {
			return nil, err
		}
		paths[i] = abs
	}
	return paths, nil
}

func runJob(path, modelSize, lang string, threads, speakers int, noDiarize bool) (string, string, error) {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return "", "", fmt.Errorf("resolving path: %w", err)
	}

	loadStart := time.Now()
	var loadSpinner *pterm.SpinnerPrinter
	var diarStart, transcribeStart time.Time
	lastPct := -1
	transcribeDone := false
	diarDone := false

	hooks := transcriber.Hooks{
		OnPhase: func(p transcriber.Phase) {
			switch p {
			case transcriber.PhaseLoadingAudio:
				loadStart = time.Now()
				loadSpinner = ui.Spinner("Loading audio...")
			case transcriber.PhaseTranscribing:
				if loadSpinner != nil {
					_ = loadSpinner.Stop()
					ui.Tickf("Audio loaded (%.1fs)", time.Since(loadStart).Seconds())
					loadSpinner = nil
				}
				ui.Stage("Transcribing...")
				transcribeStart = time.Now()
				lastPct = -1
			case transcriber.PhaseDiarizing:
				if !transcribeDone && !transcribeStart.IsZero() {
					ui.ClearProgress()
					ui.Tickf("Transcribed (%.0fs)", time.Since(transcribeStart).Seconds())
					transcribeDone = true
				}
				ui.Stage("Diarizing...")
				diarStart = time.Now()
				lastPct = -1
			case transcriber.PhaseWriting:
				if !transcribeDone && !transcribeStart.IsZero() {
					ui.ClearProgress()
					ui.Tickf("Transcribed (%.0fs)", time.Since(transcribeStart).Seconds())
					transcribeDone = true
				}
				if !diarDone && !diarStart.IsZero() {
					ui.ClearProgress()
					ui.Tickf("Diarized (%.0fs)", time.Since(diarStart).Seconds())
					diarDone = true
				}
			}
		},
		OnProgress: func(p transcriber.Progress) {
			pct := int(p.Pct + 0.5)
			if pct == lastPct {
				return
			}
			lastPct = pct
			start := transcribeStart
			if p.Phase == transcriber.PhaseDiarizing {
				start = diarStart
			}
			if start.IsZero() {
				start = time.Now()
			}
			ui.ProgressLine(pct, time.Since(start).Seconds())
		},
		OnLog: func(level, msg string) {
			switch level {
			case "warn":
				ui.Warn(msg)
			case "info":
				ui.Tick(msg)
			default:
				if ui.Verbose {
					ui.Debug("", msg)
				}
			}
		},
	}

	engine := shared.EngineWhisperONNX
	if mgr := models.NewManager(); mgr != nil {
		if eng, _ := models.EngineForActiveASR(mgr.Active(models.FamilyASR)); eng != "" {
			engine = eng
		}
	}

	job := &transcriber.Job{Hooks: hooks}
	spec := transcriber.JobSpec{
		SourcePath: absPath,
		ModelSize:  modelSize,
		Language:   lang,
		Engine:     engine,
		Threads:    threads,
		Speakers:   speakers,
		NoDiarize:  noDiarize,
	}

	res, err := job.Run(context.Background(), spec)
	if loadSpinner != nil {
		_ = loadSpinner.Stop()
	}
	if err != nil {
		return absPath, "", err
	}

	dest := filepath.Join(filepath.Dir(absPath), transcriber.OutputFilename(filepath.Base(absPath), modelSize))
	if !res.Cached {
		if err := cache.Export(res.CacheKey, dest); err != nil {
			return absPath, "", fmt.Errorf("exporting transcript: %w", err)
		}
		ui.Done(fmt.Sprintf("Output: %s (%d segments)", dest, len(res.Transcript.Utterances)))
	} else {
		if err := cache.Export(res.CacheKey, dest); err != nil {
			return absPath, "", fmt.Errorf("exporting cached transcript: %w", err)
		}
		ui.Tickf("Cached: %s", dest)
	}
	return absPath, dest, nil
}
