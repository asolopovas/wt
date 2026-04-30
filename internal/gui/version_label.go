package gui

func versionLabel(version, buildDate string) string {
	if buildDate != "" {
		return buildDate
	}
	if version != "" {
		return version
	}
	return "dev"
}
