//go:build android

package platsvc

import "time"

func PollPermission(id string, onChange func()) {
	initial := CheckPermission(id)
	deadline := time.Now().Add(30 * time.Second)
	for time.Now().Before(deadline) {
		time.Sleep(500 * time.Millisecond)
		now := CheckPermission(id)
		if now != initial {
			if onChange != nil {
				onChange()
			}
			return
		}
	}
}
