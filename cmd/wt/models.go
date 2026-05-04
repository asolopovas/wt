package main

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/pterm/pterm"
	"github.com/urfave/cli/v3"

	"github.com/asolopovas/wt/internal/models"
)

func modelsCommand() *cli.Command {
	return &cli.Command{
		Name:  "models",
		Usage: "Manage downloadable models (whisper, diarizer, LLM)",
		Commands: []*cli.Command{
			{
				Name:   "list",
				Usage:  "List all catalog entries with status",
				Action: modelsList,
			},
			{
				Name:      "get",
				Usage:     "Download a model by ID",
				ArgsUsage: "<id>",
				Action:    modelsGet,
			},
			{
				Name:      "delete",
				Usage:     "Delete an installed model by ID",
				ArgsUsage: "<id>",
				Action:    modelsDelete,
			},
			{
				Name:      "set-active",
				Usage:     "Set the active model for its family",
				ArgsUsage: "<id>",
				Action:    modelsSetActive,
			},
			{
				Name:      "path",
				Usage:     "Print on-disk path for a model id",
				ArgsUsage: "<id>",
				Action:    modelsPath,
			},
		},
	}
}

func modelsList(_ context.Context, _ *cli.Command) error {
	mgr := models.NewManager()
	for _, fam := range []models.Family{models.FamilyASR, models.FamilyDiarizer, models.FamilyLLM} {
		pterm.DefaultBasicText.Printf("\n  [%s]\n", strings.ToUpper(string(fam)))
		for _, e := range models.ByFamily(fam) {
			st := mgr.Status(e.ID)
			marker := " "
			if mgr.Active(e.Family) == e.ID {
				marker = "*"
			}
			fmt.Printf("  %s %-32s  %-13s  %8s  %s\n", marker, e.ID, st, humanSize(e.SizeBytes), e.DisplayName)
		}
	}
	return nil
}

func mustEntry(c *cli.Command) (models.Entry, error) {
	id := c.Args().First()
	if id == "" {
		return models.Entry{}, fmt.Errorf("missing model id")
	}
	e, ok := models.ByID(id)
	if !ok {
		return models.Entry{}, fmt.Errorf("unknown model: %s", id)
	}
	return e, nil
}

func modelsGet(ctx context.Context, c *cli.Command) error {
	e, err := mustEntry(c)
	if err != nil {
		return err
	}
	mgr := models.NewManager()
	pb, _ := pterm.DefaultProgressbar.WithTitle("Downloading " + e.ID).Start()
	last := 0
	err = mgr.Get(ctx, e.ID, func(p models.Progress) {
		if p.Total <= 0 {
			return
		}
		mb := int(p.Downloaded / (1024 * 1024))
		totalMB := int(p.Total / (1024 * 1024))
		if pb.Total != totalMB {
			pb = pb.WithTotal(totalMB)
		}
		if mb > last {
			pb.Add(mb - last)
			last = mb
		}
	})
	_, _ = pb.Stop()
	if err != nil {
		return err
	}
	fmt.Println("ok:", models.PathFor(e))
	return nil
}

func modelsDelete(_ context.Context, c *cli.Command) error {
	e, err := mustEntry(c)
	if err != nil {
		return err
	}
	mgr := models.NewManager()
	if err := mgr.Delete(e.ID); err != nil {
		return err
	}
	fmt.Println("deleted:", e.ID)
	return nil
}

func modelsSetActive(_ context.Context, c *cli.Command) error {
	e, err := mustEntry(c)
	if err != nil {
		return err
	}
	mgr := models.NewManager()
	if err := mgr.SetActive(e.ID); err != nil {
		return err
	}
	fmt.Println("active:", e.ID)
	return nil
}

func modelsPath(_ context.Context, c *cli.Command) error {
	e, err := mustEntry(c)
	if err != nil {
		return err
	}
	p := models.PathFor(e)
	if _, err := os.Stat(p); err == nil {
		fmt.Println(p)
		return nil
	}
	fmt.Fprintln(os.Stderr, "(not installed)", p)
	return nil
}

func humanSize(n int64) string {
	const (
		mb int64 = 1 << 20
		gb       = 1 << 30
	)
	switch {
	case n >= gb:
		return fmt.Sprintf("%.1fGB", float64(n)/float64(gb))
	case n >= mb:
		return fmt.Sprintf("%dMB", n/mb)
	default:
		return fmt.Sprintf("%dKB", n/(1<<10))
	}
}
