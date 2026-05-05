package models

func registerForTest(e Entry) {
	mu.Lock()
	entries = append(entries, e)
	mu.Unlock()
}

func unregisterForTest(id string) {
	mu.Lock()
	out := entries[:0]
	for _, e := range entries {
		if e.ID != id {
			out = append(out, e)
		}
	}
	entries = out
	mu.Unlock()
}
