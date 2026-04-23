package transcriber

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/asolopovas/wt/internal/diarizer"
	whisper "github.com/ggerganov/whisper.cpp/bindings/go/pkg/whisper"
)

func OutputFilename(inputBase, model string) string {
	stamp := time.Now().Format("2006-01-02_150405")
	return fmt.Sprintf("%s_%s_%s.json", inputBase, model, stamp)
}

type Transcript struct {
	Model            string      `json:"model"`
	Language         string      `json:"language"`
	DurationMs       int64       `json:"duration_ms"`
	Diarizer         string      `json:"diarizer,omitempty"`
	Device           string      `json:"device,omitempty"`
	SpeakersDetected int         `json:"speakers_detected"`
	Utterances       []Utterance `json:"utterances"`
	Words            []Word      `json:"words"`
}

type Utterance struct {
	Start   int64  `json:"start"`
	End     int64  `json:"end"`
	Speaker string `json:"speaker"`
	Text    string `json:"text"`
}

type Word struct {
	Text       string  `json:"text"`
	Start      int64   `json:"start"`
	End        int64   `json:"end"`
	Speaker    string  `json:"speaker"`
	Confidence float32 `json:"confidence"`
}

func msFromDuration(d time.Duration) int64 {
	return d.Milliseconds()
}

func ExtractSegments(ctx whisper.Context) []diarizer.TranscriptSegment {
	var segs []diarizer.TranscriptSegment
	for {
		segment, err := ctx.NextSegment()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			break
		}

		var tokens []diarizer.TokenData
		for _, tok := range segment.Tokens {
			if !ctx.IsText(tok) {
				continue
			}
			tokens = append(tokens, diarizer.TokenData{
				Text:  tok.Text,
				Start: tok.Start,
				End:   tok.End,
				P:     tok.P,
			})
		}

		segs = append(segs, diarizer.TranscriptSegment{
			Start:  segment.Start,
			End:    segment.End,
			Text:   segment.Text,
			Tokens: tokens,
		})
	}
	return segs
}

type TranscriptMeta struct {
	Model      string
	Language   string
	DurationMs int64
	Diarizer   string
	Device     string
}

func BuildTranscript(segs []diarizer.TranscriptSegment, diarSegs []diarizer.Segment, diarOK bool, meta TranscriptMeta) *Transcript {
	labels := diarizer.SpeakerLabels(diarSegs)

	var utterances []Utterance
	var words []Word
	speakers := make(map[string]struct{})

	for _, seg := range segs {
		segSpeaker := assignSpeaker(seg.Start, seg.End, diarSegs, labels, diarOK)

		utterances = append(utterances, Utterance{
			Start:   msFromDuration(seg.Start),
			End:     msFromDuration(seg.End),
			Speaker: segSpeaker,
			Text:    seg.Text,
		})
		speakers[segSpeaker] = struct{}{}

		for _, tok := range seg.Tokens {
			tokSpeaker := assignSpeaker(tok.Start, tok.End, diarSegs, labels, diarOK)
			words = append(words, Word{
				Text:       tok.Text,
				Start:      msFromDuration(tok.Start),
				End:        msFromDuration(tok.End),
				Speaker:    tokSpeaker,
				Confidence: tok.P,
			})
			speakers[tokSpeaker] = struct{}{}
		}
	}

	return &Transcript{
		Model:            meta.Model,
		Language:         meta.Language,
		DurationMs:       meta.DurationMs,
		Diarizer:         meta.Diarizer,
		Device:           meta.Device,
		SpeakersDetected: len(speakers),
		Utterances:       utterances,
		Words:            words,
	}
}

func assignSpeaker(start, end time.Duration, diarSegs []diarizer.Segment, labels map[int]string, diarOK bool) string {
	if !diarOK || len(diarSegs) == 0 {
		return "SPEAKER_01"
	}
	return diarizer.SpeakerForTime(start.Seconds(), end.Seconds(), diarSegs, labels)
}

func WriteJSON(outputPath string, t *Transcript) (string, error) {
	f, err := os.Create(outputPath)
	if err != nil {
		ext := filepath.Ext(outputPath)
		base := strings.TrimSuffix(outputPath, ext)
		stamp := time.Now().Format("06-01-02-150405")
		outputPath = base + "-" + stamp + ext
		f, err = os.Create(outputPath)
		if err != nil {
			return "", fmt.Errorf("creating output file: %w", err)
		}
	}

	bw := bufio.NewWriterSize(f, 64*1024)
	enc := json.NewEncoder(bw)
	enc.SetIndent("", "  ")

	if err := enc.Encode(t); err != nil {
		_ = f.Close()
		return "", fmt.Errorf("writing JSON: %w", err)
	}

	if err := bw.Flush(); err != nil {
		_ = f.Close()
		return "", fmt.Errorf("flushing buffer: %w", err)
	}

	if err := f.Close(); err != nil {
		return "", fmt.Errorf("closing output file: %w", err)
	}

	return outputPath, nil
}
