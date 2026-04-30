//go:build android

package gui

import "time"

func pollPermission(id string, onChange func()) {
	initial := checkPermission(id)
	deadline := time.Now().Add(30 * time.Second)
	for time.Now().Before(deadline) {
		time.Sleep(500 * time.Millisecond)
		now := checkPermission(id)
		if now != initial {
			if onChange != nil {
				onChange()
			}
			return
		}
	}
}
