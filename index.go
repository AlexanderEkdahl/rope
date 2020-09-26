package main

import (
	"context"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path"
	"strconv"
	"strings"

	"github.com/AlexanderEkdahl/rope/version"
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
	name = NormalizePackageName(name)

	wheel, err := checkCache(ctx, name, v)
	if err != nil {
		return nil, err
	} else if wheel != nil {
		return wheel, nil
	}

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

	var foundPackages []Package
	var foundVersion version.Version
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
				if v.Unspecified() {
					if !foundVersion.Unspecified() && p.Version().GreaterThan(foundVersion) {
						// Reset found packages since a greater version has been found.
						foundPackages = nil
					}

					foundPackages = append(foundPackages, p)
					foundVersion = p.Version()
				} else if p.Version().Match(v) {
					foundPackages = append(foundPackages, p)
				} else if !foundVersion.Unspecified() {
					// stop early since all packages matching the version v has been found.
					break
				}
			}
		}
	}

	if len(foundPackages) == 0 {
		return nil, fmt.Errorf("compatible package not found")
	}
	foundPackage := selectPrefered(foundPackages, env)

	if v.Unspecified() {
		// If the original query did not specify a version, check the cache to see if
		// the found package is already present in the cache.
		wheel, err := checkCache(ctx, name, foundPackage.Version())
		if err != nil {
			return nil, err
		} else if wheel != nil {
			return wheel, nil
		}
	}

	if p, ok := foundPackage.(interface{ extractDependencies(context.Context) error }); ok {
		if err := p.extractDependencies(ctx); err != nil {
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

	filename := path.Base(url.Path)
	if strings.HasSuffix(filename, ".whl") {
		whl, err := ParseWheelFilename(filename)
		if err != nil {
			return nil, false
		}
		whl.URL = href

		if !whl.Compatible(env) {
			return nil, false
		}

		return whl, true
	} else if sdistSuffix := sourceDistributionSuffix(filename); sdistSuffix != "" {
		sdist, err := ParseSdistFilename(filename, sdistSuffix)
		if err != nil {
			return nil, false
		}
		sdist.url = href

		return sdist, true
	} else {
		return nil, false
	}
}

// LinkIndex is a simple form of an index such as:
// https://download.pytorch.org/whl/torch_stable.html
type LinkIndex struct {
	url string
}

// FindPackage finds the package with the specified name and optionally version in
// the index.
func (i *LinkIndex) FindPackage(ctx context.Context, name string, v version.Version) (Package, error) {
	name = NormalizePackageName(name)

	wheel, err := checkCache(ctx, name, v)
	if err != nil {
		return nil, err
	} else if wheel != nil {
		return wheel, nil
	}

	r, err := http.NewRequestWithContext(ctx, http.MethodGet, i.url, nil)
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
	default:
		return nil, fmt.Errorf("failed HTTP request: %s", res.Status)
	}

	dec := xml.NewDecoder(res.Body)
	for {
		token, err := dec.Token()
		var syntaxError *xml.SyntaxError
		if err == io.EOF {
			break
		} else if errors.As(err, &syntaxError) && syntaxError.Msg == "unexpected EOF" {
			// Safe to assume no more links will be found unless index download was
			// unexpectedly terminated half way through. This is here since pip
			// seemingly does not care about invalid XML.
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

			url, err := url.Parse(href)
			if err != nil {
				return nil, err
			}

			var p Package
			filename := path.Base(url.Path)
			if strings.HasSuffix(filename, ".whl") {
				whl, err := ParseWheelFilename(filename)
				if err != nil {
					return nil, err
				}

				if !whl.Compatible(env) {
					continue
				}

				p = whl
			} else if suffix := sourceDistributionSuffix(filename); suffix != "" {
				sdist, err := ParseSdistFilename(filename, suffix)
				if err != nil {
					return nil, err
				}

				p = sdist
			}

			if !p.Version().Equal(v) {
				continue
			}

			// TODO: Support ordering found packages by preference
			return p, nil
		}
	}

	return nil, nil
}

func checkCache(ctx context.Context, name string, v version.Version) (*Wheel, error) {
	// TODO: Move this into the cache(and cache dependency list).
	if wheel, err := cache.GetWheel(name, v); err != nil {
		return nil, err
	} else if cacheOnly, _ := strconv.ParseBool(os.Getenv("ROPE_CACHE_ONLY")); cacheOnly && wheel == nil {
		return nil, fmt.Errorf("package not found in cache (ROPE_CACHE_ONLY is set)")
	} else if wheel != nil {
		return wheel, nil
	}

	return nil, nil
}

// Multiple packages may match a query; select the preferred package
func selectPrefered(packages []Package, env *Environment) Package {
	var p Package = packages[0]

	for i := 1; i < len(packages); i++ {
		// This will select that last preferred package in the case of multiple
		// packages being equally preferred.
		if preferred(packages[i], p, env) {
			p = packages[i]
		}
	}

	return p
}

// preferred returns true if a should be the preferred distribution to install
// compared to b. Binary distributions are preferred over source distributions
// and competing binary distributions are compared using the specifity of the
// tag.
func preferred(a, b Package, env *Environment) bool {
	aWheel, aIsWheel := a.(*Wheel)
	bWheel, bIsWheel := b.(*Wheel)
	if aIsWheel && !bIsWheel {
		return true
	} else if !aIsWheel && bIsWheel {
		return false
	} else if !aIsWheel && !bIsWheel {
		// Both packages are source distributions. Prefer a over b for stability.
		return true
	}

	return aWheel.Preference(env) >= bWheel.Preference(env)
}

// sourceDistributionSuffix returns the suffix of the source distribution or
// an empty string if the filename is not associated with a common source
// distribution suffix.
func sourceDistributionSuffix(filename string) string {
	switch {
	case strings.HasSuffix(filename, ".tar.gz"):
		return ".tar.gz"
	case strings.HasSuffix(filename, ".zip"):
		return ".zip"
	case strings.HasSuffix(filename, ".tar.bz2"):
		return ".tar.bz2"
	case strings.HasSuffix(filename, ".tgz"):
		return ".tgz"
	default:
		return ""
	}
}
