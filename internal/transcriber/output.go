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
			Start:           segment.Start,
			End:             segment.End,
			Text:            segment.Text,
			SpeakerTurnNext: segment.SpeakerTurnNext,
			Tokens:          tokens,
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
	useDiar := diarOK && len(diarSegs) > 0

	type rawWord struct {
		text       string
		start, end time.Duration
		conf       float32
		spkID      int
		hasID      bool
	}

	labels := map[int]string{}
	next := 1
	assign := func(id int, has bool) string {
		if !has {
			return "SPEAKER_01"
		}
		if l, ok := labels[id]; ok {
			return l
		}
		l := fmt.Sprintf("SPEAKER_%02d", next)
		next++
		labels[id] = l
		return l
	}

	// Collect every word across every segment. If a segment has no
	// tokens, treat the whole segment as a single "word" with its full
	// text (this is the Whisper case — sentence-level segments).
	var rawWords []rawWord
	for _, seg := range segs {
		if len(seg.Tokens) > 0 {
			for _, tok := range seg.Tokens {
				w := rawWord{text: tok.Text, start: tok.Start, end: tok.End, conf: tok.P}
				if useDiar {
					w.spkID, w.hasID = diarizer.SpeakerIDForTime(tok.Start.Seconds(), tok.End.Seconds(), diarSegs)
				}
				rawWords = append(rawWords, w)
			}
			continue
		}
		w := rawWord{text: seg.Text, start: seg.Start, end: seg.End}
		if useDiar {
			w.spkID, w.hasID = diarizer.SpeakerIDForTime(seg.Start.Seconds(), seg.End.Seconds(), diarSegs)
		}
		rawWords = append(rawWords, w)
	}

	speakers := make(map[string]struct{})
	words := make([]Word, 0, len(rawWords))
	for _, w := range rawWords {
		lbl := assign(w.spkID, w.hasID)
		words = append(words, Word{
			Text:       w.text,
			Start:      msFromDuration(w.start),
			End:        msFromDuration(w.end),
			Speaker:    lbl,
			Confidence: w.conf,
		})
		speakers[lbl] = struct{}{}
	}

	// Group consecutive same-speaker words into utterances. This is
	// what avoids per-word speaker flicker when the underlying ASR
	// produces word-level segments (sherpa engines): a single contiguous
	// run of "SPEAKER_01" words becomes one utterance instead of N.
	// Whisper inputs have one word per sentence-level segment so this
	// degenerates to the previous one-utterance-per-segment behaviour.
	utterances := groupWordsIntoUtterances(words)

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

// Sentence-final punctuation marks. A word is treated as a sentence
// boundary when stripped of trailing close-quotes/brackets it ends with
// one of these.
const sentenceEndingPunct = ".?!"

func isSentenceEnd(text string) bool {
	t := strings.TrimRight(text, "\"')]}”’")
	if t == "" {
		return false
	}
	return strings.ContainsRune(sentenceEndingPunct, rune(t[len(t)-1]))
}

// realignSpeakersByPunctuation smooths per-word speaker labels using
// sentence boundaries (`.?!`). When a speaker change occurs mid-sentence
// — i.e. the previous word does not end the sentence — the algorithm
// looks at the surrounding sentence (bounded by the previous and next
// sentence-ending words) and takes a majority vote over the speakers
// assigned to those words. If a clear majority exists (more than half
// the sentence), every word in the sentence is relabelled to that
// speaker. This is the key step that prevents per-word flicker around
// speaker turns when the underlying ASR runs at word granularity.
//
// Algorithm ported from MahmoudAshraf97/whisper-diarization helpers.py:
//   get_realigned_ws_mapping_with_punctuation
// The 50-word sentence cap matches the upstream default.
func realignSpeakersByPunctuation(words []Word) {
	const maxWordsPerSentence = 50
	findSentenceLeft := func(idx int) int {
		left := idx
		for left > 0 &&
			idx-left < maxWordsPerSentence &&
			words[left-1].Speaker == words[left].Speaker &&
			!isSentenceEnd(words[left-1].Text) {
			left--
		}
		if left == 0 || isSentenceEnd(words[left-1].Text) {
			return left
		}
		return -1
	}
	findSentenceRight := func(idx, budget int) int {
		right := idx
		for right < len(words)-1 &&
			right-idx < budget &&
			!isSentenceEnd(words[right].Text) {
			right++
		}
		if right == len(words)-1 || isSentenceEnd(words[right].Text) {
			return right
		}
		return -1
	}

	for k := 0; k < len(words); {
		if k >= len(words)-1 ||
			words[k].Speaker == words[k+1].Speaker ||
			isSentenceEnd(words[k].Text) {
			k++
			continue
		}
		left := findSentenceLeft(k)
		if left < 0 {
			k++
			continue
		}
		right := findSentenceRight(k, maxWordsPerSentence-(k-left)-1)
		if right < 0 {
			k++
			continue
		}
		// Majority vote across [left, right].
		counts := map[string]int{}
		for i := left; i <= right; i++ {
			counts[words[i].Speaker]++
		}
		var bestSpk string
		bestN := 0
		for spk, n := range counts {
			if n > bestN {
				bestSpk = spk
				bestN = n
			}
		}
		n := right - left + 1
		if bestN < n/2 {
			// No clear majority — leave labels alone.
			k++
			continue
		}
		for i := left; i <= right; i++ {
			words[i].Speaker = bestSpk
		}
		k = right + 1
	}
}

// groupWordsIntoUtterances merges consecutive same-speaker words into
// utterance-level segments. After punctuation-based realignment, a new
// utterance starts when:
//   • the speaker label changes, OR
//   • the previous word ended a sentence (`.?!`).
//
// This is the Go equivalent of whisper-diarization's
// get_sentences_speaker_mapping (without the NLTK Punkt tokenizer — we
// rely on the punctuation that ASR engines like Whisper / SenseVoice /
// Parakeet emit natively).
func groupWordsIntoUtterances(words []Word) []Utterance {
	if len(words) == 0 {
		return nil
	}
	realignSpeakersByPunctuation(words)

	utts := make([]Utterance, 0, len(words)/4+1)
	curStart := words[0].Start
	curEnd := words[0].End
	curSpk := words[0].Speaker
	curParts := []string{words[0].Text}
	prevSentenceEnd := isSentenceEnd(words[0].Text)

	flush := func() {
		utts = append(utts, Utterance{
			Start:   curStart,
			End:     curEnd,
			Speaker: curSpk,
			Text:    joinWords(curParts),
		})
	}

	for i := 1; i < len(words); i++ {
		w := words[i]
		if w.Speaker != curSpk || prevSentenceEnd {
			flush()
			curStart = w.Start
			curSpk = w.Speaker
			curParts = curParts[:0]
		}
		curEnd = w.End
		curParts = append(curParts, w.Text)
		prevSentenceEnd = isSentenceEnd(w.Text)
	}
	flush()
	return utts
}

// joinWords concatenates a list of word tokens with single spaces, then
// tightens spacing around common punctuation so output reads naturally.
func joinWords(parts []string) string {
	if len(parts) == 0 {
		return ""
	}
	s := strings.Join(parts, " ")
	for _, p := range []string{",", ".", "?", "!", ";", ":"} {
		s = strings.ReplaceAll(s, " "+p, p)
	}
	return s
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
