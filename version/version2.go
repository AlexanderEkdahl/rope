package version

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// Phases of a pre-release
const (
	PrereleaseAlpha = iota + 1
	PrereleaseBeta
	PrereleaseCandidate
)

// Version2 holds a PEP 440 compatible version.
// https://www.python.org/dev/peps/pep-0440/
type Version2 struct {
	Epoch int
	// PEP 440 allows the release vector to be of infinite length. Limiting the length
	// to 6 allows the structure to be directly comparable and is compatible with almost
	// all packages found on PyPi.
	ReleaseVersions    int
	Release            [6]int
	PrereleasePhase    int
	PrereleaseVersion  int
	PostRelease        bool
	PostReleaseVersion int
	DevRelease         bool
	DevReleaseVersion  int
	LocalVersion       string
}

// The following clause ensures that the structure is directly comparable and can
// therefore be used as the key in a map.
var _ = Version2{} == Version2{}

// https://www.python.org/dev/peps/pep-0440/#appendix-b-parsing-version-strings-with-regular-expressions
var re = regexp.MustCompile(`v?(?:(?:(?P<epoch>[0-9]+)!)?(?P<release>[0-9]+(?:\.[0-9]+)*)(?P<pre>[-_\.]?(?P<pre_l>(a|b|c|rc|alpha|beta|pre|preview))[-_\.]?(?P<pre_n>[0-9]+)?)?(?P<post>(?:-(?P<post_n1>[0-9]+))|(?:[-_\.]?(?P<post_l>post|rev|r)[-_\.]?(?P<post_n2>[0-9]+)?))?(?P<dev>[-_\.]?(?P<dev_l>dev)[-_\.]?(?P<dev_n>[0-9]+)?)?)(?:\+(?P<local>[a-z0-9]+(?:[-_\.][a-z0-9]+)*))?`)

// Parse2 parses a PEP 440 compatible version. If the version is invalid
// the returned bool is false.
func Parse2(input string) (Version2, bool) {
	matches := re.FindStringSubmatch(strings.ToLower(input))
	if matches == nil {
		return Version2{}, false
	}
	// for i, m := range matches {
	// 	fmt.Printf("matches[%d] = %s\n", i, m)
	// }

	var epoch int
	if matches[1] != "" {
		var err error
		epoch, err = strconv.Atoi(matches[1])
		if err != nil {
			return Version2{}, false
		}
	}
	releaseVersions := 0
	release := [6]int{}
	for i, part := range strings.Split(matches[2], ".") {
		if i >= len(release) {
			// return version2{}, false
			break
		}
		n, err := strconv.Atoi(part)
		if err != nil {
			return Version2{}, false
		}
		release[i] = n
		releaseVersions = i + 1
	}
	preReleasePhase := 0
	switch matches[4] {
	case "a", "alpha":
		preReleasePhase = PrereleaseAlpha
	case "b", "beta":
		preReleasePhase = PrereleaseBeta
	case "rc", "c", "pre", "preview":
		preReleasePhase = PrereleaseCandidate
	}
	var preReleaseVersion int
	if matches[6] != "" {
		var err error
		preReleaseVersion, err = strconv.Atoi(matches[6])
		if err != nil {
			return Version2{}, false
		}
	}
	postRelease := false
	var postReleaseVersion int
	if matches[9] != "" {
		postRelease = true
		if matches[10] != "" {
			var err error
			postReleaseVersion, err = strconv.Atoi(matches[10])
			if err != nil {
				return Version2{}, false
			}
		}
	}
	devRelease := false
	var devReleaseVersion int
	if matches[12] != "" {
		devRelease = true
		if matches[13] != "" {
			var err error
			devReleaseVersion, err = strconv.Atoi(matches[13])
			if err != nil {
				return Version2{}, false
			}
		}
	}

	return Version2{
		Epoch:              epoch,
		ReleaseVersions:    releaseVersions,
		Release:            release,
		PrereleasePhase:    preReleasePhase,
		PrereleaseVersion:  preReleaseVersion,
		PostRelease:        postRelease,
		PostReleaseVersion: postReleaseVersion,
		DevRelease:         devRelease,
		DevReleaseVersion:  devReleaseVersion,
		LocalVersion:       matches[14],
	}, true
}

// Canonical returns the canonical representation of
func (v Version2) Canonical() string {
	sb := &strings.Builder{}

	if v.Epoch > 0 {
		fmt.Fprintf(sb, "%d!", v.Epoch)
	}

	for i := 0; i < v.ReleaseVersions; i++ {
		if i > 0 {
			sb.WriteRune('.')
		}
		fmt.Fprintf(sb, "%d", v.Release[i])
	}

	switch v.PrereleasePhase {
	case PrereleaseAlpha:
		fmt.Fprintf(sb, "a%d", v.PrereleaseVersion)
	case PrereleaseBeta:
		fmt.Fprintf(sb, "b%d", v.PrereleaseVersion)
	case PrereleaseCandidate:
		fmt.Fprintf(sb, "rc%d", v.PrereleaseVersion)
	}

	if v.PostRelease {
		fmt.Fprintf(sb, ".post%d", v.PostReleaseVersion)
	}

	if v.DevRelease {
		fmt.Fprintf(sb, ".dev%d", v.DevReleaseVersion)
	}

	if v.LocalVersion != "" {
		fmt.Fprintf(sb, "+%s", v.LocalVersion)
	}

	return sb.String()
}
