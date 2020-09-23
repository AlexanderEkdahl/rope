package main

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"sync"
	"time"
)

var (
	ErrUnknownEnvironmentMarker = fmt.Errorf("invalid environment marker")
)

// Environment abstracts away interactions with the python environment.
// TODO: Environment should provide abstractions for all python interactions.
type Environment struct {
	env  map[string]string
	init sync.Once
	err  error
}

const environmentShim = `
import os
import sys
import platform

def format_full_version(info):
    version = '{0.major}.{0.minor}.{0.micro}'.format(info)
    kind = info.releaselevel
    if kind != 'final':
        version += kind[0] + str(info.serial)
    return version

print(os.name)
print(sys.platform)
print(platform.machine())
print(platform.python_implementation())
print(platform.release())
print(platform.system())
print(platform.version())
print('.'.join(platform.python_version_tuple()[:2]))
print(platform.python_version())
print(sys.implementation.name)

if hasattr(sys, 'implementation'):
    print(format_full_version(sys.implementation.version))
else:
    print("0")
`

func (e *Environment) Get(k string) (string, error) {
	if k == "extra" {
		return "", nil
	}

	e.init.Do(e.resolveEnvironment)
	if e.err != nil {
		return "", e.err
	}

	// TODO: Provide Go implementations for these to avoid invoking
	// the Python interpreter. `python --version` is also faster
	// than `python -c ""`.
	v, ok := e.env[k]
	if !ok {
		return "", ErrUnknownEnvironmentMarker
	}

	return v, nil
}

func (e *Environment) resolveEnvironment() {
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "python3", "-c", environmentShim)
	output, err := cmd.CombinedOutput()
	if cmd.ProcessState.ExitCode() != 0 {
		e.err = fmt.Errorf("resolving environment: python exit code: %d", cmd.ProcessState.ExitCode())
		return
	} else if err != nil {
		e.err = fmt.Errorf("resolving environment: %w", err)
		return
	}

	split := strings.Split(string(output), "\n")
	e.env = map[string]string{
		"os_name":                        split[0],
		"sys_platform":                   split[1],
		"platform_machine":               split[2],
		"platform_python_implementation": split[3],
		"platform_release":               split[4],
		"platform_system":                split[5],
		"platform_version":               split[6],
		"python_version":                 split[7],
		"python_full_version":            split[8],
		"implementation_name":            split[9],
		"implementation_version":         split[10],
	}
}

// EnvironmentExtra wraps an environment with a provided extra.
type EnvironmentExtra struct {
	extra string
	env   *Environment
}

func (e *EnvironmentExtra) Get(k string) (string, error) {
	if k == "extra" {
		return e.extra, nil
	}

	return e.env.Get(k)
}
