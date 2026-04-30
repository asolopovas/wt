//go:build android

package transcriber

/*
#cgo android LDFLAGS: -lmediandk -llog

#include <stdlib.h>
#include <stdint.h>
#include <string.h>
#include <media/NdkMediaExtractor.h>
#include <media/NdkMediaCodec.h>
#include <media/NdkMediaFormat.h>

static int decode_audio_to_pcm(int fd, off64_t offset, off64_t length,
	int16_t **out_samples, int *out_sample_rate,
	int *out_channels, int *out_num_samples) {

	*out_samples = NULL;
	*out_num_samples = 0;
	*out_sample_rate = 0;
	*out_channels = 0;

	AMediaExtractor *ex = AMediaExtractor_new();
	if (!ex) return -1;

	if (AMediaExtractor_setDataSourceFd(ex, fd, offset, length) != AMEDIA_OK) {
		AMediaExtractor_delete(ex);
		return -1;
	}

	int audio_track = -1;
	AMediaFormat *track_fmt = NULL;
	size_t num_tracks = AMediaExtractor_getTrackCount(ex);

	for (size_t i = 0; i < num_tracks; i++) {
		AMediaFormat *fmt = AMediaExtractor_getTrackFormat(ex, i);
		const char *mime = NULL;
		AMediaFormat_getString(fmt, AMEDIAFORMAT_KEY_MIME, &mime);
		if (mime && strncmp(mime, "audio/", 6) == 0) {
			audio_track = (int)i;
			track_fmt = fmt;
			break;
		}
		AMediaFormat_delete(fmt);
	}

	if (audio_track < 0) {
		AMediaExtractor_delete(ex);
		return -2;
	}

	AMediaExtractor_selectTrack(ex, (size_t)audio_track);

	const char *mime = NULL;
	AMediaFormat_getString(track_fmt, AMEDIAFORMAT_KEY_MIME, &mime);

	AMediaCodec *codec = AMediaCodec_createDecoderByType(mime);
	if (!codec) {
		AMediaFormat_delete(track_fmt);
		AMediaExtractor_delete(ex);
		return -3;
	}

	if (AMediaCodec_configure(codec, track_fmt, NULL, NULL, 0) != AMEDIA_OK) {
		AMediaCodec_delete(codec);
		AMediaFormat_delete(track_fmt);
		AMediaExtractor_delete(ex);
		return -4;
	}

	if (AMediaCodec_start(codec) != AMEDIA_OK) {
		AMediaCodec_delete(codec);
		AMediaFormat_delete(track_fmt);
		AMediaExtractor_delete(ex);
		return -5;
	}

	size_t pcm_capacity = 1024 * 1024;
	int16_t *pcm_buf = (int16_t *)malloc(pcm_capacity * sizeof(int16_t));
	if (!pcm_buf) {
		AMediaCodec_stop(codec);
		AMediaCodec_delete(codec);
		AMediaFormat_delete(track_fmt);
		AMediaExtractor_delete(ex);
		return -6;
	}

	size_t pcm_len = 0;
	int input_finished = 0;
	int output_finished = 0;
	int decoded_sample_rate = 0;
	int decoded_channels = 0;

	while (!output_finished) {
		if (!input_finished) {
			ssize_t in_idx = AMediaCodec_dequeueInputBuffer(codec, 5000);
			if (in_idx >= 0) {
				size_t in_size = 0;
				uint8_t *in_buf = AMediaCodec_getInputBuffer(codec, in_idx, &in_size);
				ssize_t sample_size = AMediaExtractor_readSampleData(ex, in_buf, in_size);
				if (sample_size < 0) {
					AMediaCodec_queueInputBuffer(codec, in_idx, 0, 0, 0,
						AMEDIACODEC_BUFFER_FLAG_END_OF_STREAM);
					input_finished = 1;
				} else {
					int64_t pts = AMediaExtractor_getSampleTime(ex);
					AMediaCodec_queueInputBuffer(codec, in_idx, 0, (size_t)sample_size, (uint64_t)pts, 0);
					AMediaExtractor_advance(ex);
				}
			}
		}

		AMediaCodecBufferInfo info;
		memset(&info, 0, sizeof(info));
		ssize_t out_idx = AMediaCodec_dequeueOutputBuffer(codec, &info, 5000);

		if (out_idx >= 0) {
			if (info.size > 0) {
				size_t out_size = 0;
				uint8_t *out_buf = AMediaCodec_getOutputBuffer(codec, out_idx, &out_size);
				size_t num_int16 = (size_t)info.size / sizeof(int16_t);

				while (pcm_len + num_int16 > pcm_capacity) {
					pcm_capacity *= 2;
					int16_t *new_buf = (int16_t *)realloc(pcm_buf, pcm_capacity * sizeof(int16_t));
					if (!new_buf) {
						free(pcm_buf);
						AMediaCodec_releaseOutputBuffer(codec, out_idx, 0);
						AMediaCodec_stop(codec);
						AMediaCodec_delete(codec);
						AMediaFormat_delete(track_fmt);
						AMediaExtractor_delete(ex);
						return -6;
					}
					pcm_buf = new_buf;
				}
				memcpy(pcm_buf + pcm_len, out_buf + info.offset, (size_t)info.size);
				pcm_len += num_int16;
			}

			AMediaCodec_releaseOutputBuffer(codec, out_idx, 0);

			if (info.flags & AMEDIACODEC_BUFFER_FLAG_END_OF_STREAM) {
				output_finished = 1;
			}
		} else if (out_idx == AMEDIACODEC_INFO_OUTPUT_FORMAT_CHANGED) {
			AMediaFormat *out_fmt = AMediaCodec_getOutputFormat(codec);
			if (out_fmt) {
				AMediaFormat_getInt32(out_fmt, AMEDIAFORMAT_KEY_SAMPLE_RATE, &decoded_sample_rate);
				AMediaFormat_getInt32(out_fmt, AMEDIAFORMAT_KEY_CHANNEL_COUNT, &decoded_channels);
				AMediaFormat_delete(out_fmt);
			}
		}
	}

	AMediaCodec_stop(codec);
	AMediaCodec_delete(codec);
	AMediaFormat_delete(track_fmt);
	AMediaExtractor_delete(ex);

	if (pcm_len == 0 || decoded_sample_rate <= 0 || decoded_channels <= 0) {
		free(pcm_buf);
		return -7;
	}

	*out_samples = pcm_buf;
	*out_sample_rate = decoded_sample_rate;
	*out_channels = decoded_channels;
	*out_num_samples = (int)pcm_len;
	return (int)pcm_len;
}

static int64_t probe_audio_duration_us(int fd, off64_t offset, off64_t length) {
	AMediaExtractor *ex = AMediaExtractor_new();
	if (!ex) return -1;
	if (AMediaExtractor_setDataSourceFd(ex, fd, offset, length) != AMEDIA_OK) {
		AMediaExtractor_delete(ex);
		return -1;
	}
	int64_t duration_us = -1;
	size_t num_tracks = AMediaExtractor_getTrackCount(ex);
	for (size_t i = 0; i < num_tracks; i++) {
		AMediaFormat *fmt = AMediaExtractor_getTrackFormat(ex, i);
		const char *mime = NULL;
		AMediaFormat_getString(fmt, AMEDIAFORMAT_KEY_MIME, &mime);
		if (mime && strncmp(mime, "audio/", 6) == 0) {
			int64_t d = 0;
			if (AMediaFormat_getInt64(fmt, AMEDIAFORMAT_KEY_DURATION, &d) && d > 0) {
				duration_us = d;
			}
			AMediaFormat_delete(fmt);
			break;
		}
		AMediaFormat_delete(fmt);
	}
	AMediaExtractor_delete(ex);
	return duration_us;
}
*/
import "C"

