package version

import (
	"io"
	"runtime"
	"strconv"
	"strings"
	"text/template"
)

var versionTemplate = ` Version:      {{.Version}}
 Git commit:   {{.GitCommit}}
 Go version:   {{.GoVersion}}
 Built:        {{.BuildTime}}
 OS/Arch:      {{.Os}}/{{.Arch}}
`

// set by build LD_FLAGS
var (
	version   string
	gitCommit string
	buildAt   string
)

type VersionInfo struct {
	Version   string
	GoVersion string
	GitCommit string
	BuildTime string
	Os        string
	Arch      string
}

func Info() VersionInfo {
	return VersionInfo{
		Version:   version,
		GitCommit: gitCommit,
		BuildTime: buildAt,
		GoVersion: runtime.Version(),
		Os:        runtime.GOOS,
		Arch:      runtime.GOARCH,
	}
}

func (v VersionInfo) WriteTo(w io.Writer) error {
	tmpl, _ := template.New("version").Parse(versionTemplate)
	return tmpl.Execute(w, v)
}

func GetVersion() string {
	return version
}

func GetGitCommit() string {
	return gitCommit
}

// Version provides utility methods for comparing versions.
type Version string

func (v Version) compareTo(other Version) int {
	var (
		currTab  = strings.Split(string(v), ".")
		otherTab = strings.Split(string(other), ".")
	)

	max := len(currTab)
	if len(otherTab) > max {
		max = len(otherTab)
	}
	for i := 0; i < max; i++ {
		var currInt, otherInt int

		if len(currTab) > i {
			currInt, _ = strconv.Atoi(currTab[i])
		}
		if len(otherTab) > i {
			otherInt, _ = strconv.Atoi(otherTab[i])
		}
		if currInt > otherInt {
			return 1
		}
		if otherInt > currInt {
			return -1
		}
	}
	return 0
}

// LessThan checks if a version is less than another version
func (v Version) LessThan(other Version) bool {
	return v.compareTo(other) == -1
}

// LessThanOrEqualTo checks if a version is less than or equal to another
func (v Version) LessThanOrEqualTo(other Version) bool {
	return v.compareTo(other) <= 0
}

// GreaterThan checks if a version is greater than another one
func (v Version) GreaterThan(other Version) bool {
	return v.compareTo(other) == 1
}

// GreaterThanOrEqualTo checks ia version is greater than or equal to another
func (v Version) GreaterThanOrEqualTo(other Version) bool {
	return v.compareTo(other) >= 0
}

// Equal checks if a version is equal to another
func (v Version) Equal(other Version) bool {
	return v.compareTo(other) == 0
}

// CompatibleWith checks if a version is in tervalIn of a big version
func (v Version) CompatibleWith(other Version) bool {
	if v.LessThanOrEqualTo(other) {
		return true
	}

	var (
		currTab  = strings.Split(string(v), ".")
		otherTab = strings.Split(string(other), ".")
	)
	if len(currTab) >= 2 && len(otherTab) >= 2 {
		if currTab[0] == otherTab[0] && currTab[1] == otherTab[1] {
			return true
		}
	}
	return false
}

