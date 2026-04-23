package transcriber

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
)

const WhisperSampleRate = 16000

func readPCM16WAV(path string) ([]float32, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = f.Close()
	}()

	var header [12]byte
	if _, err := io.ReadFull(f, header[:]); err != nil {
		return nil, fmt.Errorf("reading RIFF header: %w", err)
	}
	if string(header[:4]) != "RIFF" {
		return nil, fmt.Errorf("not a RIFF file")
	}
	if string(header[8:12]) != "WAVE" {
		return nil, fmt.Errorf("not a WAVE file")
	}

	var (
		sampleRate    uint32
		numChannels   uint16
		bitsPerSample uint16
		audioFormat   uint16
		foundFmt      bool
	)

	for {
		var chunkHeader [8]byte
		if _, err := io.ReadFull(f, chunkHeader[:]); err != nil {
			if errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) {
				break
			}
			return nil, err
		}

		chunkID := string(chunkHeader[:4])
		chunkSize := binary.LittleEndian.Uint32(chunkHeader[4:])

		switch chunkID {
		case "fmt ":
			if chunkSize < 16 {
				return nil, fmt.Errorf("fmt chunk too small: %d", chunkSize)
			}
			var fmtData [16]byte
			if _, err := io.ReadFull(f, fmtData[:]); err != nil {
				return nil, fmt.Errorf("reading fmt chunk: %w", err)
			}
			audioFormat = binary.LittleEndian.Uint16(fmtData[0:2])
			numChannels = binary.LittleEndian.Uint16(fmtData[2:4])
			sampleRate = binary.LittleEndian.Uint32(fmtData[4:8])
			bitsPerSample = binary.LittleEndian.Uint16(fmtData[14:16])

			if remaining := int64(chunkSize) - 16; remaining > 0 {
				if _, err := f.Seek(remaining, io.SeekCurrent); err != nil {
					return nil, err
				}
			}
			foundFmt = true

		case "data":
			if !foundFmt {
				return nil, fmt.Errorf("data chunk before fmt chunk")
			}
			if audioFormat != 1 {
				return nil, fmt.Errorf("not PCM format (format=%d)", audioFormat)
			}
			if sampleRate != WhisperSampleRate {
				return nil, fmt.Errorf("wrong sample rate: %d", sampleRate)
			}
			if numChannels != 1 {
				return nil, fmt.Errorf("not mono: %d channels", numChannels)
			}
			if bitsPerSample != 16 {
				return nil, fmt.Errorf("not 16-bit: %d bits", bitsPerSample)
			}

			numSamples := int(chunkSize) / 2
			pcm := make([]byte, chunkSize)
			if _, err := io.ReadFull(f, pcm); err != nil {
				return nil, fmt.Errorf("reading PCM data: %w", err)
			}

			return pcmToFloat32(pcm, numSamples), nil

		default:
			skipSize := int64(chunkSize)
			if skipSize%2 != 0 {
				skipSize++
			}
			if _, err := f.Seek(skipSize, io.SeekCurrent); err != nil {
				return nil, err
			}
		}
	}

	return nil, fmt.Errorf("no data chunk found")
}

func AudioCacheKey(path string) (string, error) {
	info, err := os.Stat(path)
	if err != nil {
		return "", err
	}
	absPath, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	key := fmt.Sprintf("%s|%d|%d", absPath, info.Size(), info.ModTime().UnixNano())
	hash := sha256.Sum256([]byte(key))
	return hex.EncodeToString(hash[:12]) + ".wav", nil
}

func pcmToFloat32(buf []byte, numSamples int) []float32 {
	const scale = 1.0 / float32(math.MaxInt16)
	samples := make([]float32, numSamples)
	for i := range numSamples {
		sample := int16(binary.LittleEndian.Uint16(buf[i*2:]))
		samples[i] = float32(sample) * scale
	}
	return samples
}
