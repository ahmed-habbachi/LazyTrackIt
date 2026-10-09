package update

import (
	"strconv"
	"strings"
)

// parseVersion parses a "vX.Y.Z" (or "X.Y.Z") tag into its three numeric
// components. It intentionally doesn't handle pre-release/build suffixes
// (e.g. "-rc1", "+build5") beyond ignoring them, since every tag this
// project cuts is a plain "vX.Y.Z".
func parseVersion(s string) (major, minor, patch int, ok bool) {
	s = strings.TrimPrefix(s, "v")
	if i := strings.IndexAny(s, "-+"); i >= 0 {
		s = s[:i]
	}
	parts := strings.Split(s, ".")
	if len(parts) != 3 {
		return 0, 0, 0, false
	}
	nums := make([]int, 3)
	for i, p := range parts {
		n, err := strconv.Atoi(p)
		if err != nil {
			return 0, 0, 0, false
		}
		nums[i] = n
	}
	return nums[0], nums[1], nums[2], true
}

// isNewer reports whether latest is a strictly newer version than current.
// It returns false (rather than erroring) if either string doesn't parse as
// "vX.Y.Z" — most notably "dev", which local builds use and which should
// never be treated as newer or older than anything.
func isNewer(latest, current string) bool {
	lMaj, lMin, lPatch, ok := parseVersion(latest)
	if !ok {
		return false
	}
	cMaj, cMin, cPatch, ok := parseVersion(current)
	if !ok {
		return false
	}
	if lMaj != cMaj {
		return lMaj > cMaj
	}
	if lMin != cMin {
		return lMin > cMin
	}
	return lPatch > cPatch
}
