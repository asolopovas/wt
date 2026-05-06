//go:build !android

package transcribe

import (
	"fmt"
	"path/filepath"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/widget"

	"github.com/asolopovas/wt/internal/gui/assets"
)

type recBtnState int

const (
	stateIdle recBtnState = iota
	stateRecording
)

func (p *Panel) applyRecordingButtonState(s recBtnState) {
	if p.RecordBtn != nil {
		switch s {
		case stateRecording:
			p.RecordBtn.SetIcon(assets.PauseIcon)
			p.RecordBtn.SetText("STOP")
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

func (p *Panel) OnToggleRecord(_ *pointerButton) {
	if isRecording() {
		p.finishRecording(true)
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

func (p *Panel) OnAddOrSave(_ *pointerButton) {
	if isRecording() {
		p.finishRecording(true)
		return
	}
	p.OnBrowse()
}

func (p *Panel) finishRecording(addToList bool) {
	path, err := stopRecording()
	p.applyRecordingButtonState(stateIdle)
	if err != nil {
		p.AppendLog("Recording stop failed: " + err.Error())
		return
	}
	p.AppendLog("Recording saved: " + path)
	if addToList {
		fyne.Do(func() {
			if p.AddLocalFile(path) {
				p.RebuildChips()
				p.UpdateDropLabel()
				p.AppendLog("Added recording to file list: " + filepath.Base(path))
			}
		})
	}
}
