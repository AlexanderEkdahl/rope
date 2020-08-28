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
- `github.com/blang/semver/v4`: Library for parsing [semver](https://semver.org). Mostly compatible with the versioning patterns in Python. PEP 440 compatible version parsing is still and outstanding task.

## Links

- https://classic.yarnpkg.com/blog/2017/07/11/lets-dev-a-package-manager/
- https://github.com/pypa/wheel
- https://research.swtch.com/vgo-mvs

### Relevant PEPs
- https://www.python.org/dev/peps/pep-0427/
- https://www.python.org/dev/peps/pep-0425/
- https://www.python.org/dev/peps/pep-0440/

### TODO

- Parallelize version selection/installation process.
- Support for platform specific wheels across all platforms supported by Python
- Written package folders should be write protected to prevent inadvertendly changing files for other projects/users(match go modules)
- Demonstrate how rope can be used with Docker
- If the ropefile listed all architectures that all dependencies must resolve for it can ensure upfront that it is possible
- Top-level version exclusions: https://research.swtch.com/vgo-mvs
- Support extras e.g. `pip install urllib3[secure]`
- Support `pip -f` flag to build dependencies from other sources.
- Install missing packages when using commands other than `add`
- Support a mode where it will not write to the ropefile and fail any command that tries to do so.
- Somehow store the Python interpreter path for each project...(for sdist and platform discovery) or automatically try and find a compatible Python distribution.
- Investigate why it needs network when doing for example: `rope add tensorflow` twice in a row
- Top-level replace directive for developing local packages.
- If a package depends on an unspecified version it should not lead to the latest version being installed if another package depends on a specific minimal version(this can be implemented by updating the reduce method to not pick <latest> over a specific version *unless* the latest requirement comes from the top-level package)
- Support --no-binary packag installs
- Verify files have not been tampered with using the RECORD
- Russ Cox Algorithm R. Compute a Minimal Requirement List
- Windows support
- Integration tests against data pulled through public BigQuery dataset
- Integration tests against top-N python packages.
