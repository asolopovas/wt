package whisper

import (
	"fmt"
	"io"
	"runtime"
	"strings"
	"time"

	whisper "github.com/ggerganov/whisper.cpp/bindings/go"
)

type context struct {
	n      int
	model  *model
	params whisper.Params
}

var _ Context = (*context)(nil)

func newContext(model *model, params whisper.Params) (Context, error) {
	return &context{model: model, params: params}, nil
}

func (context *context) SetLanguage(lang string) error {
	if context.model.ctx == nil {
		return ErrInternalAppError
	}
	if !context.model.IsMultilingual() {
		return ErrModelNotMultilingual
	}
	if lang == "auto" {
		context.params.SetLanguage(-1)
		return nil
	}
	id := context.model.ctx.Whisper_lang_id(lang)
	if id < 0 {
		return ErrUnsupportedLanguage
	}
	return context.params.SetLanguage(id)
}

func (context *context) IsMultilingual() bool {
	return context.model.IsMultilingual()
}

func (context *context) Language() string {
	id := context.params.Language()
	if id == -1 {
		return "auto"
	}
	return whisper.Whisper_lang_str(context.params.Language())
}

func (context *context) DetectedLanguage() string {
	return whisper.Whisper_lang_str(context.model.ctx.Whisper_full_lang_id())
}

func (context *context) SetTranslate(v bool) {
	context.params.SetTranslate(v)
}

func (context *context) SetVAD(v bool) {
	context.params.SetVAD(v)
}

func (context *context) SetVADModelPath(path string) {
	context.params.SetVADModelPath(path)
}

func (context *context) SetVADThreshold(t float32) {
	context.params.SetVADThreshold(t)
}

func (context *context) SetVADMinSpeechMs(ms int) {
	context.params.SetVADMinSpeechMs(ms)
}

func (context *context) SetVADMinSilenceMs(ms int) {
	context.params.SetVADMinSilenceMs(ms)
}

func (context *context) SetVADMaxSpeechSec(s float32) {
	context.params.SetVADMaxSpeechSec(s)
}

func (context *context) SetVADSpeechPadMs(ms int) {
	context.params.SetVADSpeechPadMs(ms)
}

func (context *context) SetVADSamplesOverlap(sec float32) {
	context.params.SetVADSamplesOverlap(sec)
}

func (context *context) SetSplitOnWord(v bool) {
	context.params.SetSplitOnWord(v)
}

func (context *context) SetTDRZ(v bool) {
	context.params.SetTDRZ(v)
}

func (context *context) SetThreads(v uint) {
	context.params.SetThreads(int(v))
}

func (context *context) SetOffset(v time.Duration) {
	context.params.SetOffset(int(v.Milliseconds()))
}

func (context *context) SetDuration(v time.Duration) {
	context.params.SetDuration(int(v.Milliseconds()))
}

func (context *context) SetTokenThreshold(t float32) {
	context.params.SetTokenThreshold(t)
}

func (context *context) SetTokenSumThreshold(t float32) {
	context.params.SetTokenSumThreshold(t)
}

func (context *context) SetMaxSegmentLength(n uint) {
	context.params.SetMaxSegmentLength(int(n))
}

func (context *context) SetTokenTimestamps(b bool) {
	context.params.SetTokenTimestamps(b)
}

func (context *context) SetMaxTokensPerSegment(n uint) {
	context.params.SetMaxTokensPerSegment(int(n))
}

func (context *context) SetAudioCtx(n uint) {
	context.params.SetAudioCtx(int(n))
}

func (context *context) SetMaxContext(n int) {
	context.params.SetMaxContext(n)
}

func (context *context) SetBeamSize(n int) {
	context.params.SetBeamSize(n)
}

func (context *context) SetEntropyThold(t float32) {
	context.params.SetEntropyThold(t)
}

func (context *context) SetTemperature(t float32) {
	context.params.SetTemperature(t)
}

func (context *context) SetTemperatureFallback(t float32) {
	context.params.SetTemperatureFallback(t)
}

func (context *context) SetInitialPrompt(prompt string) {
	context.params.SetInitialPrompt(prompt)
}

func (context *context) SetSuppressBlank(v bool) {
	context.params.SetSuppressBlank(v)
}

func (context *context) SetSuppressNST(v bool) {
	context.params.SetSuppressNST(v)
}

func (context *context) SetNoSpeechThold(t float32) {
	context.params.SetNoSpeechThold(t)
}

func (context *context) SetLogprobThold(t float32) {
	context.params.SetLogprobThold(t)
}

