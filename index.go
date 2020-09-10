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

	// TODO: Normalize request to avoid redirect?
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

	var foundPackage Package
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
				if v.Unspecified() {
					foundPackage = p
				} else if p.Version().Match(v) {
					foundPackage = p
					break
				}
			}
		}
	}

	if foundPackage == nil {
		return nil, ErrPackageNotFound
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
		fi, err := ParseWheelFilename(filename)
		if err != nil {
			return nil, false
		}

		if !fi.Compatible("cp37", "", runtime.GOOS, runtime.GOARCH) {
			return nil, false
		}

		return &Wheel{
			name:     fi.Name,
			filename: filename,
			version:  fi.Version,
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
		return nil, false
	case strings.HasSuffix(filename, ".exe"):
		return nil, false
	case strings.HasSuffix(filename, ".zip"):
		return nil, false
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