import (
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strings"
	"unsafe"
)

func findFFmpeg() string {
	return ""
}

func ProbeDurationMs(path string) int64 {
	f, err := os.Open(path)
	if err != nil {
		return 0
	}
	defer func() { _ = f.Close() }()
	info, err := f.Stat()
	if err != nil {
		return 0
	}
	us := C.probe_audio_duration_us(C.int(f.Fd()), C.off64_t(0), C.off64_t(info.Size()))
	if us <= 0 {
		return 0
	}
	return int64(us) / 1000
}

func LoadAudioSamples(path string) ([]float32, error) {
	if _, err := os.Stat(path); err != nil {
		return nil, fmt.Errorf("audio file not found: %w", err)
	}

	if strings.EqualFold(filepath.Ext(path), ".wav") {
		samples, err := readPCM16WAV(path)
		if err == nil {
			return samples, nil
		}
	}

	return ndkDecodeAndResample(path)
}

func ndkDecodeAndResample(path string) ([]float32, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("opening file: %w", err)
	}
	defer func() {
		_ = f.Close()
	}()

	info, err := f.Stat()
	if err != nil {
		return nil, fmt.Errorf("stat file: %w", err)
	}

	var samples *C.int16_t
	var sampleRate, channels, numSamples C.int

	ret := C.decode_audio_to_pcm(
		C.int(f.Fd()),
		C.off64_t(0),
		C.off64_t(info.Size()),
		&samples, &sampleRate, &channels, &numSamples,
	)
	if ret < 0 {
		return nil, fmt.Errorf("NDK audio decode failed (code %d) for %s", int(ret), filepath.Base(path))
	}
	defer C.free(unsafe.Pointer(samples))

	n := int(numSamples)
	ch := int(channels)
	sr := int(sampleRate)

	pcm := unsafe.Slice((*int16)(unsafe.Pointer(samples)), n)

	frames := n / ch
	mono := make([]float32, frames)
	for i := range frames {
		var sum float64
		for c := range ch {
			sum += float64(pcm[i*ch+c])
		}
		mono[i] = float32(sum / float64(ch) / float64(math.MaxInt16))
	}

	if sr == WhisperSampleRate {
		return mono, nil
	}

	outLen := int(math.Round(float64(len(mono)) * float64(WhisperSampleRate) / float64(sr)))
	if outLen <= 0 {
		return nil, fmt.Errorf("invalid resample output length")
	}

	out := make([]float32, outLen)
	scale := float64(sr) / float64(WhisperSampleRate)
	for i := range outLen {
		pos := float64(i) * scale
		i0 := int(pos)
		if i0 >= len(mono)-1 {
			out[i] = mono[len(mono)-1]
			continue
		}
		frac := float32(pos - float64(i0))
		out[i] = mono[i0] + (mono[i0+1]-mono[i0])*frac
	}

	return out, nil
}
