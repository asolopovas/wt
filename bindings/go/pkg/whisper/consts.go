package whisper

import (
	"errors"

	whisper "github.com/ggerganov/whisper.cpp/bindings/go"
)

var (
	ErrUnableToLoadModel    = errors.New("unable to load model")
	ErrInternalAppError     = errors.New("internal application error")
	ErrProcessingFailed     = errors.New("processing failed")
	ErrUnsupportedLanguage  = errors.New("unsupported language")
	ErrModelNotMultilingual = errors.New("model is not multilingual")
)

const SampleRate = whisper.SampleRate

const SampleBits = whisper.SampleBits
