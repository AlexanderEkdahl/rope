package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// Normalize package names.
var normalizationRe = regexp.MustCompile(`[-_.]+`)

// NormalizePackageName normalizes the package name.
//
// https://www.python.org/dev/peps/pep-0503/#normalized-names
func NormalizePackageName(name string) string {
	return strings.ToLower(normalizationRe.ReplaceAllString(name, "-"))
}

// ReadRopefile finds and reads the rope.json file by recursively
// looking in the parent directory starting from the current working
// directory.
// TODO: Support explicitly provided rope.json path
func ReadRopefile() (*Project, error) {
	path, err := FindRopefile()
	if err != nil {
		return nil, err
	}

	bytes, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var rope *Project
	if err := json.Unmarshal(bytes, &rope); err != nil {
		return nil, err
	}

	return rope, nil
}

func WriteRopefile(p *Project, path string) error {
	if path == "" {
		var err error
		path, err = FindRopefile()
		if err != nil {
			return err
		}
	}

	bytes, err := json.MarshalIndent(p, "", "\t")
	if err != nil {
		return err
	}

	return ioutil.WriteFile(path, append(bytes, []byte{'\n'}...), 0666)
}

var ErrRopefileNotFound = fmt.Errorf("rope.json not found (or in any of the parent directories)")

func FindRopefile() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}

	for {
		path := filepath.Join(dir, "rope.json")
		_, err := os.Stat(path)
		if errors.Is(err, os.ErrNotExist) {
			if filepath.Dir(dir) == dir {
				return "", ErrRopefileNotFound
			}
			dir = filepath.Dir(dir)

			continue
		} else if err != nil {
			return "", err
		}

		return path, nil
	}
}
