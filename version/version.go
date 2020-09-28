package version

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// Phases of a pre-release
const (
	PrereleaseAlpha     = -3
	PrereleaseBeta      = -2
	PrereleaseCandidate = -1
)

// Version holds a PEP 440 compatible version.
// https://www.python.org/dev/peps/pep-0440/
type Version struct {
	Epoch int
	// PEP 440 allows the release vector to be of infinite length. Limiting the length
	// to 6 allows the structure to be directly comparable and is compatible with almost
	// all packages found on PyPi.
	ReleaseVersions    int
	Release            [6]int
	Wildcard           bool
	PreReleasePhase    int
	PreReleaseVersion  int
	PostRelease        bool
	PostReleaseVersion int
	DevRelease         bool
	DevReleaseVersion  int
	LocalVersion       string
}

// The following clause ensures that the structure is directly comparable and can
// therefore be used as the key in a map.
var _ = Version{} == Version{}

// https://www.python.org/dev/peps/pep-0440/#appendix-b-parsing-version-strings-with-regular-expressions
// Minor modification to allow '*' in release segment
var re = regexp.MustCompile(`^v?(?:(?:(?P<epoch>[0-9]+)!)?(?P<release>[0-9]+(?:\.(?:[0-9]+|\*$))*)(?P<pre>[-_\.]?(?P<pre_l>(a|b|c|rc|alpha|beta|pre|preview))[-_\.]?(?P<pre_n>[0-9]+)?)?(?P<post>(?:-(?P<post_n1>[0-9]+))|(?:[-_\.]?(?P<post_l>post|rev|r)[-_\.]?(?P<post_n2>[0-9]+)?))?(?P<dev>[-_\.]?(?P<dev_l>dev)[-_\.]?(?P<dev_n>[0-9]+)?)?)(?:\+(?P<local>[a-z0-9]+(?:[-_\.][a-z0-9]+)*))?$`)

// Parse parses a PEP 440 compatible version. If the version is invalid
// the returned bool is false.
func Parse(input string) (Version, bool) {
	matches := re.FindStringSubmatch(strings.ToLower(input))
	if matches == nil {
		return Version{}, false
	}

	var epoch int
	if matches[1] != "" {
		var err error
		epoch, err = strconv.Atoi(matches[1])
		if err != nil {
			return Version{}, false
		}
	}
	releaseVersions := 0
	release := [6]int{}
	for i, part := range strings.Split(matches[2], ".") {
		if i >= len(release) {
			return Version{}, false
		}
		if part == "*" {
			return Version{
				Epoch:           epoch,
				ReleaseVersions: releaseVersions,
				Release:         release,
				Wildcard:        true,
			}, true
		}

		n, err := strconv.Atoi(part)
		if err != nil {
			return Version{}, false
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
			return Version{}, false
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
				return Version{}, false
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
				return Version{}, false
			}
		}
	}

	return Version{
		Epoch:              epoch,
		ReleaseVersions:    releaseVersions,
		Release:            release,
		PreReleasePhase:    preReleasePhase,
		PreReleaseVersion:  preReleaseVersion,
		PostRelease:        postRelease,
		PostReleaseVersion: postReleaseVersion,
		DevRelease:         devRelease,
		DevReleaseVersion:  devReleaseVersion,
		LocalVersion:       matches[14],
	}, true
}

// MustParse parses the version and panics if the version cannot be parsed.
func MustParse(input string) Version {
	v, valid := Parse(input)
	if !valid {
		panic(fmt.Sprintf("invalid version: '%s'", input))
	}

	return v
}

func (v Version) String() string {
	if v.Unspecified() {
		return "<latest>"
	}

	return v.Canonical()
}

