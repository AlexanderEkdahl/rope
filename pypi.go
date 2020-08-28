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

// PyPi is an abstraction over interacting with the Python Package Index.
// Check if PEP503 is the relevant PEP for this?
// TODO: Rename to Index?
type PyPI struct {
	url string
}

func (p *PyPI) FindPackage(ctx context.Context, name string, v version.Version) (Package, error) {
	var foundPackage Package

	// Search the local cache if the version is specified
	if v.NE(semver.Version{}) {
		// TODO: Cache must be keyed on the package index or it risks getting the wrong package
		matches, err := filepath.Glob(fmt.Sprintf("./ropedir/cache/%s-%s*", name, v))
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

				if fi.Compatible("cp37", "", runtime.GOOS) {
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
		r, err := http.NewRequestWithContext(ctx, http.MethodGet, fmt.Sprintf("%s/%s", p.url, name), nil)
		if err != nil {
			return nil, err
		}

		res, err := http.DefaultClient.Do(r)
		if err != nil {
			return nil, err
		}
		defer res.Body.Close()

		// TODO: Check non 200

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

				p, ok := p.checkCompatability(href)
				if ok {
					// If no version is specified find the latest compatible version
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
		return nil, fmt.Errorf("package %q not found in %q", name, fmt.Sprintf("%s/%s", p.url, name))
	}

	if wheel, ok := foundPackage.(*Wheel); ok {
		if err := wheel.extractDependencies(ctx); err != nil {
			return nil, fmt.Errorf("extracting dependencies: %w", err)
		}
	}

	return foundPackage, nil
}

func (p *PyPI) checkCompatability(href string) (Package, bool) {
	url, err := url.Parse(href)
	if err != nil {
		return nil, false
	}

	filename := path.Base(url.EscapedPath())
	if strings.HasSuffix(filename, ".whl") {
		fi, err := ParseWheelFilename(filename)
		if err != nil {
			// fmt.Println("debug_err", err)
			return nil, false
		}

		if !fi.Compatible("cp37", "", runtime.GOOS) {
			return nil, false
		}

		v, err := version.Parse(fi.Version)
		if err != nil {
			// fmt.Println("debug_err", "invalid semver", err)
			return nil, false
		}

		return &Wheel{
			name:     fi.Name,
			filename: filename,
			version:  v,
			url:      href,
		}, true
	} else if strings.HasSuffix(filename, ".tar.gz") {
		sdist, err := ParseSdistFilename(filename)
		if err != nil {
			return nil, false
		}
		sdist.url = href

		return sdist, true
	} else if strings.HasSuffix(filename, ".egg") {
		// Ignore .egg distributions
		return nil, false
	} else if strings.HasSuffix(filename, ".exe") {
		// Ignore .exe distributions
		return nil, false
	} else if strings.HasSuffix(filename, ".zip") {
		// Ignore .zip distributions
		return nil, false
	}

	fmt.Println("debug", "unknown distribution type:", filename)
	return nil, false
}
