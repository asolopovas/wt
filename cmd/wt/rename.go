package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/urfave/cli/v3"

	"github.com/asolopovas/wt/internal/namer"
)

func renameCommand() *cli.Command {
	return &cli.Command{
		Name:      "rename",
		Usage:     "Rename a transcript file to YYMMDD-HHMMSS_topic using the active LLM",
		ArgsUsage: "<transcript-file>",
		Flags: []cli.Flag{
			&cli.BoolFlag{
				Name:  "dry-run",
				Usage: "print suggestion without renaming",
			},
		},
		Action: renameAction,
	}
}

func renameAction(ctx context.Context, c *cli.Command) error {
	src := c.Args().First()
	if src == "" {
		return fmt.Errorf("missing path argument")
	}
	abs, err := filepath.Abs(src)
	if err != nil {
		return err
	}
	st, err := os.Stat(abs)
	if err != nil {
		return err
	}
	if st.IsDir() {
		return fmt.Errorf("not a file: %s", abs)
	}

	text, err := namer.ExtractTranscriptText(abs)
	if err != nil {
		return fmt.Errorf("reading transcript: %w", err)
	}

	fallback := st.ModTime()
	if fallback.IsZero() {
		fallback = time.Now()
	}

	rctx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()

	s, err := namer.Suggest(rctx, text, fallback)
	if err != nil {
		return err
	}

	newName := s.Filename(filepath.Ext(abs))
	dst := filepath.Join(filepath.Dir(abs), newName)

	if c.Bool("dry-run") {
		fmt.Println(newName)
		return nil
	}

	if abs == dst {
		fmt.Println("already named:", newName)
		return nil
	}
	if err := os.Rename(abs, dst); err != nil {
		return err
	}
	fmt.Println(dst)
	return nil
}
