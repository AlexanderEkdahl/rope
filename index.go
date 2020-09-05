package main

import (
	"context"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/AlexanderEkdahl/rope/version"
	"github.com/blang/semver/v4"
)

// Defalt constants for the Python Package index.
const (
	PythonPackageIndex = "https://pypi.python.org"
	DefaultIndex       = PythonPackageIndex + "/simple"
)

// Index is an abstraction over interacting with a Python package repository
// such as the Python Package Index.
// The "API" is defined at: https://www.python.org/dev/peps/pep-0503/
type Index struct {
	url string
}

func (i *Index) FindPackage(ctx context.Context, name string, v version.Version) (Package, error) {
	var foundPackage Package

	// Search the local cache if the version is specified
	if v.NE(semver.Version{}) {
		cache, err := GetCacheDir()
		if err != nil {
			return nil, err
		}

		// TODO: Cache must be keyed on the package index or it risks getting the wrong package
		matches, err := filepath.Glob(filepath.Join(cache, fmt.Sprintf("%s-%s*", name, v)))
		if err != nil {
			return nil, err
		}

		// TODO: Ensure matches are of the right python/arch
		for _, match := range matches {
			filename := path.Base(match)
			if strings.HasSuffix(filename, ".whl") {
				fi, err := ParseWheelFilename(filename)
				if err != nil {
					return nil, err
				}

				if fi.Compatible("cp37", "", runtime.GOOS, runtime.GOARCH) {
					foundPackage = &Wheel{
						name:     fi.Name,
						filename: filename,
						version:  v,
					}
					break
				}
			} else if strings.HasSuffix(filename, ".tar.gz") {
				sdist, err := ParseSdistFilename(filename)
				if err != nil {
					return nil, err
				}
				foundPackage = sdist
				break
			}
		}
	}

	if foundPackage == nil {
		r, err := http.NewRequestWithContext(ctx, http.MethodGet, fmt.Sprintf("%s/%s/", i.url, name), nil)
		if err != nil {
			return nil, err
		}

		res, err := http.DefaultClient.Do(r)
		if err != nil {
			return nil, err
		}
		defer res.Body.Close()

		switch res.StatusCode {
		case http.StatusOK:
			// continue
		case http.StatusNotFound:
			return nil, ErrPackageNotFound
		default:
			return nil, fmt.Errorf("failed HTTP request: %s", res.Status)
		}

		dec := xml.NewDecoder(res.Body)
		for {
			token, err := dec.Token()
			if err == io.EOF {
				break
			} else if err != nil {
				return nil, err
			}

			if token, ok := token.(xml.StartElement); ok && token.Name.Local == "a" {
				href := ""
				for _, attr := range token.Attr {
					if attr.Name.Local == "href" {
						href = attr.Value
					}
				}

				p, ok := i.checkCompatability(href)
				if ok {
					// If no version is specified find the latest compatible version
					// TODO: This is unstable as it may select a different version depending on
					// whether or not looking for the latest version or the same version but
					// specified explicitly.
					// For example in the case of https://pypi.org/simple/six/ the <latest> is
					// six-1.15.0.tar.gz but six==1.15.0 is six-1.15.0-py2.py3-none-any.whl
					// Must implement version selection preference to ensure the same variant
					// is being selected.
					if v.EQ(semver.Version{}) {
						foundPackage = p
					} else if p.Version().GTE(v.Version) {
						// Finds the first compatible package. This may result in a higher version
						// than requested.
						// TODO: Investigate merits of this vs using Equals

						// Return early as the minimal version has been found(assumes index is ordered?) verify
						// Investigate this to avoid pre-release versions etc(unless specifically requested?)
						foundPackage = p
						break
					}
				}
			}
		}
	}

	if foundPackage == nil {
		return nil, ErrPackageNotFound
	}

	if wheel, ok := foundPackage.(*Wheel); ok {
		if err := wheel.extractDependencies(ctx); err != nil {
			return nil, fmt.Errorf("extracting dependencies: %w", err)
		}
	}

	return foundPackage, nil
}

func (i *Index) checkCompatability(href string) (Package, bool) {
	url, err := url.Parse(href)
	if err != nil {
		return nil, false
	}

	filename := path.Base(url.EscapedPath())
	switch {
	case strings.HasSuffix(filename, ".whl"):
		fi, err := ParseWheelFilename(filename)
		if err != nil {
			return nil, false
		}

		if !fi.Compatible("cp37", "", runtime.GOOS, runtime.GOARCH) {
			return nil, false
		}

		v, err := version.Parse(fi.Version)
		if err != nil {
			return nil, false
		}

		return &Wheel{
			name:     fi.Name,
			filename: filename,
			version:  v,
			url:      href,
		}, true
	case strings.HasSuffix(filename, ".tar.gz"):
		sdist, err := ParseSdistFilename(filename)
		if err != nil {
			return nil, false
		}
		sdist.url = href

		return sdist, true
	case strings.HasSuffix(filename, ".egg"):
		fallthrough
	case strings.HasSuffix(filename, ".exe"):
		fallthrough
	case strings.HasSuffix(filename, ".zip"):
		fallthrough
	case strings.HasSuffix(filename, ".rpm"):
		return nil, false
	case strings.HasSuffix(filename, ".tar.bz2"):
		return nil, false
	case strings.HasSuffix(filename, ".tgz"):
		return nil, false
	default:
		fmt.Println("unknown distribution type:", filename)
		return nil, false
	}
}
