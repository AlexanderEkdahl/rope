package main

import (
	"archive/tar"
	"archive/zip"
	"bytes"
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
	"strings"

	"github.com/AlexanderEkdahl/rope/version"
)

// ParseSdistFilename returns a Sdist package from a given filename
func ParseSdistFilename(filename, suffix string) (*Sdist, error) {
	sep := strings.LastIndex(filename, "-")
	if sep < 0 {
		return nil, fmt.Errorf("expected sdist filename to be <name>-<version>%s, got: %s", suffix, filename)
	}
	versionString := strings.TrimSuffix(filename, suffix)[sep+1:]

	v, valid := version.Parse(versionString)
	if !valid {
		return nil, fmt.Errorf("invalid version: '%s'", versionString)
	}

	return &Sdist{
		name:     NormalizePackageName(filename[:sep]),
		version:  v,
		filename: filename,
		suffix:   suffix,
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
	suffix   string

	// url is only set when the package was found in a remote package repository.
	url string

	// Wheel built from source distribituion
	wheel *Wheel
}

// Name returns the canonical name of the source distribution package.
func (s *Sdist) Name() string { return s.name }

// Version returns the canonical version of the source distribution package.
func (s *Sdist) Version() version.Version { return s.version }

// Dependencies returns the transitive dependencies of this package.
func (s *Sdist) Dependencies() []Dependency {
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

	switch s.suffix {
	case ".tar.gz", ".tgz":
		if err := s.untar(body, tmp); err != nil {
			return err
		}
	case ".zip":
		if err := s.unzip(body, tmp); err != nil {
			return err
		}
	}

	root := filepath.Join(tmp, strings.TrimSuffix(s.filename, s.suffix))
	if _, err := os.Stat(root); errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("invalid source distribution: expected %s to exist after extraction", root)
	}

	wheelPath := filepath.Join(tmp, "wheel")
	installCmd := exec.CommandContext(
		ctx,
		"python",
		"-c",
		setuptoolsShim,
		"bdist_wheel",
		"-d",
		wheelPath,
	)
	installCmd.Dir = root
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
	whl, err := ParseWheelFilename(filename)
	if err != nil {
		return err
	}

	if !whl.Compatible(env) {
		return fmt.Errorf("built source distribution is incompatible with the current environment: '%s'", filename)
	}

	whl.Path = matches[0]
	if err := whl.extractDependencies(ctx); err != nil {
		return fmt.Errorf("failed extracting dependencies from built wheel: %w", err)
	}
	// Cache resulting wheel
	cachedPath, err := cache.AddWheel(whl, matches[0])
	if err != nil {
		return err
	}
	whl.Path = cachedPath

	s.wheel = whl
	return nil
}

func (s *Sdist) untar(body io.Reader, tmp string) error {
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

	return nil
}

func (s *Sdist) unzip(body io.Reader, tmp string) error {
	// Risks using too much memory by using a memory backed buffer as intermediate storage.
	var buf bytes.Buffer
	if _, err := io.Copy(&buf, body); err != nil {
		return err
	}

	r, err := zip.NewReader(bytes.NewReader(buf.Bytes()), int64(buf.Len()))
	if err != nil {
		return err
	}

	for _, file := range r.File {
		f, err := file.Open()
		if err != nil {
			return err
		}

		if file.FileInfo().IsDir() {
			continue
		}

		target := filepath.Join(tmp, file.Name)
		if err := os.MkdirAll(filepath.Dir(target), 0777); err != nil {
			return err
		}

		dst, err := os.Create(target)
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
