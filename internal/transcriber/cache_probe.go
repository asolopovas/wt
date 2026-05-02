package transcriber

import "github.com/asolopovas/wt/internal/transcriber/cache"

func init() {
	cache.ProbeDurationMsFn = ProbeDurationMs
}
