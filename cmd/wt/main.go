package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/pterm/pterm"
	"github.com/urfave/cli/v3"

	shared "github.com/asolopovas/wt/internal"
	"github.com/asolopovas/wt/internal/transcriber"
	"github.com/asolopovas/wt/internal/ui"
)

var Version = "dev"

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
		Version:   Version,
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
		},
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
				cmd.Args().Slice(),
			)
		},
	}

	if err := app.Run(context.Background(), os.Args); err != nil {
		ui.Cross(err.Error())
		os.Exit(1)
	}
}

func run(lang, modelSize, modelPath string, threads, speakers int, tdrz, noDiarize, live bool, args []string) error {
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

	ui.Banner(Version, modelSize)

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
		output := transcriber.OutputFilename(filename, modelSize)
		ui.FileHeader(i+1, len(paths), filename)
		if err := transcriber.TranscribeToJSON(model, path, output, modelSize, lang, threads, speakers, tdrz, noDiarize); err != nil {
			ui.Errorf("%v", err)
			errCount++
			continue
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
