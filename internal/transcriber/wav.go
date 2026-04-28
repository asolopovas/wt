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

func WritePCM16WAV(path string, samples []float32, sampleRate int) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer func() { _ = f.Close() }()

	const numChannels = 1
	const bitsPerSample = 16
	byteRate := sampleRate * numChannels * bitsPerSample / 8
	blockAlign := numChannels * bitsPerSample / 8
	dataSize := uint32(len(samples) * 2)
	chunkSize := 36 + dataSize

	w := f
	write := func(b []byte) error { _, err := w.Write(b); return err }
	u32 := func(v uint32) []byte {
		b := make([]byte, 4)
		binary.LittleEndian.PutUint32(b, v)
		return b
	}
	u16 := func(v uint16) []byte {
		b := make([]byte, 2)
		binary.LittleEndian.PutUint16(b, v)
		return b
	}

	if err := write([]byte("RIFF")); err != nil {
		return err
	}
	if err := write(u32(chunkSize)); err != nil {
		return err
	}
	if err := write([]byte("WAVEfmt ")); err != nil {
		return err
	}
	if err := write(u32(16)); err != nil {
		return err
	}
	if err := write(u16(1)); err != nil {
		return err
	}
	if err := write(u16(uint16(numChannels))); err != nil {
		return err
	}
	if err := write(u32(uint32(sampleRate))); err != nil {
		return err
	}
	if err := write(u32(uint32(byteRate))); err != nil {
		return err
	}
	if err := write(u16(uint16(blockAlign))); err != nil {
		return err
	}
	if err := write(u16(uint16(bitsPerSample))); err != nil {
		return err
	}
	if err := write([]byte("data")); err != nil {
		return err
	}
	if err := write(u32(dataSize)); err != nil {
		return err
	}

	buf := make([]byte, 2)
	for _, s := range samples {
		v := s * 32767
		if v > 32767 {
			v = 32767
		} else if v < -32768 {
			v = -32768
		}
		binary.LittleEndian.PutUint16(buf, uint16(int16(v)))
		if _, err := w.Write(buf); err != nil {
			return err
		}
	}
	return nil
}
