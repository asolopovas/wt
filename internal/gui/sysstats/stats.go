package sysstats

import (
	"fmt"
	"os/exec"
	"strconv"
	"strings"

	shared "github.com/asolopovas/wt/internal"
)

func FormatMB(mb int64) string {
	if mb >= 1024 {
		return fmt.Sprintf("%.1f GB", float64(mb)/1024)
	}
	return fmt.Sprintf("%d MB", mb)
}

func GPUStats() (utilPct int, memMB int) {
	utilPct = -1
	memMB = -1

	if u := AndroidGPU(); u >= 0 {
		utilPct = u
		return
	}

	cmd := exec.Command("nvidia-smi",
		"--query-gpu=utilization.gpu,memory.used",
		"--format=csv,noheader,nounits")
	shared.HideWindow(cmd)
	out, err := cmd.Output()
	if err != nil {
		return
	}

	line := strings.TrimSpace(string(out))
	parts := strings.Split(line, ",")
	if len(parts) < 2 {
		return
	}

	if v, err := strconv.Atoi(strings.TrimSpace(parts[0])); err == nil {
		utilPct = v
	}
	if v, err := strconv.Atoi(strings.TrimSpace(parts[1])); err == nil {
		memMB = v
	}
	return
}
