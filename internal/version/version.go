// Package version holds LazyTrackIt's build-time version string.
package version

// Version is the running build's version (e.g. "v0.0.2"), baked in by the
// release workflow via:
//
//	-ldflags "-X github.com/ahmed-habbachi/lazytrackit/internal/version.Version=vX.Y.Z"
//
// Local "go run"/"go build" builds leave it at "dev", which the update
// package treats as "never check for updates" since there's nothing
// meaningful to compare a release tag against.
var Version = "dev"
