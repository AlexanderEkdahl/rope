package main

import (
	"archive/zip"
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/AlexanderEkdahl/rope/pep508"
	"github.com/AlexanderEkdahl/rope/version"
)

// https://www.python.org/dev/peps/pep-0427/#file-name-convention
type WheelFilename struct {
	// Merge this with Wheel?
	Name          string
	Version       string
	Build         string
	PythonVersion string
	ABI           string
	Platform      string
}

func ParseWheelFilename(filename string) (WheelFilename, error) {
	trim := strings.TrimSuffix(filename, ".whl")
	if filename == trim {
		return WheelFilename{}, fmt.Errorf("not a wheel")
	}

	build := ""
	split := strings.Split(trim, "-")
	switch {
	case len(split) < 5:
		return WheelFilename{}, fmt.Errorf("expected wheel to be in at least 5 parts, got: %s", filename)
	case len(split) == 6:
		build = split[2]
	case len(split) > 6:
		return WheelFilename{}, fmt.Errorf("expected wheel to be in at most 6 parts, got: %s", filename)
	}

	return WheelFilename{
		Name:          split[0],
		Version:       split[1],
		Build:         build,
		PythonVersion: split[len(split)-3],
		ABI:           split[len(split)-2],
		Platform:      split[len(split)-1],
	}, nil
}

func (fi *WheelFilename) Compatible(pythonVersion, abi, goos string) bool {
	if !CompatiblePython(fi.PythonVersion, pythonVersion) {
		return false
	}

	if !CompatibleABI(fi.ABI, abi) {
		return false
	}

	if !CompatiblePlatform(fi.Platform, goos) {
		return false
	}

	return true
}

// https://www.python.org/dev/peps/pep-0425/
// The following functions should return how specific they are so that the most
// specific version can be selected.

// CompatiblePython checks if two python versions are compatible.
func CompatiblePython(candidateVersion, pythonVersion string) bool {
	split := strings.Split(candidateVersion, ".")
	if len(split) > 1 {
		for _, part := range split {
			if CompatiblePython(part, pythonVersion) {
				return true
			}
		}
	}

	if candidateVersion == "py3" {
		return true
	}

	if strings.HasPrefix(pythonVersion, candidateVersion) {
		return true
	}

	return false
}

func CompatibleABI(candidate, abi string) bool {
	return true
}

// TODO: Rank by specificity
func CompatiblePlatform(platform, goos string) bool {
	if platform == "any" {
		return true
	}

	split := strings.Split(platform, ".")
	if len(split) > 1 {
		for _, part := range split {
			if CompatiblePlatform(part, goos) {
				return true
			}
		}
	}

	switch goos {
	case "darwin":
		return strings.HasPrefix(platform, "macosx")
	default:
		panic(fmt.Sprintf("unsupported OS: %s", goos))
	}
}

// https://pythonwheels.com
type Wheel struct {
	name     string
	filename string
	version  version.Version
	url      string // Only set when found in package manager?

	dependencies []Dependency
}

func (p *Wheel) Name() string {
	// Currently working on the casing of name differs from request version
	// to the found version. This should be resolved by using the index
	// lookup as the source of truth
	return p.name
}

func (p *Wheel) Version() version.Version {
	return p.version
}

func (p *Wheel) Dependencies() []Dependency {
	return p.dependencies
}

func (p *Wheel) Install(ctx context.Context) error {
	if err := p.download(ctx); err != nil {
		return err
	}

	cachePath := fmt.Sprintf("./ropedir/cache/%s", p.filename)
	return ExtractWheel(cachePath)
}

