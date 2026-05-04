//go:build android

package transcriber

import (
	"context"
	"fmt"
)

func StreamingEnabled() bool { return false }

type StreamOptions struct {
	ChunkSamples int
	CacheWAVPath string
}

type AudioStream struct{}

func OpenAudioStream(_ context.Context, _ string, _ StreamOptions) (*AudioStream, error) {
	return nil, fmt.Errorf("audio streaming not supported on android")
}

func (s *AudioStream) Next() ([]float32, error) { return nil, fmt.Errorf("not supported") }
func (s *AudioStream) Close() error             { return nil }
func (s *AudioStream) Abort()                   {}
func (s *AudioStream) Duration() float64        { return 0 }
func (s *AudioStream) EmittedSeconds() float64  { return 0 }
