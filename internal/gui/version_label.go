package gui

import "github.com/asolopovas/wt/internal/appinfo"

func versionLabel(version, buildDate string) string {
	label := appinfo.DisplayVersion(version, buildDate)

	if buildDate == "" && label != "dev" && label[0] != 'v' {
		return "v" + label
	}
	return label
}
