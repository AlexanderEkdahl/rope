package main

import (
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"

	"github.com/AlexanderEkdahl/rope/version"
)

/*

Files downloaded from an index are stored in the same way they would be stored
in a Python Package Repository (PEP 508). The cache also holds wheel packages
that have been built from source distributions. The cache is not supposed to
store downloaded source distributions as they should be converted to Python
wheels after download.

<os.UserCacheDir()> / <cacheVersion> / <Index> / <NormalizedPackageName> / <file>

TODO: Add <Index> to cache path.
TODO: The cache should contain rope metadata files(name, version, dependencies, checksum)
inspired by https://github.com/rust-lang/crates.io-index
TODO: Automatically disable caching when running in docker(by checking existence of /.dockerenv)
unless explicitly enabled.

*/

// cacheVersion is the version of the cache. If the cache is ever changed in
// a backward incompatible manner this value will be changed.
const cacheVersion = "0"

// Cache is responsible for caching package package downloads and built
// source distributions.
//
// If temporary is true, path will be ignored and a temporary cache will
// be created. If path is provided that will be the path to the cache.
type Cache struct {
	Temporary bool
	Path      string

	once sync.Once
	err  error
}

// GetWheel searches the cache for the package identified by name and the provided version.
// If no cached entry can be found nil is returned.
// TODO: Full URL from the index should be part of the cache path.
func (c *Cache) GetWheel(name string, v version.Version) (*Wheel, error) {
	if v.Unspecified() {
		return nil, nil
	}
	fmt.Printf("searching cache for %s-%s", name, v)

	c.once.Do(c.setup)
	if c.err != nil {
		return nil, c.err
	}

	dir := c.getPath(name)
	entries, err := ioutil.ReadDir(dir)
	if errors.Is(err, os.ErrNotExist) {
		fmt.Printf("⛔️\n")
		return nil, nil
	} else if err != nil {
		return nil, err
	}

	for _, cachedDownload := range entries {
		filename := cachedDownload.Name()

		// Only search for .whl files.
		if strings.HasSuffix(filename, ".whl") {
			fi, err := ParseWheelFilename(filename)
			if err != nil {
				return nil, err
			}

			if fi.Version.Equal(v) && fi.Compatible("cp37", "", runtime.GOOS, runtime.GOARCH) {
				fmt.Printf("✅\n")
				return &Wheel{
					name:     fi.Name,
					filename: filename,
					version:  fi.Version,

					path: filepath.Join(dir, filename),
				}, nil
			}
		}
	}
	fmt.Printf("⛔️\n")

	return nil, nil
}

// AddWheel moves the Python Wheel located at path to the cache.
// TODO: Full URL from the index should be part of the cache path.
func (c *Cache) AddWheel(w *Wheel, path string) (string, error) {
	c.once.Do(c.setup)
	newpath := filepath.Join(c.getPath(w.name), w.filename)

	if err := os.MkdirAll(filepath.Dir(newpath), 0777); err != nil {
		return "", fmt.Errorf("creating cache directory: %w", err)
	}

	if err := os.Rename(path, newpath); err != nil {
		return "", fmt.Errorf("moving item to cache: %w", err)
	}

	return newpath, nil
}

func (c *Cache) getPath(name string) string {
	return filepath.Join(c.Path, cacheVersion, NormalizePackageName(name))
}

func (c *Cache) setup() {
	if c.Temporary {
		path, err := ioutil.TempDir("", "rope-cache-*")
		if err != nil {
			c.err = err
		}
		c.Path = path
		return
	}

	if c.Path == "" {
		userCacheDir, err := os.UserCacheDir()
		if err != nil {
			c.err = err
			return
		}
		c.Path = filepath.Join(userCacheDir, "rope")
	}

	if err := os.MkdirAll(c.Path, 0777); err != nil {
		c.err = fmt.Errorf("creating cache directory: %w", err)
	}
}

// Close removes the cache directory if caching is temporary.
func (c *Cache) Close() error {
	setup := true
	c.once.Do(func() {
		setup = false
	})
	if setup && c.Temporary {
		os.RemoveAll(c.Path)
	}

	return nil
}
