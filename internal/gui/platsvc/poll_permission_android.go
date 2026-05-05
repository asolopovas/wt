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

func PollBatteryOptimization(onChange func()) {
	initial := IsIgnoringBatteryOptimizations()
	deadline := time.Now().Add(60 * time.Second)
	for time.Now().Before(deadline) {
		time.Sleep(500 * time.Millisecond)
		now := IsIgnoringBatteryOptimizations()
		if now != initial {
			if onChange != nil {
				onChange()
			}
			return
		}
	}
}
