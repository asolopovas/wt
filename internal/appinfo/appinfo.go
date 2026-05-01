package appinfo

// Name is the user-facing app name shown in window titles, notifications,
// and tray menus. Overridden at build time via appinfo_generated.go (which
// the Taskfile writes from {{.APP_NAME}}). The default keeps `go run`/tests
// working without a build pipeline.
var Name = "WTranscribe"

// Version is the released version. Overridden the same way.
var Version = "dev"

// BuildDate is the commit date of the build (when not on a tagged release).
var BuildDate = ""
