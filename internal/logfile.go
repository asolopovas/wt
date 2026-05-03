// Package shared — persistent run log helpers.
//
// Every line that flows through Panel.AppendLog (and Panel.debugLog when
// debug is enabled) is also appended to a single text file on disk so
// users can review crashes / engine errors after the fact, even after
// the app is force-killed by the OS.
//
// File location: <MediaDir>/wt.log
//
//	Android: /storage/emulated/0/Documents/WTranscribe/wt.log
//	Desktop: <CacheDir>/imports/wt.log
//
// Rotation is delegated to gopkg.in/natefinch/lumberjack.v2 — the
// de-facto Go log-rotation library (used by Kubernetes, Docker, etcd).
// Rotation triggers at 5 MB; archives are kept for the user-configured
// retention window (LogRetentionDays in shared.Config; defaults to 1).
//
// Lumberjack's MaxAge is whole-day granularity; for the 24h preset we
// supplement it with manual pruneOldArchives() on each AppendLogLine
// (throttled every 5 min) to enforce a true rolling window.
package shared

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"gopkg.in/natefinch/lumberjack.v2"
)

const (
	logMaxSizeMB         = 5
	logMaxBackups        = 48
	logPruneEvery        = 5 * time.Minute
	logTailDefault       = 256 * 1024
	defaultRetentionDays = 1
)

var (
	logMu         sync.Mutex
	logRot        *lumberjack.Logger
	logPathS      string
	logRetainDays int = defaultRetentionDays
	lastPrune     time.Time
)

func SetLogRetentionDays(days int) {
	logMu.Lock()
	defer logMu.Unlock()
	if days < 0 {
		days = 0
	}
	logRetainDays = days
	if logRot != nil {
		logRot.MaxAge = days
	}
}

func LogFilePath() string {
	logMu.Lock()
	defer logMu.Unlock()
	return logFilePathLocked()
}

func logFilePathLocked() string {
	if logPathS != "" {
		return logPathS
	}
	dir := MediaDir()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		dir = CacheDir()
		_ = os.MkdirAll(dir, 0o755)
	}
	logPathS = filepath.Join(dir, "wt.log")
	return logPathS
}

func rotator() *lumberjack.Logger {
	if logRot == nil {
		logRot = &lumberjack.Logger{
			Filename:   logFilePathLocked(),
			MaxSize:    logMaxSizeMB,
			MaxBackups: logMaxBackups,
			MaxAge:     logRetainDays,
			LocalTime:  true,
			Compress:   false,
		}
	}
	return logRot
}

func AppendLogLine(msg string) {
	writeLogLine("INFO ", msg)
}

func LogInfo(msg string) { writeLogLine("INFO ", msg) }

func LogWarn(msg string) { writeLogLine("WARN ", msg) }

func LogError(msg string) { writeLogLine("ERROR", msg) }

func LogDebug(msg string) { writeLogLine("DEBUG", msg) }

func writeLogLine(level, msg string) {
	logMu.Lock()
	defer logMu.Unlock()
	stamp := time.Now().Format("15:04:05")
	_, _ = fmt.Fprintf(rotator(), "%s %s %s\n", stamp, level, msg)
	pruneOldArchives()
}

func LogProcessStart(label string) {
	logMu.Lock()
	defer logMu.Unlock()
	stamp := time.Now().Format("2006-01-02 15:04:05")
	_, _ = fmt.Fprintf(rotator(), "\n----- %s  %s started\n", stamp, label)
	pruneOldArchives()
}

func LogProcessEnd(label, outcome, details string) {
	logMu.Lock()
	defer logMu.Unlock()
	stamp := time.Now().Format("2006-01-02 15:04:05")
	line := fmt.Sprintf("----- %s  %s %s", stamp, label, outcome)
	if details != "" {
		line += " — " + details
	}
	_, _ = fmt.Fprintln(rotator(), line)
	pruneOldArchives()
}

func pruneOldArchives() {
	if logRetainDays == 0 {
		return
	}
	now := time.Now()
	if now.Sub(lastPrune) < logPruneEvery {
		return
	}
	lastPrune = now

	cutoff := now.Add(-time.Duration(logRetainDays) * 24 * time.Hour)
	dir := filepath.Dir(logFilePathLocked())
	matches, _ := filepath.Glob(filepath.Join(dir, "wt-*.log"))
	for _, m := range matches {
		if st, err := os.Stat(m); err == nil && st.ModTime().Before(cutoff) {
			_ = os.Remove(m)
		}
	}
}

func ReadLogTail(maxBytes int64) string {
	logMu.Lock()
	defer logMu.Unlock()

	path := logFilePathLocked()
	st, err := os.Stat(path)
	if err != nil {
		return ""
	}
	f, err := os.Open(path)
	if err != nil {
		return ""
	}
	defer func() { _ = f.Close() }()

	off := int64(0)
	if st.Size() > maxBytes {
		off = st.Size() - maxBytes
	}
	if _, err := f.Seek(off, 0); err != nil {
		return ""
	}
	buf := make([]byte, st.Size()-off)
	n, _ := f.Read(buf)
	return string(buf[:n])
}

func ClearLog() error {
	logMu.Lock()
	defer logMu.Unlock()

	if logRot != nil {
		_ = logRot.Close()
		logRot = nil
	}
	path := logFilePathLocked()

	dir := filepath.Dir(path)
	matches, _ := filepath.Glob(filepath.Join(dir, "wt-*.log"))
	for _, m := range matches {
		_ = os.Remove(m)
	}
	if err := os.Truncate(path, 0); err != nil {
		f, err := os.Create(path)
		if err != nil {
			return err
		}
		_ = f.Close()
	}
	return nil
}
