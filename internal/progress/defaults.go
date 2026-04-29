package progress

import "strings"

func DefaultRTF(model, device string) float64 {
	m := strings.ToLower(model)
	isCPU := device == "" || strings.Contains(strings.ToLower(device), "cpu")
	var base float64
	switch {
	case strings.HasPrefix(m, "tiny"):
		base = 3
	case strings.HasPrefix(m, "base"):
		base = 2
	case strings.HasPrefix(m, "small"):
		base = 1
	case strings.HasPrefix(m, "medium"):
		base = 0.5
	case strings.Contains(m, "large-v3-turbo") || strings.Contains(m, "turbo"):
		base = 0.8
	case strings.HasPrefix(m, "large"):
		base = 0.3
	default:
		base = 1
	}
	if isCPU {
		base /= 3
	}
	if base < 0.05 {
		base = 0.05
	}
	return base
}
