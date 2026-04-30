package diarizer

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
	"sync/atomic"
	"time"
)

type nemoDiarizer struct {
	pythonExe  string
	scriptPath string
}

func newNemoDiarizer() (Backend, error) {
	pythonExe, err := resolvePython()
	if err != nil {
		return nil, err
	}

	scriptPath, err := findScript("diarize.py")
	if err != nil {
		return nil, err
	}

	return &nemoDiarizer{pythonExe: pythonExe, scriptPath: scriptPath}, nil
}

func (d *nemoDiarizer) Name() string { return "nemo-sortformer" }

func (d *nemoDiarizer) Diarize(ctx context.Context, wavPath string, numSpeakers int, audioDurSec float64, progress ProgressFunc) ([]Segment, error) {
	args := []string{d.scriptPath}
	if numSpeakers > 0 {
		args = append(args, "--num-speakers", fmt.Sprintf("%d", numSpeakers))
	}
	args = append(args, wavPath)

	return runPythonDiarizer(ctx, d.pythonExe, args, "nemo-sortformer", audioDurSec, progress)
}

type jsonSegment struct {
	Start   float64 `json:"start"`
	End     float64 `json:"end"`
	Speaker string  `json:"speaker"`
}

func runPythonDiarizer(ctx context.Context, pythonExe string, args []string, name string, audioDurSec float64, progress ProgressFunc) ([]Segment, error) {
	cmd := exec.CommandContext(ctx, pythonExe, args...)

	var done atomic.Bool
	sp, err := startSubproc(ctx, name, cmd, func(line string) bool {
		if strings.HasPrefix(line, "done:") {
			done.Store(true)
		}
		return false
	})
	if err != nil {
		return nil, err
	}

	progStop := make(chan struct{})
	if progress != nil {
		go runTimeBasedProgress(progStop, &done, audioDurSec, progress)
	}

	var rawJSON []byte
	scanner := bufio.NewScanner(sp.Stdout)
	scanner.Buffer(make([]byte, 0, 1024*1024), 10*1024*1024)
	for scanner.Scan() {
		rawJSON = append(rawJSON, scanner.Bytes()...)
	}

	close(progStop)
	if err := sp.wait(ctx); err != nil {
		return nil, err
	}

	var parsed []jsonSegment
	if err := json.Unmarshal(rawJSON, &parsed); err != nil {
		return nil, fmt.Errorf("parsing %s output: %w", name, err)
	}

	speakerMap := make(map[string]int)
	nextID := 0
	segments := make([]Segment, 0, len(parsed))
	for _, ps := range parsed {
		id, ok := speakerMap[ps.Speaker]
		if !ok {
			id = nextID
			speakerMap[ps.Speaker] = id
			nextID++
		}
		segments = append(segments, Segment{
			Speaker:  id,
			StartSec: ps.Start,
			EndSec:   ps.End,
		})
	}

	if progress != nil {
		progress(100)
	}
	return segments, nil
}

func runTimeBasedProgress(stop <-chan struct{}, done *atomic.Bool, audioDurSec float64, progress ProgressFunc) {
	start := time.Now()
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()
	lastReported := 0.0
	for {
		select {
		case <-stop:
			return
		case <-ticker.C:
			if done.Load() {
				if lastReported < 95 {
					lastReported = 95
					progress(95)
				}
				continue
			}
			elapsed := time.Since(start).Seconds()
			estTotal := audioDurSec * 0.15
			if estTotal < 3 {
				estTotal = 3
			}
			pct := elapsed / estTotal * 90
			if pct > 90 {
				pct = 90
			}
			if pct > lastReported {
				lastReported = pct
				progress(pct)
			}
		}
	}
}
