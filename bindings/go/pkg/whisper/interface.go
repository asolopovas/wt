package whisper

import (
	"io"
	"time"
)

type SegmentCallback func(Segment)

type ProgressCallback func(int)

type EncoderBeginCallback func() bool

type Model interface {
	io.Closer

	NewContext() (Context, error)

	IsMultilingual() bool

	Languages() []string
}

type Context interface {
	SetLanguage(string) error
	SetTranslate(bool)
	IsMultilingual() bool
	Language() string
	DetectedLanguage() string

	SetOffset(time.Duration)
	SetDuration(time.Duration)
	SetThreads(uint)
	SetSplitOnWord(bool)
	SetTokenThreshold(float32)
	SetTokenSumThreshold(float32)
	SetMaxSegmentLength(uint)
	SetTokenTimestamps(bool)
	SetMaxTokensPerSegment(uint)
	SetAudioCtx(uint)
	SetMaxContext(n int)
	SetBeamSize(n int)
	SetEntropyThold(t float32)
	SetInitialPrompt(prompt string)
	SetTemperature(t float32)
	SetTemperatureFallback(t float32)

	SetVAD(v bool)
	SetVADModelPath(path string)
	SetVADThreshold(t float32)
	SetVADMinSpeechMs(ms int)
	SetVADMinSilenceMs(ms int)
	SetVADMaxSpeechSec(s float32)
	SetVADSpeechPadMs(ms int)
	SetVADSamplesOverlap(sec float32)

	SetTDRZ(bool)

	SetSuppressBlank(bool)
	SetSuppressNST(bool)
	SetNoSpeechThold(float32)
	SetLogprobThold(float32)

	Process([]float32, EncoderBeginCallback, SegmentCallback, ProgressCallback) error

	NextSegment() (Segment, error)

	IsBEG(Token) bool
	IsSOT(Token) bool
	IsEOT(Token) bool
	IsPREV(Token) bool
	IsSOLM(Token) bool
	IsNOT(Token) bool
	IsLANG(Token, string) bool
	IsText(Token) bool

	PrintTimings()
	ResetTimings()

	SystemInfo() string
}

type Segment struct {
	Num int

	Start, End time.Duration

	Text string

	SpeakerTurnNext bool

	Tokens []Token
}

type Token struct {
	Id         int
	Text       string
	P          float32
	Start, End time.Duration
}
