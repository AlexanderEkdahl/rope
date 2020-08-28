package main

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/AlexanderEkdahl/rope/version"
)

type Project struct {
	Python       string       `json:"python"`
	Dependencies []Dependency `json:"dependencies"`
}

type Dependency struct {
	Name    string // Name may differ in casing depending on where it comes from
	Version version.Version

	// This is required to ensure reproduceable build in the event of missing
	// requirement specifier of transitive dependencies and preventing always
	// using the latest version in cases where transitive dependencies do not
	// specify a version.
	RequestedVersion version.Version
}

func (d *Dependency) UnmarshalJSON(b []byte) error {
	var s string
	if err := json.Unmarshal(b, &s); err != nil {
		return err
	}

	split := strings.Split(s, "-")
	if len(split) != 2 {
		return fmt.Errorf("expected dependency to be in the form of <name>-<version>, got: '%s'", s)
	}
	d.Name = string(split[0])

	var err error
	d.Version, err = version.Parse(split[1])
	if err != nil {
		return fmt.Errorf("dependency version not valid semver: %w", err)
	}

	return nil
}

func (d Dependency) MarshalJSON() ([]byte, error) {
	return json.Marshal(fmt.Sprintf("%s-%s", d.Name, d.Version))
}
