package gui

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/asolopovas/wt/internal/models"
)

// setupTestModels lays out a fake $WT_HOME with a couple of installed
// model files so models.Manager treats the catalog entries as installed.
// Returns the manager wired to that home and a teardown.
func setupTestModels(t *testing.T) *models.Manager {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home) // windows
	t.Setenv("XDG_DATA_HOME", filepath.Join(home, ".local", "share"))
	t.Setenv("APPDATA", filepath.Join(home, "AppData", "Roaming"))

	root := filepath.Join(home, "wt-test-models")
	t.Setenv("WT_MODELS_DIR", root)
	mgr := models.NewManager() // create after env so paths use the temp root

	// Materialise files for: whisper-turbo + whisper-tiny + parakeet ASR + diar default
	wantInstalled := []string{
		"whisper-tiny",
		"whisper-turbo",
		"parakeet-tdt-0.6b-v2-int8",
		"sense-voice-zh-en-ja-ko-yue-int8",
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

// Verify the unified transcription dropdown enumerates installed entries
// from BOTH FamilyWhisper and FamilyASR.
func TestTranscriptionPickerOptions_IncludesWhisperAndASR(t *testing.T) {
	mgr := setupTestModels(t)
	opts := transcriptionPickerOptions(mgr)

	got := pickerLabels(opts)
	mustContain := []string{
		"Whisper tiny (multilingual)",
		"Whisper large-v3-turbo",
		"Parakeet TDT 0.6B v2 (English)",
	}
	for _, want := range mustContain {
		if !containsDisplayName(opts, want) {
			t.Errorf("dropdown missing %q; got %v", want, got)
		}
	}
}

// Verify diarizer dropdown enumerates only FamilyDiarizer entries.
func TestDiarizerPickerOptions_OnlyDiarizers(t *testing.T) {
	mgr := setupTestModels(t)
	opts := diarizerPickerOptions(mgr)
	if len(opts) == 0 {
		t.Fatalf("expected at least 1 installed diarizer")
	}
	for _, o := range opts {
		e, _ := models.ByID(o.ID)
		if e.Family != models.FamilyDiarizer {
			t.Errorf("diarizer dropdown contains non-diarizer entry %q (family=%s)", o.ID, e.Family)
		}
	}
}

// Sync test: SetActive in the manager must be reflected by
// activeTranscriptionID + activeTranscriptionDisplayName (which is what
// the dropdown reads on refresh).
func TestSync_ManagerActive_FlowsToDropdown(t *testing.T) {
	mgr := setupTestModels(t)

	// Pick whisper-tiny via SetActive.
	if err := setActiveTranscription(mgr, "whisper-tiny"); err != nil {
		t.Fatalf("setActiveTranscription whisper-tiny: %v", err)
	}
	opts := transcriptionPickerOptions(mgr)
	if got := activeTranscriptionDisplayName(opts, mgr); got != "Whisper tiny (multilingual)" {
		t.Errorf("after SetActive(whisper-tiny), display name = %q, want %q",
			got, "Whisper tiny (multilingual)")
	}

	// Pick Parakeet ASR via SetActive.
	if err := setActiveTranscription(mgr, "parakeet-tdt-0.6b-v2-int8"); err != nil {
		t.Fatalf("setActiveTranscription parakeet: %v", err)
	}
	if got := activeTranscriptionDisplayName(opts, mgr); got != "Parakeet TDT 0.6B v2 (English)" {
		t.Errorf("after SetActive(parakeet), display name = %q, want Parakeet", got)
	}
	// Ensure FamilyASR active is set so runner.go's engine resolver
	// picks parakeet over whisper.
	if mgr.Active(models.FamilyASR) != "parakeet-tdt-0.6b-v2-int8" {
		t.Errorf("FamilyASR active = %q, want parakeet", mgr.Active(models.FamilyASR))
	}

	// Switch back to a whisper entry: setActiveTranscription must clear
	// the FamilyASR pick (otherwise runner.go would still route to ASR).
	if err := setActiveTranscription(mgr, "whisper-turbo"); err != nil {
		t.Fatalf("setActiveTranscription whisper-turbo: %v", err)
	}
	if mgr.Active(models.FamilyASR) != "" {
		t.Errorf("after picking whisper, FamilyASR should be cleared, got %q",
			mgr.Active(models.FamilyASR))
	}
	if mgr.Active(models.FamilyWhisper) != "whisper-turbo" {
		t.Errorf("FamilyWhisper active = %q, want whisper-turbo",
			mgr.Active(models.FamilyWhisper))
	}
}

// Sync test: choosing a display name in the dropdown must call SetActive
// on the manager (verified via pickerByDisplayName + setActiveTranscription
// which is the same chain onModelChanged uses).
func TestSync_DropdownChange_FlowsToManager(t *testing.T) {
	mgr := setupTestModels(t)

	opts := transcriptionPickerOptions(mgr)
	// Simulate user picking Parakeet from the dropdown.
	id := pickerByDisplayName(opts, "Parakeet TDT 0.6B v2 (English)")
	if id == "" {
		t.Fatalf("pickerByDisplayName returned empty for Parakeet")
	}
	if err := setActiveTranscription(mgr, id); err != nil {
		t.Fatalf("setActiveTranscription: %v", err)
	}
	if mgr.Active(models.FamilyASR) != "parakeet-tdt-0.6b-v2-int8" {
		t.Errorf("FamilyASR not updated by dropdown pick: %q", mgr.Active(models.FamilyASR))
	}

	// Simulate user picking a whisper entry from the dropdown.
	id = pickerByDisplayName(opts, "Whisper large-v3-turbo")
	if err := setActiveTranscription(mgr, id); err != nil {
		t.Fatalf("setActiveTranscription whisper: %v", err)
	}
	if mgr.Active(models.FamilyASR) != "" {
		t.Errorf("FamilyASR should be cleared after whisper pick, got %q",
			mgr.Active(models.FamilyASR))
	}
}

// Diarizer sync test: same round-trip but for FamilyDiarizer.
func TestSync_Diarizer_RoundTrip(t *testing.T) {
	mgr := setupTestModels(t)

	// Materialise a second diarizer's files so we can switch.
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

	// Switch via the dropdown lookup chain.
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

// Per-engine language gating: when an English-only engine like Parakeet
// is active, the LANGUAGE dropdown options must collapse to ["en"].
// SenseVoice (multilingual) must show its supported subset. Whisper
// (no Languages constraint) must show the full list unchanged.
//
// Filter operates on codes; UI conversion to display names is tested
// separately in TestLanguageDisplay_RoundTrip.
func TestLanguageFilter_PerEngine(t *testing.T) {
	mgr := setupTestModels(t)

	// Use a representative whisper-style "all languages" superset for
	// the test — anything not in the engine whitelist must be removed.
	all := []string{"auto", "en", "zh", "ja", "ko", "de", "es", "fr", "ru"}

	tests := []struct {
		name     string
		activeID string
		current  string
		wantOpts []string
		wantSel  string
	}{
		{
			name:     "parakeet collapses to en only",
			activeID: "parakeet-tdt-0.6b-v2-int8",
			current:  "de",
			wantOpts: []string{"en"},
			wantSel:  "en", // current "de" not allowed -> reset to first option
		},
		{
			name:     "sensevoice keeps zh/en/ja/ko + auto + adds yue",
			activeID: "sense-voice-zh-en-ja-ko-yue-int8",
			current:  "ja",
			wantOpts: []string{"auto", "en", "zh", "ja", "ko", "yue"},
			wantSel:  "ja", // ja is allowed -> preserved
		},
		{
			name:     "whisper keeps everything",
			activeID: "whisper-turbo",
			current:  "de",
			wantOpts: all,
			wantSel:  "de",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := setActiveTranscription(mgr, tt.activeID); err != nil {
				t.Fatalf("setActiveTranscription: %v", err)
			}
			allowed := supportedLanguagesForActive(mgr)
			gotOpts, gotSel := filterLanguageOptions(all, allowed, tt.current)
			if !equalStringSlice(gotOpts, tt.wantOpts) {
				t.Errorf("options = %v, want %v", gotOpts, tt.wantOpts)
			}
			if gotSel != tt.wantSel {
				t.Errorf("selected = %q, want %q", gotSel, tt.wantSel)
			}
		})
	}
}

// Round-trip the dropdown's display-name layer: every code in the
// canonical list must have a name, and lookups in both directions must
// match. Catches typos in languageNames and ensures Russian (the user's
// reported missing-from-the-dropdown lang) is present and correctly
// labelled.
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

	// Whisper supports ~99 languages — catalog must list at least 90.
	if len(codes) < 90 {
		t.Errorf("only %d languages registered, expected ~99 (whisper coverage)", len(codes))
	}
}