// Canonical returns the canonical representation of
func (v Version) Canonical() string {
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
	if v.Wildcard {
		sb.WriteString(".*")
		return sb.String()
	}

	switch v.PreReleasePhase {
	case PrereleaseAlpha:
		fmt.Fprintf(sb, "a%d", v.PreReleaseVersion)
	case PrereleaseBeta:
		fmt.Fprintf(sb, "b%d", v.PreReleaseVersion)
	case PrereleaseCandidate:
		fmt.Fprintf(sb, "rc%d", v.PreReleaseVersion)
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

// Equal returns true if the two versions are equal.
// It also "zero-pads" the release segment.
func (v Version) Equal(v2 Version) bool {
	return true &&
		v.Epoch == v2.Epoch &&
		v.Release == v2.Release &&
		v.PreReleasePhase == v2.PreReleasePhase &&
		v.PreReleaseVersion == v2.PreReleaseVersion &&
		v.PostRelease == v2.PostRelease &&
		v.PostReleaseVersion == v2.PostReleaseVersion &&
		v.DevRelease == v2.DevRelease &&
		v.DevReleaseVersion == v2.DevReleaseVersion &&
		v.LocalVersion == v2.LocalVersion
}

// Match returns true if the version v matches the version v2.
// https://www.python.org/dev/peps/pep-0440/#version-matching
// TODO: This should instead use []Requirement in order to
// more accurately find matching versions. When the found
// version differs from the minimal version it will be
// recorded in rope.json
// TODO: Does this differentiate from Compare(v, v2) == 0?
func (v Version) Match(v2 Version) bool {
	if v.Epoch != v2.Epoch {
		return false
	}

	// equivalent to "zero-padding" the release
	for i := range v.Release {
		if v.Release[i] != v2.Release[i] {
			return false
		}
	}
	if v.Wildcard || v2.Wildcard {
		// If the epoch and release versions are a match and either version is a wildcard
		// then the versions match.
		return true
	}

	if v.PreReleasePhase != v2.PreReleasePhase {
		return false
	}
	if v.PreReleaseVersion != v2.PreReleaseVersion {
		return false
	}

	// TODO: Match should be a method on Requirement(or even []Requirement)
	// to accurately account for post-releases(and where an exact minimal version may not exist).
	// // Special handling for when v is a post release
	// if v.PostRelease {
	// 	if v.PostReleaseVersion != v2.PostReleaseVersion {
	// 		return false
	// 	}
	// }

	if v.DevRelease != v2.DevRelease {
		return false
	}
	if v.DevReleaseVersion != v2.DevReleaseVersion {
		return false
	}

	return true
}

// GreaterThan returns true if v is greater than v2.
func (v Version) GreaterThan(v2 Version) bool {
	return Compare(v, v2) == 1
}

// Unspecified returns true if the version v is unspecified(empty)
func (v Version) Unspecified() bool {
	return v == Version{}
}

// Compare returns an integer comparing two versions.
// The result will be 0 if a==b, -1 if a < b, and +1 if a > b.
func Compare(a, b Version) int {
	compare := func(a, b Version) int {
		if a.Epoch > b.Epoch {
			return 1
		} else if a.Epoch < b.Epoch {
			return -1
		}

		for i := 0; i < len(a.Release); i++ {
			if a.Release[i] > b.Release[i] {
				return 1
			} else if a.Release[i] < b.Release[i] {
				return -1
			}
		}
		if a.Wildcard || b.Wildcard {
			// If the epoch and release versions are a match and either version is a wildcard
			// then the versions can be considered equivalent.
			return 0
		}

		if a.PreReleasePhase > b.PreReleasePhase {
			return 1
		} else if a.PreReleasePhase < b.PreReleasePhase {
			return -1
		}
		if a.PreReleaseVersion > b.PreReleaseVersion {
			return 1
		} else if a.PreReleaseVersion < b.PreReleaseVersion {
			return -1
		}

		if a.PostReleaseVersion > b.PostReleaseVersion {
			return 1
		} else if a.PostReleaseVersion < b.PostReleaseVersion {
			return -1
		}

		if !a.DevRelease && b.DevRelease {
			return 1
		} else if a.DevRelease && !b.DevRelease {
			return -1
		}
		if a.DevReleaseVersion > b.DevReleaseVersion {
			return 1
		} else if a.DevReleaseVersion < b.DevReleaseVersion {
			return -1
		}

		return 0
	}

	if compare(b, a) != -1*compare(a, b) {
		// TODO: Remove this assertion once the implementation is considered stable.
		panic(fmt.Sprintf("version.Compare is not symmetric for a: %s, b: %s", a, b))
	}

	return compare(a, b)
}

func (v *Version) UnmarshalJSON(b []byte) error {
	var s string
	if err := json.Unmarshal(b, &s); err != nil {
		return err
	}

	var valid bool
	*v, valid = Parse(s)
	if !valid {
		return fmt.Errorf("unmarshaling invalid version: '%s'", s)
	}

	return nil
}

func (v Version) MarshalJSON() ([]byte, error) {
	if v.Unspecified() {
		return nil, fmt.Errorf("marshaling unspecified version")
	}

	return json.Marshal(v.Canonical())
}
