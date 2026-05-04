package gui

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"

	"github.com/asolopovas/wt/internal/models"
)

var sherpaASRBinaryAvailable = func() bool {
	if v := os.Getenv("WT_FORCE_SHERPA_ASR"); v == "1" {
		return true
	}
	name := "sherpa-onnx-offline"
	if runtime.GOOS == "windows" {
		name = "sherpa-onnx-offline.exe"
	}
	if runtime.GOOS == "android" {
		return true
	}
	if exe, err := os.Executable(); err == nil {
		if st, err := os.Stat(filepath.Join(filepath.Dir(exe), name)); err == nil && !st.IsDir() {
			return true
		}
	}
	if _, err := exec.LookPath(name); err == nil {
		return true
	}
	return false
}

type pickerOption struct {
	ID          string
	DisplayName string
}

func pickerLabels(opts []pickerOption) []string {
	out := make([]string, len(opts))
	for i, o := range opts {
		out[i] = o.DisplayName
	}
	return out
}

func pickerByDisplayName(opts []pickerOption, name string) string {
	for _, o := range opts {
		if o.DisplayName == name {
			return o.ID
		}
	}
	return ""
}

func transcriptionPickerOptions(mgr *models.Manager) []pickerOption {
	var out []pickerOption
	asrAvailable := sherpaASRBinaryAvailable()
	for _, fam := range []models.Family{models.FamilyASR} {
		if fam == models.FamilyASR && !asrAvailable {
			continue
		}
		for _, e := range models.ByFamily(fam) {
			if mgr.Status(e.ID) != models.StatusInstalled {
				continue
			}
			out = append(out, pickerOption{ID: e.ID, DisplayName: e.DisplayName})
		}
	}
	return out
}

func diarizerPickerOptions(mgr *models.Manager) []pickerOption {
	var out []pickerOption
	for _, e := range models.ByFamily(models.FamilyDiarizer) {
		if mgr.Status(e.ID) != models.StatusInstalled {
			continue
		}
		out = append(out, pickerOption{ID: e.ID, DisplayName: e.DisplayName})
	}
	return out
}

func activeTranscriptionID(mgr *models.Manager) string {
	if sherpaASRBinaryAvailable() {
		if id := mgr.Active(models.FamilyASR); id != "" {
			return id
		}
	}
	return mgr.Active(models.FamilyASR)
}

func setActiveTranscription(mgr *models.Manager, id string) error {
	if _, ok := models.ByID(id); !ok {
		return fmt.Errorf("unknown model: %s", id)
	}
	return mgr.SetActive(id)
}

func activeTranscriptionDisplayName(opts []pickerOption, mgr *models.Manager) string {
	id := activeTranscriptionID(mgr)
	for _, o := range opts {
		if o.ID == id {
			return o.DisplayName
		}
	}
	if len(opts) > 0 {
		return opts[0].DisplayName
	}
	return ""
}

func activeDiarizerDisplayName(opts []pickerOption, mgr *models.Manager) string {
	id := mgr.Active(models.FamilyDiarizer)
	for _, o := range opts {
		if o.ID == id {
			return o.DisplayName
		}
	}
	if len(opts) > 0 {
		return opts[0].DisplayName
	}
	return ""
}

func containsDisplayName(opts []pickerOption, name string) bool {
	return slices.ContainsFunc(opts, func(o pickerOption) bool {
		return o.DisplayName == name
	})
}

func supportedLanguagesForActive(mgr *models.Manager) []string {
	id := activeTranscriptionID(mgr)
	if id == "" {
		return nil
	}
	return models.LanguagesFor(id)
}

func filterLanguageOptions(all, allowed []string, current string) (opts []string, selected string) {
	if len(allowed) == 0 {
		return all, current
	}
	allowSet := make(map[string]struct{}, len(allowed))
	for _, l := range allowed {
		allowSet[l] = struct{}{}
	}
	for _, l := range all {
		if _, ok := allowSet[l]; ok {
			opts = append(opts, l)
		}
	}

	for _, l := range allowed {
		if slices.Contains(opts, l) {
			continue
		}
		opts = append(opts, l)
	}
	if slices.Contains(opts, current) {
		return opts, current
	}
	if len(opts) > 0 {
		return opts, opts[0]
	}
	return opts, current
}
