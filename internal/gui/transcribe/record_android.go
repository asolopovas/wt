//go:build android

package transcribe

import (
	"fmt"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/widget"

	"github.com/asolopovas/wt/internal/gui/assets"
	"github.com/asolopovas/wt/internal/gui/decor"
	"github.com/asolopovas/wt/internal/gui/platsvc"
)

var showConfirm = decor.ShowConfirm

// OnToggleRecord toggles between idle and recording. The same physical button
// reads RECORD when idle and PAUSE while recording. The neighbouring ADD FILES
// button is repurposed as SAVE during recording — see OnAddOrSave.
func (p *Panel) OnToggleRecord(btn *pointerButton) {
	if isRecording() {
		// Click on PAUSE: stop recording, don't auto-add to file list.
		p.finishRecording(false)
		return
	}

	if !platsvc.CheckPermission(platsvc.PermRecordAudio) {
		showConfirm(p.window, "Microphone permission",
			"Recording requires microphone access. Grant now?",
			func() {
				platsvc.RequestPermissions([]string{platsvc.PermRecordAudio})
			})
		return
	}

	path, err := startRecording()
	if err != nil {
		showError(p.window, fmt.Errorf("could not start recording: %w", err))
		return
	}
	btn.SetIcon(assets.PauseIcon)
	btn.SetText("PAUSE")
	btn.Importance = widget.DangerImportance
	btn.Refresh()
	if p.AddFilesBtn != nil {
		p.AddFilesBtn.SetIcon(assets.SaveIcon)
		p.AddFilesBtn.SetText("SAVE")
		p.AddFilesBtn.Refresh()
	}
	p.AppendLog("Recording: " + path)
}

// OnAddOrSave is wired to the ADD FILES / SAVE button. When idle it opens the
// file picker; while recording it stops and saves the in-progress recording.
func (p *Panel) OnAddOrSave(btn *pointerButton) {
	if isRecording() {
		// Click on SAVE: stop recording AND add the file to the working set.
		p.finishRecording(true)
		return
	}
	p.OnBrowse()
}

// finishRecording stops the recorder, restores button labels, optionally adds
// the produced file to the working set and publishes it to Documents.
func (p *Panel) finishRecording(addToList bool) {
	path, err := stopRecording()
	if p.RecordBtn != nil {
		p.RecordBtn.SetIcon(assets.MicIcon)
		p.RecordBtn.SetText("RECORD")
		p.RecordBtn.Importance = widget.DangerImportance
		p.RecordBtn.Refresh()
	}
	if p.AddFilesBtn != nil {
		p.AddFilesBtn.SetIcon(assets.AddFileIcon)
		p.AddFilesBtn.SetText("ADD FILES")
		p.AddFilesBtn.Refresh()
	}
	if err != nil {
		p.AppendLog("Recording stop failed: " + err.Error())
		return
	}
	p.AppendLog("Recording saved: " + path)
	go func(srcPath string) {
		if err := publishRecordingToDocuments(srcPath); err != nil {
			fyne.Do(func() { p.AppendLog("warn: " + err.Error()) })
		} else {
			fyne.Do(func() {
				p.AppendLog("Saved to Documents/wt/" + filenameOnly(srcPath))
			})
		}
	}(path)
	if addToList {
		fyne.Do(func() {
			if p.AddLocalFile(path) {
				p.RebuildChips()
				p.UpdateDropLabel()
				p.AppendLog("Added recording to file list")
			}
		})
	}
}

func filenameOnly(path string) string {
	for i := len(path) - 1; i >= 0; i-- {
		if path[i] == '/' || path[i] == '\\' {
			return path[i+1:]
		}
	}
	return path
}
