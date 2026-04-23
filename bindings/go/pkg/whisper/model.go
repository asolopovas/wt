package whisper

import (
	"fmt"
	"os"
	"runtime"

	whisper "github.com/ggerganov/whisper.cpp/bindings/go"
)

type model struct {
	path string
	ctx  *whisper.Context
}

var _ Model = (*model)(nil)

func SetLogQuiet(quiet bool) {
	whisper.SetLogQuiet(quiet)
}

func BackendSetSearchPath(path string) {
	whisper.BackendSetSearchPath(path)
}

func BackendLoadAll() {
	whisper.BackendLoadAll()
}

func SystemInfo() string {
	return whisper.Whisper_print_system_info()
}

type BackendDeviceInfo = whisper.BackendDeviceInfo

func BackendDevices() []BackendDeviceInfo {
	return whisper.BackendDevices()
}

func New(path string) (Model, error) {
	if _, err := os.Stat(path); err != nil {
		return nil, err
	}
	ctx := whisper.Whisper_init(path)
	if ctx == nil {
		return nil, ErrUnableToLoadModel
	}
	return &model{path: path, ctx: ctx}, nil
}

func (model *model) Close() error {
	if model.ctx != nil {
		model.ctx.Whisper_free()
		model.ctx = nil
	}
	return nil
}

func (model *model) String() string {
	str := "<whisper.model"
	if model.ctx != nil {
		str += fmt.Sprintf(" model=%q", model.path)
	}
	return str + ">"
}

func (model *model) IsMultilingual() bool {
	return model.ctx.Whisper_is_multilingual() != 0
}

func (model *model) Languages() []string {
	result := make([]string, 0, whisper.Whisper_lang_max_id())
	for i := 0; i < whisper.Whisper_lang_max_id(); i++ {
		str := whisper.Whisper_lang_str(i)
		if model.ctx.Whisper_lang_id(str) >= 0 {
			result = append(result, str)
		}
	}
	return result
}

func (model *model) NewContext() (Context, error) {
	if model.ctx == nil {
		return nil, ErrInternalAppError
	}
	params := model.ctx.Whisper_full_default_params(whisper.SAMPLING_BEAM_SEARCH)
	params.SetTranslate(false)
	params.SetPrintSpecial(false)
	params.SetPrintProgress(false)
	params.SetPrintRealtime(false)
	params.SetPrintTimestamps(false)
	params.SetThreads(runtime.NumCPU())
	params.SetNoContext(true)

	params.SetSuppressBlank(true)
	params.SetSuppressNST(true)

	return newContext(model, params)
}
