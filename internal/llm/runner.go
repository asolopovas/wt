package llm

import (
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

	var gPath string
	if opts.Grammar != "" {
		gf, err := os.CreateTemp("", "wt-llm-grammar-*.gbnf")
		if err != nil {
			return "", err
		}
		gPath = gf.Name()
		_, _ = gf.WriteString(opts.Grammar)
		_ = gf.Close()
		defer func() { _ = os.Remove(gPath) }()
	}

	buildArgs := func(cpuOnly bool) []string {
		a := []string{
			"-m", r.ModelPath,
			"-f", pPath,
			"-n", fmt.Sprintf("%d", opts.MaxTokens),
			"-t", fmt.Sprintf("%d", r.Threads),
			"--temp", fmt.Sprintf("%.2f", opts.Temp),
			"--no-display-prompt",
			"--log-disable",
			"--no-conversation",
			"--single-turn",
			"--simple-io",
			"--no-warmup",
		}
		if cpuOnly {
			a = append(a, "-ngl", "0")
		}
		if gPath != "" {
			a = append(a, "--grammar-file", gPath)
		}
		return a
	}

	cpuOnly := os.Getenv("WT_LLM_DEVICE") == "cpu"
	out, waitErr, stderrStr, runErr := r.runOnce(ctx, buildArgs(cpuOnly), cpuOnly)
	if runErr != nil {
		return "", runErr
	}
	if obj := lastBalancedJSON(out); obj != "" {
		return obj, nil
	}

	if !cpuOnly && waitErr != nil {
		out2, waitErr2, stderrStr2, runErr2 := r.runOnce(ctx, buildArgs(true), true)
		if runErr2 == nil {
			if obj := lastBalancedJSON(out2); obj != "" {
				return obj, nil
			}
			waitErr, stderrStr, out = waitErr2, stderrStr2, out2
		}
	}

	return "", fmt.Errorf("no JSON object in llm output (waitErr=%v): stderr=%s; stdout=%s",
		waitErr, stderrTail(stderrStr, 6), stdoutTail(out, 400))
}

func (r *Runner) runOnce(ctx context.Context, args []string, hideCUDA bool) (string, error, string, error) {
	rctx, cancel := context.WithTimeout(ctx, 120*time.Second)
	defer cancel()

	cmd := exec.CommandContext(rctx, r.BinaryPath, args...)
	cmd.Env = os.Environ()
	if hideCUDA {
		cmd.Env = append(cmd.Env, "CUDA_VISIBLE_DEVICES=-1", "GGML_CUDA_DISABLE=1")
	}
	cmd.Stdin = nil
	hideLlamaWindow(cmd)

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return "", nil, "", err
	}
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Start(); err != nil {
		return "", nil, "", fmt.Errorf("starting llama-cli: %w", err)
	}

	raw, readErr := io.ReadAll(stdout)
	waitErr := cmd.Wait()
	out := string(raw)

	if rctx.Err() != nil && rctx.Err() != context.Canceled {
		return out, waitErr, stderr.String(), fmt.Errorf("llm timeout: %s", stderrTail(stderr.String(), 8))
	}
	if readErr != nil && !errors.Is(readErr, io.EOF) {
		return out, waitErr, stderr.String(), fmt.Errorf("reading llm output: %w", readErr)
	}
	return out, waitErr, stderr.String(), nil
}

func lastBalancedJSON(s string) string {
	depth := 0
	end := -1
	for i := len(s) - 1; i >= 0; i-- {
		c := s[i]
		switch c {
		case '}':
			if depth == 0 {
				end = i
			}
			depth++
		case '{':
			if depth > 0 {
				depth--
				if depth == 0 && end >= 0 {
					return s[i : end+1]
				}
			}
		}
	}
	return ""
}

func stdoutTail(s string, n int) string {
	s = strings.TrimSpace(s)
	if len(s) > n {
		return "..." + s[len(s)-n:]
	}
	return s
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
		exeDir := filepath.Dir(exe)
		out = append(out,
			filepath.Join(exeDir, "llama", name),
			filepath.Join(exeDir, name),
		)
	}
	if runtime.GOOS == "android" {
		for _, dir := range androidLibDirs() {
			out = append(out, filepath.Join(dir, "libllama-cli.so"))
		}
	}
	out = append(out,
		filepath.Join(shared.Dir(), "llama", name),
		filepath.Join(shared.Dir(), name),
	)
	if cwd, err := os.Getwd(); err == nil {
		out = append(out,
			filepath.Join(cwd, "dist", "bin", "llama", name),
			filepath.Join(cwd, "dist", "bin", name),
			filepath.Join(cwd, "dist", "llama", name),
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
