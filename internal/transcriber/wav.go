package transcriber

import (
	"bufio"
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

const pcmStreamBlockBytes = 1 << 20

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
			return streamPCMToFloat32(f, numSamples)

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
	for i := 0; i < numSamples; i++ {
		sample := int16(binary.LittleEndian.Uint16(buf[i*2:]))
		samples[i] = float32(sample) * scale
	}
	return samples
}

func streamPCMToFloat32(r io.Reader, numSamples int) ([]float32, error) {
	const scale = 1.0 / float32(math.MaxInt16)
	samples := make([]float32, numSamples)
	block := pcmStreamBlockBytes
	if block%2 != 0 {
		block--
	}
	buf := make([]byte, block)
	idx := 0
	for idx < numSamples {
		want := (numSamples - idx) * 2
		if want > len(buf) {
			want = len(buf)
		}
		n, err := io.ReadFull(r, buf[:want])
		if err != nil && !(errors.Is(err, io.ErrUnexpectedEOF) && n > 0) {
			return nil, fmt.Errorf("reading PCM data: %w", err)
		}
		n -= n % 2
		for i := 0; i < n; i += 2 {
			s := int16(binary.LittleEndian.Uint16(buf[i:]))
			samples[idx] = float32(s) * scale
			idx++
		}
		if n == 0 {
			break
		}
	}
	return samples[:idx], nil
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

	bw := bufio.NewWriterSize(f, 256*1024)

	var hdr [44]byte
	copy(hdr[0:4], "RIFF")
	binary.LittleEndian.PutUint32(hdr[4:8], chunkSize)
	copy(hdr[8:16], "WAVEfmt ")
	binary.LittleEndian.PutUint32(hdr[16:20], 16)
	binary.LittleEndian.PutUint16(hdr[20:22], 1)
	binary.LittleEndian.PutUint16(hdr[22:24], uint16(numChannels))
	binary.LittleEndian.PutUint32(hdr[24:28], uint32(sampleRate))
	binary.LittleEndian.PutUint32(hdr[28:32], uint32(byteRate))
	binary.LittleEndian.PutUint16(hdr[32:34], uint16(blockAlign))
	binary.LittleEndian.PutUint16(hdr[34:36], uint16(bitsPerSample))
	copy(hdr[36:40], "data")
	binary.LittleEndian.PutUint32(hdr[40:44], dataSize)
	if _, err := bw.Write(hdr[:]); err != nil {
		return err
	}

	const batch = 4096
	buf := make([]byte, batch*2)
	for i := 0; i < len(samples); i += batch {
		end := i + batch
		if end > len(samples) {
			end = len(samples)
		}
		n := 0
		for _, s := range samples[i:end] {
			v := s * 32767
			if v > 32767 {
				v = 32767
			} else if v < -32768 {
				v = -32768
			}
			binary.LittleEndian.PutUint16(buf[n:], uint16(int16(v)))
			n += 2
		}
		if _, err := bw.Write(buf[:n]); err != nil {
			return err
		}
	}
	return bw.Flush()
}
