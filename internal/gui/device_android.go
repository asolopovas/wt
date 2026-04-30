//go:build android

package gui

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
)

type deviceStat struct {
	Label string
	Value string
}

func deviceStats() []deviceStat {
	return []deviceStat{
		{"CPU", fmt.Sprintf("%s · %d cores", detectCPU(), runtime.NumCPU())},
		{"RAM", readMemTotal()},
		{"GPU", detectGPU()},
	}
}

func detectCPU() string {
	mfr := getprop("ro.soc.manufacturer")
	model := getprop("ro.soc.model")
	switch {
	case mfr != "" && model != "":
		return mfr + " " + model
	case model != "":
		return model
	case mfr != "":
		return mfr
	}

	if v := readSocFile("/sys/devices/soc0/machine"); v != "" {
		return v
	}

	if hw := readCPUInfoField("Hardware"); hw != "" {
		return hw
	}

	return runtime.GOARCH
}

func getprop(key string) string {
	out, err := exec.Command("getprop", key).Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

func readSocFile(path string) string {
	b, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(b))
}

func readCPUInfoField(field string) string {
	f, err := os.Open("/proc/cpuinfo")
	if err != nil {
		return ""
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	prefix := field + "\t"
	prefix2 := field + " "
	for sc.Scan() {
		line := sc.Text()
		if !strings.HasPrefix(line, prefix) && !strings.HasPrefix(line, prefix2) {
			continue
		}
		idx := strings.Index(line, ":")
		if idx < 0 {
			continue
		}
		return strings.TrimSpace(line[idx+1:])
	}
	return ""
}

func detectGPU() string {
	info := detectDevice()
	if info == "" || info == "CPU ONLY" {
		return "—"
	}
	return info
}

func readMemTotal() string {
	f, err := os.Open("/proc/meminfo")
	if err != nil {
		return "—"
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := sc.Text()
		if !strings.HasPrefix(line, "MemTotal:") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 2 {
			return "—"
		}
		kb, err := strconv.ParseInt(fields[1], 10, 64)
		if err != nil {
			return "—"
		}
		return fmt.Sprintf("%.1f GB", float64(kb)/1024.0/1024.0)
	}
	return "—"
}

