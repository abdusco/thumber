package version

import (
	"fmt"
	"runtime/debug"
	"time"
)

var Commit string
var CommitTime string

type VersionInfo struct {
	Commit     string
	CommitTime string
}

func (v VersionInfo) String() string {
	return fmt.Sprintf("%s.%s", v.CommitTime, v.Commit)
}

func GitVersion() VersionInfo {
	var info VersionInfo
	if build, ok := debug.ReadBuildInfo(); ok {
		for _, setting := range build.Settings {
			switch setting.Key {
			case "vcs.revision":
				info.Commit = setting.Value[:7]
			case "vcs.time":
				d, _ := time.Parse(time.RFC3339, setting.Value)
				info.CommitTime = d.Format("20060102150405")
			}
		}
	}

	return info
}
