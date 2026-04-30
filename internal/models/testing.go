package models

func registerForTest(e Entry) {
	llmEntries = append(llmEntries, e)
}

func unregisterForTest(id string) {
	out := llmEntries[:0]
	for _, e := range llmEntries {
		if e.ID != id {
			out = append(out, e)
		}
	}
	llmEntries = out
}
