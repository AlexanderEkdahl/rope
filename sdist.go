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
	"strings"

	"github.com/AlexanderEkdahl/rope/version"
)

func ParseSdistFilename(filename string) (*Sdist, error) {
	split := strings.Split(strings.TrimSuffix(filename, ".tar.gz"), "-")
	if len(split) != 2 {
		return nil, fmt.Errorf("expected sdist .tar.gz file to <name>-<version>.tar.gz, got: %s", filename)
	}

	v, err := version.Parse(split[1])
	if err != nil {
		return nil, fmt.Errorf("invalid semver: %v(%s)", split[1], err)
	}

	return &Sdist{
		name:     split[0],
		filename: filename,
		version:  v,
	}, nil
}

// Sdist is an abstraction over an "sdist" source distribution.
// This format is deprecated in favour of the Wheel format for
// distributing binary packages.
//
// Installing sdist packages requires invoking the Python
// interpreter.
type Sdist struct {
	url      string
	name     string
	filename string
	version  version.Version

	wheel *Wheel
}

// Name returns the canonical name of the source distribution package.
func (s *Sdist) Name() string { return s.name }

// Version returns the canonical version of the source distribution package.
func (s *Sdist) Version() version.Version { return s.version }

// Dependencies is supposed to return the dependencies of this package.
// This is not implemented yet as it requires invoking the Python interpreter.
func (s *Sdist) Dependencies() []Dependency {
	// TODO: Implement `python3 setup.py --requires`
	return nil
}

// Shim to wrap setup.py invocation with setuptools. This allows rope
// to install legacy packages. This is the same method as used by pip.
//
// https://github.com/pypa/pip/blob/9cbe8fbdd0a1bd1bd4e483c9c0a556e9910ef8bb/src/pip/_internal/utils/setuptools_build.py#L14-L20
const setuptoolsShim = `import sys, setuptools, tokenize; sys.argv[0] = 'setup.py'; __file__='setup.py';f=getattr(tokenize, 'open', open)(__file__);code=f.read().replace('\\r\\n', '\\n');f.close();exec(compile(code, __file__, 'exec'))`

// Install extracts the source distribution and invokes the Python interpreter to
// run a shim around setuptools to create a Python wheel package. If successful
// the wheel is then installed.
func (s *Sdist) Install(ctx context.Context) error {
	fmt.Printf("💩 installing: %s-%s %s\n", s.name, s.version, s.url)

	// TODO: Check if there is a compatible .whl matching the name and version in the
	// wheels directory already.

	file, err := s.fetch(ctx)
	if err != nil {
		return err
	}
	defer file.Close()

	tmp, err := ioutil.TempDir("", fmt.Sprintf("%s-%s-*", s.name, s.version))
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmp)

	gzipReader, err := gzip.NewReader(file)
	if err != nil {
		return err
	}
	defer gzipReader.Close()

	tarRoot := ""
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
			if err := os.Mkdir(filepath.Join(tmp, hdr.Name), 0755); err != nil {
				return err
			}

			if tarRoot == "" {
				tarRoot = filepath.Join(tmp, hdr.Name)
			}
		case tar.TypeReg:
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
		default:
			fmt.Println("unexpect tar type:", hdr.Typeflag)
		}
	}

	fmt.Println("untared to:", tmp)

	wheelPath := filepath.Join(tmp, "wheel")
	installCmd := exec.CommandContext(
		ctx,
		"python3",
		"-u",
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

	// TODO: Move wheel to the cache

	return ExtractWheel(matches[0])
}

func (s *Sdist) fetch(ctx context.Context) (*os.File, error) {
	path := fmt.Sprintf("./ropedir/cache/%s", s.filename)

	file, err := os.Open(path)
	if errors.Is(err, os.ErrNotExist) {
		// proceed to downloading file
	} else if err != nil {
		return nil, err
	} else if err == nil {
		fmt.Printf("Found cache for %s\n", s.filename)
		return file, nil
	}

	fmt.Printf("Downloading %s\n", s.filename)

	r, err := http.NewRequestWithContext(ctx, http.MethodGet, s.url, nil)
	if err != nil {
		return nil, err
	}

	res, err := http.DefaultClient.Do(r)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	// TODO: check 200-status

	file, err = os.Create(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	// TODO: Verify download
	_, err = io.Copy(file, res.Body)
	if err != nil {
		return nil, err
	}

	if err := file.Close(); err != nil {
		return nil, err
	}

	// TODO: Instead of closing the file do file.Seek(0)

	return os.Open(path)
}
