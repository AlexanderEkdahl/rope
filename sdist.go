package main

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/AlexanderEkdahl/rope/version"
)

func ParseSdistFilename(filename string) (*Sdist, error) {
	// TODO: Support more sdist formats
	sep := strings.LastIndex(filename, "-")
	if sep < 0 {
		return nil, fmt.Errorf("expected sdist .tar.gz file to <name>-<version>.tar.gz, got: %s", filename)
	}
	versionString := strings.TrimSuffix(filename, ".tar.gz")[sep+1:]

	v, valid := version.Parse(versionString)
	if !valid {
		return nil, fmt.Errorf("invalid version: '%s'", versionString)
	}

	return &Sdist{
		name:     NormalizePackageName(filename[:sep]),
		version:  v,
		filename: filename,
		suffix:   ".tar.gz",
	}, nil
}

// Sdist is an abstraction over an "sdist" source distribution.
// This format is deprecated in favour of the Wheel format for
// distributing binary packages.
//
// Installing sdist packages requires invoking the Python
// interpreter which may in turn execute arbitary code.
type Sdist struct {
	name     string // Canonical name
	version  version.Version
	filename string
	suffix   string // TODO: Support other suffixes for source distributions.

	// url is only set when the package was found in a remote package repository.
	url string

	// Wheel built from source distribituion
	wheel *Wheel
}

// Name returns the canonical name of the source distribution package.
func (s *Sdist) Name() string { return s.name }

// Version returns the canonical version of the source distribution package.
func (s *Sdist) Version() version.Version { return s.version }

// Dependencies returns the transitive dependencies of this package. The only
// reliable way of extra
func (s *Sdist) Dependencies() []Dependency {
	// if s.wheel == nil {
	// 	panic("sdist dependencies: wheel not built")
	// }

	// return s.wheel.dependencies
	return nil
}

func (s *Sdist) extractDependencies(ctx context.Context) error {
	// if s.wheel == nil {
	// 	if err := s.convert(ctx); err != nil {
	// 		return fmt.Errorf("converting sdist to wheel: %w", err)
	// 	}
	// }

	return nil
}

// Shim to wrap setup.py invocation with setuptools. This allows rope
// to install legacy packages. This is the same method as used by pip.
//
// https://github.com/pypa/pip/blob/9cbe8fbdd0a1bd1bd4e483c9c0a556e9910ef8bb/src/pip/_internal/utils/setuptools_build.py#L14-L20
const setuptoolsShim = `import sys, setuptools, tokenize; sys.argv[0] = 'setup.py'; __file__='setup.py';f=getattr(tokenize, 'open', open)(__file__);code=f.read().replace('\\r\\n', '\\n');f.close();exec(compile(code, __file__, 'exec'))`

// convert uses `setuptools` to build a binary distribution from
// a source distribution.
func (s *Sdist) convert(ctx context.Context) error {
	fmt.Println("converting sdist:", s.filename)

	body, err := s.fetch(ctx)
	if err != nil {
		return err
	}
	defer body.Close()

	tmp, err := ioutil.TempDir("", fmt.Sprintf("%s-%s-*", s.name, s.version))
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmp)

	gzipReader, err := gzip.NewReader(body)
	if err != nil {
		return err
	}
	defer gzipReader.Close()

	tr := tar.NewReader(gzipReader)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		} else if err != nil {
			return fmt.Errorf("reading tar header: %w", err)
		}

		switch hdr.Typeflag {
		case tar.TypeDir:
			// Some tar files are somehow built without directory entries so
			// these can not be relied upon.
		case tar.TypeReg:
			// TODO: Final directory should be created with 0500
			if err := os.MkdirAll(filepath.Dir(filepath.Join(tmp, hdr.Name)), 0777); err != nil {
				return err
			}
			out, err := os.Create(filepath.Join(tmp, hdr.Name))
			if err != nil {
				return err
			}
			if _, err := io.Copy(out, tr); err != nil {
				out.Close()
				return err
			}
			if err := out.Close(); err != nil {
				return err
			}
		}
	}
	tarRoot := filepath.Join(tmp, strings.TrimSuffix(s.filename, s.suffix))
	if _, err := os.Stat(tarRoot); errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("invalid source distribution: expected %s to exist after extraction", tarRoot)
	}

	wheelPath := filepath.Join(tmp, "wheel")
	installCmd := exec.CommandContext(
		ctx,
		"python3",
		"-c",
		setuptoolsShim,
		"bdist_wheel",
		"-d",
		wheelPath,
	)
	installCmd.Dir = tarRoot
	// Ensure command is not inherenting PYTHONPATH which may inadvertendely
	// use a version of system dependencies that is too old due to a minimal
	// version selected by this program...
	installCmd.Env = append(os.Environ(), "PYTHONPATH=")
	output, err := installCmd.CombinedOutput()
	if err != nil {
		fmt.Println(string(output))
		return err
	}

	matches, err := filepath.Glob(filepath.Join(wheelPath, "*.whl"))
	if err != nil {
		return err
	}
	if len(matches) != 1 {
		return fmt.Errorf("expected a single .whl file to be in: %s", wheelPath)
	}

	filename := filepath.Base(matches[0])
	fi, err := ParseWheelFilename(filename)
	if err != nil {
		return err
	}

	if !fi.Compatible("cp37", "", runtime.GOOS, runtime.GOARCH) {
		return fmt.Errorf("built source distribution is incompatible")
	}

	// Cache resulting wheel
	whl := &Wheel{
		name:     fi.Name,
		filename: filename,
		version:  fi.Version,

		path: matches[0],
	}
	if err := whl.extractDependencies(ctx); err != nil {
		return fmt.Errorf("failed extracting dependencies from built wheel: %w", err)
	}
	cachedPath, err := cache.AddWheel(whl, matches[0])
	if err != nil {
		return err
	}
	whl.path = cachedPath

	s.wheel = whl
	return nil
}

// Install extracts the source distribution and invokes the Python interpreter to
// run a shim around setuptools to create a Python wheel package. If successful
// the wheel is then installed.
func (s *Sdist) Install(ctx context.Context) (string, error) {
	if s.wheel == nil {
		if err := s.convert(ctx); err != nil {
			return "", fmt.Errorf("converting sdist to wheel: %w", err)
		}
	}

	return s.wheel.Install(ctx)
}

func (s *Sdist) fetch(ctx context.Context) (io.ReadCloser, error) {
	r, err := http.NewRequestWithContext(ctx, http.MethodGet, s.url, nil)
	if err != nil {
		return nil, err
	}

	res, err := http.DefaultClient.Do(r)
	if err != nil {
		return nil, err
	}

	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed HTTP request: %s", res.Status)
	}

	// TODO: Verify checksum

	return res.Body, nil
}
