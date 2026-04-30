//go:build android

package gui

import (
	"fmt"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"
)

func (p *transcribePanel) onToggleRecord(btn *pointerButton) {
	if isRecording() {
		path, err := stopRecording()
		btn.SetIcon(micIconResource)
		btn.SetText("RECORD")
		btn.Importance = widget.HighImportance
		btn.Refresh()
		if err != nil {
			p.appendLog("Recording stop failed: " + err.Error())
			return
		}
		p.appendLog("Recording saved: " + path)
		go func(srcPath string) {
			if err := publishRecordingToDocuments(srcPath); err != nil {
				fyne.Do(func() { p.appendLog("warn: " + err.Error()) })
			} else {
				fyne.Do(func() {
					p.appendLog("Saved to Documents/wt/" + filenameOnly(srcPath))
				})
			}
		}(path)
		fyne.Do(func() {
			if p.addLocalFile(path) {
				p.rebuildChips()
				p.updateDropLabel()
				p.appendLog("Added recording to file list")
			}
		})
		return
	}

	if !checkPermission(permRecordAudio) {
		dialog.ShowConfirm("Microphone permission",
			"Recording requires microphone access. Grant now?",
			func(ok bool) {
				if !ok {
					return
				}
				requestPermissions([]string{permRecordAudio})
			}, p.window)
		return
	}

	path, err := startRecording()
	if err != nil {
		dialog.ShowError(fmt.Errorf("could not start recording: %w", err), p.window)
		return
	}
	btn.SetText("STOP")
	btn.Importance = widget.DangerImportance
	btn.Refresh()
	p.appendLog("Recording: " + path)
}

func filenameOnly(path string) string {
	for i := len(path) - 1; i >= 0; i-- {
		if path[i] == '/' || path[i] == '\\' {
			return path[i+1:]
		}
	}
	return path
}
