package transcriber

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestSherpaCudaRuntimeDirs_HonorsEnvOverride(t *testing.T) {
	if runtime.GOOS != "windows" && runtime.GOOS != "linux" {
		t.Skip("CUDA runtime path only used on windows/linux")
	}
	t.Setenv("WT_SHERPA_CUDA_DIR", "/custom/sherpa-cuda")
	dirs := sherpaCudaRuntimeDirs()
	if len(dirs) == 0 {
		t.Fatal("expected at least one runtime dir")
	}
	want := filepath.Join("/custom/sherpa-cuda", "bin")
	if dirs[0] != want {
		t.Errorf("first dir: got %q want %q", dirs[0], want)
	}
}

func TestSherpaCudaRuntimeDirs_IncludesLocalAppData(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("LOCALAPPDATA fallback is windows-only")
	}
	t.Setenv("WT_SHERPA_CUDA_DIR", "")
	t.Setenv("LOCALAPPDATA", `C:\fake\local`)
	dirs := sherpaCudaRuntimeDirs()
	want := filepath.Join(`C:\fake\local`, "wt", "sherpa-cuda", "bin")
	found := false
	for _, d := range dirs {
		if d == want {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected %q in dirs %v", want, dirs)
	}
}

func TestFindSherpaBinaryIn_FindsFirstMatch(t *testing.T) {
	tmpA := t.TempDir()
	tmpB := t.TempDir()
	name := "phantom-bin"
	target := filepath.Join(tmpB, name)
	if err := os.WriteFile(target, []byte("x"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	got := findSherpaBinaryIn([]string{tmpA, tmpB}, name)
	if got != target {
		t.Errorf("got %q want %q", got, target)
	}
	if findSherpaBinaryIn([]string{tmpA}, name) != "" {
		t.Errorf("missing match should return empty")
	}
}

func TestSherpaProvider_DefaultsToCPU(t *testing.T) {
	t.Setenv("WT_ZIPFORMER_PROVIDER", "")
	if got := sherpaProvider(); got != "cpu" {
		t.Errorf("default provider: got %q want cpu", got)
	}
	t.Setenv("WT_ZIPFORMER_PROVIDER", "cuda")
	if got := sherpaProvider(); got != "cuda" {
		t.Errorf("override: got %q want cuda", got)
	}
}
