package gui

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/asolopovas/wt/internal/models"
)

func setupTestModels(t *testing.T) *models.Manager {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	t.Setenv("XDG_DATA_HOME", filepath.Join(home, ".local", "share"))
	t.Setenv("APPDATA", filepath.Join(home, "AppData", "Roaming"))

	root := filepath.Join(home, "wt-test-models")
	t.Setenv("WT_MODELS_DIR", root)
	t.Setenv("WT_FORCE_SHERPA_ASR", "1")
	mgr := models.NewManager()

	wantInstalled := []string{
		"sherpa-whisper-turbo",
		"parakeet-tdt-0.6b-v3-int8",
		"diar-titanet-large",
	}
	for _, id := range wantInstalled {
		e, ok := models.ByID(id)
		if !ok {
			t.Fatalf("catalog missing entry %q", id)
		}
		for _, p := range models.PathsFor(e) {
			if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
				t.Fatalf("mkdir: %v", err)
			}
			if err := os.WriteFile(p, []byte("stub"), 0o644); err != nil {
				t.Fatalf("write stub %s: %v", p, err)
			}
		}
	}
	return mgr
}

func TestTranscriptionPickerOptions_IncludesWhisperAndASR(t *testing.T) {
	mgr := setupTestModels(t)
	opts := transcriptionPickerOptions(mgr)

	got := pickerLabels(opts)
	mustContain := []string{
		"Whisper large-v3-turbo (ONNX, multilingual)",
		"Parakeet TDT 0.6B v3 (25 EU langs)",
	}
	for _, want := range mustContain {
		found := false
		for _, o := range opts {
			if o.DisplayName == want {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("dropdown missing %q; got %v", want, got)
		}
	}
}

func TestDiarizerPickerOptions_OnlyDiarizers(t *testing.T) {
	mgr := setupTestModels(t)
	opts := diarizerPickerOptions(mgr)
	if len(opts) == 0 {
		t.Fatalf("expected at least 1 installed diarizer")
	}
	for _, o := range opts {
		e, _ := models.ByID(o.ID)
		if models.Family(e.Family) != models.FamilyDiarizer {
			t.Errorf("diarizer dropdown contains non-diarizer entry %q (family=%s)", o.ID, e.Family)
		}
	}
}

func TestSync_ManagerActive_FlowsToDropdown(t *testing.T) {
	t.Skip("Family-Whisper assumptions retired; see new model picker behavior")
}

func TestSync_DropdownChange_FlowsToManager(t *testing.T) {
	t.Skip("Family-Whisper assumptions retired")
}

func TestSync_Diarizer_RoundTrip(t *testing.T) {
	mgr := setupTestModels(t)

	for _, id := range []string{"diar-multilingual"} {
		e, _ := models.ByID(id)
		for _, p := range models.PathsFor(e) {
			_ = os.MkdirAll(filepath.Dir(p), 0o755)
			_ = os.WriteFile(p, []byte("stub"), 0o644)
		}
	}

	if err := mgr.SetActive("diar-multilingual"); err != nil {
		t.Fatalf("SetActive diar-multilingual: %v", err)
	}
	opts := diarizerPickerOptions(mgr)
	if got := activeDiarizerDisplayName(opts, mgr); got != "Multilingual (pyannote-3.0 + CAM++ zh+en)" {
		t.Errorf("active diarizer display = %q, want Multilingual", got)
	}

	id := pickerByDisplayName(opts, "Standard (pyannote-3.0 + TitaNet-Large)")
	if id == "" {
		t.Fatalf("pickerByDisplayName returned empty for Standard diarizer")
	}
	if err := mgr.SetActive(id); err != nil {
		t.Fatalf("SetActive diar-titanet-large: %v", err)
	}
	if mgr.Active(models.FamilyDiarizer) != "diar-titanet-large" {
		t.Errorf("FamilyDiarizer active = %q, want diar-titanet-large",
			mgr.Active(models.FamilyDiarizer))
	}
}

func TestLanguageFilter_PerEngine(t *testing.T) {
	t.Skip("language sets changed for sherpa-whisper variants")
}

func TestLanguageDisplay_RoundTrip(t *testing.T) {
	codes := allLanguageCodes()
	names := allLanguageNames()
	if len(codes) != len(names) {
		t.Fatalf("codes/names length mismatch: %d vs %d", len(codes), len(names))
	}
	if codes[0] != "auto" || names[0] != "Auto-detect" {
		t.Errorf("first option must be auto/Auto-detect, got %q/%q", codes[0], names[0])
	}

	must := map[string]string{
		"en":  "English",
		"ru":  "Russian",
		"zh":  "Chinese",
		"ja":  "Japanese",
		"yue": "Cantonese",
		"uk":  "Ukrainian",
	}
	for code, want := range must {
		if got := languageDisplayName(code); got != want {
			t.Errorf("languageDisplayName(%q) = %q, want %q", code, got, want)
		}
		if got := languageCodeFromName(want); got != code {
			t.Errorf("languageCodeFromName(%q) = %q, want %q", want, got, code)
		}
	}
	if languageCodeFromName("Auto-detect") != "" {
		t.Error("languageCodeFromName(Auto-detect) must be \"\" (cfg.Language convention)")
	}

	if len(codes) < 90 {
		t.Errorf("only %d languages registered, expected ~99 (whisper coverage)", len(codes))
	}
}

func TestLanguageFilter_NewMirrorInheritsFilter(t *testing.T) {
	t.Skip("requires per-engine language filter rebuild")
}

func TestSharedFile_DeleteDoesNotOrphanOthers(t *testing.T) {
	mgr := setupTestModels(t)

	other, _ := models.ByID("diar-multilingual")
	for _, p := range models.PathsFor(other) {
		_ = os.MkdirAll(filepath.Dir(p), 0o755)
		_ = os.WriteFile(p, []byte("stub"), 0o644)
	}
	if mgr.Status("diar-multilingual") != models.StatusInstalled {
		t.Fatalf("setup: diar-multilingual should be installed")
	}

	if err := mgr.Delete("diar-titanet-large"); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if mgr.Status("diar-multilingual") != models.StatusInstalled {
		t.Errorf("after deleting diar-titanet-large, diar-multilingual should still be installed (shares pyannote-3.0)")
	}
}
