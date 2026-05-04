package progress

import (
	"sync"
	"time"
)

const (
	rtfWindowSize     = 6
	rtfPriorWeight    = 0.7
	rtfSampleAlpha    = 0.35
	displayMaxAdvance = 0.25
)

type rtfSample struct {
	pctDelta float64
	elapsed  float64
}

type Smoother struct {
	mu           sync.Mutex
	audioDurSec  float64
	priorRTF     float64
	rtf          float64
	samples      []rtfSample
	totalSamples int
	lastPct      int
	lastTick     time.Time
	startTime    time.Time
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
		priorRTF:    initialRTF,
		rtf:         initialRTF,
		lastTick:    now,
		startTime:   now,
		samples:     make([]rtfSample, 0, rtfWindowSize),
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
	pctDelta := float64(pct - s.lastPct)
	s.lastPct = pct
	s.lastTick = now

	if elapsed <= 0 || pctDelta <= 0 {
		return
	}

	s.samples = append(s.samples, rtfSample{pctDelta: pctDelta, elapsed: elapsed})
	if len(s.samples) > rtfWindowSize {
		s.samples = s.samples[1:]
	}
	s.totalSamples++

	var totalPct, totalElapsed float64
	for _, sm := range s.samples {
		totalPct += sm.pctDelta
		totalElapsed += sm.elapsed
	}
	if totalElapsed <= 0 {
		return
	}
	windowAudio := totalPct / 100.0 * s.audioDurSec
	windowRTF := windowAudio / totalElapsed

	priorBlend := rtfPriorWeight
	if s.totalSamples >= 3 {
		priorBlend = 0.0
	} else if s.totalSamples == 2 {
		priorBlend = 0.35
	}
	blended := priorBlend*s.priorRTF + (1-priorBlend)*windowRTF

	if s.totalSamples <= 1 {
		s.rtf = blended
	} else {
		s.rtf = (1-rtfSampleAlpha)*s.rtf + rtfSampleAlpha*blended
	}
}

func (s *Smoother) Snapshot() (display, etaSec float64) {
	s.mu.Lock()
	defer s.mu.Unlock()

	rtf := s.rtf
	if rtf <= 0 {
		rtf = s.priorRTF
	}
	if rtf <= 0 {
		rtf = 1
	}

	elapsedSinceTick := time.Since(s.lastTick).Seconds()
	secPerPct := s.audioDurSec / 100.0 / rtf
	if secPerPct <= 0 {
		secPerPct = 0.1
	}
	predicted := elapsedSinceTick / secPerPct
	maxJump := displayMaxAdvance * 100
	if predicted > maxJump {
		predicted = maxJump
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
	etaSec = remainingAudio / rtf
	return display, etaSec
}

func (s *Smoother) Elapsed() time.Duration {
	s.mu.Lock()
	defer s.mu.Unlock()
	return time.Since(s.startTime)
}
