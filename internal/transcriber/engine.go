package transcriber

import (
	"context"
	"fmt"

	shared "github.com/asolopovas/wt/internal"
	"github.com/asolopovas/wt/internal/diarizer"
)

// runASR dispatches to the configured ASR engine.
//
// Engine selection precedence:
//  1. JobSpec.Engine (when set)
//  2. shared.Config.Engine (loaded from config / env WT_ENGINE)
//  3. EngineWhisper (default)
//
// All engines must return segments with timestamps relative to the start of
// the input audio (not chunk-relative), the detected language ("" if unknown),
// and observed RTF (audio_seconds / wall_seconds, 0 if not measured).
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
	case shared.EngineWhisper:
		return j.runWhisper(ctx, spec, samples, audioDurSec, rawKey)
	default:
		return nil, "", 0, fmt.Errorf("unknown engine %q (valid: %s, %s, %s, %s, %s)",
			spec.Engine, shared.EngineWhisper, shared.EngineParakeet, shared.EngineSenseVoice, shared.EngineMoonshine, shared.EngineZipformer)
	}
}

// resolveEngine normalises an engine identifier, falling back to whisper.
func resolveEngine(name string) string {
	switch name {
	case "":
		return shared.EngineWhisper
	default:
		return name
	}
}
