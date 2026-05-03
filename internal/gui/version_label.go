package gui

import "github.com/asolopovas/wt/internal/appinfo"

func versionLabel(version, buildDate string) string {
	label := appinfo.DisplayVersion(version, buildDate)
	// Prefix tagged versions with "v" (e.g. "0.0.7" -> "v0.0.7").
	// Date-shaped labels ("2026-05-03") and "dev" are returned as-is.
	if buildDate == "" && label != "dev" && label[0] != 'v' {
		return "v" + label
	}
	return label
}
