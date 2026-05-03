package assets

import (
	_ "embed"

	"fyne.io/fyne/v2"
)

//go:embed icon_256.png
var appIconData []byte

var AppIcon = &fyne.StaticResource{
	StaticName:    "icon_256.png",
	StaticContent: appIconData,
}

//go:embed icon_mic.svg
var micIconData []byte

var MicIcon = &fyne.StaticResource{
	StaticName:    "icon_mic.svg",
	StaticContent: micIconData,
}

//go:embed icon_play.svg
var playIconData []byte

var PlayIcon = &fyne.StaticResource{
	StaticName:    "icon_play.svg",
	StaticContent: playIconData,
}

//go:embed icon_pause.svg
var pauseIconData []byte

var PauseIcon = &fyne.StaticResource{
	StaticName:    "icon_pause.svg",
	StaticContent: pauseIconData,
}

//go:embed icon_edit_audio.svg
var editAudioIconData []byte

var EditAudioIcon = &fyne.StaticResource{
	StaticName:    "icon_edit_audio.svg",
	StaticContent: editAudioIconData,
}

//go:embed icon_cpu.svg
var cpuIconData []byte

var CPUIcon = &fyne.StaticResource{
	StaticName:    "icon_cpu.svg",
	StaticContent: cpuIconData,
}

//go:embed icon_ram.svg
var ramIconData []byte

var RAMIcon = &fyne.StaticResource{
	StaticName:    "icon_ram.svg",
	StaticContent: ramIconData,
}

//go:embed icon_gpu.svg
var gpuIconData []byte

var GPUIcon = &fyne.StaticResource{
	StaticName:    "icon_gpu.svg",
	StaticContent: gpuIconData,
}

//go:embed icon_download.svg
var downloadIconData []byte

var DownloadIcon = &fyne.StaticResource{
	StaticName:    "icon_download.svg",
	StaticContent: downloadIconData,
}

//go:embed icon_cancel.svg
var cancelIconData []byte

var CancelIcon = &fyne.StaticResource{
	StaticName:    "icon_cancel.svg",
	StaticContent: cancelIconData,
}

//go:embed icon_add_file.svg
var addFileIconData []byte

var AddFileIcon = &fyne.StaticResource{
	StaticName:    "icon_add_file.svg",
	StaticContent: addFileIconData,
}

//go:embed icon_save.svg
var saveIconData []byte

var SaveIcon = &fyne.StaticResource{
	StaticName:    "icon_save.svg",
	StaticContent: saveIconData,
}
