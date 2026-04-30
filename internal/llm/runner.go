package llm

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	shared "github.com/asolopovas/wt/internal"
	"github.com/asolopovas/wt/internal/models"
)

type Runner struct {
	BinaryPath string
	ModelPath  string
	Threads    int
}

func NewRunner() (*Runner, error) {
	bin, err := findBinary()
	if err != nil {
		return nil, err
	}

	mgr := models.NewManager()
	id := mgr.Active(models.FamilyLLM)
	if id == "" {
		return nil, fmt.Errorf("no active LLM selected (download one in Settings → Models or run: wt models get qwen3-1.7b-q4km && wt models set-active qwen3-1.7b-q4km)")
	}
	entry, ok := models.ByID(id)
	if !ok {
		return nil, fmt.Errorf("active LLM %q not in catalog", id)
	}
	mp := models.PathFor(entry)
	if _, err := os.Stat(mp); err != nil {
		return nil, fmt.Errorf("active LLM %q not installed at %s", id, mp)
	}

	threads := runtime.NumCPU()
	if threads > 6 {
		threads = 6
	}
	return &Runner{BinaryPath: bin, ModelPath: mp, Threads: threads}, nil
}

type Options struct {
	Prompt    string
	Grammar   string
	MaxTokens int
	Temp      float64
}

func (r *Runner) Generate(ctx context.Context, opts Options) (string, error) {
	if opts.MaxTokens <= 0 {
		opts.MaxTokens = 128
	}
	if opts.Temp == 0 {
		opts.Temp = 0.1
	}

	pf, err := os.CreateTemp("", "wt-llm-prompt-*.txt")
	if err != nil {
		return "", err
	}
	pPath := pf.Name()
	_, _ = pf.WriteString(opts.Prompt)
	_ = pf.Close()
	defer func() { _ = os.Remove(pPath) }()

	args := []string{
		"-m", r.ModelPath,
		"-f", pPath,
		"-n", fmt.Sprintf("%d", opts.MaxTokens),
		"-t", fmt.Sprintf("%d", r.Threads),
		"--temp", fmt.Sprintf("%.2f", opts.Temp),
		"--no-display-prompt",
		"--log-disable",
	}

	if opts.Grammar != "" {
		gf, err := os.CreateTemp("", "wt-llm-grammar-*.gbnf")
		if err != nil {
			return "", err
		}
		gPath := gf.Name()
		_, _ = gf.WriteString(opts.Grammar)
		_ = gf.Close()
		defer func() { _ = os.Remove(gPath) }()
		args = append(args, "--grammar-file", gPath)
	}

	rctx, cancel := context.WithTimeout(ctx, 90*time.Second)
	defer cancel()

	cmd := exec.CommandContext(rctx, r.BinaryPath, args...)
	cmd.Env = os.Environ()
	shared.HideWindow(cmd)

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return "", err
	}
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Start(); err != nil {
		return "", fmt.Errorf("starting llama-cli: %w", err)
	}

	out, readErr := readUntilJSONClose(stdout)
	_ = cmd.Process.Kill()
	_ = cmd.Wait()

	if readErr != nil && !errors.Is(readErr, io.EOF) && rctx.Err() == nil {
		return "", fmt.Errorf("reading llm output: %w", readErr)
	}
	if rctx.Err() != nil && rctx.Err() != context.Canceled && len(out) == 0 {
		return "", fmt.Errorf("llm timeout: %s", stderrTail(stderr.String(), 8))
	}

	if i := strings.Index(out, "{"); i >= 0 {
		if j := strings.LastIndex(out, "}"); j > i {
			return out[i : j+1], nil
		}
	}
	return strings.TrimSpace(out), nil
}

func readUntilJSONClose(r io.Reader) (string, error) {
	br := bufio.NewReader(r)
	var buf bytes.Buffer
	depth := 0
	started := false
	for {
		b, err := br.ReadByte()
		if err != nil {
			return buf.String(), err
		}
		buf.WriteByte(b)
		switch b {
		case '{':
			depth++
			started = true
		case '}':
			depth--
			if started && depth <= 0 {
				return buf.String(), nil
			}
		}
	}
}

func findBinary() (string, error) {
	candidates := binaryCandidates()
	for _, p := range candidates {
		if p == "" {
			continue
		}
		if st, err := os.Stat(p); err == nil && !st.IsDir() {
			return p, nil
		}
	}
	if p, err := exec.LookPath(binaryName()); err == nil {
		return p, nil
	}
	return "", fmt.Errorf("llama-cli not found (searched: %s)", strings.Join(candidates, ", "))
}

func binaryCandidates() []string {
	name := binaryName()
	out := []string{}
	if exe, err := os.Executable(); err == nil {
		out = append(out, filepath.Join(filepath.Dir(exe), name))
	}
	if runtime.GOOS == "android" {
		for _, dir := range androidLibDirs() {
			out = append(out, filepath.Join(dir, "libllama-cli.so"))
		}
	}
	out = append(out, filepath.Join(shared.Dir(), name))
	if cwd, err := os.Getwd(); err == nil {
		out = append(out,
			filepath.Join(cwd, "dist", "bin", name),
			filepath.Join(cwd, "dist", "llama", "llama-cli-android-arm64"),
		)
	}
	return out
}

func binaryName() string {
	if runtime.GOOS == "windows" {
		return "llama-cli.exe"
	}
	return "llama-cli"
}

func stderrTail(s string, n int) string {
	lines := strings.Split(strings.TrimRight(s, "\n"), "\n")
	if len(lines) > n {
		lines = lines[len(lines)-n:]
	}
	return strings.Join(lines, "\n")
}
