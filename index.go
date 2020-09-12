package main

import (
	"context"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path"
	"runtime"
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
	// Search the local cache first
	if wheel, err := cache.GetWheel(name, v); err != nil {
		return nil, err
	} else if cacheOnly, _ := strconv.ParseBool(os.Getenv("ROPE_CACHE_ONLY")); cacheOnly && wheel == nil {
		return nil, fmt.Errorf("package not found in cache (ROPE_CACHE_ONLY is set)")
	} else if wheel != nil {
		// TODO: Packages found in the cache should also include dependencies
		if err := wheel.extractDependencies(ctx); err != nil {
			return nil, fmt.Errorf("extracting dependencies: %w", err)
		}

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
		return nil, ErrPackageNotFound
	}

	// Multiple packages may match the version; select the preferred package
	foundPackage := foundPackages[0]
	for i := 1; i < len(foundPackages); i++ {
		// This will select that last preferred package in the case of multiple
		// packages being equally preferred.
		if Preferred(foundPackages[i], foundPackage) {
			foundPackage = foundPackages[i]
		}
	}

	// If the original query did not specify a version, check the cache to see if
	// the found package is already present in the cache.
	if v.Unspecified() {
		if wheel, err := cache.GetWheel(name, foundPackage.Version()); err != nil {
			return nil, err
		} else if wheel != nil {
			// TODO: Packages found in the cache should also include dependencies
			if err := wheel.extractDependencies(ctx); err != nil {
				return nil, fmt.Errorf("extracting dependencies: %w", err)
			}

			return wheel, nil
		}
	}

	// Cache the result of this in a JSON file to speed up interactions.
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

	filename := path.Base(url.EscapedPath())
	switch {
	case strings.HasSuffix(filename, ".whl"):
		whl, err := ParseWheelFilename(filename)
		if err != nil {
			return nil, false
		}

		if !whl.Compatible("cp37", "", runtime.GOOS, runtime.GOARCH) {
			return nil, false
		}
		whl.url = href

		return whl, true
	case strings.HasSuffix(filename, ".tar.gz"):
		sdist, err := ParseSdistFilename(filename, ".tar.gz")
		if err != nil {
			return nil, false
		}
		sdist.url = href

		return sdist, true
	case strings.HasSuffix(filename, ".egg"):
		return nil, false
	case strings.HasSuffix(filename, ".exe"):
		return nil, false
	case strings.HasSuffix(filename, ".zip"):
		sdist, err := ParseSdistFilename(filename, ".zip")
		if err != nil {
			return nil, false
		}
		sdist.url = href

		return sdist, true
	case strings.HasSuffix(filename, ".rpm"):
		return nil, false
	case strings.HasSuffix(filename, ".tar.bz2"):
		return nil, false
	case strings.HasSuffix(filename, ".tgz"):
		sdist, err := ParseSdistFilename(filename, ".tgz")
		if err != nil {
			return nil, false
		}
		sdist.url = href

		return sdist, true
	case strings.HasSuffix(filename, ".msi"):
		return nil, false
	default:
		fmt.Println("unknown distribution type:", filename)
		return nil, false
	}
}

// Preferred returns true if a should be the preferred package to install compared to b.
func Preferred(a, b Package) bool {
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

	return preferred(aWheel, bWheel)
}

func preferred(a, b *Wheel) bool {
	// TODO: Expand preference logic
	if b.platform == "any" && a.platform != "any" {
		return true
	} else if a.platform == "any" && b.platform != "any" {
		return false
	}

	return true
}
