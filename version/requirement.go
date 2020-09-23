package version

import (
	"fmt"
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
	Version
}

func (vr Requirement) String() string {
	if vr.Version.Unspecified() {
		return "<latest>"
	}
	return fmt.Sprintf("%s%s", vr.Operator, vr.Version)
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
			if vr.GreaterThan(highestLowerBound) {
				highestLowerBound = vr.Version
			}
		}
	}

	return highestLowerBound
}
