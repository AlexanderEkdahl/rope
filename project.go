package main

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/AlexanderEkdahl/rope/version"
)

type Project struct {
	Python       string       `json:"python,omitempty"`
	Dependencies []Dependency `json:"dependencies"`
}

type Dependency struct {
	// Name is the canonical name of the package
	Name    string
	Version version.Version

	// Unspecified is true if no version constraint is applied
	// to this dependency. Special care must be taken in this
	// scenario to ensure reproduceable builds and not blindly
	// selecting a version that is too recent when other
	// dependencies specify a lower version.
	Unspecified bool

	// Mismatch is true if the found is not equal to the version
	// specified by dependants.
	Mismatch bool
}

func (d *Dependency) UnmarshalJSON(b []byte) error {
	var s string
	if err := json.Unmarshal(b, &s); err != nil {
		return err
	}

	sep := strings.LastIndex(s, "-")
	if sep < 0 {
		return fmt.Errorf("expected dependency to be in the form of <name>-<version>, got: '%s'", s)
	}
	d.Name = NormalizePackageName(string(s[:sep]))

	var valid bool
	d.Version, valid = version.Parse(s[sep+1:])
	if !valid {
		return fmt.Errorf("invalid version: '%s'", s[sep+1:])
	}

	return nil
}

func (d Dependency) MarshalJSON() ([]byte, error) {
	if d.Version.Unspecified() {
		return nil, fmt.Errorf("marshaling unspecified version for '%s'", d.Name)
	}
	return json.Marshal(fmt.Sprintf("%s-%s", d.Name, d.Version))
}
