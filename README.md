# Rope

Experimental package manager for Python.

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
- https://github.com/pypa/packaging

### Relevant PEPs
- https://www.python.org/dev/peps/pep-0427/
- https://www.python.org/dev/peps/pep-0425/
- https://www.python.org/dev/peps/pep-0440/
- https://www.python.org/dev/peps/pep-0508/

### TODO

## Initial release

- Use another directory for installed packages
- Avoid uneccessarily installing sdist dependencies by prioritizing binary distributions
- Avoid uneccessarily installing dependencies that are not reachable
- Extract dependencies from source distributions
- Should only extract dependencies from transitive dependencies if absolutely neccessary(lazy extraction).
- Cache package requires_dist.
- Cache sdist downloads(why?)
- Support extracting sdist dependencies from PKG-INFO
- Support specifying Python version
- Demonstrate how rope can be used with Docker
- Support `pip -f` flag to build dependencies from other sources
- Somehow configure the Python interpreter path for each project...(for sdist and platform discovery) or automatically try and find a compatible Python distribution.
- Support PEP517 builds for projects that support it.
- [Bug] Install dependencies in reverse order since `setup.py` may import transitive dependencies(and expose transitive dependencies on the PYTHONPATH) (`rope add nni`)
- Add support for `~=` in dependency evaluation.
- Instead of extracting a single `Minimal` from a list of requirements, use the full list to match possible candidates. Then use the minimal version found. In the event of unbounded requirements(i.e. `!= 1.2`) use the latest version and mark the dependency as unbounded. This may cause issues as multiple dependencies may specify as specific dependency with conflicting requirements.
- [Investigate] `pandas: pytz (>=2011k)(invalid version '2011k')` Maybe 2011k should not be considered invalid? Legacy version?
- Rename version constructs according to https://packaging.pypa.io/en/latest/
- License
- Implement version selection exception in which pre-releases are only included if no other version matches.
- [Bug] Using `python:3.4` and running `rope add tensorflow` results in `compatible package not found`.
- [Bug] `requires_dist` is not populated for packages downloaded from the legacy index due to dependencies being extracted *after* caching.

## Later

- Verify checksum for PyPI
- GitHub Actions release process
- Ensure good interoperability with https://github.com/pyenv/pyenv
- Support upgrading specific dependencies
- Support extras e.g. `pip install urllib3[secure]`
- Parallelize version selection/installation process.
- Written package folders should be write protected to prevent inadvertendly changing files for other projects/users(match go modules)
- Top-level version exclusions: https://research.swtch.com/vgo-mvs
- Support a mode where it will not write to the ropefile and fail any command that tries to do so.
- Top-level replace directive for developing local packages.
- Lock cache during interaction(except for when cache is disabled).
- Verify files have not been tampered with using the RECORD
- Windows support
- Warn users about explicit incompatabilities(`rope show`)
- Support --no-binary package installs
- An alternative approach to creating an entry in PYTHONPATH for every dependency is to create an ephemeral directory with symlinks to every dependency.
- Connection retries.
