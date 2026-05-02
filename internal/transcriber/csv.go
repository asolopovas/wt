package transcriber

import (
	"os"
	"path/filepath"
	"strings"

	shared "github.com/asolopovas/wt/internal"
	"github.com/asolopovas/wt/internal/diarizer"
)

func DeduplicateSegments(segs []diarizer.TranscriptSegment) []diarizer.TranscriptSegment {
	if len(segs) < 2 {
		return segs
	}
	out := make([]diarizer.TranscriptSegment, 0, len(segs))
	out = append(out, segs[0])
	for i := 1; i < len(segs); i++ {
		prev := out[len(out)-1]
		cur := segs[i]
		if strings.TrimSpace(cur.Text) == strings.TrimSpace(prev.Text) {
			out[len(out)-1].End = cur.End
			continue
		}
		out = append(out, cur)
	}
	return out
}

func ResolveWAVPath(absPath string) string {
	cacheFile, err := AudioCacheKey(absPath)
	if err != nil {
		return absPath
	}
	cachePath := filepath.Join(shared.CacheDir(), cacheFile)
	if _, err := os.Stat(cachePath); err == nil {
		return cachePath
	}
	return absPath
}
