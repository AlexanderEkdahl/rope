package version

import (
	"fmt"
	"os"
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

// Requirement defines a operator and a version.
type Requirement struct {
	Operator string
	Version  Version
}

func (vr Requirement) String() string {
	if vr.Version.Unspecified() {
		return "<latest>"
	}
	return fmt.Sprintf("%s%s", vr.Operator, vr.Version)
}

func (vr Requirement) Contains(v Version) bool {
	switch vr.Operator {
	case LessOrEqual:
		return Compare(v, vr.Version) <= 0
	case Less:
		return Compare(v, vr.Version) < 0
	case NotEqual:
		return Compare(v, vr.Version) != 0
	case Equal:
		return Compare(v, vr.Version) == 0
	case GreaterOrEqual:
		return Compare(v, vr.Version) >= 0
	case Greater:
		return Compare(v, vr.Version) > 0
	case CompatibleEqual:
		fmt.Fprintf(os.Stderr, "❗️ '~=' not supported")
		return false
	case TripleEqual:
		// Treat === as equivalent to == (should be string equality)
		return Compare(v, vr.Version) == 0
	default:
		panic(fmt.Sprintf("unknown version comparison operator: '%s'", vr.Operator))
	}
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
func Minimal(vrs []Requirement) Version {
	if len(vrs) == 0 {
		return Version{}
	}

	var highestLowerBound Version
	for _, vr := range vrs {
		switch vr.Operator {
		case GreaterOrEqual, CompatibleEqual, Equal, TripleEqual:
			if vr.Version.GreaterThan(highestLowerBound) {
				highestLowerBound = vr.Version
			}
		}
	}

	return highestLowerBound
}
