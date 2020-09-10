# Rope

Experimental package manager for Python.

- Parallel downloads and package installation
- Optimized for Docker
- Fast version selection algorithm

## Usage

``` bash
rope init      # Initialize a new project
rope add torch # Download and add the latest version of 'torch'

rope run python train.py
# or
export PYTHONPATH=`rope pythonpath`; python script.py

rope requirements > requirements.txt
```

## Minimal version selection

Unlike pip/conda/pipenv/poetry `rope` uses a different algorithm to select the version of dependencies named Minimal Version Selection first introduced by Russ Cox for Go. The algorithm recursively visits every dependency's dependencies and builds a list of the minimal version required by each dependency. This list is then reduced to remove duplicate dependencies by only keeping the greatest version of each entry. This algorithm is guaranteed to run in polynomial time allowing for fast builds.

## Internal dependencies

The following dependencies are statically included in the resulting binary and does not have to be installed by an end-user.

- `github.com/spf13/pflag`: Drop-in replacement of the built-in flag package implementing POSIX/GNU-style --flags familiar to Python users.

## Development setup

### Building from source

`rope` is written in Go as it easy to write performant CLI applications with very low resource requirements. Building `rope` from source therefore requires installing [Go 1.15](https://golang.org/dl/).

Assuming Go is correctly installed and configured:

``` sh
go install
rope version
```

### Running integration tests

The integration tests for rope runs the binary in a Python docker image verifying the installed dependencies work correctly.

```
docker build -t rope_integration -f Dockerfile-integration . && docker run --rm rope_integration -test.v
```

Debug rope interactively in a consistent environment:

```
docker build -t rope_integration -f Dockerfile-integration . && docker run --rm -it --entrypoint /bin/bash rope_integration
```

## Links

- https://classic.yarnpkg.com/blog/2017/07/11/lets-dev-a-package-manager/
- https://github.com/pypa/wheel
- https://research.swtch.com/vgo-mvs

### Relevant PEPs
- https://www.python.org/dev/peps/pep-0427/
- https://www.python.org/dev/peps/pep-0425/
- https://www.python.org/dev/peps/pep-0440/

### TODO

- Return the most compatible version if multiple version are available.
    - `rope add numpy` downloads 2 versions of numpy(due to unstable package selection)
    - `rope add six` downloads a sdist and a wheel(due to unstable package selection)
- Parallelize version selection/installation process.
- Support for platform specific wheels across all platforms supported by Python
- Written package folders should be write protected to prevent inadvertendly changing files for other projects/users(match go modules)
- Demonstrate how rope can be used with Docker
- Top-level version exclusions: https://research.swtch.com/vgo-mvs
- Support extras e.g. `pip install urllib3[secure]`
- Support `pip -f` flag to build dependencies from other sources.
- Support a mode where it will not write to the ropefile and fail any command that tries to do so.
- Somehow store the Python interpreter path for each project...(for sdist and platform discovery) or automatically try and find a compatible Python distribution.
- Support specified Python version
- Top-level replace directive for developing local packages.
- Verify files have not been tampered with using the RECORD
- Windows support
- Use https://warehouse.pypa.io/api-reference/json/ for faster(?) dependency search with additional metadata.
- GitHub Actions release process
- Ensure all packages found at https://johnfraney.ca/posts/2019/11/19/pipenv-poetry-benchmarks-ergonomics-2/ can be installed.
- If the ropefile listed all architectures that all dependencies must resolve for it can ensure upfront that it is possible
- Warn users about explicit incompatabilities(`rope show`)
- Support --no-binary package installs
- `rope add pytz>=2017.2` installs the latest version of `pytz` not `2017.2`
- `rope export` is taking longer than it should. Is it doing network calls?
- Use a memory backed buffer for downloads(up to a point). Use BigQuery to find suitable limit
- An alternative approach to creating an entry in PYTHONPATH for every dependency is to create an ephemeral directory with symlinks to every dependency.
- Support PEP517 builds for projects that support it.
- Attempt to extract sdist dependencies from PKG-INFO
- Cache sdist downloads
- Use BigQuery to download a list of packages that are popular and don't have transitive dependencies(i.e. numpy) since checking PKG-INFO can not eliminate the possibility of there being transitive dependencies.
