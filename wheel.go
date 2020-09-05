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
	"runtime"
	"strings"

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
		Name:          NormalizePackageName(split[0]),
		Version:       split[1],
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

	url string // Only set when found in package manager?

	dependencies []Dependency
}

func (p *Wheel) Name() string {
	return p.name
}

func (p *Wheel) Version() version.Version {
	return p.version
}

func (p *Wheel) Dependencies() []Dependency {
	return p.dependencies
}

// Install installs the wheel and returns the path to the installaction location.
func (p *Wheel) Install(ctx context.Context) (string, error) {
	path, err := p.download(ctx)
	if err != nil {
		return "", err
	}

	return InstallWheel(path)
}

// TODO: Move this method into Wheel.Install(and have sdist embed a Wheel after it has been installed)
func InstallWheel(whlPath string) (string, error) {
	filename := filepath.Base(whlPath)

	// TODO: os.UserCacheDir() or UserConfigDir?
	installPath := filepath.Join("./ropedir", "0", strings.TrimSuffix(filename, ".whl"))
	if _, err := os.Stat(installPath); errors.Is(err, os.ErrNotExist) {
		// continue with installation
	} else if err != nil {
		return "", err
	} else {
		return installPath, nil
	}
	fmt.Println("installing wheel: ", filename)

	// TODO: Verify files as they are being read
	whlFile, err := zip.OpenReader(whlPath)
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

// download fetches the package if not in cache.
func (p *Wheel) download(ctx context.Context) (string, error) {
	cache, err := GetCacheDir()
	if err != nil {
		return "", err
	}
	cachePath := filepath.Join(cache, "wheel", p.name, p.filename)

	// Check that it is the correct file?
	if _, err := os.Stat(cachePath); errors.Is(err, os.ErrNotExist) {
		// download file
	} else if err != nil {
		return "", err
	} else if err == nil {
		// fmt.Printf("Found cache for %s\n", p.filename)
		return cachePath, nil
	}
	fmt.Printf("Downloading %s\n", p.filename)
	if err := os.MkdirAll(filepath.Dir(cachePath), 0777); err != nil {
		return "", err
	}

	parsedURL, err := url.Parse(p.url)
	if err != nil {
		return "", err
	}
	values, err := url.ParseQuery(parsedURL.Fragment)
	if err != nil {
		return "", err
	}

	r, err := http.NewRequestWithContext(ctx, http.MethodGet, p.url, nil)
	if err != nil {
		return "", err
	}

	res, err := http.DefaultClient.Do(r)
	if err != nil {
		return "", err
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		return "", fmt.Errorf("failed HTTP request: %s", res.Status)
	}

	// TODO: Make this atomic in case of failures/interruptions
	file, err := os.Create(cachePath)
	if err != nil {
		return "", err
	}
	defer file.Close()

	var sum []byte
	var hash hash.Hash
	var reader io.Reader = res.Body
	if len(values["sha256"]) > 0 {
		var err error
		sum, err = hex.DecodeString(values["sha256"][0])
		if err != nil {
			return "", fmt.Errorf("sha256 checksum invalid hex: %w", err)
		}
		hash = sha256.New()
		reader = io.TeeReader(res.Body, hash)
	}

	_, err = io.Copy(file, reader)
	if err != nil {
		return "", err
	}

	if len(sum) > 0 && !bytes.Equal(sum, hash.Sum(nil)) {
		return "", fmt.Errorf("checksum mismatch, got: %x, expected: %x", hash.Sum(nil), sum)
	}

	return cachePath, file.Close()
}

func (p *Wheel) extractDependencies(ctx context.Context) error {
	path, err := p.download(ctx)
	if err != nil {
		return err
	}

	whlFile, err := zip.OpenReader(path)
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
	return nil
}

func (p *Wheel) extractDependenciesFromMetadata(metadata io.Reader) ([]Dependency, error) {
	dependencies := []Dependency{}

	scanner := bufio.NewScanner(metadata)
	for scanner.Scan() {
		row := scanner.Text()
		if strings.HasPrefix(row, "Requires-Dist: ") {
			// Remove split as it is being handled by ParseDependencySpecification
			split := strings.Split(strings.TrimPrefix(row, "Requires-Dist: "), ";")
			if len(split) > 1 {
				// fmt.Printf("skipping dependency: '%s'\n", row)
				continue
			}
			dep, err := version.ParseDependencySpecification(split[0])
			if err != nil {
				return nil, err
			}

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

// findCachedWheel searches the wheel cache to see if there is a wheel already
// built that is compatible with the current environment.
// Note: name is expected to be normalized
func findCachedWheel(name string, v version.Version) (string, error) {
	if v.Unspecified() {
		// If version is not specified then a new version may have
		// been released and it should check upstream index.
		return "", nil
	}

	cache, err := GetCacheDir()
	if err != nil {
		return "", err
	}

	dir := filepath.Join(cache, "wheel", name)
	wheels, err := ioutil.ReadDir(dir)
	if errors.Is(err, os.ErrNotExist) {
		return "", nil
	} else if err != nil {
		return "", err
	}

	for _, wheel := range wheels {
		fi, err := ParseWheelFilename(wheel.Name())
		if err != nil {
			return "", err
		}

		// TODO: Proper version equality check
		if fi.Version == v.String() && fi.Compatible("cp37", "", runtime.GOOS, runtime.GOARCH) {
			return filepath.Join(dir, wheel.Name()), nil
		}
	}

	return "", nil
}
