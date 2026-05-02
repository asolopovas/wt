package transcriber

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/asolopovas/wt/internal/transcriber/cache"
	whisper "github.com/ggerganov/whisper.cpp/bindings/go/pkg/whisper"
)

type panicModel struct{}

func (panicModel) Close() error                         { panic("Close called") }
func (panicModel) NewContext() (whisper.Context, error) { panic("NewContext called") }
func (panicModel) IsMultilingual() bool                 { panic("IsMultilingual called") }
func (panicModel) Languages() []string                  { panic("Languages called") }

func redirectAppDir(t *testing.T) {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("APPDATA", dir)
	t.Setenv("XDG_CONFIG_HOME", dir)
	t.Setenv("HOME", dir)
	if err := os.MkdirAll(filepath.Join(dir, "wt"), 0o755); err != nil {
		t.Fatal(err)
	}
}

func writeFakeAudio(t *testing.T, name string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(path, []byte("fake audio bytes"), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

type recordingHooks struct {
	phases   []Phase
	logs     []string
	resumeFn func(ResumePrompt) ResumeChoice
}

func (r *recordingHooks) hooks() Hooks {
	return Hooks{
		OnPhase: func(p Phase) { r.phases = append(r.phases, p) },
		OnLog:   func(level, msg string) { r.logs = append(r.logs, level+":"+msg) },
		OnResume: func(p ResumePrompt) ResumeChoice {
			if r.resumeFn != nil {
				return r.resumeFn(p)
			}
			return ResumeFresh
		},
	}
}

func TestJob_CacheHitShortCircuits(t *testing.T) {
	redirectAppDir(t)

	src := writeFakeAudio(t, "test.wav")

	spec := JobSpec{
		SourcePath: src,
		ModelSize:  "tiny",
		Language:   "en",
		Threads:    2,
		Speakers:   0,
		NoDiarize:  false,
	}

	params, err := cache.BuildKeyParams(src, spec.ModelSize, spec.Language, spec.Speakers, spec.NoDiarize)
	if err != nil {
		t.Fatalf("BuildKeyParams: %v", err)
	}
	key := cache.ComputeKey(params)
	entry := cache.Entry{
		Key:        key,
		SourcePath: src,
		SourceName: filepath.Base(src),
		Model:      spec.ModelSize,
		Language:   spec.Language,
		Speakers:   spec.Speakers,
		Utterances: 1,
		DurationMs: 1000,
		CreatedAt:  time.Now(),
	}
	if _, err := cache.Store(entry, []byte(`{"model":"tiny"}`+"\n")); err != nil {
		t.Fatalf("seeding cache: %v", err)
	}

	rec := &recordingHooks{}
	job := &Job{Model: panicModel{}, Hooks: rec.hooks()}

	res, err := job.Run(context.Background(), spec)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if !res.Cached {
		t.Errorf("Cached=false, want true")
	}
	if res.CacheKey != key {
		t.Errorf("CacheKey=%q want %q", res.CacheKey, key)
	}
	if res.CachePath == "" {
		t.Error("CachePath empty")
	}
	if len(rec.phases) != 1 || rec.phases[0] != PhaseCacheCheck {
		t.Errorf("phases=%v, want exactly [cache_check]", rec.phases)
	}
}

func TestHooks_ZeroValueIsSilent(t *testing.T) {
	var h Hooks
	h.phase(PhaseLoadingAudio)
	h.progress(PhaseTranscribing, 50)
	h.log("info", "x")
	if got := h.resume(ResumePrompt{}); got != ResumeFresh {
		t.Errorf("zero-value resume()=%v want ResumeFresh", got)
	}
}

func TestJob_RejectsMissingModel(t *testing.T) {
	redirectAppDir(t)
	job := &Job{}
	_, err := job.Run(context.Background(), JobSpec{SourcePath: "x"})
	if err == nil {
		t.Fatal("want error, got nil")
	}
}

func TestJob_RejectsMissingPath(t *testing.T) {
	job := &Job{Model: panicModel{}}
	_, err := job.Run(context.Background(), JobSpec{SourcePath: ""})
	if err == nil {
		t.Fatal("want error, got nil")
	}
}
