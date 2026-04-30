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

func (p *Panel) OnToggleRecord(btn *pointerButton) {
	if isRecording() {
		path, err := stopRecording()
		btn.SetIcon(assets.MicIcon)
		btn.SetText("RECORD")
		btn.Importance = widget.HighImportance
		btn.Refresh()
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
		fyne.Do(func() {
			if p.AddLocalFile(path) {
				p.RebuildChips()
				p.UpdateDropLabel()
				p.AppendLog("Added recording to file list")
			}
		})
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
	btn.SetText("STOP")
	btn.Importance = widget.DangerImportance
	btn.Refresh()
	p.AppendLog("Recording: " + path)
}

func filenameOnly(path string) string {
	for i := len(path) - 1; i >= 0; i-- {
		if path[i] == '/' || path[i] == '\\' {
			return path[i+1:]
		}
	}
	return path
}
