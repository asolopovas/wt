package gui

func versionLabel(version, buildDate string) string {
	if version != "" {
		if version[0] != 'v' {
			return "v" + version
		}
		return version
	}
	if buildDate != "" {
		return buildDate
	}
	return "dev"
}
