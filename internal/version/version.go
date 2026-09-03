// Package version reports build information for the osp-release binary.
package version

import (
	"runtime"
	"runtime/debug"
)

// Values injected at release time with -ldflags -X. When they are empty the
// information is read back from the embedded build info instead, so both
// `go install ...@latest` and a plain `go build` report something useful.
var (
	Version string
	Commit  string
	Date    string
)

// Info describes the running binary.
type Info struct {
	Version   string `json:"version"`
	Commit    string `json:"commit"`
	Date      string `json:"date"`
	GoVersion string `json:"go_version"`
	Platform  string `json:"platform"`
}

// Get returns the build information for the running binary.
func Get() Info {
	info := Info{
		Version:   Version,
		Commit:    Commit,
		Date:      Date,
		GoVersion: runtime.Version(),
		Platform:  runtime.GOOS + "/" + runtime.GOARCH,
	}

	if build, ok := debug.ReadBuildInfo(); ok {
		if info.Version == "" && build.Main.Version != "" && build.Main.Version != "(devel)" {
			info.Version = build.Main.Version
		}
		for _, setting := range build.Settings {
			switch setting.Key {
			case "vcs.revision":
				if info.Commit == "" {
					info.Commit = setting.Value
				}
			case "vcs.time":
				if info.Date == "" {
					info.Date = setting.Value
				}
			}
		}
	}

	if info.Version == "" {
		info.Version = "dev"
	}
	return info
}

// ToMap renders the build information as a row for the output renderer.
func (i Info) ToMap() map[string]any {
	return map[string]any{
		"version":    i.Version,
		"commit":     i.Commit,
		"date":       i.Date,
		"go_version": i.GoVersion,
		"platform":   i.Platform,
	}
}

// Fields are the selectable field names for the version command.
func Fields() []string {
	return []string{"version", "commit", "date", "go_version", "platform"}
}
