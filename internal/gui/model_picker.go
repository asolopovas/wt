package gui

import (
	"fmt"
	"slices"

	"github.com/asolopovas/wt/internal/models"
)

// pickerOption is a single row in a unified model dropdown.
type pickerOption struct {
	ID          string // catalog entry ID
	DisplayName string // shown in the dropdown
}

// pickerLabels returns just the display names (used for Fyne select Options).
func pickerLabels(opts []pickerOption) []string {
	out := make([]string, len(opts))
	for i, o := range opts {
		out[i] = o.DisplayName
	}
	return out
}

// pickerByDisplayName resolves a display name back to its entry ID.
// Returns "" if no match (e.g. legacy/unknown selection).
func pickerByDisplayName(opts []pickerOption, name string) string {
	for _, o := range opts {
		if o.DisplayName == name {
			return o.ID
		}
	}
	return ""
}

// transcriptionPickerOptions enumerates installed transcription engines:
// FamilyWhisper entries first, then FamilyASR entries (Parakeet, etc.).
// Only installed entries are listed; the dropdown otherwise shows broken
// choices the user can't actually use.
func transcriptionPickerOptions(mgr *models.Manager) []pickerOption {
	var out []pickerOption
	for _, fam := range []models.Family{models.FamilyWhisper, models.FamilyASR} {
		for _, e := range models.ByFamily(fam) {
			if mgr.Status(e.ID) != models.StatusInstalled {
				continue
			}
			out = append(out, pickerOption{ID: e.ID, DisplayName: e.DisplayName})
		}
	}
	return out
}

// diarizerPickerOptions enumerates installed FamilyDiarizer entries.
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

// activeTranscriptionID returns the entry ID currently driving transcription:
// FamilyASR's active wins (because runner.go's engine resolver prefers it),
// otherwise FamilyWhisper's active, otherwise "".
func activeTranscriptionID(mgr *models.Manager) string {
	if id := mgr.Active(models.FamilyASR); id != "" {
		return id
	}
	return mgr.Active(models.FamilyWhisper)
}

// setActiveTranscription updates the manager so the given entry ID becomes
// the picked transcription engine. For whisper entries, also clears any
// FamilyASR active (otherwise the engine resolver would keep preferring
// the ASR engine over the freshly-picked whisper). Returns an error if
// the entry is unknown or not installed.
func setActiveTranscription(mgr *models.Manager, id string) error {
	e, ok := models.ByID(id)
	if !ok {
		return fmt.Errorf("unknown model: %s", id)
	}
	if err := mgr.SetActive(id); err != nil {
		return err
	}
	if e.Family == models.FamilyWhisper {
		if err := mgr.ClearActive(models.FamilyASR); err != nil {
			return err
		}
	}
	return nil
}

// activeTranscriptionDisplayName returns the display name to show in the
// dropdown. Falls back to the first option if no active selection matches.
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

// activeDiarizerDisplayName returns the display name for the active
// FamilyDiarizer entry, falling back to the first option.
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

// containsDisplayName is a helper for tests that need to assert an option
// is present in the dropdown.
func containsDisplayName(opts []pickerOption, name string) bool {
	return slices.ContainsFunc(opts, func(o pickerOption) bool {
		return o.DisplayName == name
	})
}

// supportedLanguagesForActive returns the list of language codes the
// currently active transcription engine accepts. nil means "unrestricted"
// (whisper-style multilingual) and the caller should show the full
// language picker.
//
// Used by the LANGUAGE dropdown to constrain options when an English-only
// engine like Parakeet is picked.
func supportedLanguagesForActive(mgr *models.Manager) []string {
	id := activeTranscriptionID(mgr)
	if id == "" {
		return nil
	}
	return models.LanguagesFor(id)
}

// filterLanguageOptions intersects the global `all` language list with
// the engine's `allowed` whitelist, preserving the order of `all` so the
// dropdown stays stable. If `allowed` is nil/empty, returns `all`
// unchanged. If `current` isn't in the resulting list, the first allowed
// option is returned as the new selection (otherwise `current`).
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
	// Append any whitelisted entries that weren't in `all` (e.g. "yue"
	// for SenseVoice which isn't in the whisper language list).
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
