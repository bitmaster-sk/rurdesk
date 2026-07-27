// Package buildinfo exposes the release identity of the running binary.
//
// The values are stamped at link time by script/build.sh:
//
//	go build -ldflags "-X github.com/bitmaster-sk/rurdesk/api/internal/buildinfo.version=1.0.0"
//
// They stay unexported so nothing can mutate them at runtime — the build
// identity of a binary must not change while it runs.
package buildinfo

var (
	version = "dev"
	commit  = "unknown"
)

// Info is the build identity of the running binary.
type Info struct {
	Version string `json:"version"`
	Commit  string `json:"commit"`
}

// Get returns the stamped build identity, or the dev defaults for an
// unstamped (local) build.
func Get() Info {
	return Info{Version: version, Commit: commit}
}
