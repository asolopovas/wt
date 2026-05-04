package transcriber

import (
	"context"
	"fmt"

	shared "github.com/asolopovas/wt/internal"
	"github.com/asolopovas/wt/internal/diarizer"
)

func (j *Job) runASR(
	ctx context.Context,
	spec JobSpec,
	samples []float32,
	audioDurSec float64,
	rawKey string,
) ([]diarizer.TranscriptSegment, string, float64, error) {
	switch resolveEngine(spec.Engine) {
	case shared.EngineParakeet:
		return j.runParakeet(ctx, spec, samples, audioDurSec, rawKey)
	case shared.EngineSenseVoice:
		return j.runSenseVoice(ctx, spec, samples, audioDurSec, rawKey)
	case shared.EngineMoonshine:
		return j.runMoonshine(ctx, spec, samples, audioDurSec, rawKey)
	case shared.EngineZipformer:
		return j.runZipformer(ctx, spec, samples, audioDurSec, rawKey)
	case shared.EngineWhisperONNX:
		return j.runWhisperONNX(ctx, spec, samples, audioDurSec, rawKey)
	case shared.EngineCanary:
		return j.runCanary(ctx, spec, samples, audioDurSec, rawKey)
	case shared.EngineNemoCTC:
		return j.runNemoCTC(ctx, spec, samples, audioDurSec, rawKey)
	default:
		return nil, "", 0, fmt.Errorf("unknown engine %q (valid: %s, %s, %s, %s, %s, %s, %s)",
			spec.Engine, shared.EngineWhisperONNX, shared.EngineParakeet, shared.EngineSenseVoice, shared.EngineMoonshine, shared.EngineZipformer, shared.EngineCanary, shared.EngineNemoCTC)
	}
}

func resolveEngine(name string) string {
	switch name {
	case "", "whisper":
		return shared.EngineWhisperONNX
	default:
		return name
	}
}