func equalStringSlice(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// Regression: when an engine has only one valid language (e.g. Parakeet
// -> ["en"]), a NEWLY-CREATED language mirror (transcode tab built
// after settings panel) must inherit the filtered options, not the
// full whisper-style 99-lang list. Bug fixed by constructing the
// mirror from the master's already-filtered Options.
func TestLanguageFilter_NewMirrorInheritsFilter(t *testing.T) {
	mgr := setupTestModels(t)
	if err := setActiveTranscription(mgr, "parakeet-tdt-0.6b-v2-int8"); err != nil {
		t.Fatalf("setActiveTranscription: %v", err)
	}

	// Same chain newLangSelectMirror uses: filter the global list against
	// the active engine's whitelist; the mirror must be built from this
	// filtered set rather than the raw `languages` slice.
	allowed := supportedLanguagesForActive(mgr)
	masterOpts, masterSel := filterLanguageOptions(languages, allowed, "auto")

	if len(masterOpts) != 1 || masterOpts[0] != "en" {
		t.Fatalf("master filter expected [en], got %v", masterOpts)
	}
	if masterSel != "en" {
		t.Errorf("master selection = %q, want en", masterSel)
	}
}

// Shared-file deletion safety: deleting one diarizer preset must not
// orphan another preset that shares the same segmentation file
// (pyannote-3.0 is shared by 3 of the 5 presets).
func TestSharedFile_DeleteDoesNotOrphanOthers(t *testing.T) {
	mgr := setupTestModels(t)

	// Install diar-multilingual too (shares pyannote-3.0 with default).
	other, _ := models.ByID("diar-multilingual")
	for _, p := range models.PathsFor(other) {
		_ = os.MkdirAll(filepath.Dir(p), 0o755)
		_ = os.WriteFile(p, []byte("stub"), 0o644)
	}
	if mgr.Status("diar-multilingual") != models.StatusInstalled {
		t.Fatalf("setup: diar-multilingual should be installed")
	}

	// Delete the default. Mobile-light must remain fully installed.
	if err := mgr.Delete("diar-titanet-large"); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if mgr.Status("diar-multilingual") != models.StatusInstalled {
		t.Errorf("after deleting diar-titanet-large, diar-multilingual should still be installed (shares pyannote-3.0)")
	}
}
