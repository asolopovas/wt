package shared

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

type DownloadProgress func(downloaded, total int64)

func DownloadFile(dst, url string, prog DownloadProgress) error {
	const maxReadAttempts = 8
	const maxDialAttempts = 30
	tmp := dst + ".part"

	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}

	client := &http.Client{Timeout: 0}

	var totalSize int64
	var lastErr error
	readAttempt := 0
	dialAttempt := 0

	for {
		if readAttempt >= maxReadAttempts || dialAttempt >= maxDialAttempts {
			break
		}

		var offset int64
		if st, err := os.Stat(tmp); err == nil {
			offset = st.Size()
		}

		req, err := http.NewRequest("GET", url, nil)
		if err != nil {
			return err
		}
		if offset > 0 {
			req.Header.Set("Range", fmt.Sprintf("bytes=%d-", offset))
		}

		resp, err := client.Do(req)
		if err != nil {
			dialAttempt++
			lastErr = err
			if prog != nil {
				prog(-int64(dialAttempt), 0)
			}
			sleep := time.Duration(dialAttempt) * 2 * time.Second
			if sleep > 15*time.Second {
				sleep = 15 * time.Second
			}
			time.Sleep(sleep)
			continue
		}

		if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusPartialContent {
			_ = resp.Body.Close()
			return fmt.Errorf("HTTP %d: %s", resp.StatusCode, resp.Status)
		}

		if resp.StatusCode == http.StatusOK {
			offset = 0
			_ = os.Remove(tmp)
		}

		flag := os.O_WRONLY | os.O_CREATE
		if offset > 0 {
			flag |= os.O_APPEND
		} else {
			flag |= os.O_TRUNC
		}
		f, err := os.OpenFile(tmp, flag, 0o644)
		if err != nil {
			_ = resp.Body.Close()
			return err
		}

		totalSize = resp.ContentLength + offset
		if prog != nil {
			prog(offset, totalSize)
		}

		var reader io.Reader = resp.Body
		if prog != nil {
			reader = &dlProgressReader{
				reader:    resp.Body,
				totalSize: totalSize,
				written:   offset,
				cb:        prog,
			}
		}

		_, copyErr := io.Copy(f, reader)
		_ = f.Close()
		_ = resp.Body.Close()

		if copyErr == nil {
			if prog != nil {
				prog(totalSize, totalSize)
			}
			return os.Rename(tmp, dst)
		}

		readAttempt++
		lastErr = copyErr
		if prog != nil {
			prog(-int64(readAttempt), totalSize)
		}
		sleep := time.Duration(readAttempt) * 2 * time.Second
		if sleep > 15*time.Second {
			sleep = 15 * time.Second
		}
		time.Sleep(sleep)
	}

	partSize := int64(0)
	if st, err := os.Stat(tmp); err == nil {
		partSize = st.Size()
	}
	return fmt.Errorf("gave up after %d read / %d connect retries (partial download preserved: %.1f MB at %s.part — re-run to resume): %w",
		readAttempt, dialAttempt, float64(partSize)/(1024*1024), filepath.Base(dst), lastErr)
}

type dlProgressReader struct {
	reader    io.Reader
	totalSize int64
	written   int64
	cb        DownloadProgress
	lastCb    time.Time
}

func (pr *dlProgressReader) Read(p []byte) (int, error) {
	n, err := pr.reader.Read(p)
	pr.written += int64(n)
	now := time.Now()
	if now.Sub(pr.lastCb) >= 250*time.Millisecond {
		pr.cb(pr.written, pr.totalSize)
		pr.lastCb = now
	}
	return n, err
}
