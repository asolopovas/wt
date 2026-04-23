package transcriber

import (
	"strconv"
	"time"
)

func FormatDuration(d time.Duration) string {
	return strconv.FormatFloat(d.Seconds(), 'f', 3, 64)
}

func FormatHMS(d time.Duration) string {
	total := int(d.Seconds())
	h := total / 3600
	m := (total % 3600) / 60
	s := total % 60
	if h > 0 {
		buf := make([]byte, 0, 8)
		buf = strconv.AppendInt(buf, int64(h), 10)
		buf = append(buf, ':')
		buf = appendPadded(buf, m)
		buf = append(buf, ':')
		buf = appendPadded(buf, s)
		return string(buf)
	}
	buf := make([]byte, 0, 5)
	buf = strconv.AppendInt(buf, int64(m), 10)
	buf = append(buf, ':')
	buf = appendPadded(buf, s)
	return string(buf)
}

func appendPadded(buf []byte, v int) []byte {
	if v < 10 {
		buf = append(buf, '0')
	}
	return strconv.AppendInt(buf, int64(v), 10)
}
