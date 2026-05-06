package transcriber

import (
	"os"
	"strconv"
	"strings"
	"unicode"

	"github.com/asolopovas/wt/internal/diarizer"
)

const (
	dedupMaxNgram      = 5
	dedupMinRunByN1    = 4
	dedupMinRunByN2    = 3
	dedupMinRunByLarge = 2
)

func dedupRepeatsEnabled() bool {
	return os.Getenv("WT_NO_REPEAT_DEDUP") == ""
}

func minRunForNgram(n int) int {
	switch n {
	case 1:
		return dedupMinRunByN1
	case 2:
		return dedupMinRunByN2
	default:
		return dedupMinRunByLarge
	}
}

func envInt(name string, lo, hi int) (int, bool) {
	v := os.Getenv(name)
	if v == "" {
		return 0, false
	}
	n, err := strconv.Atoi(v)
	if err != nil || n < lo || n > hi {
		return 0, false
	}
	return n, true
}

func maxNgramWindow() int {
	if n, ok := envInt("WT_DEDUP_MAX_NGRAM", 1, 12); ok {
		return n
	}
	return dedupMaxNgram
}

func canonWord(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	for _, r := range s {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(unicode.ToLower(r))
		}
	}
	return b.String()
}

func tokensEqual(a, b []diarizer.TokenData, aOff, bOff, n int) bool {
	if aOff+n > len(a) || bOff+n > len(b) {
		return false
	}
	for i := 0; i < n; i++ {
		if canonWord(a[aOff+i].Text) != canonWord(b[bOff+i].Text) {
			return false
		}
	}
	return true
}

func collapseRepeats(tokens []diarizer.TokenData) []diarizer.TokenData {
	if len(tokens) < 2 {
		return tokens
	}
	maxN := maxNgramWindow()
	out := make([]diarizer.TokenData, 0, len(tokens))
	i := 0
	for i < len(tokens) {
		bestN, bestRuns := 0, 1
		for n := 1; n <= maxN && i+n*2 <= len(tokens); n++ {
			runs := 1
			for j := i + n; j+n <= len(tokens) && tokensEqual(tokens, tokens, i, j, n); j += n {
				runs++
			}
			if runs >= minRunForNgram(n) && (n*runs) > (bestN*bestRuns) {
				bestN, bestRuns = n, runs
			}
		}
		if bestN > 0 {
			out = append(out, tokens[i:i+bestN]...)
			i += bestN * bestRuns
			continue
		}
		out = append(out, tokens[i])
		i++
	}
	return out
}

func collapseCrossSegmentRepeats(segs []diarizer.TranscriptSegment) []diarizer.TranscriptSegment {
	if len(segs) < 2 {
		return segs
	}
	maxN := maxNgramWindow()
	for i := 1; i < len(segs); i++ {
		prev := segs[i-1].Tokens
		cur := segs[i].Tokens
		if len(prev) == 0 || len(cur) == 0 {
			continue
		}
		bestN := 0
		for n := 1; n <= maxN && n <= len(cur) && n <= len(prev); n++ {
			if tokensEqual(prev, cur, len(prev)-n, 0, n) {
				bestN = n
			}
		}
		if bestN > 0 {
			segs[i].Tokens = cur[bestN:]
			segs[i] = rebuildSegmentFromTokens(segs[i])
		}
	}
	return segs
}

func rebuildSegmentFromTokens(seg diarizer.TranscriptSegment) diarizer.TranscriptSegment {
	if len(seg.Tokens) == 0 {
		seg.Text = ""
		return seg
	}
	parts := make([]string, len(seg.Tokens))
	for i, t := range seg.Tokens {
		parts[i] = t.Text
	}
	seg.Text = strings.Join(parts, " ")
	seg.Start = seg.Tokens[0].Start
	seg.End = seg.Tokens[len(seg.Tokens)-1].End
	return seg
}

func collapseRepeatsInText(text string) string {
	words := strings.Fields(text)
	if len(words) < 2 {
		return text
	}
	synth := make([]diarizer.TokenData, len(words))
	for i, w := range words {
		synth[i] = diarizer.TokenData{Text: w}
	}
	collapsed := collapseRepeats(synth)
	if len(collapsed) == len(synth) {
		return text
	}
	parts := make([]string, len(collapsed))
	for i, t := range collapsed {
		parts[i] = t.Text
	}
	return strings.Join(parts, " ")
}

func DedupRepeatedNgrams(segs []diarizer.TranscriptSegment) []diarizer.TranscriptSegment {
	if !dedupRepeatsEnabled() || len(segs) == 0 {
		return segs
	}
	for i := range segs {
		if len(segs[i].Tokens) >= 2 {
			collapsed := collapseRepeats(segs[i].Tokens)
			if len(collapsed) != len(segs[i].Tokens) {
				segs[i].Tokens = collapsed
				segs[i] = rebuildSegmentFromTokens(segs[i])
			}
		} else if t := strings.TrimSpace(segs[i].Text); t != "" {
			segs[i].Text = collapseRepeatsInText(t)
		}
	}
	out := make([]diarizer.TranscriptSegment, 0, len(segs))
	for _, s := range segs {
		if len(s.Tokens) > 0 || strings.TrimSpace(s.Text) != "" {
			out = append(out, s)
		}
	}
	return collapseCrossSegmentRepeats(out)
}
