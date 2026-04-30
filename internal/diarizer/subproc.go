package diarizer

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"sync"

	shared "github.com/asolopovas/wt/internal"
)

type subproc struct {
	name   string
	cmd    *exec.Cmd
	Stdout io.ReadCloser

	mu          sync.Mutex
	stderrLines []string
	stderrDone  chan struct{}
	onStderr    func(string) bool
}

func startSubproc(_ context.Context, name string, cmd *exec.Cmd, onStderr func(string) bool) (*subproc, error) {
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

	sp := &subproc{
		name:       name,
		cmd:        cmd,
		Stdout:     stdout,
		stderrDone: make(chan struct{}),
		onStderr:   onStderr,
	}
	go sp.collectStderr(stderr)
	return sp, nil
}

func (sp *subproc) collectStderr(r io.Reader) {
	defer close(sp.stderrDone)
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for scanner.Scan() {
		line := scanner.Text()
		if sp.onStderr != nil && sp.onStderr(line) {
			continue
		}
		sp.mu.Lock()
		sp.stderrLines = append(sp.stderrLines, line)
		sp.mu.Unlock()
	}
}

func (sp *subproc) stderrTail(n int) []string {
	sp.mu.Lock()
	defer sp.mu.Unlock()
	tail := sp.stderrLines
	if len(tail) > n {
		tail = tail[len(tail)-n:]
	}
	out := make([]string, len(tail))
	copy(out, tail)
	return out
}

func (sp *subproc) wait(ctx context.Context) error {
	err := sp.cmd.Wait()
	<-sp.stderrDone
	if err == nil {
		return nil
	}
	if ctx.Err() != nil {
		return fmt.Errorf("diarization cancelled")
	}
	tail := sp.stderrTail(20)
	if len(tail) > 0 {
		return fmt.Errorf("%s failed:\n%s", sp.name, strings.Join(tail, "\n"))
	}
	return fmt.Errorf("%s exited with error: %w", sp.name, err)
}
