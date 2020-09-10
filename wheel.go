package main

import (
	"archive/zip"
	"bufio"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"hash"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/AlexanderEkdahl/rope/version"
)

// https://www.python.org/dev/peps/pep-0427/#file-name-convention
type WheelFilename struct {
	// TODO: Merge this with Wheel like Sdist
	Name          string
	Version       version.Version
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

	v, valid := version.Parse(split[1])
	if !valid {
		return WheelFilename{}, fmt.Errorf("invalid version in wheel package name: '%s'", split[1])
	}

	return WheelFilename{
		Name:          NormalizePackageName(split[0]),
		Version:       v,
		Build:         build,
		PythonVersion: split[len(split)-3],
		ABI:           split[len(split)-2],
		Platform:      split[len(split)-1],
	}, nil
}

func (fi *WheelFilename) Compatible(pythonVersion, abi, goos, gorch string) bool {
	if !CompatiblePython(fi.PythonVersion, pythonVersion) {
		return false
	}

	if !CompatibleABI(fi.ABI, abi) {
		return false
	}

	if !CompatiblePlatform(fi.Platform, goos, gorch) {
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
func CompatiblePlatform(platform, goos, goarch string) bool {
	if platform == "any" {
		return true
	}

	split := strings.Split(platform, ".")
	if len(split) > 1 {
		for _, part := range split {
			if CompatiblePlatform(part, goos, goarch) {
				return true
			}
		}
	}

	return compatibleOS(platform, goos) && compatibleCPUArchitecture(platform, goarch)
}

func compatibleOS(platform string, goos string) bool {
	// TODO: Implement proper matching
	switch goos {
	case "darwin":
		return strings.HasPrefix(platform, "macosx")
	case "linux":
		// https://www.python.org/dev/peps/pep-0513/
		// https://www.python.org/dev/peps/pep-0571/
		// https://www.python.org/dev/peps/pep-0599/
		// matches manylinux1, manylinux2010, manylinux2014
		// TODO: Implement platform detection to check for specific manylinux support
		// Investigate if it should prefer the highest version?
		return strings.HasPrefix(platform, "manylinux") || strings.HasPrefix(platform, "linux")
	default:
		fmt.Printf("unknown OS: %s\n", goos)
		return false
	}
}

func compatibleCPUArchitecture(platform, goarch string) bool {
	switch goarch {
	case "amd64":
		return strings.HasSuffix(platform, "x86_64") || strings.HasSuffix(platform, "amd64")
	case "i386":
		return strings.HasSuffix(platform, "i686")
	case "arm64":
		return strings.HasSuffix(platform, "aarch64")
	default:
		return false
	}
}

// https://pythonwheels.com
// TODO: Merge this and WheelFilename?
type Wheel struct {
	name     string // Canonical name
	filename string
	version  version.Version

	// path is only set when the package has been made available on the filesystem.
	path string
	// url is only set when the package was found in a remote package repository.
	url string

	dependencies []Dependency
}

// Name returns the canonical name of the Wheel package.
func (p *Wheel) Name() string {
	return p.name
}

func (p *Wheel) Version() version.Version {
	return p.version
}

func (p *Wheel) Dependencies() []Dependency {
	return p.dependencies
}

// Install unpacks the wheel and returns the path to the installaction location.
func (p *Wheel) Install(ctx context.Context) (string, error) {
	if err := p.fetch(ctx); err != nil {
		return "", err
	}

	filename := filepath.Base(p.path)

	// TODO: UserConfigDir?
	installPath := filepath.Join("./ropedir", "0", strings.TrimSuffix(filename, ".whl"))
	if _, err := os.Stat(installPath); errors.Is(err, os.ErrNotExist) {
		// continue with installation
	} else if err != nil {
		return "", err
	} else {
		return installPath, nil
	}
	fmt.Println("installing wheel:", filename)

	// TODO: Verify files as they are being read
	whlFile, err := zip.OpenReader(p.path)
	if err != nil {
		return "", err
	}
	defer whlFile.Close()

	for _, file := range whlFile.File {
		// TODO: Use Record to read files and verify SHA256
		f, err := file.Open()
		if err != nil {
			return "", err
		}

		if file.FileInfo().IsDir() {
			// Skip directories as parent directories are automatically
			// created. However, this may cause issues if a package
			// expects an empty folder in a certain location.
			continue
		}

		target := filepath.Join(installPath, file.Name)
		// TODO: Final directory should be created with 0500
		if err := os.MkdirAll(filepath.Dir(target), 0777); err != nil {
			return "", err
		}

		// Write-protected files prevents users from inadvertendly modifying its
		// dependencies and thereby affecing other projects.
		dst, err := os.OpenFile(target, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0444)
		if err != nil {
			return "", err
		}

		if _, err := io.Copy(dst, f); err != nil {
			return "", err
		}

		if err := f.Close(); err != nil {
			return "", err
		}
	}

	// TODO: Make this atomic in the event of concurrent installs?
	return installPath, nil
}

// fetch downloads the package from the remote index.
func (p *Wheel) fetch(ctx context.Context) error {
	if p.path != "" {
		// wheel already fetched
		return nil
	}

	if p.url == "" {
		panic("wheel download: missing url")
	}
	fmt.Printf("Downloading %s\n", p.filename)
	parsedURL, err := url.Parse(p.url)
	if err != nil {
		return err
	}
	values, err := url.ParseQuery(parsedURL.Fragment)
	if err != nil {
		return err
	}

	r, err := http.NewRequestWithContext(ctx, http.MethodGet, p.url, nil)
	if err != nil {
		return err
	}

	res, err := http.DefaultClient.Do(r)
	if err != nil {
		return err
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		return fmt.Errorf("failed HTTP request: %s", res.Status)
	}

	file, err := ioutil.TempFile("", fmt.Sprintf("%s-*", p.filename))
	if err != nil {
		return err
	}
	defer file.Close()

	var sum []byte
	var hash hash.Hash
	var reader io.Reader = res.Body
	if len(values["sha256"]) > 0 {
		var err error
		sum, err = hex.DecodeString(values["sha256"][0])
		if err != nil {
			return fmt.Errorf("sha256 checksum invalid hex: %w", err)
		}
		hash = sha256.New()
		reader = io.TeeReader(res.Body, hash)
	}

	_, err = io.Copy(file, reader)
	if err != nil {
		return err
	}

	if len(sum) > 0 && !bytes.Equal(sum, hash.Sum(nil)) {
		return fmt.Errorf("checksum mismatch, got: %x, expected: %x", hash.Sum(nil), sum)
	}

	if err := file.Close(); err != nil {
		return fmt.Errorf("closing file after download: %w", err)
	}

	cachedPath, err := cache.AddWheel(p, file.Name())
	if err != nil {
		return err
	}

	p.path = cachedPath
	return nil
}

func (p *Wheel) extractDependencies(ctx context.Context) error {
	if err := p.fetch(ctx); err != nil {
		return err
	}

	whlFile, err := zip.OpenReader(p.path)
	if err != nil {
		return err
	}
	defer whlFile.Close()

	dependencies := []Dependency{}
	var metadata *zip.File
	for _, file := range whlFile.File {
		// TODO: Verify contents of METADATA by also reading RECORD
		if filepath.Base(file.Name) == "METADATA" {
			metadata = file
			break
		}
	}
	if metadata == nil {
		return fmt.Errorf("METADATA file not found in .whl")
	}
	metadataFile, err := metadata.Open()
	if err != nil {
		return err
	}
	defer metadataFile.Close()

	dependencies, err = p.extractDependenciesFromMetadata(metadataFile)
	if err != nil {
		return fmt.Errorf("extracting dependencies from METADATA: %w", err)
	}

	p.dependencies = dependencies
	// Cache result in a newline delimited JSON file.
	return nil
}

func (p *Wheel) extractDependenciesFromMetadata(metadata io.Reader) ([]Dependency, error) {
	dependencies := []Dependency{}

	scanner := bufio.NewScanner(metadata)
	for scanner.Scan() {
		row := scanner.Text()
		if strings.HasPrefix(row, "Requires-Dist: ") {
			// Remove split as it is being handled by ParseDependencySpecification
			dep, err := version.ParseDependencySpecification(strings.TrimPrefix(row, "Requires-Dist: "))
			if err != nil {
				// fmt.Printf("💀 %s(%v)\n", row, err)
				continue
			}
			if len(dep.Extras) > 0 {
				// fmt.Printf("💀 %s(extras)\n", row)
				continue
			}

			// fmt.Printf("🍀 %s %s(minimal = %s)\n", p.name, row, version.Minimal(dep.Versions))
			dependencies = append(dependencies, Dependency{
				Name:    NormalizePackageName(dep.DistributionName),
				Version: version.Minimal(dep.Versions),
			})
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}

	return dependencies, nil
}
