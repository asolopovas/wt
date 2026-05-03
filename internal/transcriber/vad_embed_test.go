package transcriber

import (
	"os"
	"path/filepath"
	"testing"
)

func TestEmbeddedVADBytes_Present(t *testing.T) {
	if len(vadModelBytes) < 100_000 {
		t.Fatalf("embedded VAD model looks too small: %d bytes (run scripts/fetch-vad.sh and rebuild)", len(vadModelBytes))
	}
}

func TestResolveVADModelPath_ExtractsEmbed(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmp)

	path, err := ResolveVADModelPath()
	if err != nil {
		t.Fatalf("ResolveVADModelPath: %v", err)
	}

	want := filepath.Join(tmp, "wt", "models", embeddedVADName)
	if path != want {
		t.Errorf("path = %q, want %q", path, want)
	}

	st, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat extracted file: %v", err)
	}
	if st.Size() != int64(len(vadModelBytes)) {
		t.Errorf("extracted size = %d, want %d", st.Size(), len(vadModelBytes))
	}

	path2, err := ResolveVADModelPath()
	if err != nil {
		t.Fatalf("second ResolveVADModelPath: %v", err)
	}
	if path2 != path {
		t.Errorf("second call path = %q, want %q", path2, path)
	}
}