func ExtractWheel(path string) error {
	filename := filepath.Base(path)

	// TODO: os.UserCacheDir()
	installPath := fmt.Sprintf("./ropedir/packages/%s", strings.TrimSuffix(filename, ".whl"))

	if _, err := os.Stat(installPath); errors.Is(err, os.ErrNotExist) {
		// continue with installation
	} else if err != nil {
		return err
	} else {
		fmt.Println("package already installed: 🧩")
		return nil
	}

	// TODO: Verify files as they are being read
	whlFile, err := zip.OpenReader(path)
	if err != nil {
		return err
	}
	defer whlFile.Close()

	for _, file := range whlFile.File {
		// TODO: Use Record to read files and verify SHA256
		f, err := file.Open()
		if err != nil {
			return err
		}

		target := filepath.Join(installPath, file.Name)
		// TODO: Final directory should be created with 0500
		if err := os.MkdirAll(filepath.Dir(target), 0777); err != nil {
			return err
		}

		// TODO: Write-protected files prevents users from inadvertendly modifying its
		// dependencies and thereby affecing other projects.
		dst, err := os.OpenFile(target, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0444)
		if err != nil {
			return err
		}

		if _, err := io.Copy(dst, f); err != nil {
			return err
		}

		if err := f.Close(); err != nil {
			return err
		}
	}

	return nil
}

// download fetches the package if not in cache.
func (p *Wheel) download(ctx context.Context) error {
	// TODO: resolve ropedir
	path := fmt.Sprintf("./ropedir/cache/%s", p.filename)

	// Check that it is the correct file?
	_, err := os.Stat(path)
	if errors.Is(err, os.ErrNotExist) {
		// download file
	} else if err != nil {
		return err
	} else if err == nil {
		fmt.Printf("Found cache for %s\n", p.filename)
		return nil
	}

	fmt.Printf("Downloading %s\n", p.filename)

	// TODO: Figure out correct permissions
	// if err := os.MkdirAll("./ropedir/cache", 777); err != nil {
	// 	return fmt.Errorf("creating cache directory: %w", err)
	// }

	r, err := http.NewRequestWithContext(ctx, http.MethodGet, p.url, nil)
	if err != nil {
		return err
	}

	res, err := http.DefaultClient.Do(r)
	if err != nil {
		return err
	}
	defer res.Body.Close()

	// TODO: check 200-status

	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer file.Close()

	// TODO: Verify download
	_, err = io.Copy(file, res.Body)
	if err != nil {
		return err
	}

	return file.Close()
}

func (p *Wheel) extractDependencies(ctx context.Context) error {
	if err := p.download(ctx); err != nil {
		return err
	}

	path := fmt.Sprintf("./ropedir/cache/%s", p.filename)
	// TODO: Verify files as they are being read
	whlFile, err := zip.OpenReader(path)
	if err != nil {
		return err
	}
	defer whlFile.Close()

	dependencies := []Dependency{}
	foundMetadata := false
	for _, file := range whlFile.File {
		if filepath.Base(file.Name) == "METADATA" {
			foundMetadata = true

			metadata, err := file.Open()
			if err != nil {
				return err
			}
			dependencies, err = p.extractDependenciesFromMetadata(metadata)
			metadata.Close()
			if err != nil {
				return fmt.Errorf("extracting dependencies from METADATA: %w", err)
			}
		}
	}
	if !foundMetadata {
		return fmt.Errorf("METADATA file not found in .whl")
	}

	p.dependencies = dependencies
	return nil
}

func (p *Wheel) extractDependenciesFromMetadata(metadata io.Reader) ([]Dependency, error) {
	dependencies := []Dependency{}

	scanner := bufio.NewScanner(metadata)
	for scanner.Scan() {
		row := scanner.Text()
		if strings.HasPrefix(row, "Requires-Dist: ") {
			split := strings.Split(strings.TrimPrefix(row, "Requires-Dist: "), ";")
			if len(split) > 1 {
				// fmt.Printf("skipping dependency: '%s'\n", row)
				continue
			}
			dep, err := pep508.ParseDependency(split[0])
			if err != nil {
				return nil, err
			}
			fmt.Printf("%s: %s\n", p.name, row)

			dependencies = append(dependencies, Dependency{
				Name:    dep.DistributionName,
				Version: version.Minimal(dep.Versions),
			})
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}

	return dependencies, nil
}
