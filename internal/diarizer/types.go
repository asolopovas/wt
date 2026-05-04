package diarizer

import (
	"math"
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
