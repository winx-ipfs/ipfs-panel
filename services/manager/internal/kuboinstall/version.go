package kuboinstall

import (
	"strconv"
	"strings"
)

type semver struct {
	major int
	minor int
	patch int
}

func parseVer(raw string) (semver, bool) {
	s := strings.TrimPrefix(strings.TrimSpace(raw), "v")
	if s == "" || strings.ContainsAny(s, "-+") {
		return semver{}, false
	}
	parts := strings.Split(s, ".")
	if len(parts) != 3 {
		return semver{}, false
	}
	nums := [3]int{}
	for i, part := range parts {
		n, err := strconv.Atoi(part)
		if err != nil || n < 0 || (len(part) > 1 && part[0] == '0') {
			return semver{}, false
		}
		nums[i] = n
	}
	return semver{major: nums[0], minor: nums[1], patch: nums[2]}, true
}

func (v semver) less(o semver) bool {
	if v.major != o.major {
		return v.major < o.major
	}
	if v.minor != o.minor {
		return v.minor < o.minor
	}
	return v.patch < o.patch
}

func (v semver) String() string {
	return strconv.Itoa(v.major) + "." + strconv.Itoa(v.minor) + "." + strconv.Itoa(v.patch)
}

func inRange(v semver) bool {
	min, okMin := parseVer(SupportedMin)
	max, okMax := parseVer(SupportedMax)
	if !okMin || !okMax {
		return false
	}
	return !v.less(min) && v.less(max)
}

// pick returns the newest stable version inside the supported range, and
// the newest stable version with no cap. Empty strings mean the list had
// none. Pre-releases never qualify.
func pick(lines []string) (latest, upstream string) {
	var best, top semver
	var haveBest, haveTop bool
	for _, line := range lines {
		v, ok := parseVer(line)
		if !ok {
			continue
		}
		if !haveTop || top.less(v) {
			top = v
			haveTop = true
		}
		if inRange(v) && (!haveBest || best.less(v)) {
			best = v
			haveBest = true
		}
	}
	if haveBest {
		latest = best.String()
	}
	if haveTop {
		upstream = top.String()
	}
	return latest, upstream
}
