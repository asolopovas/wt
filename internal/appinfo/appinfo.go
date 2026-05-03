package appinfo

var Name = "WTranscribe"

var Version = "dev"

var BuildDate = ""

func DisplayVersion(version, buildDate string) string {
	if buildDate != "" {
		return buildDate
	}
	if version != "" {
		return version
	}
	return "dev"
}
