package version

import (
	"strings"
	"text/scanner"

	"github.com/blang/semver/v4"
)

// Version comparison operators
const (
	LessOrEqual     = "<="
	Less            = "<"
	NotEqual        = "!="
	Equal           = "=="
	GreaterOrEqual  = ">="
	Greater         = ">"
	CompatibleEqual = "~="
	TripleEqual     = "==="
)

type Version struct {
	semver.Version
	raw string
}

// TODO: Should use scanner instead of semver
func Parse(s string) (Version, error) {
	semverVersion, err := semver.ParseTolerant(s)
	if err != nil {
		return Version{}, err
	}

	return Version{
		Version: semverVersion,
		raw:     s,
	}, nil
}

func MustParse(s string) Version {
	v, err := Parse(s)
	if err != nil {
		panic(err)
	}

	return v
}

func (v Version) String() string {
	if v.raw == "" {
		return "<latest>"
	}
	return v.raw
}

func (v Version) Unspecified() bool {
	return v.raw == ""
}

type VersionRequirement struct {
	Operator string
	Version  Version
}

func ParseVersionRequirements(input string) ([]VersionRequirement, error) {
	var s scanner.Scanner
	s.Init(strings.NewReader(input))
	return ScanVersionRequirements(&s)
}

// Minimal reads multiple versions requirements and tries to establish what
// the minimal required version is in that range. If a lower bound can not
// be found and a higher lower bound is specified the returned version is
// the highest lower bound.
//
// 	<1.19.0, >=1.16.0 -> 1.16.0
// 	<1.3.4, >=1.3.6 -> 1.3.6
//
// The intention of this function is to extract the minimal version the
// package was verified to work with.
func Minimal(vrs []VersionRequirement) Version {
	if len(vrs) == 0 {
		return Version{}
	}

	var highestLowerBound Version
	for _, vr := range vrs {
		switch vr.Operator {
		case GreaterOrEqual, CompatibleEqual, Equal, TripleEqual:
			if vr.Version.GT(highestLowerBound.Version) {
				highestLowerBound = vr.Version
			}
		}
	}

	return highestLowerBound
}
