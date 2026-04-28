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

func SpeakerForTime(startSec, endSec float64, diarSegs []Segment, labels map[int]string) string {
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
		bestSpk := 0
		for _, ds := range diarSegs {
			dsMid := (ds.StartSec + ds.EndSec) / 2
			if d := math.Abs(mid - dsMid); d < bestDist {
				bestDist = d
				bestSpk = ds.Speaker
			}
		}
		if l, ok := labels[bestSpk]; ok {
			return l
		}
		return "SPEAKER_01"
	}

	bestSpk := 0
	bestOvl := 0.0
	for spk, ovl := range overlapBySpeaker {
		if ovl > bestOvl {
			bestOvl = ovl
			bestSpk = spk
		}
	}
	if l, ok := labels[bestSpk]; ok {
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
