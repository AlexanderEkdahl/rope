package main

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/AlexanderEkdahl/rope/pep508"
	"github.com/AlexanderEkdahl/rope/version"
)

// TODO: Add update parameter for when to use the latest version of transitive
// dependency.
func add(timeout time.Duration, packages []string) error {
	ctx := context.Background()
	if timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, timeout)
		defer cancel()
	}

	project, err := ReadRopefile()
	if err != nil {
		return err
	}

	for _, p := range packages {
		d, err := pep508.ParseDependency(p)
		if err != nil {
			return err
		}

		if len(d.Versions) > 1 {
			return fmt.Errorf("expected at most a single version, got: %s", d.Versions)
		}
		// Should this be PEP508 conformant as well? Do whatever pip does
		split := strings.Split(p, "=")
		var v version.Version
		if len(split) > 1 {
			var err error
			v, err = version.Parse(split[1])
			if err != nil {
				return err
			}
		}

		project.Dependencies = append(project.Dependencies, Dependency{
			Name:    split[0],
			Version: v,
		})
	}

	index := &PyPI{
		url: "https://pypi.python.org/simple",
	}

	list, err := MinimalVersionSelection(ctx, project.Dependencies, index)
	if err != nil {
		return fmt.Errorf("failed version selection: %w", err)
	}

	for _, d := range list {
		p, err := index.FindPackage(ctx, d.Name, d.Version)
		if err != nil {
			return fmt.Errorf("failed to find package after version selection: %w", err)
		}

		// This function need to find the package AGAIN? doesn't make sense
		if err := p.Install(ctx); err != nil {
			return fmt.Errorf("installing '%s-%s': %w", d.Name, d.Version, err)
		}
	}

	project.Dependencies = list
	return WriteRopefile(project)
}
