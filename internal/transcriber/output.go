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

// coalesceWhisperTokens merges whisper.cpp BPE sub-word pieces into
// word-level TokenData. Whisper's tokenizer marks word boundaries by
// prefixing the first piece of each word with a space (e.g.
//   " Good", " morning", " I", "'m", " O", "'D", "on", "nell",
//   " F", "ul", "ham")
// Continuation pieces (no leading space) are glued onto the previous
// word, extending its End. Without this, downstream joinWords inserts
// spaces inside words and contractions, producing output like
// "O 'D on nell" / "I 'm" / "F ul ham" / double-spaced word gaps.
// Mirrors coalesceTokens in engine_zipformer.go.
func coalesceWhisperTokens(toks []whisper.Token) []diarizer.TokenData {
	if len(toks) == 0 {
		return nil
	}
	out := make([]diarizer.TokenData, 0, len(toks))
	for _, tok := range toks {
		piece := tok.Text
		if piece == "" {
			continue
		}
		isBoundary := len(out) == 0 || strings.HasPrefix(piece, " ")
		piece = strings.TrimPrefix(piece, " ")
		if piece == "" {
			continue
		}
		if isBoundary {
			out = append(out, diarizer.TokenData{
				Text:  piece,
				Start: tok.Start,
				End:   tok.End,
				P:     tok.P,
			})
			continue
		}
		last := &out[len(out)-1]
		last.Text += piece
		last.End = tok.End
		if tok.P > 0 && (last.P == 0 || tok.P < last.P) {
			last.P = tok.P
		}
	}
	return out
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

		textToks := make([]whisper.Token, 0, len(segment.Tokens))
		for _, tok := range segment.Tokens {
			if !ctx.IsText(tok) {
				continue
			}
			textToks = append(textToks, tok)
		}
		tokens := coalesceWhisperTokens(textToks)

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
	//
	// Speaker assignment uses a continuity hint (previous word's
	// speaker) so per-word flicker on close ties is suppressed. This is
	// the standard pyannote/whisperX trick.
	var rawWords []rawWord
	hint := -1
	lookup := func(start, end time.Duration) (int, bool) {
		if !useDiar {
			return 0, false
		}
		id, ok := diarizer.SpeakerIDForTimeWithHint(start.Seconds(), end.Seconds(), diarSegs, hint)
		if ok {
			hint = id
		}
		return id, ok
	}
	for _, seg := range segs {
		if len(seg.Tokens) > 0 {
			for _, tok := range seg.Tokens {
				w := rawWord{text: tok.Text, start: tok.Start, end: tok.End, conf: tok.P}
				w.spkID, w.hasID = lookup(tok.Start, tok.End)
				rawWords = append(rawWords, w)
			}
			continue
		}
		w := rawWord{text: seg.Text, start: seg.Start, end: seg.End}
		w.spkID, w.hasID = lookup(seg.Start, seg.End)
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

// smoothIsolatedFlickers cleans single-word speaker flickers — a word
// labelled X sandwiched between two words both labelled Y gets
// relabelled Y. This is the most conservative form of speaker smoothing
// and is what we use instead of the upstream punctuation-based
// majority-vote algorithm (MahmoudAshraf97/whisper-diarization). The
// majority-vote variant works well with Whisper's sentence-level
// punctuation but is too aggressive with sherpa-onnx engines
// (SenseVoice/Parakeet) which often emit long un-punctuated runs: a
// single sentence can span 26+ words including a full speaker turn,
// causing the minority speaker to be folded into the dominant one.
//
// Two-word flickers (X X surrounded by Y Y) are also smoothed because
// pyannote occasionally produces a brief 2-frame artefact at speaker
// boundaries. Three-or-more-word runs are left alone — we trust those.
func smoothIsolatedFlickers(words []Word) {
	n := len(words)
	if n < 3 {
		return
	}
	// Single-word flicker: w[i-1] == w[i+1] != w[i]
	for i := 1; i < n-1; i++ {
		if words[i].Speaker != words[i-1].Speaker &&
			words[i-1].Speaker == words[i+1].Speaker {
			words[i].Speaker = words[i-1].Speaker
		}
	}
	// Two-word flicker: w[i-1] == w[i+2] != w[i] == w[i+1]
	for i := 1; i < n-2; i++ {
		if words[i].Speaker != words[i-1].Speaker &&
			words[i].Speaker == words[i+1].Speaker &&
			words[i-1].Speaker == words[i+2].Speaker {
			words[i].Speaker = words[i-1].Speaker
			words[i+1].Speaker = words[i-1].Speaker
			i++ // skip the partner we just relabelled
		}
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
	smoothIsolatedFlickers(words)

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