func (context *context) ResetTimings() {
	context.model.ctx.Whisper_reset_timings()
}

func (context *context) PrintTimings() {
	context.model.ctx.Whisper_print_timings()
}

func (context *context) SystemInfo() string {
	return fmt.Sprintf("system_info: n_threads = %d / %d | %s\n",
		context.params.Threads(),
		runtime.NumCPU(),
		whisper.Whisper_print_system_info(),
	)
}

func (context *context) WhisperLangAutoDetect(offsetMs int, nThreads int) ([]float32, error) {
	return context.model.ctx.Whisper_lang_auto_detect(offsetMs, nThreads)
}

func (context *context) Process(
	data []float32,
	callEncoderBegin EncoderBeginCallback,
	callNewSegment SegmentCallback,
	callProgress ProgressCallback,
) error {
	if context.model.ctx == nil {
		return ErrInternalAppError
	}
	if callNewSegment != nil {
		context.params.SetSingleSegment(true)
	}

	segmentCb := func(new int) {
		if callNewSegment != nil {
			numSegments := context.model.ctx.Whisper_full_n_segments()
			for i := numSegments - new; i < numSegments; i++ {
				callNewSegment(toSegment(context.model.ctx, i))
			}
		}
	}

	progressCb := func(progress int) {
		if callProgress != nil {
			callProgress(progress)
		}
	}

	if err := context.model.ctx.Whisper_full(context.params, data, callEncoderBegin, segmentCb, progressCb); err != nil {
		return err
	}

	context.n = 0
	return nil
}

func (context *context) NextSegment() (Segment, error) {
	if context.model.ctx == nil {
		return Segment{}, ErrInternalAppError
	}
	if context.n >= context.model.ctx.Whisper_full_n_segments() {
		return Segment{}, io.EOF
	}
	seg := toSegment(context.model.ctx, context.n)
	context.n++
	return seg, nil
}

func (context *context) IsText(t Token) bool {
	switch {
	case context.IsBEG(t):
		return false
	case context.IsSOT(t):
		return false
	case whisper.Token(t.Id) >= context.model.ctx.Whisper_token_eot():
		return false
	case context.IsPREV(t):
		return false
	case context.IsSOLM(t):
		return false
	case context.IsNOT(t):
		return false
	default:
		return true
	}
}

func (context *context) IsBEG(t Token) bool {
	return whisper.Token(t.Id) == context.model.ctx.Whisper_token_beg()
}

func (context *context) IsSOT(t Token) bool {
	return whisper.Token(t.Id) == context.model.ctx.Whisper_token_sot()
}

func (context *context) IsEOT(t Token) bool {
	return whisper.Token(t.Id) == context.model.ctx.Whisper_token_eot()
}

func (context *context) IsPREV(t Token) bool {
	return whisper.Token(t.Id) == context.model.ctx.Whisper_token_prev()
}

func (context *context) IsSOLM(t Token) bool {
	return whisper.Token(t.Id) == context.model.ctx.Whisper_token_solm()
}

func (context *context) IsNOT(t Token) bool {
	return whisper.Token(t.Id) == context.model.ctx.Whisper_token_not()
}

func (context *context) IsLANG(t Token, lang string) bool {
	id := context.model.ctx.Whisper_lang_id(lang)
	if id < 0 {
		return false
	}
	return whisper.Token(t.Id) == context.model.ctx.Whisper_token_lang(id)
}

func toSegment(ctx *whisper.Context, n int) Segment {
	return Segment{
		Num:             n,
		Text:            strings.TrimSpace(ctx.Whisper_full_get_segment_text(n)),
		Start:           time.Duration(ctx.Whisper_full_get_segment_t0(n)) * time.Millisecond * 10,
		End:             time.Duration(ctx.Whisper_full_get_segment_t1(n)) * time.Millisecond * 10,
		SpeakerTurnNext: ctx.Whisper_full_get_segment_speaker_turn_next(n),
		Tokens:          toTokens(ctx, n),
	}
}

func toTokens(ctx *whisper.Context, n int) []Token {
	result := make([]Token, ctx.Whisper_full_n_tokens(n))
	for i := 0; i < len(result); i++ {
		data := ctx.Whisper_full_get_token_data(n, i)

		result[i] = Token{
			Id:    int(ctx.Whisper_full_get_token_id(n, i)),
			Text:  ctx.Whisper_full_get_token_text(n, i),
			P:     ctx.Whisper_full_get_token_p(n, i),
			Start: time.Duration(data.T0()) * time.Millisecond * 10,
			End:   time.Duration(data.T1()) * time.Millisecond * 10,
		}
	}
	return result
}
