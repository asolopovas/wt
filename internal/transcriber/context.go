package transcriber

import (
	whisper "github.com/ggerganov/whisper.cpp/bindings/go/pkg/whisper"
)

type ContextConfig struct {
	Threads int
	TDRZ    bool
}

func ConfigureContext(ctx whisper.Context, cfg ContextConfig) {
	ctx.SetThreads(uint(max(cfg.Threads, 1)))
	ctx.SetBeamSize(5)
	ctx.SetTDRZ(cfg.TDRZ)
	ctx.SetTemperature(0.0)
	ctx.SetTemperatureFallback(0.1)
	ctx.SetEntropyThold(2.0)
	ctx.SetLogprobThold(-0.3)
	ctx.SetNoSpeechThold(0.5)
	ctx.SetTokenTimestamps(true)
	ctx.SetSplitOnWord(true)
}

func SetLanguage(ctx whisper.Context, language string) {
	if language != "" {
		_ = ctx.SetLanguage(language)
		return
	}
	if err := ctx.SetLanguage("auto"); err != nil {
		_ = ctx.SetLanguage("en")
	}
}

func ConfigureVAD(ctx whisper.Context) bool {
	vadPath, err := ResolveVADModelPath()
	if err != nil {
		return false
	}
	ctx.SetVAD(true)
	ctx.SetVADModelPath(vadPath)
	ctx.SetVADThreshold(0.5)
	ctx.SetVADMinSpeechMs(250)
	ctx.SetVADMinSilenceMs(100)
	ctx.SetVADMaxSpeechSec(30.0)
	ctx.SetVADSpeechPadMs(30)
	ctx.SetVADSamplesOverlap(0.1)
	return true
}
