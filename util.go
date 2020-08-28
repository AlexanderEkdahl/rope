package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
)

// ReadRopefile finds and reads the rope.json file by recursively
// looking in the parent directory starting from the current working
// directory.
// TODO: Support explicitly provided rope.json path
func ReadRopefile() (*Project, error) {
	path, err := findRopefile()
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

func WriteRopefile(p *Project) error {
	path, err := findRopefile()
	if err != nil {
		return err
	}

	bytes, err := json.MarshalIndent(p, "", "\t")
	if err != nil {
		return err
	}

	// TODO: TODO: Sync?
	return ioutil.WriteFile(path, bytes, 0666)
}

func findRopefile() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}

	for {
		path := filepath.Join(dir, "rope.json")
		_, err := os.Stat(path)
		if errors.Is(err, os.ErrNotExist) {
			if filepath.Dir(dir) == dir {
				return "", fmt.Errorf("rope.json not found (or in any of the parent directories)")
			}
			dir = filepath.Dir(dir)

			continue
		} else if err != nil {
			return "", err
		}

		return path, nil
	}
}
