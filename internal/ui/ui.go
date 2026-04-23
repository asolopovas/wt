package ui

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/pterm/pterm"
)

var Verbose bool

var (
	dimStyle  = pterm.NewStyle(pterm.FgDarkGray)
	boldStyle = pterm.NewStyle(pterm.Bold)
)

var (
	tickPrefix  = pterm.NewStyle(pterm.FgGreen).Sprint("✓")
	crossPrefix = pterm.NewStyle(pterm.FgRed).Sprint("✗")
)

func Tick(msg string) {
	pterm.Printf("  %s %s\n", tickPrefix, msg)
}

func Tickf(format string, args ...any) {
	Tick(fmt.Sprintf(format, args...))
}

func Cross(msg string) {
	pterm.Printf("  %s %s\n", crossPrefix, msg)
}

func Crossf(format string, args ...any) {
	Cross(fmt.Sprintf(format, args...))
}

func Spinner(text string) *pterm.SpinnerPrinter {
	s, _ := pterm.SpinnerPrinter{
		Sequence:            []string{"  ⠋", "  ⠙", "  ⠹", "  ⠸", "  ⠼", "  ⠴", "  ⠦", "  ⠧", "  ⠇", "  ⠏"},
		Style:               pterm.NewStyle(pterm.FgCyan),
		Delay:               80 * time.Millisecond,
		ShowTimer:           true,
		TimerRoundingFactor: time.Second,
		TimerStyle:          &pterm.ThemeDefault.TimerStyle,
		MessageStyle:        &pterm.ThemeDefault.SpinnerTextStyle,
		RemoveWhenDone:      true,
		Writer:              os.Stderr,
	}.Start(text)
	return s
}

func Banner(version, model string) {
	gpu := gpuName()
	cpu := cpuName()
	ram := totalRAM()
	parts := []string{fmt.Sprintf("wt %s", version), model}
	if gpu != "" {
		parts = append(parts, gpu)
	}
	if cpu != "" {
		parts = append(parts, cpu)
	}
	if ram != "" {
		parts = append(parts, ram)
	}
	pterm.Println(strings.Join(parts, " · "))
}

func gpuName() string {
	out, err := exec.Command("nvidia-smi", "--query-gpu=gpu_name", "--format=csv,noheader").Output()
	if err != nil {
		return ""
	}
	name := strings.TrimSpace(strings.Split(string(out), "\n")[0])
	name = strings.TrimPrefix(name, "NVIDIA ")
	return name
}

func cpuName() string {
	out, err := exec.Command("wmic", "cpu", "get", "name", "/value").Output()
	if err != nil {
		return ""
	}
	for line := range strings.SplitSeq(string(out), "\n") {
		line = strings.TrimSpace(line)
		if after, ok := strings.CutPrefix(line, "Name="); ok {
			name := after
			name = strings.TrimSpace(name)
			name = strings.ReplaceAll(name, "(R)", "")
			name = strings.ReplaceAll(name, "(TM)", "")
			name = strings.ReplaceAll(name, "  ", " ")
			return name
		}
	}
	return ""
}

func totalRAM() string {
	out, err := exec.Command("powershell", "-NoProfile", "-Command",
		"$m=Get-CimInstance Win32_PhysicalMemory;"+
			"$gb=[math]::Round(($m|Measure-Object -Property Capacity -Sum).Sum/1GB);"+
			"$spd=($m|Select-Object -First 1).Speed;"+
			"$t=($m|Select-Object -First 1).SMBIOSMemoryType;"+
			"$tn=switch($t){20{'DDR'};21{'DDR2'};24{'DDR3'};26{'DDR4'};34{'DDR5'};default{''}}; "+
			"if($tn-and $spd){\"$gb GB $tn-$spd\"}elseif($tn){\"$gb GB $tn\"}else{\"$gb GB RAM\"}",
	).Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

func Debug(key, value string) {
	if !Verbose {
		return
	}
	pterm.Printf("  %s %s\n", dimStyle.Sprint(key+":"), dimStyle.Sprint(value))
}

func FileHeader(index, total int, filename string) {
	pterm.Printf("\n  %s %s\n",
		pterm.LightMagenta(fmt.Sprintf("[%d/%d]", index, total)),
		boldStyle.Sprint(filename))
}

func Warn(msg string) {
	pterm.Warning.Println(msg)
}

func Errorf(format string, args ...any) {
	fmt.Fprintf(os.Stderr, pterm.Red("  Error: ")+format+"\n", args...)
}

func Done(msg string) {
	Tick(msg)
}

func Stage(msg string) {
	pterm.Println()
	pterm.Printf("  %s\n", boldStyle.Sprint(msg))
}

func formatETA(elapsedSec, pct float64) string {
	if pct <= 0 {
		return ""
	}
	totalEstimate := elapsedSec / pct * 100
	remaining := totalEstimate - elapsedSec
	if remaining < 0 {
		remaining = 0
	}
	if remaining < 60 {
		return fmt.Sprintf("~%.0fs", remaining)
	}
	mins := int(remaining) / 60
	secs := int(remaining) % 60
	return fmt.Sprintf("~%dm%02ds", mins, secs)
}

func ProgressLine(pct int, elapsedSec float64) {
	if pct > 100 {
		pct = 100
	}
	bar := progressBar(pct, 30)
	eta := formatETA(elapsedSec, float64(pct))
	line := fmt.Sprintf("\r  %s %d%%  %s", bar, pct, dimStyle.Sprint(eta))
	fmt.Fprintf(os.Stderr, "%-80s", line)
}

func ClearProgress() {
	fmt.Fprintf(os.Stderr, "\r%s\r", strings.Repeat(" ", 80))
}

func progressBar(pct int, width int) string {
	filled := min(width*pct/100, width)
	var bar strings.Builder
	for i := range width {
		if i < filled {
			bar.WriteString("█")
		} else {
			bar.WriteString("░")
		}
	}
	return bar.String()
}
