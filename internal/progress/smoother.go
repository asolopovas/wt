package progress

import (
	"sync"
	"time"
)

type Smoother struct {
	mu           sync.Mutex
	audioDurSec  float64
	rtf          float64
	lastPct      int
	lastTick     time.Time
	startTime    time.Time
	samples      int
	emaETA       float64
	displayShown float64
}

func NewSmoother(audioDurSec, initialRTF float64) *Smoother {
	if initialRTF <= 0 {
		initialRTF = 1.0
	}
	if audioDurSec <= 0 {
		audioDurSec = 1.0
	}
	now := time.Now()
	return &Smoother{
		audioDurSec: audioDurSec,
		rtf:         initialRTF,
		lastTick:    now,
		startTime:   now,
	}
}

func (s *Smoother) Report(pct int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if pct <= s.lastPct {
		return
	}
	now := time.Now()
	elapsed := now.Sub(s.lastTick).Seconds()
	pctDelta := pct - s.lastPct
	if elapsed > 0 && pctDelta > 0 {
		audioProcessed := float64(pctDelta) / 100.0 * s.audioDurSec
		observedRTF := audioProcessed / elapsed
		s.samples++
		switch {
		case s.samples == 1:
		case s.samples == 2:
			s.rtf = observedRTF
		default:
			s.rtf = 0.6*s.rtf + 0.4*observedRTF
		}
	}
	s.lastPct = pct
	s.lastTick = now
}

func (s *Smoother) Snapshot() (display, etaSec float64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	elapsedSinceTick := time.Since(s.lastTick).Seconds()
	rtf := s.rtf
	if rtf <= 0 {
		rtf = 1
	}
	secPerPct := s.audioDurSec / 100.0 / rtf
	if secPerPct <= 0 {
		secPerPct = 0.1
	}

	chunkPct := 30.0 / s.audioDurSec * 100
	if chunkPct < 0.5 {
		chunkPct = 0.5
	}
	if chunkPct > 50 {
		chunkPct = 50
	}
	maxAdvance := 1.5 * chunkPct
	if maxAdvance > 25 {
		maxAdvance = 25
	}

	predicted := elapsedSinceTick / secPerPct
	if predicted > maxAdvance {
		excess := predicted - maxAdvance
		predicted = maxAdvance + excess*0.1
	}
	display = float64(s.lastPct) + predicted
	if display > 99 {
		display = 99
	}
	if display < s.displayShown {
		display = s.displayShown
	}
	s.displayShown = display

	remainingAudio := s.audioDurSec * (1 - display/100.0)
	if remainingAudio < 0 {
		remainingAudio = 0
	}
	rawETA := remainingAudio / rtf

	if s.emaETA == 0 || rawETA < s.emaETA {
		s.emaETA = rawETA
	} else {
		s.emaETA = 0.7*s.emaETA + 0.3*rawETA
	}
	return display, s.emaETA
}

func (s *Smoother) ObservedRTF() float64 {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.samples < 2 {
		return 0
	}
	return s.rtf
}
