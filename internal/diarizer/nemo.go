package diarizer

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"

	shared "github.com/asolopovas/wt/internal"
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
	cmd.Env = os.Environ()
	shared.HideWindow(cmd)

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("creating stdout pipe: %w", err)
	}

	stderr, err := cmd.StderrPipe()
	if err != nil {
		return nil, fmt.Errorf("creating stderr pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("starting %s: %w", name, err)
	}

	var mu sync.Mutex
	var stderrLines []string
	done := false

	go func() {
		scanner := bufio.NewScanner(stderr)
		for scanner.Scan() {
			line := scanner.Text()
			mu.Lock()
			stderrLines = append(stderrLines, line)
			if strings.HasPrefix(line, "done:") {
				done = true
			}
			mu.Unlock()
		}
	}()

	doneCh := make(chan struct{})
	lastReported := 0.0
	if progress != nil {
		go func() {
			start := time.Now()
			ticker := time.NewTicker(500 * time.Millisecond)
			defer ticker.Stop()
			for {
				select {
				case <-doneCh:
					return
				case <-ticker.C:
					mu.Lock()
					finished := done
					mu.Unlock()

					if finished {
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
					if estTotal <= 0 {
						estTotal = 60
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
		}()
	}

	var rawJSON []byte
	scanner := bufio.NewScanner(stdout)
	scanner.Buffer(make([]byte, 0, 1024*1024), 10*1024*1024)
	for scanner.Scan() {
		rawJSON = append(rawJSON, scanner.Bytes()...)
	}

	if err := cmd.Wait(); err != nil {
		close(doneCh)
		if ctx.Err() != nil {
			return nil, fmt.Errorf("diarization cancelled")
		}
		mu.Lock()
		captured := stderrLines
		mu.Unlock()
		if len(captured) > 0 {
			tail := captured
			if len(tail) > 20 {
				tail = tail[len(tail)-20:]
			}
			return nil, fmt.Errorf("%s failed:\n%s", name, strings.Join(tail, "\n"))
		}
		return nil, fmt.Errorf("%s exited with error: %w", name, err)
	}

	close(doneCh)

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
