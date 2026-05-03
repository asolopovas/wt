package diarizer

import (
	"cmp"
	"fmt"
	"math"
	"slices"
	"time"
)

type TokenData struct {
	Text       string
	Start, End time.Duration
	P          float32
}

type TranscriptSegment struct {
	Start           time.Duration
	End             time.Duration
	Text            string
	SpeakerTurnNext bool
	Tokens          []TokenData
}

type SpeakerSegment struct {
	Start   time.Duration
	End     time.Duration
	Speaker string
	Text    string
}

func SpeakerLabels(diarSegs []Segment) map[int]string {
	sorted := sortedCopy(diarSegs)
	labels := make(map[int]string)
	next := 1
	for _, seg := range sorted {
		if _, ok := labels[seg.Speaker]; !ok {
			labels[seg.Speaker] = fmt.Sprintf("SPEAKER_%02d", next)
			next++
		}
	}
	return labels
}

func SpeakerIDForTime(startSec, endSec float64, diarSegs []Segment) (int, bool) {
	return SpeakerIDForTimeWithHint(startSec, endSec, diarSegs, -1)
}

// SpeakerIDForTimeWithHint is like SpeakerIDForTime but breaks ties
// (overlap with multiple speakers within ~5 ms) by preferring the
// `hintSpeaker` (typically the previous word's speaker). Pass -1 to
// disable the hint. This is the standard "continuity bias" used by
// pyannote's word-level speaker assignment to suppress per-word
// flicker around speaker turns: a word straddling two diarizer
// segments stays with the previous word unless the new speaker has a
// clear overlap advantage.
func SpeakerIDForTimeWithHint(startSec, endSec float64, diarSegs []Segment, hintSpeaker int) (int, bool) {
	if len(diarSegs) == 0 {
		return 0, false
	}
	overlapBySpeaker := make(map[int]float64)
	for _, ds := range diarSegs {
		if ds.EndSec <= startSec || ds.StartSec >= endSec {
			continue
		}
		oStart := max(ds.StartSec, startSec)
		oEnd := min(ds.EndSec, endSec)
		if overlap := oEnd - oStart; overlap > 0 {
			overlapBySpeaker[ds.Speaker] += overlap
		}
	}

	if len(overlapBySpeaker) == 0 {
		mid := (startSec + endSec) / 2
		bestDist := math.MaxFloat64
		bestSpk := diarSegs[0].Speaker
		for _, ds := range diarSegs {
			dsMid := (ds.StartSec + ds.EndSec) / 2
			if d := math.Abs(mid - dsMid); d < bestDist {
				bestDist = d
				bestSpk = ds.Speaker
			}
		}
		return bestSpk, true
	}

	// If the hint speaker has overlap and is within 5 ms of the leader,
	// prefer the hint (continuity). Otherwise pick the leader
	// deterministically (lowest speaker ID on exact ties to avoid the
	// nondeterminism of Go's randomised map iteration).
	bestSpk := -1
	bestOvl := -1.0
	for spk, ovl := range overlapBySpeaker {
		if ovl > bestOvl || (ovl == bestOvl && spk < bestSpk) {
			bestOvl = ovl
			bestSpk = spk
		}
	}
	if hintSpeaker >= 0 {
		if hintOvl, ok := overlapBySpeaker[hintSpeaker]; ok && bestOvl-hintOvl < 0.005 {
			return hintSpeaker, true
		}
	}
	return bestSpk, true
}

func SpeakerForTime(startSec, endSec float64, diarSegs []Segment, labels map[int]string) string {
	id, ok := SpeakerIDForTime(startSec, endSec, diarSegs)
	if !ok {
		return "SPEAKER_01"
	}
	if l, ok := labels[id]; ok {
		return l
	}
	return "SPEAKER_01"
}

func MapSegmentsToSpeakers(transcriptSegs []TranscriptSegment, diarSegs []Segment) []SpeakerSegment {
	if len(diarSegs) == 0 {
		result := make([]SpeakerSegment, len(transcriptSegs))
		for i, seg := range transcriptSegs {
			result[i] = SpeakerSegment{
				Start:   seg.Start,
				End:     seg.End,
				Speaker: "SPEAKER_01",
				Text:    seg.Text,
			}
		}
		return result
	}

	sorted := sortedCopy(diarSegs)
	labels := SpeakerLabels(sorted)

	result := make([]SpeakerSegment, len(transcriptSegs))
	for i, tseg := range transcriptSegs {
		speaker := SpeakerForTime(tseg.Start.Seconds(), tseg.End.Seconds(), sorted, labels)
		result[i] = SpeakerSegment{
			Start:   tseg.Start,
			End:     tseg.End,
			Speaker: speaker,
			Text:    tseg.Text,
		}
	}
	return result
}

func SpeakerTurnSegments(transcriptSegs []TranscriptSegment) []Segment {
	if len(transcriptSegs) == 0 {
		return nil
	}

	segments := make([]Segment, 0, len(transcriptSegs))
	speaker := 0
	for _, seg := range transcriptSegs {
		segments = append(segments, Segment{
			Speaker:  speaker,
			StartSec: seg.Start.Seconds(),
			EndSec:   seg.End.Seconds(),
		})
		if seg.SpeakerTurnNext {
			speaker = (speaker + 1) % 2
		}
	}

	return segments
}

func sortedCopy(diarSegs []Segment) []Segment {
	sorted := make([]Segment, len(diarSegs))
	copy(sorted, diarSegs)
	slices.SortFunc(sorted, func(a, b Segment) int {
		return cmp.Compare(a.StartSec, b.StartSec)
	})
	return sorted
}
