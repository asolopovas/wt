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
	whisper "github.com/ggerganov/whisper.cpp/bindings/go/pkg/whisper"
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
		Usage:     "Transcribe audio files using whisper.cpp",
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
				Name:  "tdrz",
				Usage: "enable tinydiarize speaker turn detection",
				Value: cfg.TDRZ,
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
				cmd.Bool("tdrz"),
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

func run(lang, modelSize, modelPath string, threads, speakers int, tdrz, noDiarize, live, noRename bool, args []string) error {
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

	ui.Banner(appinfo.DisplayVersion(Version, BuildDate), modelSize)

	model, err := transcriber.LoadModel(modelSize, modelPath, threads)
	if err != nil {
		return fmt.Errorf("loading model: %w", err)
	}
	defer func() {
		_ = model.Close()
	}()

	ui.Debug("Files", fmt.Sprintf("%d", len(paths)))

	errCount := 0
	for i, path := range paths {
		filename := filepath.Base(path)
		ui.FileHeader(i+1, len(paths), filename)
		absPath, jsonPath, err := runJob(model.Model, path, modelSize, lang, threads, speakers, tdrz, noDiarize)
		if err != nil {
			ui.Errorf("%v", err)
			errCount++
			continue
		}
		if !noRename {
			autoRename(absPath, jsonPath)
		}
	}

	if errCount > 0 {
		pterm.Warning.Printf("\n  Done: %d/%d transcribed, %d failed.\n", len(paths)-errCount, len(paths), errCount)
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
			pterm.Warning.Printf("'%s' not found, skipping.\n", pattern)
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

func runJob(model whisper.Model, path, modelSize, lang string, threads, speakers int, tdrz, noDiarize bool) (string, string, error) {
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

	engine := shared.EngineWhisper
	if mgr := models.NewManager(); mgr != nil {
		if eng, _ := models.EngineForActiveASR(mgr.Active(models.FamilyASR)); eng != "" {
			engine = eng
		}
	}

	job := &transcriber.Job{Model: model, Hooks: hooks}
	spec := transcriber.JobSpec{
		SourcePath: absPath,
		ModelSize:  modelSize,
		Language:   lang,
		Engine:     engine,
		Threads:    threads,
		Speakers:   speakers,
		NoDiarize:  noDiarize,
		TDRZ:       tdrz,
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
