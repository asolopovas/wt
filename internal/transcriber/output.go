package transcriber

import (
	"fmt"
	"strings"
	"time"

	"github.com/asolopovas/wt/internal/diarizer"
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

const sentenceEndingPunct = ".?!"

func isSentenceEnd(text string) bool {
	t := strings.TrimRight(text, "\"')]}”’")
	if t == "" {
		return false
	}
	return strings.ContainsRune(sentenceEndingPunct, rune(t[len(t)-1]))
}

func smoothIsolatedFlickers(words []Word) {
	n := len(words)
	if n < 3 {
		return
	}

	for i := 1; i < n-1; i++ {
		if words[i].Speaker != words[i-1].Speaker &&
			words[i-1].Speaker == words[i+1].Speaker {
			words[i].Speaker = words[i-1].Speaker
		}
	}

	for i := 1; i < n-2; i++ {
		if words[i].Speaker != words[i-1].Speaker &&
			words[i].Speaker == words[i+1].Speaker &&
			words[i-1].Speaker == words[i+2].Speaker {
			words[i].Speaker = words[i-1].Speaker
			words[i+1].Speaker = words[i-1].Speaker
			i++
		}
	}
}

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
