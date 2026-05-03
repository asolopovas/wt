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
	switch {
	case isPaused():
		if err := resumeRecording(); err != nil {
			showError(p.window, fmt.Errorf("resume failed: %w", err))
			return
		}
		p.applyRecordingButtonState(stateRecording)
		p.AppendLog("Recording resumed")
		return
	case isRecording():
		if err := pauseRecording(); err != nil {
			showError(p.window, fmt.Errorf("pause failed: %w", err))
			return
		}
		p.applyRecordingButtonState(statePaused)
		p.AppendLog("Recording paused")
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
	p.applyRecordingButtonState(stateRecording)
	p.AppendLog("Recording: " + path)
}

func (p *Panel) OnAddOrSave(btn *pointerButton) {
	if isRecording() || isPaused() {
		p.finishRecording(true)
		return
	}
	p.OnBrowse()
}

type recBtnState int

const (
	stateIdle recBtnState = iota
	stateRecording
	statePaused
)

func (p *Panel) applyRecordingButtonState(s recBtnState) {
	if p.RecordBtn != nil {
		switch s {
		case stateRecording:
			p.RecordBtn.SetIcon(assets.PauseIcon)
			p.RecordBtn.SetText("PAUSE")
		case statePaused:
			p.RecordBtn.SetIcon(assets.MicIcon)
			p.RecordBtn.SetText("RECORD")
		default:
			p.RecordBtn.SetIcon(assets.MicIcon)
			p.RecordBtn.SetText("RECORD")
		}
		p.RecordBtn.Importance = widget.DangerImportance
		p.RecordBtn.Refresh()
	}
	if p.AddFilesBtn != nil {
		if s == stateIdle {
			p.AddFilesBtn.SetIcon(assets.AddFileIcon)
			p.AddFilesBtn.SetText("ADD FILES")
		} else {
			p.AddFilesBtn.SetIcon(assets.SaveIcon)
			p.AddFilesBtn.SetText("SAVE")
		}
		p.AddFilesBtn.Refresh()
	}
}

func (p *Panel) finishRecording(addToList bool) {
	path, err := stopRecording()
	p.applyRecordingButtonState(stateIdle)
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
